package hdl

import (
	"fmt"

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
	ips := []core.Path{}

	for _, ip := range FlattenIpGraph(rule.Ips) {
		for _, src := range ip.Sources() {
			if IsSimulationArchive(src.String()) {
				ips = append(ips, src)
			} else if IsRtl(src.String()) {
				srcs = append(srcs, src)
			}
			ins = append(ins, src)
		}
	}
	srcs = append(srcs, rule.Srcs...)
	ins = append(ins, rule.Srcs...)

	data := QuestaSimScriptParams{
		PartName:     PartName.Value(),
		BoardName:    BoardName.Value(),
		Name:         rule.Name,
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
