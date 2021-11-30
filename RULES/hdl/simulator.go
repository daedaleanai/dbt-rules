package hdl

import (
	"log"
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
	Params  []string
	Top     string
}

func (rule Simulation) Build(ctx core.Context) {
	switch Simulator.Value() {
	case "xsim":
		SimulationXsim{
			Name:    rule.Name,
			Srcs:    rule.Srcs,
			Ips:     rule.Ips,
			Libs:    rule.Libs,
			Verbose: false,
		}.Build(ctx)
	case "questa":
		SimulationQuesta{
			Name:    rule.Name,
			Srcs:    rule.Srcs,
			Ips:     rule.Ips,
			Libs:    rule.Libs,
			Params:  rule.Params,
			Top:     rule.Top,
		}.Build(ctx)
	default:
		log.Fatal("invalid value '%s' for hdl-simulator flag", Simulator.Value())
	}
}

func (rule Simulation) Run(args []string) string {
	res := ""

	switch Simulator.Value() {
	case "questa":
		res = SimulationQuesta{
			Name:    rule.Name,
			Srcs:    rule.Srcs,
			Ips:     rule.Ips,
			Libs:    rule.Libs,
			Params:  rule.Params,
			Top:     rule.Top,
		}.Run(args)
	default:
		log.Fatal("'run' target not supported for hdl-simulator flag '%s'", Simulator.Value())
	}

	return res
}

func (rule Simulation) Test(args []string) string {
	res := ""
	switch Simulator.Value() {
	case "questa":
		res = SimulationQuesta{
			Name:    rule.Name,
			Srcs:    rule.Srcs,
			Ips:     rule.Ips,
			Libs:    rule.Libs,
			Params:  rule.Params,
			Top:     rule.Top,
		}.Test(args)
	default:
		log.Fatal("'test' target not supported for hdl-simulator flag '%s'", Simulator.Value())
	}

	return res
}