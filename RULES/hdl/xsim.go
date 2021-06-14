package hdl

import (
	"fmt"
	"strings"

	"dbt-rules/RULES/core"
	"dbt-rules/hdl"
)

type XSimScriptParams struct {
	Name         string
	PartName     string
	BoardName    string
	OutDir       core.Path
	OutScript    core.Path
	OutSimScript core.Path
	Srcs         []core.Path
	Ips          []core.Path
	Libs         []string
	Verbose      bool
}

type SimulationXsim struct {
	Name    string
	Srcs    []core.Path
	Ips     []Ip
	Libs    []string
	Verbose bool
}

func (rule SimulationXsim) Build(ctx core.Context) {
	outDir := ctx.Cwd().WithSuffix("/" + rule.Name)
	outScript := outDir.WithSuffix(".sh")
	outSimScript := outDir.WithSuffix(".xsim.tcl")

	ins := []core.Path{}
	srcs := []core.Path{}
	ips := []core.Path{}

	srcs = append(srcs, rule.Srcs...)
	ins = append(ins, rule.Srcs...)
	for _, ip := range rule.Ips {
		for _, src := range ip.Sources() {
			if strings.HasSuffix(src.String(), ".xci") {
				ips = append(ips, src)
			} else {
				srcs = append(srcs, src)
			}
			ins = append(ins, src)
		}
	}

	data := XSimScriptParams{
		PartName:     PartName.Value(),
		BoardName:    BoardName.Value(),
		Name:         strings.ToLower(rule.Name),
		OutDir:       outDir,
		OutScript:    outScript,
		OutSimScript: outSimScript,
		Srcs:         srcs,
		Ips:          ips,
		Libs:         rule.Libs,
		Verbose:      rule.Verbose,
	}

	ctx.AddBuildStep(core.BuildStep{
		Outs:   []core.OutPath{outDir, outScript, outSimScript},
		Ins:    ins,
		Script: core.CompileTemplateFile(hdl.XSimScriptTmpl.String(), data),
		Descr:  fmt.Sprintf("Generating XSim simulation %s", outScript.Relative()),
	})
}
