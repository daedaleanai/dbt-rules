package cc

import (
	"fmt"
	"strings"

	"dbt-rules/RULES/core"
)

type Toolchain interface {
	Name() string
	ObjectFile(out core.OutPath, depfile core.OutPath, flags []string, includes []core.Path, src core.Path) string
	StaticLibrary(out core.Path, objs []core.Path) string
	SharedLibrary(out core.Path, objs []core.Path) string
	Binary(out core.Path, objs []core.Path, alwaysLinkLibs []core.Path, libs []core.Path, flags []string, script core.Path) string
	EmbeddedBlob(out core.OutPath, src core.Path) string
	RawBinary(out core.Path, elfSrc core.Path) string
	StdDeps() []Dep
}

// Toolchain represents a C++ toolchain.
type GccToolchain struct {
	Ar      core.GlobalPath
	As      core.GlobalPath
	Cc      core.GlobalPath
	Cpp     core.GlobalPath
	Cxx     core.GlobalPath
	Objcopy core.GlobalPath
	Ld      core.GlobalPath

	Includes     []core.Path
	Deps         []Dep
	LinkerScript core.Path

	CompilerFlags []string
	LinkerFlags   []string

	ToolchainName string
	ArchName      string
	TargetName    string
}

func (gcc GccToolchain) NewWithStdLib(includes []core.Path, deps []Dep, linkerScript core.Path, toolchainName string) GccToolchain {
	gcc.Includes = includes
	gcc.Deps = deps
	gcc.LinkerScript = linkerScript
	gcc.ToolchainName = toolchainName
	return gcc
}

// ObjectFile generates a compile command.
func (gcc GccToolchain) ObjectFile(out core.OutPath, depfile core.OutPath, flags []string, includes []core.Path, src core.Path) string {
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

// StaticLibrary generates the command to build a static library.
func (gcc GccToolchain) StaticLibrary(out core.Path, objs []core.Path) string {
	return fmt.Sprintf(
		"%q rv %q %s >/dev/null 2>/dev/null",
		gcc.Ar,
		out,
		joinQuoted(objs))
}

// SharedLibrary generates the command to build a shared library.
func (gcc GccToolchain) SharedLibrary(out core.Path, objs []core.Path) string {
	return fmt.Sprintf(
		"%q -pipe -shared -o %q %s",
		gcc.Cxx,
		out,
		joinQuoted(objs))
}

// Binary generates the command to build an executable.
func (gcc GccToolchain) Binary(out core.Path, objs []core.Path, alwaysLinkLibs []core.Path, libs []core.Path, flags []string, script core.Path) string {
	flags = append(gcc.LinkerFlags, flags...)
	if script != nil {
		flags = append(flags, "-T", fmt.Sprintf("%q", script))
	} else if gcc.LinkerScript != nil {
		flags = append(flags, "-T", fmt.Sprintf("%q", gcc.LinkerScript))
	}

	return fmt.Sprintf(
		"%q -pipe -o %q %s -Wl,-whole-archive %s -Wl,-no-whole-archive %s %s",
		gcc.Cxx,
		out,
		joinQuoted(objs),
		joinQuoted(alwaysLinkLibs),
		joinQuoted(libs),
		strings.Join(flags, " "))
}

// EmbeddedBlob creates an object file from any binary blob of data
func (gcc GccToolchain) EmbeddedBlob(out core.OutPath, src core.Path) string {
	return fmt.Sprintf(
		"%q -r -b binary -o %q %q",
		gcc.Ld,
		out,
		src)
}

// RawBinary strips ELF metadata to create a raw binary image
func (gcc GccToolchain) RawBinary(out core.Path, elfSrc core.Path) string {
	return fmt.Sprintf(
		"%q -O binary %q %q",
		gcc.Objcopy,
		elfSrc,
		out)
}

func (gcc GccToolchain) StdDeps() []Dep {
	return gcc.Deps
}

func (gcc GccToolchain) Name() string {
	return gcc.ToolchainName
}

func joinQuoted(paths []core.Path) string {
	b := strings.Builder{}
	for _, p := range paths {
		fmt.Fprintf(&b, "%q ", p)
	}
	return b.String()
}

var toolchains = make(map[string]Toolchain)

func (gcc GccToolchain) Register() Toolchain {
	if _, found := toolchains[gcc.Name()]; found {
		core.Fatal("A toolchain with name %s has already been registered", gcc.Name())
	}
	toolchains[gcc.Name()] = gcc
	return gcc
}

var NativeGcc = GccToolchain{
	Ar:      core.NewGlobalPath("ar"),
	As:      core.NewGlobalPath("as"),
	Cc:      core.NewGlobalPath("gcc"),
	Cpp:     core.NewGlobalPath("gcc -E"),
	Cxx:     core.NewGlobalPath("g++"),
	Objcopy: core.NewGlobalPath("objcopy"),
	Ld:      core.NewGlobalPath("ld"),

	CompilerFlags: []string{"-std=c++14", "-O3", "-fdiagnostics-color=always"},
	LinkerFlags:   []string{"-fdiagnostics-color=always"},

	ToolchainName: "native-gcc",
}.Register()

var defaultToolchainFlag = core.StringFlag{
	Name:        "cc-toolchain",
	Description: "Default toolchain to compile generic C/C++ targets",
	DefaultFn:   func() string { return NativeGcc.Name() },
}.Register()

func defaultToolchain() Toolchain {
	if toolchain, ok := toolchains[defaultToolchainFlag.Value()]; ok {
		return toolchain
	}
	core.Fatal("No toolchain has been registered with the name %s", defaultToolchainFlag.Value())
	return nil
}
