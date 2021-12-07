package hdl

import (
	"fmt"
	"path"
	"strings"
	"log"
	"os"

	"dbt-rules/RULES/core"
)

// VlogFlags enables the user to specify additional flags for the 'vlog' command.
var VlogFlags = core.StringFlag{
	Name: "questa-vlog-flags",
	DefaultFn: func() string {
		return ""
	},
}.Register()

// VcomFlags enables the user to specify additional flags for the 'vcom' command.
var VcomFlags = core.StringFlag{
	Name: "questa-vcom-flags",
	DefaultFn: func() string {
		return ""
	},
}.Register()

// Access enables the user to control the accessibility in the compiled design for
// debugging purposes.
var Access = core.StringFlag{
	Name: "questa-access",
	DefaultFn: func() string {
		return "rna"
	},
}.Register()

// Coverage enables the user to run the simulation with code coverage.
var Coverage = core.BoolFlag{
	Name: "questa-coverage",
	DefaultFn: func() bool {
		return false
	},
}.Register()

// Lib returns the standard Questa library name defined for this rule.
func (rule Simulation) Lib() string {
	return rule.Name + "Lib"
}

// Target returns the optimization target name defined for this rule.
func (rule Simulation) Target() string {
	if Coverage.Value() {
		return rule.Name + "Cov"
	} else {
		return rule.Name
	}
}

// Path returns the default root path for log files defined for this rule.
func (rule Simulation) Path() core.Path {
	return core.BuildPath("/" + rule.Name)
}

// Instance returns the instance name of the rule based on the Top and the DUT
// fields.
func (rule Simulation) Instance() string {
	// Defaults
	top := "board"
	dut := "u_dut"

	// Pick top from rule
	if rule.Top != "" {
		top = rule.Top
	} 

	// Pick DUT from rule
	if rule.Dut != "" {
		dut = rule.Dut
	} 

	return "/" + top + "/" + dut
}

// rules holds a map of all defined rules to prevent defining the same rule
// multiple times.
var rules = make(map[string]bool)

// common_flags holds common flags used for the 'vlog', 'vcom', 'vopt' and 'vsim' commands.
const common_flags = "-nologo -quiet"

// CompileSrcs compiles a list of sources using the specified context ctx, rule,
// dependencies and include paths. It returns the resulting dependencies and include paths
// that result from compiling the source files.
func CompileSrcs(ctx core.Context, rule Simulation, 
	               deps []core.Path, incs []core.Path, srcs []core.Path) ([]core.Path, []core.Path) {
	for _, src := range srcs {
		// We handle header files separately from other source files
		if IsHeader(src.String()) {
			incs = append(incs, src)
		} else if IsRtl(src.String()) {
			// log will point to the log file to be generated when compiling the code
			log := rule.Path().WithSuffix("/" + src.Relative() + ".log")

			// If we already have a rule for this file, skip it.
			if rules[log.String()] {
				continue
			}

			// Holds common flags for both 'vlog' and 'vcom' commands
			cmd := fmt.Sprintf("%s +acc=%s -work %s -l %s", common_flags, Access.Value(), rule.Lib(), log.String())
			
			// tool will point to the tool to execute (also used for logging below)
			var tool string
			if IsVerilog(src.String()) {
				tool = "vlog"
				cmd = cmd + " " + VlogFlags.Value()
				cmd = cmd + " -suppress 2583 -svinputport=net"
				cmd = cmd + fmt.Sprintf(" +incdir+%s", core.SourcePath("").String())
				for _, inc := range incs {
					cmd = cmd + fmt.Sprintf(" +incdir+%s", path.Dir(inc.Absolute()))
				}
			} else if IsVhdl(src.String()) {
				tool = "vcom"
			}
			
			// Remove the log file if the command fails to ensure we can recompile it
			cmd = tool + " " + cmd + " " + src.String() + " || rm " + log.String()
			
			// Add the compilation command as a build step with the log file as the
			// generated output
			ctx.AddBuildStep(core.BuildStep{
				Out:   log,
				Ins:   append(deps, src),
				Cmd:   cmd,
				Descr: fmt.Sprintf("%s: %s", tool, src.Relative()),
			})
			
			// Add the log file to the dependencies of the next files
			deps = append(deps, log)

			// Note down the created rule
			rules[log.String()] = true
		}	
	}

	return deps, incs
}

// CompileIp compiles the IP dependencies and the source files and an IP.
func CompileIp(ctx core.Context, rule Simulation, ip Ip, 
	             deps []core.Path, incs []core.Path) ([]core.Path, []core.Path) {
	for _, sub_ip := range ip.Ips() {
		deps, incs = CompileIp(ctx, rule, sub_ip, deps, incs)
	}
	deps, incs = CompileSrcs(ctx, rule, deps, incs, ip.Sources())

	return deps, incs
}

// Compile compiles the IP dependencies and source files of a simulation rule.
func Compile(ctx core.Context, rule Simulation) []core.Path {
	incs := []core.Path{}
	deps := []core.Path{}

	for _, ip := range rule.Ips {
		deps, incs = CompileIp(ctx, rule, ip, deps, incs)
	}
	CompileSrcs(ctx, rule, deps, incs, rule.Srcs)

	return deps
}

// Optimize creates and optimized version of the design optionally including
// coverage recording functionality. The optimized design unit can then conveniently
// be simulated using 'vsim'.
func Optimize(ctx core.Context, rule Simulation, deps []core.Path) {
	top := "board"
	if rule.Top != "" {
		top = rule.Top
	}
	
	cover_flag := ""
	log_file_suffix := "/vopt.log"
	if Coverage.Value() {
		cover_flag = "+cover"
		log_file_suffix = "/vopt_cover.log"
	}

	log_files := []core.Path{}
	targets := []string{}
	params := []string{}
	if rule.Params != nil {
		for key, _ := range rule.Params {
			log_files = append(log_files, rule.Path().WithSuffix(key))
			targets = append(targets, rule.Target() + key)
			params = append(params, key)
		}
	} else {
		log_files = append(log_files, rule.Path())
		targets = append(targets, rule.Target())
		params = append(params, "")
	}

	for i := range log_files {
		log_file := log_files[i].WithSuffix(log_file_suffix)
		target := targets[i]
		param_set := params[i]

		// Skip if we already have a rule
		if rules[log_file.String()] {
			return
		}

		cmd := fmt.Sprintf("vopt %s %s +acc=%s -l %s -work %s %s -o %s", 
											common_flags, cover_flag, Access.Value(),
											log_file.String(), rule.Lib(), top, target)

		// Set up parameters
		if param_set != "" {
			// Check that the parameters exist
			if params, ok := rule.Params[param_set]; ok {
				// Add parameters for all generics
				for param, value := range params {
					cmd = fmt.Sprintf("%s -G %s=%s", cmd, param, value)
				}
			} else {
				log.Fatal(fmt.Sprintf("parameter set '%s' not defined for Simulation target '%s'!", 
				          params, rule.Name))
			}
		}
		
		// Add the rule to run 'vopt'.
		ctx.AddBuildStep(core.BuildStep{
			Out:   log_file,
			Ins:   deps,
			Cmd:   cmd,
			Descr: fmt.Sprintf("vopt: %s %s", rule.Lib() + "." + top, target),
		})

		// Note that we created this rule
		rules[log_file.String()] = true
	}
}

// BuildQuesta will compile and optimize the source and IPs associated with the given
// rule.
func BuildQuesta(ctx core.Context, rule Simulation) {
	// Compile the code
	deps := Compile(ctx, rule)

	// Optimize the code
	Optimize(ctx, rule, deps)
}

// VerbosityLevelToFlag takes a verbosity level of none, low, medium or high and
// converts it to the corresponding DVM_ level.
func VerbosityLevelToFlag(level string) (string, bool) {
	var verbosity_flag string
	var print_output bool
	switch level {
		case "none":   
			verbosity_flag = " +verbosity=DVM_VERB_NONE"
			print_output = false
		case "low":    
			verbosity_flag = " +verbosity=DVM_VERB_LOW"
			print_output = true
		case "medium":
			verbosity_flag = " +verbosity=DVM_VERB_MED"
			print_output = true
		case "high":
			verbosity_flag = " +verbosity=DVM_VERB_HIGH"
			print_output = true
		default:
			log.Fatal(fmt.Sprintf("invalid verbosity flag '%s', only 'low', 'medium'," + 
			                      " 'high' or 'none' allowed!", level))
		}

		return verbosity_flag, print_output
}

// Preamble creates a preamble for the simulation command for the purpose of generating
// a testcase.
func Preamble(rule Simulation, testcase string) (string, string) {
	preamble := ""

	// Create a testcase generation command if necessary
	if rule.TestCaseGenerator != nil {
		// No testcase specified, use default
		if testcase == "" {
			// If directory of testcases available, pick the first one
			if rule.TestCasesDir != nil {
				if items, err := os.ReadDir(rule.TestCasesDir.String()); err == nil {
					if len(items) == 0 {
						log.Fatal(fmt.Sprintf("TestCasesDir directory '%s' empty!", rule.TestCasesDir.String()))
					}

					// Create path to testcase
					testcase = rule.TestCasesDir.Absolute() + "/" + items[0].Name()
				}
			}
		} else if testcase != "" && rule.TestCasesDir != nil {
			// Create path to testcase
			testcase = rule.TestCasesDir.Absolute() + "/" + testcase
		}

		if testcase == "" {
			// Create the preamble for testcase generator without any argument
			preamble = fmt.Sprintf("{ %s . ; }", rule.TestCaseGenerator.String())
			testcase = "default"
		} else {
			// Create the preamble for testcase generator with arguments
			preamble = fmt.Sprintf("{ %s %s . ; }", rule.TestCaseGenerator.String(), testcase)
		}

		// Add information to command
		preamble = fmt.Sprintf("{ echo Testcase %s; } && ", testcase) + preamble

		// Trim testcase for use in coverage databaes
		testcase = strings.TrimSuffix(path.Base(testcase), path.Ext(testcase))
	}

	return preamble, testcase
}

// Simulate will start 'vsim' on the compiled design with flags set in accordance
// with what is specified on the command line.
func SimulateQuesta(rule Simulation, args []string, gui bool) string {
	// Prefix the vsim command with this
	cmd_preamble := ""
	
	// Default log file
	log_file := rule.Path().WithSuffix("/vsim.log")

	// Default flag values
	vsim_flags     := " -onfinish stop -l " + log_file.String()
	seed_flag      := " -sv_seed random"
	verbosity_flag := " +verbosity=DVM_VERB_NONE"
	mode_flag      := " -batch -quiet"
	plusargs_flag  := ""

	// Enable coverage in simulator
	coverage_flag := ""
	if Coverage.Value() {
		coverage_flag = " -coverage"
	}

	// Determine the names of the coverage databases
	coverage_db := rule.Name

	// Collect do-files here
	var do_flags []string

	// Turn off output unless verbosity is activated
	print_output := false

	// Optional testcase goes here
	testcase := ""

	// Optional parameter set goes here
	params := ""

	// Parse additional arguments
	for _, arg := range args {
		if strings.HasPrefix(arg, "-seed=") {
			// Define simulator seed
			var seed int64
			if _, err := fmt.Sscanf(arg, "-seed=%d", &seed); err == nil {
				seed_flag = fmt.Sprintf(" -sv_seed %d", seed)
			} else {
				log.Fatal("-seed expects an integer argument!")
			}
		} else if strings.HasPrefix(arg, "-verbosity=") {
			// Define verbosity level
			var level string
			if _, err := fmt.Sscanf(arg, "-verbosity=%s", &level); err == nil {
				verbosity_flag, print_output = VerbosityLevelToFlag(level)
			} else {
				log.Fatal("-verbosity expects an argument of 'low', 'medium', 'high' or 'none'!")
			}
		}	else if strings.HasPrefix(arg, "-testcase=")  && rule.TestCaseGenerator != nil {
			if _, err := fmt.Sscanf(arg, "-testcase=%s", &testcase); err != nil {
				log.Fatal(fmt.Sprintf("-testcase expects a string argument!"))
			}
		}	else if strings.HasPrefix(arg, "-params=")  && rule.Params != nil {
			if _, err := fmt.Sscanf(arg, "-params=%s", &params); err != nil {
				log.Fatal(fmt.Sprintf("-params expects a string argument!"))
			} else {
				if _, ok := rule.Params[params]; !ok {
					log.Fatal(fmt.Sprintf("parameter set '%s' not defined for Simulation target '%s'!", params, rule.Name))
				}
			}
		}	else if strings.HasPrefix(arg, "+") {
			// All '+' arguments go directly to the simulator
			plusargs_flag = plusargs_flag + " " + arg
		} 
	}

	// Create optional command preamble
	cmd_preamble, testcase = Preamble(rule, testcase)

	// Update params name
	if rule.Params != nil && params == "" {
		// Pick first parameter set
		for params = range rule.Params {
			break
		}
	}

	if rule.Params != nil && params != "" {
		// Update coverage database name
		if testcase != "" {
			coverage_db = coverage_db + "_" + params + "_" + testcase
			testcase = params + "_" + testcase
		} else {
			coverage_db = coverage_db + "_" + params
			testcase = params
		}
	} else {
		// Update coverage database name
		if testcase != "" {
			coverage_db = coverage_db + "_" + testcase
		} else {
			testcase = "default"
		}
	}

	cmd_postamble := "" 
	if gui {
		mode_flag = " -gui"
		if rule.WaveformInit != nil {
			do_flags = append(do_flags, rule.WaveformInit.String())
		}
	} else {
		if !print_output {
			mode_flag = mode_flag + " -nostdout"
		}
		do_flags = append(do_flags, "\"run -all\"")
		if Coverage.Value() {
			do_flags = append(do_flags, 
				fmt.Sprintf("\"coverage report -html -output %s_covhtml -details -assert" + 
				            " -directive -cvg -code bcefst -threshL 50 -threshH 90\"", coverage_db))
			do_flags = append(do_flags, fmt.Sprintf("\"coverage save -assert" +
			                                        " -directive -cvg -codeAll -testname %s" + 
																							" -instance %s %s.ucdb\"", 
																							testcase, rule.Instance(), coverage_db))
			do_flags = append(do_flags, fmt.Sprintf("\"vcover merge -out %s.ucdb {*}[glob %s*.ucdb]\"", rule.Name, rule.Name))
			do_flags = append(do_flags, fmt.Sprintf("\"file delete {*}[glob -nocomplain %s_*.ucdb]\"", rule.Name))
		}
		do_flags = append(do_flags, "\"quit -code [coverage attribute -name TESTSTATUS -concise]\"")
		cmd_postamble = fmt.Sprintf("|| { cat %s; exit 1; }", log_file.String())
	}

	vsim_flags = vsim_flags + mode_flag + seed_flag + coverage_flag + verbosity_flag + plusargs_flag

	for _, do_flag := range do_flags {
		vsim_flags = vsim_flags + " -do " + do_flag
	}

	cmd := fmt.Sprintf("{ vsim %s -work %s %s; }", vsim_flags, rule.Lib(), rule.Target() + params)
	if cmd_preamble == "" {
		cmd = cmd + " " + cmd_postamble
	} else {
		cmd = "{ { " + cmd_preamble + " } && " + cmd + " } " + cmd_postamble
	}

	return cmd
}

// Run will build the design and run a simulation in GUI mode.
func RunQuesta(rule Simulation, args []string) string {
	return SimulateQuesta(rule, args, true)
}

// Test will build the design and run a simulation in batch mode.
func TestQuesta(rule Simulation, args []string) string {
	return SimulateQuesta(rule, args, false)
}
