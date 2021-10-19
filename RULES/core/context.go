package core

import (
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

type Context interface {
	AddBuildStep(BuildStep)
	Cwd() OutPath

	// Built reports whether this is the first time Built has been
	// called with the given id.
	// It can be used to build a target at most once:
	//   if ctx.Built(out) { return }
	//   ... actually build out
	Built(id string) bool

	// WithTrace calls the given function, with the given value added
	// to the trace.
	WithTrace(id string, f func(Context))

	// Trace returns the strings in the current trace (most recent last).
	Trace() []string

	addTargetDependency(interface{})
}

// BuildStep represents one build step (i.e., one build command).
// Each BuildStep produces `Out` and `Outs` from `Ins` and `In` by running `Cmd`.
type BuildStep struct {
	Out          OutPath
	Outs         []OutPath
	In           Path
	Ins          []Path
	Depfile      OutPath
	Cmd          string
	Script       string
	Data         string
	DataFileMode os.FileMode
	Descr        string
}

func (step *BuildStep) outs() []OutPath {
	if step.Out == nil {
		return step.Outs
	}
	return append(step.Outs, step.Out)
}

func (step *BuildStep) ins() []Path {
	if step.In == nil {
		return step.Ins
	}
	return append(step.Ins, step.In)
}

type buildInterface interface {
	Build(ctx Context)
}

type outputsInterface interface {
	Outputs() []Path
}

type descriptionInterface interface {
	Description() string
}

type runInterface interface {
	Run(args []string) string
}

type context struct {
	cwd                OutPath
	targetDependencies []string
	leafOutputs        map[Path]bool

	targetNames  map[interface{}]string
	buildOutputs map[string]BuildStep
	ninjaFile    strings.Builder
	bashFile     strings.Builder
	nextRuleID   int

	trace    []string
	seenOnce map[string]bool
}

func newContext(vars map[string]interface{}) *context {
	ctx := &context{
		cwd:         outPath{""},
		leafOutputs: map[Path]bool{},

		targetNames:  map[interface{}]string{},
		buildOutputs: map[string]BuildStep{},
		ninjaFile:    strings.Builder{},
		bashFile:     strings.Builder{},
		seenOnce:     map[string]bool{},
	}

	for name := range vars {
		ctx.targetNames[vars[name]] = name
	}

	fmt.Fprintf(&ctx.ninjaFile, "build __phony__: phony\n\n")

	return ctx
}

func (ctx *context) Built(id string) bool {
	if ctx.seenOnce[id] {
		return true
	}
	ctx.seenOnce[id] = true
	return false
}

func (ctx *context) WithTrace(id string, f func(Context)) {
	ctx.trace = append(ctx.trace, id)
	defer func() {
		ctx.trace = ctx.trace[:len(ctx.trace)-1]
	}()
	f(ctx)
}

func (ctx *context) Trace() []string {
	// We return a copy of the trace, to avoid mutations.
	return append([]string{}, ctx.trace...)
}

// AddBuildStep adds a build step for the current target.
func (ctx *context) AddBuildStep(step BuildStep) {
	outs := []string{}
	for _, out := range step.outs() {
		ctx.buildOutputs[out.Absolute()] = step
		outs = append(outs, ninjaEscape(out.Absolute()))
		ctx.leafOutputs[out] = true
	}

	if len(outs) == 0 {
		return
	}

	ins := []string{}
	for _, in := range step.ins() {
		ins = append(ins, ninjaEscape(in.Absolute()))
		delete(ctx.leafOutputs, in)
	}

	data := ""
	dataFileMode := os.FileMode(0644)
	dataFilePath := ""

	if step.Script != "" {
		if step.Cmd != "" {
			Fatal("cannot specify both Cmd and Script in a build step")
		}
		data = step.Script
		dataFileMode = 0755
	} else if step.Data != "" {
		if step.Cmd != "" {
			Fatal("cannot specify both Cmd and Data in a build step")
		}
		if step.Out == nil || step.Outs != nil {
			Fatal("a single Out is required for Data in a build step")
		}
		data = step.Data
		if step.DataFileMode != 0 {
			dataFileMode = step.DataFileMode
		}
	}

	if data != "" {
		buffer := []byte(data)
		hash := crc32.ChecksumIEEE([]byte(buffer))
		dataFileName := fmt.Sprintf("%08X", hash)
		dataFilePath = path.Join(filepath.Dir(buildDir()), "DATA", dataFileName)
		if err := os.MkdirAll(filepath.Dir(dataFilePath), os.ModePerm); err != nil {
			Fatal("Failed to create directory for data files: %s", err)
		}
		if err := ioutil.WriteFile(dataFilePath, buffer, dataFileMode); err != nil {
			Fatal("Failed to write data file: %s", err)
		}
	}

	if step.Script != "" {
		step.Cmd = dataFilePath
	} else if step.Data != "" {
		step.Cmd = fmt.Sprintf("cp %q %q", dataFilePath, step.Out)
	}

	fmt.Fprintf(&ctx.ninjaFile, "# trace: %s\n", strings.Join(ctx.Trace(), " // "))
	fmt.Fprintf(&ctx.ninjaFile, "rule r%d\n", ctx.nextRuleID)
	if step.Depfile != nil {
		depfile := ninjaEscape(step.Depfile.Absolute())
		fmt.Fprintf(&ctx.ninjaFile, "  depfile = %s\n", depfile)
	}
	fmt.Fprintf(&ctx.ninjaFile, "  command = %s\n", step.Cmd)
	if step.Descr != "" {
		fmt.Fprintf(&ctx.ninjaFile, "  description = %s\n", step.Descr)
	}
	fmt.Fprint(&ctx.ninjaFile, "\n")
	fmt.Fprintf(&ctx.ninjaFile, "build %s: r%d %s\n", strings.Join(outs, " "), ctx.nextRuleID, strings.Join(ins, " "))
	fmt.Fprint(&ctx.ninjaFile, "\n\n")

	ctx.nextRuleID++
}

// Cwd returns the build directory of the current target.
func (ctx *context) Cwd() OutPath {
	return ctx.cwd
}

func (ctx *context) handleTarget(targetPath string, target buildInterface) {
	currentTarget = targetPath
	ctx.cwd = outPath{path.Dir(targetPath)}
	ctx.leafOutputs = map[Path]bool{}
	ctx.targetDependencies = []string{}

	ctx.WithTrace("top:"+targetPath, target.Build)

	if !unicode.IsUpper([]rune(path.Base(targetPath))[0]) {
		return
	}

	ninjaOuts := []string{}
	for out := range ctx.leafOutputs {
		ninjaOuts = append(ninjaOuts, ninjaEscape(out.Absolute()))
	}
	sort.Strings(ninjaOuts)

	printOuts := []string{}
	if iface, ok := target.(outputsInterface); ok {
		for _, out := range iface.Outputs() {
			relPath, _ := filepath.Rel(input.WorkingDir, out.Absolute())
			printOuts = append(printOuts, relPath)
		}
	} else {
		for out := range ctx.leafOutputs {
			relPath, _ := filepath.Rel(input.WorkingDir, out.Absolute())
			printOuts = append(printOuts, relPath)
		}
	}
	sort.Strings(printOuts)

	if len(printOuts) == 0 {
		printOuts = []string{"<no outputs produced>"}
	}

	fmt.Fprintf(&ctx.ninjaFile, "rule r%d\n", ctx.nextRuleID)
	fmt.Fprintf(&ctx.ninjaFile, "  command = echo \"%s\"\n", strings.Join(printOuts, "\\n"))
	fmt.Fprintf(&ctx.ninjaFile, "  description = Created %s:", targetPath)
	fmt.Fprintf(&ctx.ninjaFile, "\n")
	fmt.Fprintf(&ctx.ninjaFile, "build %s: r%d %s %s __phony__\n", targetPath, ctx.nextRuleID, strings.Join(ninjaOuts, " "), strings.Join(ctx.targetDependencies, " "))
	fmt.Fprintf(&ctx.ninjaFile, "\n")
	fmt.Fprintf(&ctx.ninjaFile, "\n")
	ctx.nextRuleID++

	if runIface, ok := target.(runInterface); ok {
		runCmd := runIface.Run(input.RunArgs)
		fmt.Fprintf(&ctx.ninjaFile, "rule r%d\n", ctx.nextRuleID)
		fmt.Fprintf(&ctx.ninjaFile, "  command = %s\n", runCmd)
		fmt.Fprintf(&ctx.ninjaFile, "  description = Running %s:\n", targetPath)
		fmt.Fprintf(&ctx.ninjaFile, "  pool = console\n")
		fmt.Fprintf(&ctx.ninjaFile, "\n")
		fmt.Fprintf(&ctx.ninjaFile, "build %s#run: r%d %s __phony__\n", targetPath, ctx.nextRuleID, targetPath)
		fmt.Fprintf(&ctx.ninjaFile, "\n")
		fmt.Fprintf(&ctx.ninjaFile, "\n")
		ctx.nextRuleID++
	}
}

func (ctx *context) addTargetDependency(target interface{}) {
	if reflect.TypeOf(target).Kind() != reflect.Ptr {
		Fatal("adding target dependency to non-pointer target")
	}
	name, exists := ctx.targetNames[target]
	if !exists {
		Fatal("adding target dependency to invalid target")
	}
	ctx.targetDependencies = append(ctx.targetDependencies, name)
}

func ninjaEscape(s string) string {
	return strings.ReplaceAll(s, " ", "$ ")
}
