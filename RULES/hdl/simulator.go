package hdl

import (
  "dbt-rules/RULES/core"
  "fmt"
  "log"
  "os"
)

var Simulator = core.StringFlag{
  Name:        "hdl-simulator",
  Description: "HDL simulator to use when generating simulation targets",
  DefaultFn: func() string {
    return "questa"
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
  // Print the rule name as its needed for parameter selection
  description := " "
  first := true
  for param, _ := range rule.Params {
    if first {
      description = description + "Params: "
      first = false
    }
    description = description + param + " "
  }

  if rule.TestCaseGenerator != nil && rule.TestCasesDir != nil {
    description = description + "TestCases: "

    // Loop through all defined testcases in directory
    if items, err := os.ReadDir(rule.TestCasesDir.String()); err == nil {
      for _, item := range items {
        description = description + item.Name() + " "
      }
    } else {
      log.Fatal(err)
    }
  }

  return description
}
