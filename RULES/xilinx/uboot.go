package xilinx

import (
	"fmt"
	"regexp"

	"dbt-rules/RULES/core"
)

type UBootScriptParams struct {
	Out    core.OutPath
	Repo   core.Path
	Config string
}

var uBootScript = `#!/bin/bash
set -eu -o pipefail

TMPDIR=$(mktemp -d -t ci-XXXXXXXXXX)
rsync --exclude=.git -az {{ .Repo }} ${TMPDIR}
(
    cd ${TMPDIR}/u-boot
    make ARCH=arm CROSS_COMPILE=aarch64-none-elf- {{ .Config }} > /dev/null
    make ARCH=arm CROSS_COMPILE=aarch64-none-elf- -j12 > /dev/null
    cp u-boot.elf "{{ .Out }}"
)

rm -rf ${TMPDIR}
`

type UBoot struct {
	Out     core.OutPath
	Configs map[string]string
}

func (rule UBoot) Build(ctx core.Context) {
	var config string
	board := BoardName()
	for pattern, cfg := range rule.Configs {
		matched, err := regexp.MatchString(pattern, board)
		if err != nil {
			core.Fatal("UBoot config: %s", err)
		}
		if matched {
			config = cfg
		}
	}

	if config == "" {
		core.Fatal("Unable to determine U-Boot config for board: %s", BoardName())
	}

	data := UBootScriptParams{
		Out:    rule.Out,
		Repo:   ctx.SourcePath("u-boot"),
		Config: config,
	}

	ctx.AddBuildStep(core.BuildStep{
		Out:    rule.Out,
		In:     ctx.SourcePath("u-boot"),
		Script: core.CompileTemplate(uBootScript, "uboot-script", data),
		Descr:  fmt.Sprintf("Building U-Boot for board %s", BoardName()),
	})
}
