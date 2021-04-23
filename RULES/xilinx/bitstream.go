package xilinx

import (
	"fmt"
	"strings"

	"dbt-rules/RULES/core"
	"dbt-rules/RULES/verilog"
)

type BuildFileScriptParams struct {
	Out     core.Path
	Part    string
	Name    string
	Timing  core.Path
	Ips     []core.Path
	Constrs []core.Path
	Rtls    []core.Path
}

var buildFileScript = `#!/bin/bash
set -eu -o pipefail

cat > {{ .Out }} <<EOF
set_part "{{ .Part }}"

{{ range .Ips }}
set path "{{ . }}"
set normalized [file normalize [string range \$path 1 [string length \$path]]]
set dir [file join [pwd] [file dirname \$normalized]]
set filename [file tail \$normalized]
puts \$dir
file mkdir \$dir
file copy "{{ . }}" \$dir
read_ip [file join \$dir \$filename]
{{ end }}

{{ range .Rtls }}
read_verilog "{{ . }}"
{{ end }}

{{ range .Constrs }}
read_xdc "{{ . }}"
{{ end }}

synth_ip [get_ips -all]

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
}

var runSynthesisScript = `#!/bin/bash
set -eu -o pipefail

TMPDIR=$(mktemp -d -t ci-XXXXXXXXXX)
(
    cd $TMPDIR
    vivado -mode batch -nolog -nojournal  -notrace -source {{ .BuildScript }} | ( grep -E "^(ERROR|WARNING|CRITICAL)" || true )
    echo "all: { bitstream.bit }" > bitstream.bif
    bootgen -image bitstream.bif -arch zynqmp -process_bitstream bin -w | ( grep -E "^\[ERROR\]" || true )
    cp bitstream.bit.bin {{ .Bitstream }}
)

rm -rf ${TMPDIR}
`

type Bitstream struct {
	Name string
	Src  core.Path
	Ips  []verilog.Ip
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

	bfData := BuildFileScriptParams{
		Out:     outBf,
		Name:    rule.Name,
		Part:    PartName.Value(),
		Timing:  outTiming,
		Ips:     ips,
		Rtls:    rtls,
		Constrs: constrs,
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
	}

	outs := []core.OutPath{out, outTiming}
	ctx.AddBuildStep(core.BuildStep{
		Outs:   outs,
		In:     outBf,
		Script: core.CompileTemplate(runSynthesisScript, "run-synthesis-script", rsData),
		Descr:  fmt.Sprintf("Generating bitstream %s", out.Relative()),
	})
}
