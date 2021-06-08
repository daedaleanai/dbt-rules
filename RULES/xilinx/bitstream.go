package xilinx

import (
	"fmt"
	"strings"

	"dbt-rules/RULES/core"
	"dbt-rules/RULES/hdl"
)

type BuildFileScriptParams struct {
	Out       core.Path
	PartName  string
	BoardName string
	Name      string
	Timing    core.Path
	Ips       []core.Path
	Constrs   []core.Path
	Rtls      []core.Path
}

var buildFileScript = `#!/bin/bash
set -eu -o pipefail

cat > {{ .Out }} <<EOF
set_part "{{ .PartName }}"
set_property "board_part"         "{{ .BoardName}}"       [current_project]
set_property "target_language"    "Verilog"        [current_project]

{{ range .Ips }}
set path "{{ . }}"
set normalized [file normalize [string range \$path 1 [string length \$path]]]
set dir [file join [pwd] [file dirname \$normalized]]
set filename [file tail \$normalized]
file mkdir \$dir
file copy "{{ . }}" \$dir
set ip [file join \$dir \$filename]
read_ip \$ip
generate_target all [get_files \$ip]
set_property GENERATE_SYNTH_CHECKPOINT true [get_files \$ip]
synth_ip [get_files \$ip]
{{ end }}

report_ip_status

{{ range .Rtls }}
read_verilog "{{ . }}"
{{ end }}

{{ range .Constrs }}
read_xdc "{{ . }}"
{{ end }}

synth_design -top {{ .Name }}
opt_design
place_design
phys_opt_design
route_design
report_timing_summary -file {{ .Timing }}
write_bitstream -force bitstream.bit
EOF
`

type RunSynthesisScriptParams struct {
	BuildScript core.Path
	Bitstream   core.Path
	Verbose     bool
	Postprocess string
}

var runSynthesisScript = `#!/bin/bash
set -eu -o pipefail

TMPDIR=$(mktemp -d -t ci-XXXXXXXXXX)
(
    cd $TMPDIR
    {{ if .Verbose }}
    vivado -mode batch -nolog -nojournal  -notrace -source {{ .BuildScript }}
    {{ else }}
    vivado -mode batch -nolog -nojournal  -notrace -source {{ .BuildScript }} | ( grep -E "^(ERROR|WARNING|CRITICAL)" || true )
    {{ end }}
    echo "all: { bitstream.bit }" > bitstream.bif
    {{ if ne .Postprocess "" }}
    {{ if .Verbose }}
    bootgen -image bitstream.bif -arch zynqmp -process_bitstream {{ .Postprocess }} -w
    {{ else }}
    bootgen -image bitstream.bif -arch zynqmp -process_bitstream {{ .Postprocess }} -w | ( grep -E "^\[ERROR\]" || true )
    {{ end }}
    cp bitstream.bit.bin {{ .Bitstream }}
    {{ else }}
    cp bitstream.bit {{ .Bitstream }}
    {{ end }}
)

rm -rf ${TMPDIR}
`

type Bitstream struct {
	Name        string
	Src         core.Path
	Constraints core.Path
	Ips         []hdl.Ip
	Postprocess string
	Verbose     bool
}

func (rule Bitstream) Build(ctx core.Context) {
	ips := []core.Path{}
	rtls := []core.Path{}
	constrs := []core.Path{}

	ins := []core.Path{}
	for _, ip := range rule.Ips {
		for _, src := range ip.Sources() {
			if strings.HasSuffix(src.String(), ".v") || strings.HasSuffix(src.String(), ".sv") {
				rtls = append(rtls, src)
			} else if strings.HasSuffix(src.String(), ".xdc") {
				constrs = append(constrs, src)
			} else if strings.HasSuffix(src.String(), ".xci") {
				ips = append(ips, src)
			}
			ins = append(ins, src)

		}
	}

	out := rule.Src.WithExt("bit")
	outTiming := rule.Src.WithExt("rpt")
	outBf := rule.Src.WithExt("tcl")

	ins = append(ins, rule.Src)
	rtls = append(rtls, rule.Src)
	if rule.Constraints != nil {
		ins = append(ins, rule.Constraints)
		constrs = append(constrs, rule.Constraints)
	}

	bfData := BuildFileScriptParams{
		Out:       outBf,
		Name:      rule.Name,
		PartName:  hdl.PartName.Value(),
		BoardName: hdl.BoardName.Value(),
		Timing:    outTiming,
		Ips:       ips,
		Rtls:      rtls,
		Constrs:   constrs,
	}

	ctx.AddBuildStep(core.BuildStep{
		Out:    outBf,
		Ins:    ins,
		Script: core.CompileTemplate(buildFileScript, "build-file-script", bfData),
		Descr:  fmt.Sprintf("Generating a bitstream build file %s", outBf.Relative()),
	})

	rsData := RunSynthesisScriptParams{
		BuildScript: outBf,
		Bitstream:   out,
		Verbose:     rule.Verbose,
		Postprocess: rule.Postprocess,
	}

	outs := []core.OutPath{out, outTiming}
	ctx.AddBuildStep(core.BuildStep{
		Outs:   outs,
		In:     outBf,
		Script: core.CompileTemplate(runSynthesisScript, "run-synthesis-script", rsData),
		Descr:  fmt.Sprintf("Generating bitstream %s", out.Relative()),
	})
}
