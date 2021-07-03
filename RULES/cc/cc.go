package cc

import (
	"dbt-rules/RULES/core"
	"fmt"
)

// ObjectFile compiles a single C++ source file.
type ObjectFile struct {
	Src      core.Path
	Includes []core.Path
	Flags    []string
}

// Build an ObjectFile.
func (obj *ObjectFile) Build(ctx core.Context) core.BuildOutput {
	depfile := ctx.OutPath(obj.Src).WithExt("d")
	out := ctx.OutPath(obj.Src).WithExt("o")
	cmd := ctx.GetBuildOption(ToolchainParam).(Toolchain).Compile(out, depfile, obj.Flags, obj.Includes, obj.Src)
	ctx.AddBuildStep(core.BuildStep{
		Out:     out,
		Depfile: depfile,
		In:      obj.Src,
		Cmd:     cmd,
		Descr:   fmt.Sprintf("CC %s", out.Relative()),
	})
	return out
}

type DependencyInfo struct {
	Includes   []core.Path
	Deps       []Dependency
	AlwaysLink bool
}

// Dependency is an interface implemented by dependencies that can be linked into a library.
type Dependency interface {
	Build(ctx core.Context) core.BuildOutput
	Info() DependencyInfo
}

// Library builds and links a static C++ library.
type Library struct {
	Name          string
	Srcs          []core.Path
	Objs          []ObjectFile
	Includes      []core.Path
	CompilerFlags []string
	Deps          []Dependency
	Shared        bool
	AlwaysLink    bool
}

// Build a Library.
func (lib *Library) Build(ctx core.Context) core.BuildOutput {
	objs := compileSources(ctx, lib.Srcs, lib.CompilerFlags, flattenDeps([]Dependency{lib}))
	for _, obj := range lib.Objs {
		objs = append(objs, ctx.Build(&obj).Output())
	}

	var cmd, descr string
	libName := "lib" + lib.Name + ".a"

	out := ctx.Cwd().WithFilename(libName)
	toolchain := ctx.GetBuildOption(ToolchainParam).(Toolchain)
	if lib.Shared {
		cmd = toolchain.LinkSharedLibrary(out, objs)
		descr = fmt.Sprintf("LD %s", out.Relative())
	} else {
		cmd = toolchain.LinkStaticLibrary(out, objs)
		descr = fmt.Sprintf("AR %s", out.Relative())
	}

	ctx.AddBuildStep(core.BuildStep{
		Out:   out,
		Ins:   objs,
		Cmd:   cmd,
		Descr: descr,
	})

	return out
}

func (lib *Library) Info() DependencyInfo {
	return DependencyInfo{
		lib.Includes,
		lib.Deps,
		lib.AlwaysLink,
	}
}

// Binary builds and links an executable.
type Binary struct {
	Name          string
	Srcs          []core.Path
	CompilerFlags []string
	LinkerFlags   []string
	Deps          []Dependency
}

// Build a Binary.
func (bin *Binary) Build(ctx core.Context) core.BuildOutput {
	deps := flattenDeps(bin.Deps)
	objs := compileSources(ctx, bin.Srcs, bin.CompilerFlags, deps)

	ins := objs
	alwaysLinkLibs := []core.Path{}
	otherLibs := []core.Path{}
	for _, dep := range deps {
		lib := ctx.Build(dep).Output()
		ins = append(ins, lib)
		if dep.Info().AlwaysLink {
			alwaysLinkLibs = append(alwaysLinkLibs, lib)
		} else {
			otherLibs = append(otherLibs, lib)
		}
	}

	out := ctx.Cwd().WithFilename(bin.Name)
	cmd := ctx.GetBuildOption(ToolchainParam).(Toolchain).LinkBinary(out, objs, alwaysLinkLibs, otherLibs, bin.LinkerFlags)
	ctx.AddBuildStep(core.BuildStep{
		Out:   out,
		Ins:   ins,
		Cmd:   cmd,
		Descr: fmt.Sprintf("LD %s", out.Relative()),
	})

	return out
}

func flattenDepsRec(deps []Dependency, visited map[Dependency]bool) []Dependency {
	flatDeps := []Dependency{}
	for _, dep := range deps {
		if _, exists := visited[dep]; !exists {
			visited[dep] = true
			flatDeps = append([]Dependency{dep}, append(flattenDepsRec(dep.Info().Deps, visited), flatDeps...)...)
		}
	}
	return flatDeps
}

func flattenDeps(deps []Dependency) []Dependency {
	return flattenDepsRec(deps, map[Dependency]bool{})
}

func compileSources(ctx core.Context, srcs []core.Path, flags []string, deps []Dependency) []core.Path {
	includes := []core.Path{core.SourcePath("")}
	for _, dep := range deps {
		includes = append(includes, dep.Info().Includes...)
	}

	objs := []core.Path{}
	for _, src := range srcs {
		obj := &ObjectFile{
			Src:      src,
			Includes: includes,
			Flags:    flags,
		}
		objs = append(objs, ctx.Build(obj).Output())
	}
	return objs
}
