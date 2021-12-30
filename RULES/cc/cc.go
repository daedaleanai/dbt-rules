package cc

import (
	"fmt"
	"path/filepath"
	"strings"

	"dbt-rules/RULES/core"
)

// objectFile compiles a single C++ source file.
type objectFile struct {
	Out       core.OutPath
	Src       core.Path
	Includes  []core.Path
	CFlags     []string
	CxxFlags     []string
	AsFlags     []string
	Toolchain Toolchain
}

func ninjaEscape(s string) string {
	return strings.ReplaceAll(s, " ", "$ ")
}

func (obj objectFile) cxxRule() core.BuildRule {
	toolchain := toolchainOrDefault(obj.Toolchain)
	return core.BuildRule {
		Name: toolchain.Name() + "-cxx",
		Variables: map[string]string{
			"depfile": "$out.d",
			"command": fmt.Sprintf("%s %s $flags -pipe -c -MD -MF $out.d -o $out $in", ninjaEscape(toolchain.CxxCompiler()), strings.Join(toolchain.CxxFlags(), " ")),
			"description": fmt.Sprintf("CXX (toolchain: %s) $out", toolchain.Name()),
		},
	}
}

func (obj objectFile) ccRule() core.BuildRule {
	toolchain := toolchainOrDefault(obj.Toolchain)
	return core.BuildRule {
		Name: toolchain.Name() + "-cc",
		Variables: map[string]string{
			"depfile": "$out.d",
			"command": fmt.Sprintf("%s %s $flags -pipe -c -MD -MF $out.d -o $out $in", ninjaEscape(toolchain.CCompiler()), strings.Join(toolchain.CFlags(), " ")),
			"description": fmt.Sprintf("CC (toolchain: %s) $out", toolchain.Name()),
		},
	}
}

func (obj objectFile) asRule() core.BuildRule {
	toolchain := toolchainOrDefault(obj.Toolchain)
	return core.BuildRule {
		Name: toolchain.Name() + "-as",
		Variables: map[string]string{
			"command": fmt.Sprintf("%s %s $flags -c -o $out $in", ninjaEscape(toolchain.Assembler()), strings.Join(toolchain.AsFlags(), " ")),
			"description": fmt.Sprintf("AS (toolchain: %s) $out", toolchain.Name()),
		},
	}
}

// Build an objectFile.
func (obj objectFile) Build(ctx core.Context) {
	rule := core.BuildRule{}

	flags := []string{}

	switch filepath.Ext(obj.Src.Absolute()) {
	case ".cc":
		rule = obj.cxxRule()
		flags = obj.CxxFlags
	case ".c":
		rule = obj.ccRule()
		flags = obj.CFlags
	case ".S":
		rule = obj.asRule()
		flags = obj.AsFlags
	default:
		core.Fatal("Unknown source extension for cc toolchain '" + filepath.Ext(obj.Src.Absolute()) + "'")
	}

	for _, include := range obj.Includes {
		flags = append(flags, fmt.Sprintf("-I%q", include))
	}

	ctx.WithTrace("obj:"+obj.Out.Relative(), func(ctx core.Context) {
		ctx.AddBuildStepWithRule(core.BuildStepWithRule{
			Outs:     []core.OutPath{obj.Out},
			Ins:      []core.Path{obj.Src},
			Rule:	  rule,
			Variables: map[string]string {
				"flags": strings.Join(flags, " "),
			},
		})
	})
}

// BlobObject creates a relocatable object file from any blob of data.
type BlobObject struct {
	In        core.Path
	Toolchain Toolchain
}

// Build a BlobObject.
func (blob BlobObject) Build(ctx core.Context) {
	ctx.WithTrace("blob:"+blob.out().Relative(), func(ctx core.Context) {
		toolchain := toolchainOrDefault(blob.Toolchain)
		ctx.AddBuildStep(core.BuildStep{
			Out:   blob.out(),
			In:    blob.In,
			Cmd:   fmt.Sprintf("%s %s -r -b binary -o %q %q", blob.Toolchain.Link(), strings.Join(blob.Toolchain.LdFlags(), " "), blob.out(), blob.In),
			Descr: fmt.Sprintf("BLOB (toolchain: %s) %s", toolchain.Name(), blob.out().Relative()),
		})
	})
}

func (blob BlobObject) out() core.OutPath {
	toolchain := toolchainOrDefault(blob.Toolchain)
	return blob.In.WithPrefix(toolchain.Name() + "/").WithExt("blob.o")
}

func collectDepsWithToolchainRec(toolchain Toolchain, dep Dep, visited map[string]int, stack *[]Library) {
	lib := dep.CcLibrary(toolchain)

	libPath := lib.Out.Absolute()

	if visited[libPath] == 2 {
		return
	}

	if visited[libPath] == 1 {
		core.Fatal("dependency loop detected")
	}

	visited[libPath] = 1

	for _, ldep := range lib.Deps {
		collectDepsWithToolchainRec(toolchain, ldep, visited, stack)
	}

	*stack = append([]Library{lib}, *stack...)
	visited[libPath] = 2
}

func collectDepsWithToolchain(toolchain Toolchain, deps []Dep) []Library {
	stack := []Library{}
	marks := map[string]int{}
	for _, dep := range deps {
		collectDepsWithToolchainRec(toolchain, dep, marks, &stack)
	}
	return stack
}

func compileSources(out core.OutPath, ctx core.Context, srcs []core.Path, cFlags []string, cxxFlags []string, asFlags []string, deps []Library, toolchain Toolchain) []core.Path {
	includes := []core.Path{core.SourcePath("")}
	for _, dep := range deps {
		includes = append(includes, dep.Includes...)
	}

	objs := []core.Path{}

	for _, src := range srcs {
		obj := objectFile{
			Out:       out.WithSuffix("_o/" + src.Relative()).WithExt("o"),
			Src:       src,
			Includes:  includes,
			CFlags:     cFlags,
			CxxFlags:     cxxFlags,
			AsFlags:     asFlags,
			Toolchain: toolchain,
		}
		obj.Build(ctx)
		objs = append(objs, obj.Out)
	}

	return objs
}

// Dep is an interface implemented by dependencies that can be linked into a library.
type Dep interface {
	CcLibrary(toolchain Toolchain) Library
}

// Library builds and links a static C++ library.
// The same library can be build with multiple toolchains. Each Toolchain might
// emit different outputs, therefore DBT needs to create unique locations for
// these outputs. The user-specified Out path is used either for user-specified
// Toolchain or for the DefaultToolchain in case user didn't specify a Toolchain.
// In all other cases, user-specified Out path is directory-prefixed with the Toolchain name.
type Library struct {
	Out           core.OutPath
	Srcs          []core.Path
	Blobs         []core.Path
	Objs          []core.Path
	Includes      []core.Path
	CFlags		  []string
	CxxFlags      []string
	AsFlags       []string
	Deps          []Dep
	Shared        bool
	AlwaysLink    bool
	Toolchain     Toolchain

	// Extra fields for handling multi-toolchain logic.
	userOut       core.OutPath
	userToolchain Toolchain
}

func (lib Library) arRule() core.BuildRule {
	toolchain := toolchainOrDefault(lib.Toolchain)
	// ar updates an existing archive. This can cause faulty builds in the case
	// where a symbol is defined in one file, that file is removed, and the
	// symbol is subsequently defined in a new file. That's because the old object file
	// can persist in the archive. See https://github.com/daedaleanai/dbt/issues/91
	// There is no option to ar to always force creation of a new archive; the "c"
	// modifier simply suppresses a warning if the archive doesn't already
	// exist. So instead we delete the target (out) if it already exists.
	return core.BuildRule {
		Name: toolchain.Name() + "-ar",
		Variables: map[string]string{
			"command": fmt.Sprintf("rm -f $out 2> /dev/null; %s rcs $out $in", ninjaEscape(toolchain.Archiver())),
			"description": fmt.Sprintf("AR (toolchain: %s) $out", toolchain.Name()),
		},
	}
}

func (lib Library) soRule() core.BuildRule {
	toolchain := toolchainOrDefault(lib.Toolchain)
	return core.BuildRule {
		Name: toolchain.Name() + "-so",
		Variables: map[string]string{
			"command": fmt.Sprintf("%s -shared %s -o $out $in", ninjaEscape(toolchain.Link()), strings.Join(toolchain.LdFlags(), " ")),
			"description": fmt.Sprintf("LD (toolchain: %s) $out", toolchain.Name()),
		},
	}
}

// Build a Library.
func (lib Library) build(ctx core.Context) {
	if lib.Out == nil {
		core.Fatal("Out field is required for cc.Library")
	}

	if ctx.Built(lib.Out.Absolute()) {
		return
	}

	toolchain := toolchainOrDefault(lib.Toolchain)

	deps := collectDepsWithToolchain(toolchain, append(toolchain.StdDeps(), lib))
	for _, d := range deps {
		d.Build(ctx)
	}

	objs := compileSources(lib.Out, ctx, lib.Srcs, lib.CFlags, lib.CxxFlags, lib.AsFlags, deps, toolchain)
	objs = append(objs, lib.Objs...)

	for _, blob := range lib.Blobs {
		blobObject := BlobObject{In: blob, Toolchain: toolchain}
		blobObject.Build(ctx)
		objs = append(objs, blobObject.out())
	}

	rule := core.BuildRule{}

	if lib.Shared {
		rule = lib.soRule()
	} else {
		rule = lib.arRule()
	}

	ctx.AddBuildStepWithRule(core.BuildStepWithRule{
		Outs:  []core.OutPath{lib.Out},
		Ins:   objs,
		Rule: rule,
	})
}

func (lib Library) Build(ctx core.Context) {
	ctx.WithTrace("lib:"+lib.Out.Relative(), lib.build)
}

// CcLibrary for Library returns the library itself, or a toolchain-specific variant
func (lib Library) CcLibrary(toolchain Toolchain) Library {
	if toolchain == nil {
		core.Fatal("CcLibrary() called with nil toolchain.")
	}

	if lib.Out == nil {
		core.Fatal("Out field is required for cc.Library")
	}

	// Ensure userOut and userToolchain are set.
	if lib.userOut == nil {
		lib.userOut = lib.Out
	}
	if lib.userToolchain == nil {
		if lib.Toolchain != nil {
			lib.userToolchain = lib.Toolchain
		} else {
			lib.userToolchain = DefaultToolchain()
		}
	}

	if toolchain.Name() == lib.userToolchain.Name() {
		lib.Out = lib.userOut
		return lib
	}

	lib.Out = lib.userOut.WithPrefix(toolchain.Name() + "/")
	lib.Toolchain = toolchain
	return lib
}

// Binary builds and links an executable.
type Binary struct {
	Out           core.OutPath
	Srcs          []core.Path
	CFlags        []string
	CxxFlags      []string
    AsFlags       []string
	LinkerFlags   []string
	Deps          []Dep
	DepsPre       []Dep
	DepsPost      []Dep
	Script        core.Path
	Toolchain     Toolchain
}

// Build a Binary.
func (bin Binary) Build(ctx core.Context) {
	if bin.Out == nil {
		core.Fatal("Out field is required for cc.Binary")
	}
	ctx.WithTrace("bin:"+bin.Out.Relative(), bin.build)
}

func (bin Binary) ldRule() core.BuildRule {
	toolchain := toolchainOrDefault(bin.Toolchain)
	return core.BuildRule {
		Name: toolchain.Name() + "-ld",
		Variables: map[string]string{
			"command": fmt.Sprintf("%s %s $flags -o $out $objs $libs", ninjaEscape(toolchain.Link()), strings.Join(toolchain.LdFlags(), " ")),
			"description": fmt.Sprintf("LD (toolchain: %s) $out", toolchain.Name()),
		},
	}
}

func (bin Binary) build(ctx core.Context) {
	toolchain := toolchainOrDefault(bin.Toolchain)

	deps := collectDepsWithToolchain(toolchain, append(bin.Deps, toolchain.StdDeps()...))
	for _, d := range deps {
		d.Build(ctx)
	}
	objs := compileSources(bin.Out, ctx, bin.Srcs, bin.CFlags, bin.CxxFlags, bin.AsFlags, deps, toolchain)

	objsToLink := []string{}

	for _, obj := range objs {
		objsToLink = append(objsToLink, fmt.Sprintf("%q", obj))
	}

	ins := objs
	
	libsPre := []Library{}
	for _, dep := range bin.DepsPre {
		lib := dep.CcLibrary(toolchain)
		ins = append(ins, lib.Out)
		libsPre = append(libsPre, lib)
	}

	deps = append(libsPre, deps...)

	for _, dep := range bin.DepsPost {
		lib := dep.CcLibrary(toolchain)
		ins = append(ins, lib.Out)
		deps = append(deps, lib)
	}

	libsToLink := []string{}

	for _, dep := range deps {
		ins = append(ins, dep.Out)
		if dep.AlwaysLink {
			libsToLink = append(libsToLink, "-whole-archive", fmt.Sprintf("%q",dep.Out), "-no-whole-archive")
		} else {
			libsToLink = append(libsToLink, fmt.Sprintf("%q",dep.Out))
		}
	}

	if bin.Script != nil {
		ins = append(ins, bin.Script)
	} else if toolchain.Script() != nil {
		ins = append(ins, toolchain.Script())
	}

	flags := bin.LinkerFlags
	if bin.Script != nil {
		flags = append(flags, "-T", fmt.Sprintf("%q", bin.Script))
	}

	ctx.AddBuildStepWithRule(core.BuildStepWithRule{
		Outs:  []core.OutPath{bin.Out},
		Ins:   ins,
		Rule:  bin.ldRule(),
		Variables: map[string]string {
			"flags": strings.Join(flags, " "),
			"libs": strings.Join(libsToLink, " "),
			"objs": strings.Join(objsToLink, " "),
		},
	})
}

func (bin Binary) Run(args []string) string {
	quotedArgs := []string{}
	for _, arg := range args {
		quotedArgs = append(quotedArgs, fmt.Sprintf("%q", arg))
	}
	return fmt.Sprintf("%q %s", bin.Out, strings.Join(quotedArgs, " "))
}
