package cc

import (
	"dbt-rules/RULES/core"
	"fmt"
	"strings"
)

type Toolchain interface {
	Name() string
	Compile(out, depfile core.OutPath, flags []string, includes []core.Path, src core.Path) string
	LinkStaticLibrary(out core.OutPath, objs []core.Path) string
	LinkSharedLibrary(out core.OutPath, objs []core.Path) string
	LinkBinary(out core.OutPath, objs []core.Path, alwaysLinkLibs []core.Path, libs []core.Path, flags []string) string
	EmbedBlob(out core.OutPath, src core.Path) string
}

var ToolchainParam = core.RegisterBuildParam((*Toolchain)(nil), "C++ Toolchain")

var SystemToolchain = GccToolchain{
	name: "system",

	Ar:      core.NewGlobalPath("ar"),
	As:      core.NewGlobalPath("as"),
	Cc:      core.NewGlobalPath("gcc"),
	Cpp:     core.NewGlobalPath("gcc -E"),
	Cxx:     core.NewGlobalPath("g++"),
	Objcopy: core.NewGlobalPath("objcopy"),

	CompilerFlags: []string{"-std=c++14", "-O3", "-fdiagnostics-color=always"},
	LinkerFlags:   []string{"-fdiagnostics-color=always"},
}

var TestToolchainA = GccToolchain{
	name: "A",

	Ar:      core.NewGlobalPath("ar"),
	As:      core.NewGlobalPath("as"),
	Cc:      core.NewGlobalPath("gcc"),
	Cpp:     core.NewGlobalPath("gcc -E"),
	Cxx:     core.NewGlobalPath("g++"),
	Objcopy: core.NewGlobalPath("objcopy"),

	CompilerFlags: []string{"-std=c++11", "-O3", "-fdiagnostics-color=always"},
	LinkerFlags:   []string{"-fdiagnostics-color=always"},
}

var TestToolchainB = GccToolchain{
	name: "B",

	Ar:      core.NewGlobalPath("ar"),
	As:      core.NewGlobalPath("as"),
	Cc:      core.NewGlobalPath("gcc"),
	Cpp:     core.NewGlobalPath("gcc -E"),
	Cxx:     core.NewGlobalPath("g++"),
	Objcopy: core.NewGlobalPath("objcopy"),

	CompilerFlags: []string{"-std=c++17", "-O3", "-fdiagnostics-color=always"},
	LinkerFlags:   []string{"-fdiagnostics-color=always"},
}

var _ = ToolchainParam.AddOption(SystemToolchain)
var _ = ToolchainParam.AddDefaultOption(TestToolchainA)
var _ = ToolchainParam.AddOption(TestToolchainB)

// Toolchain represents a C++ toolchain.
type GccToolchain struct {
	name string

	Ar      core.GlobalPath
	As      core.GlobalPath
	Cc      core.GlobalPath
	Cpp     core.GlobalPath
	Cxx     core.GlobalPath
	Objcopy core.GlobalPath

	Includes []core.Path

	CompilerFlags []string
	LinkerFlags   []string

	ArchName   string
	TargetName string
}

func (gcc GccToolchain) Name() string {
	return gcc.name
}

// Compile generates a compile command.
func (gcc GccToolchain) Compile(out, depfile core.OutPath, flags []string, includes []core.Path, src core.Path) string {
	includesStr := strings.Builder{}
	for _, include := range includes {
		includesStr.WriteString(fmt.Sprintf("-I%q ", include))
	}
	for _, include := range gcc.Includes {
		includesStr.WriteString(fmt.Sprintf("-isystem %q ", include))
	}

	return fmt.Sprintf(
		"%q -pipe -c -o %q -MD -MF %q %s %s %q",
		gcc.Cxx,
		out,
		depfile,
		strings.Join(append(gcc.CompilerFlags, flags...), " "),
		includesStr.String(),
		src)
}

// LinkStaticLibrary generates the command to build a static library.
func (gcc GccToolchain) LinkStaticLibrary(out core.OutPath, objs []core.Path) string {
	return fmt.Sprintf(
		"%q rv %q %s >/dev/null 2>/dev/null",
		gcc.Ar,
		out,
		joinQuoted(objs))
}

// LinkSharedLibrary generates the command to build a shared library.
func (gcc GccToolchain) LinkSharedLibrary(out core.OutPath, objs []core.Path) string {
	return fmt.Sprintf(
		"%q -pipe -shared -o %q %s",
		gcc.Cxx,
		out,
		joinQuoted(objs))
}

// LinkBinary generates the command to build an executable.
func (gcc GccToolchain) LinkBinary(out core.OutPath, objs []core.Path, alwaysLinkLibs []core.Path, libs []core.Path, flags []string) string {
	flags = append(gcc.LinkerFlags, flags...)
	return fmt.Sprintf(
		"%q -pipe -o %q %s -Wl,-whole-archive %s -Wl,-no-whole-archive %s %s",
		gcc.Cxx,
		out,
		joinQuoted(objs),
		joinQuoted(alwaysLinkLibs),
		joinQuoted(libs),
		strings.Join(flags, " "))
}

func (gcc GccToolchain) EmbedBlob(out core.OutPath, src core.Path) string {
	return fmt.Sprintf(
		"%q -I binary -O %s -B %s %q %q",
		gcc.Objcopy,
		gcc.TargetName,
		gcc.ArchName,
		src,
		out)
}

func joinQuoted(paths []core.Path) string {
	b := strings.Builder{}
	for _, p := range paths {
		fmt.Fprintf(&b, "%q ", p)
	}
	return b.String()
}
