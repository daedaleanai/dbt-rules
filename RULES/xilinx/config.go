package xilinx

import (
	"dbt-rules/RULES/core"
)

var BoardName = core.StringFlag{
	Name: "board",
	DefaultFn: func() string {
		return "em.avnet.com:ultra96v2:part0:1.0"
	},
}.Register()

var PartName = core.StringFlag{
	Name: "part",
	DefaultFn: func() string {
		return "xczu3eg-sbva484-1-e"
	},
}.Register()
