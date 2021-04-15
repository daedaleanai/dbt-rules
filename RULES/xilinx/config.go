package xilinx

import (
	"dbt-rules/RULES/core"
)

var _ = core.Flag("board")
var _ = core.Flag("part")

func BoardName() string {
	board := core.Flag("board")
	if board != "" {
		return board
	}
	return "em.avnet.com:ultra96v2:part0:1.0"
}

func PartName() string {
	board := core.Flag("part")
	if board != "" {
		return board
	}
	return "xczu3eg-sbva484-1-e"
}
