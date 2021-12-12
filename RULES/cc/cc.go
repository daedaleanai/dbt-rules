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
	Flags     []string
	Toolchain Toolchain
}

// Build an objectFile.
func (obj objectFile) Build(ctx core.Context) {
	toolchain := toolchainOrDefault(obj.Toolchain)
	depfile := obj.Out.WithExt("d")

	cmd := ""
	flags := []string{}

	switch filepath.Ext(obj.Src.Absolute()) {
	case ".cc":
		cmd = toolchain.CxxCompiler()
		flags = append(toolchain.CxxFlags(), "-pipe", "-c", "-MD", "-MF", fmt.Sprintf("%q", depfile))
	case ".c":
		cmd = toolchain.CCompiler()
		flags = append(toolchain.CFlags(), "-pipe", "-c", "-MD", "-MF", fmt.Sprintf("%q", depfile))
	case ".S":
		cmd = toolchain.Assembler()
		flags = append(toolchain.AsFlags(), "-c")
	default:
		panic("Unknown source extension for cc toolchain '" + filepath.Ext(obj.Src.Absolute()) + "'")
	}

	flags = append(flags, "-o", fmt.Sprintf("%q", obj.Out))

	for _, include := range obj.Includes {
		flags = append(flags, fmt.Sprintf("-I%q", include))
	}

	cmd = fmt.Sprintf("%s %s %q", cmd, strings.Join(flags, " "), obj.Src)

	ctx.WithTrace("obj:"+obj.Out.Relative(), func(ctx core.Context) {
		ctx.AddBuildStep(core.BuildStep{
			Out:     obj.Out,
			Depfile: depfile,
			In:      obj.Src,
			Cmd:     cmd,
			Descr:   fmt.Sprintf("CC (toolchain: %s) %s", toolchain.Name(), obj.Out.Relative()),
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
			Cmd:   fmt.Sprintf("%s %s -r -b binary -o %q %q", blob.Toolchain.Link(), blob.Toolchain.LdFlags(), blob.out(), blob.In),
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

func compileSources(out core.OutPath, ctx core.Context, srcs []core.Path, flags []string, deps []Library, toolchain Toolchain) []core.Path {
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
			Flags:     flags,
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
	CompilerFlags []string
	Deps          []Dep
	Shared        bool
	AlwaysLink    bool
	Toolchain     Toolchain

	// Extra fields for handling multi-toolchain logic.
	userOut       core.OutPath
	userToolchain Toolchain
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

	objs := compileSources(lib.Out, ctx, lib.Srcs, lib.CompilerFlags, deps, toolchain)
	objs = append(objs, lib.Objs...)

	for _, blob := range lib.Blobs {
		blobObject := BlobObject{In: blob, Toolchain: toolchain}
		blobObject.Build(ctx)
		objs = append(objs, blobObject.out())
	}

	var cmd, descr string
	if lib.Shared {
		cmd = fmt.Sprintf("%s -pipe -shared %s -o %q %s", toolchain.Link(), strings.Join(toolchain.LdFlags(), " "), lib.Out, joinQuoted(objs))
		descr = fmt.Sprintf("LD (toolchain: %s) %s", toolchain.Name(), lib.Out.Relative())
	} else {
		cmd = fmt.Sprintf(
			"rm %q 2>/dev/null ; %s rcs %q %s",
			lib.Out,
			toolchain.Archiver(),
			lib.Out,
			joinQuoted(objs))
		descr = fmt.Sprintf("AR (toolchain: %s) %s", toolchain.Name(), lib.Out.Relative())
	}

	ctx.AddBuildStep(core.BuildStep{
		Out:   lib.Out,
		Ins:   objs,
		Cmd:   cmd,
		Descr: descr,
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
	CompilerFlags []string
	LinkerFlags   []string
	Deps          []Dep
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

func (bin Binary) build(ctx core.Context) {
	toolchain := toolchainOrDefault(bin.Toolchain)

	deps := collectDepsWithToolchain(toolchain, append(bin.Deps, toolchain.StdDeps()...))
	for _, d := range deps {
		d.Build(ctx)
	}
	objs := compileSources(bin.Out, ctx, bin.Srcs, bin.CompilerFlags, deps, toolchain)

	ins := objs
	alwaysLinkLibs := []core.Path{}
	otherLibs := []core.Path{}
	for _, dep := range deps {
		ins = append(ins, dep.Out)
		if dep.AlwaysLink {
			alwaysLinkLibs = append(alwaysLinkLibs, dep.Out)
		} else {
			otherLibs = append(otherLibs, dep.Out)
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

	cmd := fmt.Sprintf("%s %s %s -o %q -Wl,-whole-archive %s -Wl,-no-whole-archive %s", toolchain.Link(), strings.Join(toolchain.LdFlags(), " "), strings.Join(flags, " "),
		bin.Out,
		joinQuoted(alwaysLinkLibs),
		joinQuoted(otherLibs),
	)

	ctx.AddBuildStep(core.BuildStep{
		Out:   bin.Out,
		Ins:   ins,
		Cmd:   cmd,
		Descr: fmt.Sprintf("LD (toolchain: %s) %s", toolchain.Name(), bin.Out.Relative()),
	})
}

func (bin Binary) Run(args []string) string {
	quotedArgs := []string{}
	for _, arg := range args {
		quotedArgs = append(quotedArgs, fmt.Sprintf("%q", arg))
	}
	return fmt.Sprintf("%q %s", bin.Out, strings.Join(quotedArgs, " "))
}
