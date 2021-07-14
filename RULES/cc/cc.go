package cc

import (
	"fmt"

	"dbt-rules/RULES/core"
)

const objsDirSuffix = "-OBJS"

// ObjectFile compiles a single C++ source file.
type ObjectFile struct {
	Src       core.Path
	Includes  []core.Path
	Flags     []string
	Toolchain Toolchain
}

// Build an ObjectFile.
func (obj ObjectFile) Build(ctx core.Context) {
	toolchain := obj.Toolchain
	if toolchain == nil {
		toolchain = defaultToolchain()
	}

	depfile := obj.out().WithExt("d")
	cmd := toolchain.ObjectFile(obj.out(), depfile, obj.Flags, obj.Includes, obj.Src)
	ctx.AddBuildStep(core.BuildStep{
		Out:     obj.out(),
		Depfile: depfile,
		In:      obj.Src,
		Cmd:     cmd,
		Descr:   fmt.Sprintf("CC (toolchain: %s) %s", toolchain.Name(), obj.out().Relative()),
	})
}

func (obj ObjectFile) out() core.OutPath {
	toolchain := obj.Toolchain
	if toolchain == nil {
		toolchain = defaultToolchain()
	}
	return obj.Src.WithPrefix(toolchain.Name() + "/").WithExt("o")
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

func compileSources(ctx core.Context, srcs []core.Path, flags []string, deps []Library, toolchain Toolchain) []core.Path {
	includes := []core.Path{core.SourcePath("")}
	for _, dep := range deps {
		includes = append(includes, dep.Includes...)
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

	multipleToolchains bool
	toolchainMap       map[string]Library
	baseOut            core.OutPath
}

func (lib Library) MultipleToolchains() Library {
	lib.multipleToolchains = true
	lib.toolchainMap = make(map[string]Library)
	lib.baseOut = lib.Out
	return lib
}

// Build a Library.
func (lib Library) Build(ctx core.Context) {
	toolchain := lib.Toolchain
	if toolchain == nil {
		toolchain = defaultToolchain()
	}

	if lib.multipleToolchains {
		if lib.Out == lib.baseOut {
			var defaultLib = core.CopyFile{
				From: lib.WithToolchain(ctx, defaultToolchain()).Out,
				To: lib.Out,
			}
			defaultLib.Build(ctx)
			return
		}
		if _, found := lib.toolchainMap[toolchain.Name()]; found {
			return
		}
		lib.toolchainMap[toolchain.Name()] = lib
	}

	deps := flattenDeps(append(toolchain.StdDeps(), lib))
	for i, _ := range deps {
		deps[i] = deps[i].WithToolchain(ctx, toolchain)
	}
	objs := compileSources(ctx, lib.Srcs, lib.CompilerFlags, deps, toolchain)
	objs = append(objs, lib.Objs...)

	var cmd, descr string
	if lib.Shared {
		cmd = toolchain.SharedLibrary(lib.Out, objs)
		descr = fmt.Sprintf("LD (toolchain: %s) %s", toolchain.Name(), lib.Out.Relative())
	} else {
		cmd = toolchain.StaticLibrary(lib.Out, objs)
		descr = fmt.Sprintf("AR (totoolchain: %s) %s", toolchain.Name(), lib.Out.Relative())
	}

	ctx.AddBuildStep(core.BuildStep{
		Out:   lib.Out,
		Ins:   objs,
		Cmd:   cmd,
		Descr: descr,
	})
}

func (lib Library) WithToolchain(ctx core.Context, toolchain Toolchain) Library {
	if !lib.multipleToolchains {
		return lib
	}
	if otherLib, found := lib.toolchainMap[toolchain.Name()]; found {
		return otherLib
	}
	otherLib := lib
	otherLib.Out = lib.baseOut.WithPrefix(toolchain.Name() + "/")
	otherLib.Toolchain = toolchain
	otherLib.Build(ctx)
	return otherLib
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
		toolchain = defaultToolchain()
	}

	deps := flattenDeps(append(bin.Deps, toolchain.StdDeps()...))
	for i, _ := range deps {
		deps[i] = deps[i].WithToolchain(ctx, toolchain)
	}
	objs := compileSources(ctx, bin.Srcs, bin.CompilerFlags, deps, toolchain)

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
		Descr: fmt.Sprintf("LD (toolchain: %s) %s", toolchain.Name(), bin.Out.Relative()),
	})
}
