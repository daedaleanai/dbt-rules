package verilog

import (
	"dbt-rules/RULES/core"
)

type Ip interface {
	Sources() []core.Path
	Data() []core.Path
}

type Library struct {
	Srcs      []core.Path
	DataFiles []core.Path
}

func (lib Library) Sources() []core.Path {
	return lib.Srcs
}

func (lib Library) Data() []core.Path {
	return lib.DataFiles
}
