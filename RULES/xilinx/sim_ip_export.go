package xilinx

import (
	"fmt"

	"dbt-rules/RULES/core"
	"dbt-rules/RULES/hdl"
)

type ExportScriptParams struct {
	Family    string
	Language  string
	Library   string
	Simulator string
	Output    string
}

var exportScript = `#!/bin/bash
set -eu -o pipefail

if [ {{ .Simulator }} != "questa"  ]; then
    echo "This target only supports questa. {{ .Simulator }} is not supported."
    exit 1
fi
` +
	"QUESTA=`which vsim`\n" +
	"SIMDIR=`dirname $QUESTA`\n" +
	`
mkdir -p "{{ .Output }}"
(
    cd {{ .Output }}
    cat > compile.tcl << EOF
compile_simlib -simulator {{ .Simulator }} -simulator_exec_path $SIMDIR -family {{ .Family }} -language {{ .Language }} -library {{ .Library }} -dir {{ .Output }}
EOF
    vivado -mode batch -nolog -nojournal -notrace -source compile.tcl
)
`

type ExportSimulatorIp struct {
	Family   string
	Language string
	Library  string
}

func (rule ExportSimulatorIp) Build(ctx core.Context) {
	simLibs := hdl.SimulatorLibDir.Value()
	if simLibs == "" {
		simLibs = ctx.Cwd().String()
	} else if simLibs[0] != '/' {
		core.Fatal("hdl-simulator-libs needs to contain an absolute path; current value: %s", simLibs)
	}

	family := rule.Family
	if family == "" {
		family = "all"
	}

	lang := rule.Language
	if lang == "" {
		lang = "all"
	}

	lib := rule.Library
	if lib == "" {
		lib = "all"
	}

	data := ExportScriptParams{
		Family:    family,
		Language:  lang,
		Library:   lib,
		Simulator: hdl.Simulator.Value(),
		Output:    simLibs,
	}

	ctx.AddBuildStep(core.BuildStep{
		Out:    ctx.Cwd().WithSuffix("/dummy"),
		Script: core.CompileTemplate(exportScript, "export-ip-script", data),
		Descr:  fmt.Sprintf("Exporting simulator IP for %s to %s", hdl.Simulator.Value(), simLibs),
	})
}
