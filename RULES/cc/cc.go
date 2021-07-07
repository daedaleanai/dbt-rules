package cc

import (
	"fmt"

	"dbt-rules/RULES/core"
)

const objsDirSuffix = "-OBJS"

// ObjectFile compiles a single C++ source file.
type ObjectFile struct {
	Src       core.Path
	OutDir    core.OutPath
	Includes  []core.Path
	Flags     []string
	Toolchain Toolchain
}

// Build an ObjectFile.
func (obj ObjectFile) Build(ctx core.Context) {
	toolchain := obj.Toolchain
	if toolchain == nil {
		toolchain = &DefaultToolchain
	}

	depfile := obj.out().WithExt("d")
	cmd := toolchain.ObjectFile(obj.out(), depfile, obj.Flags, obj.Includes, obj.Src)
	ctx.AddBuildStep(core.BuildStep{
		Out:     obj.out(),
		Depfile: depfile,
		In:      obj.Src,
		Cmd:     cmd,
		Descr:   fmt.Sprintf("CC %s", obj.out().Relative()),
	})
}

func (obj ObjectFile) out() core.OutPath {
	defaultOut := obj.Src.WithExt("o")
	if obj.OutDir == nil {
		return defaultOut
	}
	return obj.OutDir.WithSuffix("/"+defaultOut.Relative())
}

func flattenDepsRec(deps []Dep, visited map[string]bool) []Library {
	flatDeps := []Library{}
	for _, dep := range deps {
		lib := dep.CcLibrary()
		libPath := lib.Out.Absolute()
		if _, exists := visited[libPath]; !exists {
			visited[libPath] = true
			flatDeps = append([]Library{lib}, append(flattenDepsRec(lib.Deps, visited), flatDeps...)...)
		}
	}
	return flatDeps
}

func flattenDeps(deps []Dep) []Library {
	return flattenDepsRec(deps, map[string]bool{})
}

func compileSources(ctx core.Context, outDir core.OutPath, srcs []core.Path, flags []string, deps []Library, toolchain Toolchain) []core.Path {
	includes := []core.Path{core.SourcePath("")}
	for _, dep := range deps {
		includes = append(includes, dep.Includes...)
	}

	objs := []core.Path{}

	for _, src := range srcs {
		obj := ObjectFile{
			Src:       src,
			OutDir:    outDir,
			Includes:  includes,
			Flags:     flags,
			Toolchain: toolchain,
		}
		obj.Build(ctx)
		objs = append(objs, obj.out())
	}

	return objs
}

// Dep is an interface implemented by dependencies that can be linked into a library.
type Dep interface {
	CcLibrary() Library
}

// Library builds and links a static C++ library.
type Library struct {
	Out           core.OutPath
	Srcs          []core.Path
	Objs          []core.Path
	Includes      []core.Path
	CompilerFlags []string
	Deps          []Dep
	Shared        bool
	AlwaysLink    bool
	Toolchain     Toolchain
}

// Build a Library.
func (lib Library) Build(ctx core.Context) {
	toolchain := lib.Toolchain
	if toolchain == nil {
		toolchain = &DefaultToolchain
	}

	objsDir := lib.Out.WithSuffix(objsDirSuffix)
	objs := compileSources(ctx, objsDir, lib.Srcs, lib.CompilerFlags, flattenDeps([]Dep{lib}), toolchain)
	objs = append(objs, lib.Objs...)

	var cmd, descr string
	if lib.Shared {
		cmd = toolchain.SharedLibrary(lib.Out, objs)
		descr = fmt.Sprintf("LD %s", lib.Out.Relative())
	} else {
		cmd = toolchain.StaticLibrary(lib.Out, objs)
		descr = fmt.Sprintf("AR %s", lib.Out.Relative())
	}

	ctx.AddBuildStep(core.BuildStep{
		Out:   lib.Out,
		Ins:   objs,
		Cmd:   cmd,
		Descr: descr,
	})
}

// CcLibrary for Library is just the identity.
func (lib Library) CcLibrary() Library {
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
	toolchain := bin.Toolchain
	if toolchain == nil {
		toolchain = DefaultToolchain
	}

	deps := flattenDeps(bin.Deps)
	objsDir := bin.Out.WithSuffix(objsDirSuffix)
	objs := compileSources(ctx, objsDir, bin.Srcs, bin.CompilerFlags, deps, toolchain)

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
	}

	cmd := toolchain.Binary(bin.Out, objs, alwaysLinkLibs, otherLibs, bin.LinkerFlags, bin.Script)
	ctx.AddBuildStep(core.BuildStep{
		Out:   bin.Out,
		Ins:   ins,
		Cmd:   cmd,
		Descr: fmt.Sprintf("LD %s", bin.Out.Relative()),
	})
}
