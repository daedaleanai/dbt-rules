package hdl

import (
	"dbt-rules/RULES/core"
	"fmt"
	"log"
	"os"
	"errors"
	"strings"
	"path"
	"regexp"
	"io/ioutil"
)

var Simulator = core.StringFlag{
	Name:        "hdl-simulator",
	Description: "Select HDL simulator",
	DefaultFn: func() string {
		return "questa"
	},
	AllowedValues: []string{"xsim", "questa"},
}.Register()

var SimulatorLibDir = core.StringFlag{
	Name:        "hdl-simulator-lib-dir",
	Description: "Path to the HDL simulator libraries",
	DefaultFn: func() string {
		return ""
	},
}.Register()

// FindTestcases enables parsing of source files to discover testcases
var FindTestcases = core.BoolFlag{
	Name: "hdl-find-testcases",
	DefaultFn: func() bool {
		return false
	},
	Description: "Enable parsing of HDL files to discover testcases",
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

// Lib returns the standard library name defined for this rule.
func (rule Simulation) Lib() string {
	return rule.Name + "_lib"
}

// Path returns the default root path for log files defined for this rule.
func (rule Simulation) Path() core.Path {
	return core.BuildPath("/" + rule.Name)
}

func (rule Simulation) Build(ctx core.Context) {
	switch Simulator.Value() {
	case "xsim":
		BuildXsim(ctx, rule)
	case "questa":
		BuildQuesta(ctx, rule)
	default:
		log.Fatal(fmt.Sprintf("invalid value '%s' for hdl-simulator flag", Simulator.Value()))
	}
}

func (rule Simulation) Run(args []string) string {
	res := ""

	switch Simulator.Value() {
	case "xsim":
		res = RunXsim(rule, args)
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
	case "xsim":
		res = TestXsim(rule, args)
	case "questa":
		res = TestQuesta(rule, args)
	default:
		log.Fatal(fmt.Sprintf("'test' target not supported for hdl-simulator flag '%s'", Simulator.Value()))
	}

	return res
}

func (rule Simulation) Description() string {
	// Print the rule name as its needed for parameter selection
	description := ""
	first := true
	for param, _ := range rule.Params {
		if first {
			description = description + " -params=" + param
			first = false
		} else {
			description = description + "," + param
		}
	}

	// Collect testcases in case a test generator is used with a directory of test cases
	if rule.TestCaseGenerator != nil && rule.TestCasesDir != nil {
		// Collect testcases
		testcases := []string{}

		// Loop through all defined testcases in directory
		if items, err := os.ReadDir(rule.TestCasesDir.String()); err == nil {
			for _, item := range items {
				testcases = append(testcases, item.Name())
			}
		} else {
			log.Fatal(err)
		}

		// Append to description
		if len(testcases) > 0 {
			description = description + " -testcases=" + strings.Join(testcases, ",")
		}
	}

	// Scan source files for testcases
	if FindTestcases.Value() {
		// Collect testcases
		testcases := []string{}

		for _, src := range(rule.Srcs) {
			b, err := ioutil.ReadFile(src.Absolute())
			if err == nil {
				re1, err := regexp.Compile(`\s*` + "`" + `TEST_CASE\s*\(\s*"([^"]+)"\s*\)`)
				if err == nil {
					match := re1.FindAllSubmatch(b, -1)
					for _, submatch := range match {
						testcases = append(testcases, string(submatch[1]))
					}
				}
			} else {
				log.Fatal(err)
			}
		}

		// Append to description
		if len(testcases) > 0 {
			description = description + " +testcases=" + strings.Join(testcases, ",")
		}
	}

	if len(description) > 0 {
		description = description + " "
	}

	return description
}

// Preamble creates a preamble for the simulation command for the purpose of generating
// a testcase.
func Preamble(rule Simulation, testcase string) (string, string) {
	preamble := ""

	// Create a testcase generation command if necessary
	if rule.TestCaseGenerator != nil {
		if testcase == "" && rule.TestCasesDir != nil {
			// No testcase specified, pick the first one from the directory
			if items, err := os.ReadDir(rule.TestCasesDir.String()); err == nil {
				if len(items) == 0 {
					log.Fatal(fmt.Sprintf("TestCasesDir directory '%s' empty!", rule.TestCasesDir.String()))
				}

				// Create path to testcase
				testcase = rule.TestCasesDir.Absolute() + "/" + items[0].Name()
			}
		} else if testcase != "" && rule.TestCasesDir != nil {
			// Testcase specified, create path to testcase
			testcase = rule.TestCasesDir.Absolute() + "/" + testcase
		}

		if testcase == "" {
			// Create the preamble for testcase generator without any argument
			preamble = fmt.Sprintf("{ %s . ; }", rule.TestCaseGenerator.String())
			testcase = "default"
		} else {
			// Check that the testcase exists
			if _, err := os.Stat(testcase); errors.Is(err, os.ErrNotExist) {
				log.Fatal(fmt.Sprintf("Testcase '%s' does not exist!", testcase))
			}
			
			// Create the preamble for testcase generator with arguments
			preamble = fmt.Sprintf("{ %s %s . ; }", rule.TestCaseGenerator.String(), testcase)
		}

		// Add information to command
		preamble = fmt.Sprintf("{ echo Generating %s; } && ", testcase) + preamble

		// Trim testcase for use in coverage databaes
		testcase = strings.TrimSuffix(path.Base(testcase), path.Ext(testcase))
	}

	return preamble, testcase
}
