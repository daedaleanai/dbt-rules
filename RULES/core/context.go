package core

import (
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

var env = map[string]interface{}{}

const scriptFileMode = 0755

type Context interface {
	AddBuildStep(BuildStep)

	Cwd() OutPath
	OutPath(Path) OutPath

	GetBuildOption(*BuildParam) interface{}
	SetBuildOption(*BuildParam, interface{})
	GetFlag(string) string

	Build(target buildInterface) BuildOutput

	addTargetDependency(interface{})
}

type BuildOutput interface {
	Output() OutPath
	Outputs() []OutPath
}

// BuildStep represents one build step (i.e., one build command).
// Each BuildStep produces `Out` and `Outs` from `Ins` and `In` by running `Cmd`.
type BuildStep struct {
	Out     OutPath
	Outs    []OutPath
	In      Path
	Ins     []Path
	Depfile OutPath
	Cmd     string
	Script  string
	Descr   string
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
	Build(ctx Context) BuildOutput
}

type descriptionInterface interface {
	Description() string
}

type context struct {
	cwd                OutPath
	targetDependencies []string

	targetNames map[interface{}]string
	ninjaFile   strings.Builder
	bashFile    strings.Builder
	nextRuleID  int

	buildOptions map[*BuildParam]interface{}
	flags        map[string]string
	hash         string

	buildCache   map[string]BuildOutput
	buildOutputs map[string]BuildStep
}

func newContext(vars map[string]interface{}) *context {
	ctx := &context{
		outPath{"", ""},
		[]string{},

		map[interface{}]string{},
		strings.Builder{},
		strings.Builder{},
		0,

		map[*BuildParam]interface{}{},
		map[string]string{},
		"",

		map[string]BuildOutput{},
		map[string]BuildStep{},
	}

	for _, buildParam := range buildParams {
		optionName := flags[buildParam.Name].Value
		ctx.buildOptions[buildParam] = buildParam.Options[optionName]
	}

	for name := range vars {
		ctx.targetNames[vars[name]] = name
	}

	fmt.Fprintf(&ctx.ninjaFile, "build __phony__: phony\n\n")

	return ctx
}

func (ctx *context) Build(target buildInterface) BuildOutput {
	flagStrings := []string{}
	for name, value := range ctx.flags {
		flagStrings = append(flagStrings, fmt.Sprintf("flag:%s$%s", name, value))
	}
	for param, option := range ctx.buildOptions {
		flagStrings = append(flagStrings, fmt.Sprintf("buildOption:%s$%s", param.Name, option.(buildOptionName).Name()))
	}
	sort.Strings(flagStrings)
	ctx.hash = fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(strings.Join(flagStrings, "#"))))
	targetHash := ctx.hash + fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(fmt.Sprintf("%v", target))))

	if _, exists := ctx.buildCache[targetHash]; !exists {
		if name, exists := ctx.targetNames[target]; exists {
			ctx.cwd = outPath{ctx.hash, path.Dir(name) + "/"}
		}
		ctx.cwd = outPath{ctx.hash, ctx.cwd.relative()}

		buildOptions := map[*BuildParam]interface{}{}
		for param, value := range ctx.buildOptions {
			buildOptions[param] = value
		}
		ctx.buildCache[targetHash] = target.Build(ctx)
		ctx.buildOptions = buildOptions
	}

	return ctx.buildCache[targetHash]
}

// AddBuildStep adds a build step for the current target.
func (ctx *context) AddBuildStep(step BuildStep) {
	outs := []string{}
	for _, out := range step.outs() {
		ctx.buildOutputs[out.Absolute()] = step
		outs = append(outs, ninjaEscape(out.Absolute()))
	}
	if len(outs) == 0 {
		return
	}

	ins := []string{}
	for _, in := range step.ins() {
		ins = append(ins, ninjaEscape(in.Absolute()))
	}

	if step.Script != "" {
		if step.Cmd != "" {
			Fatal("cannot specify both Cmd and Script in a build step")
		}

		script := []byte(step.Script)
		hash := crc32.ChecksumIEEE([]byte(script))
		scriptFileName := fmt.Sprintf("%08X.sh", hash)
		scriptFilePath := path.Join(buildDir(), "..", scriptFileName)
		err := ioutil.WriteFile(scriptFilePath, script, scriptFileMode)
		if err != nil {
			Fatal("%s", err)
		}
		step.Cmd = scriptFilePath
	}

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

func (ctx *context) OutPath(p Path) OutPath {
	return outPath{ctx.hash, p.relative()}
}

func (ctx *context) GetBuildOption(param *BuildParam) interface{} {
	return ctx.buildOptions[param]
}

func (ctx *context) SetBuildOption(param *BuildParam, option interface{}) {
	if reflect.TypeOf(option) != param.Type && !reflect.TypeOf(option).Implements(param.Type) {
		Fatal("option for build param '%s' has incorrect type", param.Name)
	}
	ctx.buildOptions[param] = option
}

func (ctx *context) GetFlag(name string) string {
	return ""
}

func (ctx *context) handleTarget(name string, target buildInterface) {
	currentTarget = name
	ctx.targetDependencies = []string{}

	outs := ctx.Build(target).Outputs()
	ruleOuts := []string{}
	for _, out := range outs {
		ruleOuts = append(ruleOuts, ninjaEscape(out.Absolute()))
	}
	sort.Strings(ruleOuts)

	printOuts := []string{}
	for _, out := range outs {
		relPath, _ := filepath.Rel(workingDir(), out.Absolute())
		printOuts = append(printOuts, relPath)
	}
	sort.Strings(printOuts)

	if len(printOuts) == 0 {
		printOuts = []string{"<no outputs produced>"}
	}

	fmt.Fprintf(&ctx.ninjaFile, "rule r%d\n", ctx.nextRuleID)
	fmt.Fprintf(&ctx.ninjaFile, "  command = echo \"%s\"\n", strings.Join(printOuts, "\\n"))
	fmt.Fprintf(&ctx.ninjaFile, "  description = Created %s:", name)
	fmt.Fprintf(&ctx.ninjaFile, "\n")
	fmt.Fprintf(&ctx.ninjaFile, "build %s: r%d %s %s __phony__\n", name, ctx.nextRuleID, strings.Join(ruleOuts, " "), strings.Join(ctx.targetDependencies, " "))
	fmt.Fprintf(&ctx.ninjaFile, "\n")
	fmt.Fprintf(&ctx.ninjaFile, "\n")

	ctx.nextRuleID++
}

func (ctx *context) finish() {
	currentTarget = ""

	// Generate the bash script
	fmt.Fprintf(&ctx.bashFile, "#!/bin/bash\n\nset -e\n\n")
	for out := range ctx.buildOutputs {
		ctx.addOutputToBashScript(out)
	}
}

func (ctx *context) addOutputToBashScript(output string) {
	step, exists := ctx.buildOutputs[output]
	if !exists {
		return
	}

	for _, in := range step.ins() {
		ctx.addOutputToBashScript(in.Absolute())
	}

	for _, out := range step.outs() {
		delete(ctx.buildOutputs, out.Absolute())
		fmt.Fprintf(&ctx.bashFile, "mkdir -p %q\n", path.Dir(out.Absolute()))
	}

	fmt.Fprintf(&ctx.bashFile, "echo %q\n", step.Cmd)
	if step.Script != "" {
		fmt.Fprintf(&ctx.bashFile, "%s\n", step.Script)
	} else {
		fmt.Fprintf(&ctx.bashFile, "%s\n", step.Cmd)
	}
}

func (ctx *context) addTargetDependency(target interface{}) {
	name, exists := ctx.targetNames[target]
	if !exists {
		Fatal("adding target dependency to invalid target")
	}
	ctx.targetDependencies = append(ctx.targetDependencies, name)
}

func ninjaEscape(s string) string {
	return strings.ReplaceAll(s, " ", "$ ")
}
