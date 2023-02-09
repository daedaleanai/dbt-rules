package rust

import (
	"dbt-rules/RULES/core"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

type Binary struct {
	Out       core.OutPath
	Package   core.Path
	Toolchain string
}

func (bin Binary) binLocation() core.OutPath {
	return bin.Out.WithPrefix(fmt.Sprintf("%s/release/", filepath.Base(bin.Package.Relative())))
}

func (bin Binary) Build(ctx core.Context) {
	if bin.Toolchain == "" {
		bin.Toolchain = "stable"
	}
	ctx.AddBuildStep(core.BuildStep{
		Out: bin.binLocation(),
		Ins: bin.getInputs(),
		// With suffix is used to turn an input path into an output path
		Cmd: fmt.Sprintf("cd %q && cargo +%s build -r --locked --bins --target-dir %q", bin.Package, bin.Toolchain, bin.Package.WithSuffix("")),
	})
}

func (bin Binary) Run(args []string) string {
	quotedArgs := []string{}
	for _, arg := range args {
		quotedArgs = append(quotedArgs, fmt.Sprintf("%q", arg))
	}
	return fmt.Sprintf("%q %s", bin.binLocation(), strings.Join(quotedArgs, " "))

}

// Depend on all files in the directory. This might be overestimate, but let `cargo` do a more precise estimate.
func (bin Binary) getInputs() []core.Path {
	inputs := []core.Path{}
	filepath.WalkDir(bin.Package.Absolute(), func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(core.SourcePath("").Absolute(), path)
		if err != nil {
			return nil
		}
		inputs = append(inputs, core.SourcePath(relPath))
		return nil
	})
	return inputs
}
