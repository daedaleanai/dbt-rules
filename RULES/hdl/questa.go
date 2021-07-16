package hdl

import (
	"fmt"
	"strings"

	"dbt-rules/RULES/core"
	"dbt-rules/hdl"
)

type QuestaSimScriptParams struct {
	Name         string
	PartName     string
	BoardName    string
	OutDir       core.Path
	OutScript    core.Path
	OutSimScript core.Path
	IncDir       core.Path
	Srcs         []core.Path
	Ips          []core.Path
	Libs         []string
	LibDir       string
	Verbose      bool
}

type SimulationQuesta struct {
	Name    string
	Srcs    []core.Path
	Ips     []Ip
	Libs    []string
	Verbose bool
}

func (rule SimulationQuesta) Build(ctx core.Context) {
	outDir := ctx.Cwd().WithSuffix("/" + rule.Name)
	outScript := outDir.WithSuffix(".sh")
	outSimScript := outDir.WithSuffix(".questa.do")

	ins := []core.Path{}
	srcs := []core.Path{}
	src_pkgs := []core.Path{}
	ips := []core.Path{}

	srcs = append(srcs, rule.Srcs...)
	ins = append(ins, rule.Srcs...)
	for _, ip := range FlattenIpGraph(rule.Ips) {
		for _, src := range ip.Sources() {
			if strings.HasSuffix(src.String(), ".xci") {
				ips = append(ips, src)
			} else {
				if strings.HasSuffix(src.String(), "_pkg.sv") ||
					strings.HasSuffix(src.String(), "_pkg.v") {
					src_pkgs = append(src_pkgs, src)
				} else {
					srcs = append(srcs, src)
				}
			}
			ins = append(ins, src)
		}
	}

	srcs = append(src_pkgs, srcs...)

	data := QuestaSimScriptParams{
		PartName:     PartName.Value(),
		BoardName:    BoardName.Value(),
		Name:         strings.ToLower(rule.Name),
		OutDir:       outDir,
		OutScript:    outScript,
		OutSimScript: outSimScript,
		IncDir:       core.SourcePath(""),
		Srcs:         srcs,
		Ips:          ips,
		Libs:         rule.Libs,
		LibDir:       SimulatorLibDir.Value(),
		Verbose:      rule.Verbose,
	}

	ctx.AddBuildStep(core.BuildStep{
		Outs:   []core.OutPath{outDir, outScript, outSimScript},
		Ins:    ins,
		Script: core.CompileTemplateFile(hdl.QuestaSimScriptTmpl.String(), data),
		Descr:  fmt.Sprintf("Generating Questa simulation %s", outScript.Relative()),
	})
}
