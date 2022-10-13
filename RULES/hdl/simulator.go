package hdl

import (
	"dbt-rules/RULES/core"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"reflect"
	"strings"
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

// FindTestCases enables parsing of source files to discover testcases
var FindTestCases = core.BoolFlag{
	Name: "hdl-find-testcases",
	DefaultFn: func() bool {
		return false
	},
	Description: "Enable parsing of HDL files to discover testcases",
}.Register()

// ShowTestCasesFile enables outputting of the source file where testcases were found
var ShowTestCasesFile = core.BoolFlag{
	Name: "hdl-show-testcases-file",
	DefaultFn: func() bool {
		return false
	},
	Description: "Enable output of the file where testcases were found",
}.Register()

// DumpVcd enables outputting of all signals in the design to a VCD file
var DumpVcd = core.BoolFlag{
	Name: "hdl-dump-vcd",
	DefaultFn: func() bool {
		return false
	},
	Description: "Enable output of signals to a VCD file",
}.Register()

type ParamMap map[string]map[string]string
type DefineMap map[string]string

type Simulation struct {
	Name                   string
	Srcs                   []core.Path
	Ips                    []Ip
	Libs                   []string
	Params                 ParamMap
	Defines                DefineMap
	ToolFlags              FlagMap
	Top                    string
	Tops                   []string
	Dut                    string
	TestCaseGenerator      core.Path
	TestCaseGeneratorFlags string
	TestCaseElf            core.Path
	TestCasesDir           core.Path
	WaveformInit           core.Path
	ReportCovIps           []Ip
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
	if FindTestCases.Value() {
		re := regexp.MustCompile(`\s*` + "`" + `TEST_CASE\s*\(\s*"([^"]+)"\s*\)`)
		re_file := regexp.MustCompile(`([^\/]+)\/DEPS\/([^\/]+)`)

		for _, src := range rule.Srcs {
			// Collect testcases
			testcases := []string{}
			// Read the source file
			b, err := ioutil.ReadFile(src.Absolute())
			if err == nil {
				match := re.FindAllSubmatch(b, -1)
				for _, submatch := range match {
					testcases = append(testcases, string(submatch[1]))
				}
			} else {
				log.Fatal(err)
			}

			// Append to description
			if len(testcases) > 0 {
				if ShowTestCasesFile.Value() {
					name := src.Absolute()
					match := re_file.FindStringSubmatch(name)
					if (len(match) > 0) && (match[1] == match[2]) {
						name = re_file.ReplaceAllString(name, match[1])
					}
					description = description + " " + name
				}
				description = description + " +testcases=" + strings.Join(testcases, ",")
			}
		}
	}

	if len(description) > 0 {
		description = description + " "
	}

	return description
}

func (rule Simulation) ReportCovFiles() []string {
	files := []string{}

	for _, ip := range rule.ReportCovIps {
		for _, src := range ip.Sources() {
			files = append(files, src.String())
		}
	}

	// Remove duplicates
	set := make(map[string]bool)
	for _, file := range files {
		set[file] = true
	}

	// Convert back to string list
	keys := reflect.ValueOf(set).MapKeys()
	str_keys := make([]string, len(keys))
	for i := 0; i < len(keys); i++ {
		str_keys[i] = keys[i].String()
	}

	return str_keys
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
			testCaseGeneratorFlags := rule.TestCaseGeneratorFlags
			// Check format of the testcase file
			if strings.HasSuffix(testcase, ".json") {
				testCaseGeneratorFlags += " -testcase"
			} else if strings.HasSuffix(testcase, ".onnx") {
				testCaseGeneratorFlags += " -onnx"
			}

			// Create the preamble for testcase generator with arguments
			preamble = fmt.Sprintf("{ %s %s %s -out %s ; }", rule.TestCaseGenerator.String(), testCaseGeneratorFlags, testcase, rule.TestCaseElf.String())
		}

		// Add information to command
		preamble = fmt.Sprintf("{ echo Generating %s; } && ", testcase) + preamble

		// Trim testcase for use in coverage databaes
		testcase = strings.TrimSuffix(path.Base(testcase), path.Ext(testcase))
	}

	return preamble, testcase
}
