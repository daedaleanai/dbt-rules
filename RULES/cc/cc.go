package cc

import (
	"fmt"
	"strings"

	"dbt-rules/RULES/core"
)

// ObjectFile compiles a single C++ source file.
type ObjectFile struct {
	Src       core.Path
	Includes  []core.Path
	Flags     []string
	Toolchain Toolchain
}

// Build an ObjectFile.
func (obj ObjectFile) Build(ctx core.Context) {
	toolchain := toolchainOrDefault(obj.Toolchain)
	depfile := obj.out().WithExt("d")
	cmd := toolchain.ObjectFile(obj.out(), depfile, obj.Flags, obj.Includes, obj.Src)
	ctx.WithTrace("obj:"+obj.out().Relative(), func(ctx core.Context) {
		ctx.AddBuildStep(core.BuildStep{
			Out:     obj.out(),
			Depfile: depfile,
			In:      obj.Src,
			Cmd:     cmd,
			Descr:   fmt.Sprintf("CC (toolchain: %s) %s", toolchain.Name(), obj.out().Relative()),
		})
	})
}

func (obj ObjectFile) out() core.OutPath {
	toolchain := toolchainOrDefault(obj.Toolchain)
	return obj.Src.WithPrefix(toolchain.Name() + "/").WithExt("o")
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
			Cmd:   blob.Toolchain.BlobObject(blob.out(), blob.In),
			Descr: fmt.Sprintf("BLOB (toolchain: %s) %s", toolchain.Name(), blob.out().Relative()),
		})
	})
}

func (blob BlobObject) out() core.OutPath {
	toolchain := toolchainOrDefault(blob.Toolchain)
	return blob.In.WithPrefix(toolchain.Name() + "/").WithExt("blob.o")
}

func collectDepsWithToolchainRec(toolchain Toolchain, deps []Dep, visited map[string]bool) []Dep {
	var flatDeps []Dep
	for _, dep := range deps {
		dep = dep.WithToolchain(toolchain)
		depPath := dep.GetBuildOut().Absolute()
		if !visited[depPath] {
			visited[depPath] = true
			flatDeps = append(flatDeps, dep)
			flatDeps = append(flatDeps, collectDepsWithToolchainRec(toolchain, dep.GetDirectDeps(), visited)...)
		}
	}
	return flatDeps
}

func collectDepsWithToolchain(toolchain Toolchain, deps []Dep) []Dep {
	return collectDepsWithToolchainRec(toolchain, deps, map[string]bool{})
}

func compileSources(ctx core.Context, srcs []core.Path, flags []string, deps []Dep, toolchain Toolchain) []core.Path {
	includes := []core.Path{core.SourcePath("")}
	for _, dep := range deps {
		includes = append(includes, dep.GetIncludes()...)
	}

	objs := []core.Path{}

	for _, src := range srcs {
		obj := ObjectFile{
			Src:       src,
			Includes:  includes,
			Flags:     flags,
			Toolchain: toolchain,
		}
		obj.Build(ctx)
		objs = append(objs, obj.out())
	}

	return objs
}

// Dep is an interface implemented by dependencies that can be linked into a binary/library.
type Dep interface {
	// Returns a new instance of Dep which will be build using the specified toolchain.
	WithToolchain(toolchain Toolchain) Dep

	Build(ctx core.Context)
	GetDirectDeps() []Dep

	// Returns the build path of Dep, i.e., the exact path where DBT will emit the Dep.
	// Note that this might differ from the user-specified "Out" path, for example if non-default toolchain is used.
	GetBuildOut() core.OutPath

	GetIncludes() []core.Path
	IsAlwaysLink() bool
}

// Library builds and links a static C++ library.
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
}

// Build a Library.
func (lib Library) build(ctx core.Context) {
	if lib.Out == nil {
		core.Fatal("Out field is required for cc.Library")
	}

	buildOut := lib.GetBuildOut()
	if ctx.Built(buildOut.Absolute()) {
		return
	}

	toolchain := toolchainOrDefault(lib.Toolchain)

	deps := collectDepsWithToolchain(toolchain, append(toolchain.StdDeps(), lib))
	for _, d := range deps {
		d.Build(ctx)
	}

	objs := compileSources(ctx, lib.Srcs, lib.CompilerFlags, deps, toolchain)
	objs = append(objs, lib.Objs...)

	for _, blob := range lib.Blobs {
		blobObject := BlobObject{In: blob, Toolchain: toolchain}
		blobObject.Build(ctx)
		objs = append(objs, blobObject.out())
	}

	var cmd, descr string
	if lib.Shared {
		cmd = toolchain.SharedLibrary(buildOut, objs)
		descr = fmt.Sprintf("LD (toolchain: %s) %s", toolchain.Name(), buildOut.Relative())
	} else {
		cmd = toolchain.StaticLibrary(buildOut, objs)
		descr = fmt.Sprintf("AR (toolchain: %s) %s", toolchain.Name(), buildOut.Relative())
	}

	ctx.AddBuildStep(core.BuildStep{
		Out:   buildOut,
		Ins:   objs,
		Cmd:   cmd,
		Descr: descr,
	})
}

func (lib Library) Build(ctx core.Context) {
	ctx.WithTrace("lib:"+lib.GetBuildOut().Relative(), lib.build)
}

// Returns a toolchain-specific variant of the Library.
func (lib Library) WithToolchain(toolchain Toolchain) Dep {
	if toolchain == nil {
		core.Fatal("Using Library without toolchain is illegal (%s)", lib.GetBuildOut())
	}
	lib.Toolchain = toolchain
	return lib
}

func (lib Library) GetDirectDeps() []Dep {
	return lib.Deps
}

func (lib Library) GetBuildOut() core.OutPath {
	if lib.Toolchain == nil || lib.Toolchain.Name() == DefaultToolchain().Name() {
		return lib.Out
	}

	return lib.Out.WithPrefix(lib.Toolchain.Name() + "/")
}

func (lib Library) GetIncludes() []core.Path {
	return lib.Includes
}

func (lib Library) IsAlwaysLink() bool {
	return lib.AlwaysLink
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
	objs := compileSources(ctx, bin.Srcs, bin.CompilerFlags, deps, toolchain)

	ins := objs
	alwaysLinkLibs := []core.Path{}
	otherLibs := []core.Path{}
	for _, dep := range deps {
		depBuildOut := dep.GetBuildOut()
		ins = append(ins, depBuildOut)
		if dep.IsAlwaysLink() {
			alwaysLinkLibs = append(alwaysLinkLibs, depBuildOut)
		} else {
			otherLibs = append(otherLibs, depBuildOut)
		}
	}

	if bin.Script != nil {
		ins = append(ins, bin.Script)
	} else if toolchain.Script() != nil {
		ins = append(ins, toolchain.Script())
	}

	cmd := toolchain.Binary(bin.Out, objs, alwaysLinkLibs, otherLibs, bin.LinkerFlags, bin.Script)
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
