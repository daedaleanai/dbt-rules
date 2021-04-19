package xilinx

import (
	"fmt"

	"dbt-rules/RULES/core"
)

type AtfScriptParams struct {
	Bl31 core.Path
	Repo core.Path
}

var atfScript = `#!/bin/bash
set -eu -o pipefail

TMPDIR=$(mktemp -d -t ci-XXXXXXXXXX)
rsync --exclude=.git -az {{ .Repo }} ${TMPDIR}
(
    cd ${TMPDIR}/arm-trusted-firmware
    make CROSS_COMPILE=aarch64-none-elf- PLAT=zynqmp bl31 -j12 > /dev/null
    cp build/zynqmp/release/bl31/bl31.elf "{{ .Bl31 }}"
)

rm -rf ${TMPDIR}
`

type ArmTrustedFirmware struct {
	Bl31 core.OutPath
}

func (rule ArmTrustedFirmware) Build(ctx core.Context) {
	data := AtfScriptParams{
		Bl31: rule.Bl31,
		Repo: ctx.SourcePath("arm-trusted-firmware"),
	}
	ctx.AddBuildStep(core.BuildStep{
		Out:    rule.Bl31,
		In:     ctx.SourcePath("arm-trusted-firmware"),
		Script: core.CompileTemplate(atfScript, "atf-script", data),
		Descr:  fmt.Sprintf("Building Xilinx Arm Trusted Firmware"),
	})
}
