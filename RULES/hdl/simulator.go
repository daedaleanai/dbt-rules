package hdl

import (
	"log"
	"fmt"
	"os"
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

var SimulatorParams = core.StringFlag{
	Name:        "hdl-simulator-params",
	Description: "Name of the parameter set to use",
	DefaultFn: func() string {
		return "default"
	},
}.Register()

type ParamMap map[string]map[string]string

type Simulation struct {
	Name              string
	Srcs              []core.Path
	Ips               []Ip
	Libs              []string
	Params            ParamMap
	Top               string
	Dut               string
	TestCaseGenerator core.Path
	TestCasesDir      core.Path
	WaveformInit      core.Path
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
		BuildQuesta(ctx, rule)
	default:
		log.Fatal(fmt.Sprintf("invalid value '%s' for hdl-simulator flag", Simulator.Value()))
	}
}

func (rule Simulation) Run(args []string) string {
	res := ""

	switch Simulator.Value() {
	case "questa":
		res = RunQuesta(rule, args)
	default:
		log.Fatal(fmt.Sprintf("'run' target not supported for hdl-simulator flag '%s'", Simulator.Value()))
	}

	return res
}

func (rule Simulation) Test(args []string) string {
	res := ""
	switch Simulator.Value() {
	case "questa":
		res = TestQuesta(rule, args)
	default:
		log.Fatal(fmt.Sprintf("'test' target not supported for hdl-simulator flag '%s'", Simulator.Value()))
	}

	return res
}

func (rule Simulation) Description() string {
	description := " Name: " + rule.Name + " "
	first := true
	for param, _ := range(rule.Params) {
		if first {
			description = description + "Params: "
			first = false
		}
		description = description + param + " "
	}

	if rule.TestCaseGenerator != nil && rule.TestCasesDir != nil {
		description = description + "TestCases: "
		// Loop through all defined testcases in directory
		items, err := os.ReadDir(rule.TestCasesDir.String())
		if err != nil {
			log.Fatal(err)
		}

    for _, item := range items {
			// Handle one level of subdirectories
			if item.IsDir() {
				subitems, err := os.ReadDir(item.Name())
				if err != nil {
					log.Fatal(err)
				}
				
				for _, subitem := range subitems {
						if !subitem.IsDir() {
								// handle file there
								description = description + item.Name() + "/" + subitem.Name() + " "
						}
				}
			} else {
        // handle file there
        description = description + item.Name() + " "
      }
    }
	}

	return description
}