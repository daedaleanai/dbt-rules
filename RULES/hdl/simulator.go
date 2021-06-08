package hdl

import (
	"dbt-rules/RULES/core"
)

var Simulator = core.StringFlag{
	Name:        "hdl-simulator",
	Description: "HDL simulator to use when generating simulation targets",
	DefaultFn: func() string {
		return "xsim"
	},
	AllowedValues: []string{"xsim", "questa"},
}.Register()

var SimulatorLibDir = core.StringFlag{
	Name:        "hdl-simulator-lib-dir",
	Description: "Path to the HDL Simulator libraries",
	DefaultFn: func() string {
		return ""
	},
}.Register()

type Simulation struct {
	Name    string
	Srcs    []core.Path
	Ips     []Ip
	Libs    []string
	Verbose bool
}

func (rule Simulation) Build(ctx core.Context) {
	if Simulator.Value() == "xsim" {
		SimulationXsim{
			Name:    rule.Name,
			Srcs:    rule.Srcs,
			Ips:     rule.Ips,
			Libs:    rule.Libs,
			Verbose: rule.Verbose,
		}.Build(ctx)
	} else {
		SimulationQuesta{
			Name:    rule.Name,
			Srcs:    rule.Srcs,
			Ips:     rule.Ips,
			Libs:    rule.Libs,
			Verbose: rule.Verbose,
		}.Build(ctx)
	}
}
