package xilinx

import (
	"fmt"
	"sort"
	"strings"

	"dbt-rules/RULES/core"
)

type IpScriptParams struct {
	PartName   string
	BoardName  string
	Design     core.Path
	Out        map[string]core.OutPath
	OutOrder   []string
	BoardFiles []core.Path
}

var ipScript = `#!/bin/bash
set -eu -o pipefail

TMPDIR=$(mktemp -d -t ci-XXXXXXXXXX)

(
    cd ${TMPDIR}
    cat >> generate.tcl <<EOF
{{range .BoardFiles}}
set_param board.repoPaths [lappend board.repoPaths "{{ . }}"]
{{end}}

create_project -in memory -force test
set_msg_config -severity INFO -suppress

set_property "part"               "{{ .PartName}}"        [current_project]
set_property "board_part"         "{{ .BoardName}}"       [current_project]
set_property "default_lib"        "xil_defaultlib" [current_project]
set_property "simulator_language" "Mixed"          [current_project]
set_property "target_language"    "Verilog"        [current_project]

source "{{ .Design }}"
EOF

    vivado -mode batch -nolog -nojournal  -notrace -source generate.tcl | ( grep -E "^(ERROR|WARNING|CRITICAL)" || true )
    {{ $out := .Out }}
    {{ range .OutOrder }}
    cp {{ . }} {{ index $out  . }}
    {{ end }}
)

rm -rf ${TMPDIR}
`

type Ip struct {
	Out        map[string]core.OutPath
	Design     core.Path
	BoardFiles []core.Path
}

func (rule Ip) Build(ctx core.Context) {
	out := []core.OutPath{}
	exports := []string{}
	for k, _ := range rule.Out {
		exports = append(exports, k)
	}
	sort.Strings(exports)
	for _, k := range exports {
		out = append(out, rule.Out[k])
	}

	data := IpScriptParams{
		PartName:   PartName.Value(),
		BoardName:  BoardName.Value(),
		Design:     rule.Design,
		BoardFiles: rule.BoardFiles,
		Out:        rule.Out,
		OutOrder:   exports,
	}

	ctx.AddBuildStep(core.BuildStep{
		Outs:   out,
		In:     rule.Design,
		Script: core.CompileTemplate(ipScript, "ip-script", data),
		Descr:  fmt.Sprintf("Generating IP from %s", rule.Design.Relative()),
	})
}

func (rule Ip) Sources() []core.Path {
	exports := []string{}
	for k, _ := range rule.Out {
		exports = append(exports, k)
	}
	sort.Strings(exports)

	srcs := []core.Path{}
	for _, v := range exports {
		path := rule.Out[v]
		if !strings.HasSuffix(path.String(), ".hwdef") {
			srcs = append(srcs, path)
		}
	}
	return srcs
}

func (rule Ip) Data() []core.Path {
	exports := []string{}
	for k, _ := range rule.Out {
		exports = append(exports, k)
	}
	sort.Strings(exports)

	others := []core.Path{}
	for _, v := range exports {
		path := rule.Out[v]
		if strings.HasSuffix(path.String(), ".hwdef") {
			others = append(others, path)
		}
	}
	return others
}
