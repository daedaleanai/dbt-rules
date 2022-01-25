package hdl

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"dbt-rules/RULES/core"
)

// XvlogFlags enables the user to specify additional flags for the 'vlog' command.
var XvlogFlags = core.StringFlag{
	Name: "xsim-xvlog-flags",
	DefaultFn: func() string {
		return ""
	},
	Description: "Extra flags for the xvlog command",
}.Register()

// XvhdlFlags enables the user to specify additional flags for the 'vcom' command.
var XvhdlFlags = core.StringFlag{
	Name: "xsim-xvhdl-flags",
	DefaultFn: func() string {
		return ""
	},
	Description: "Extra flags for the xvhdl command",
}.Register()

// XsimFlags enables the user to specify additional flags for the 'vsim' command.
var XsimFlags = core.StringFlag{
	Name: "xsim-xsim-flags",
	DefaultFn: func() string {
		return ""
	},
	Description: "Extra flags for the xsim command",
}.Register()

// XelabDebug enables the user to control the accessibility in the compiled design for
// debugging purposes.
var XelabDebug = core.StringFlag{
	Name: "xsim-xelab-debug",
	DefaultFn: func() string {
		return "typical"
	},
	Description:   "Extra debug flags for the xelab command",
	AllowedValues: []string{"line", "wave", "drivers", "readers", "xlibs", "all", "typical", "subprogram", "off"},
}.Register()

// xsim_rules holds a map of all defined rules to prevent defining the same rule
// multiple times.
var xsim_rules = make(map[string]bool)

// xsimCompileSrcs compiles a list of sources using the specified context ctx, rule,
// dependencies and include paths. It returns the resulting dependencies and include paths
// that result from compiling the source files.
func xsimCompileSrcs(ctx core.Context, rule Simulation,
	deps []core.Path, incs []core.Path, srcs []core.Path) ([]core.Path, []core.Path) {
	for _, src := range srcs {
		if IsRtl(src.String()) {
			// log will point to the log file to be generated when compiling the code
			log := rule.Path().WithSuffix("/" + src.Relative() + ".log")

			// If we already have a rule for this file, skip it.
			if xsim_rules[log.String()] {
				continue
			}

			// Holds common flags for both 'vlog' and 'vcom' commands
			cmd := fmt.Sprintf("-work %s --log %s", strings.ToLower(rule.Lib()), log.String())

			// tool will point to the tool to execute (also used for logging below)
			var tool string
			if IsVerilog(src.String()) {
				tool = "xvlog"
				cmd = cmd + " --sv " + XvlogFlags.Value()
				cmd = cmd + fmt.Sprintf(" -i %s", core.SourcePath("").String())
				for _, inc := range incs {
					cmd = cmd + fmt.Sprintf(" -i %s", path.Dir(inc.Absolute()))
				}
			} else if IsVhdl(src.String()) {
				tool = "xvhdl"
				cmd = cmd + " " + XvhdlFlags.Value()
			}

			// Remove the log file if the command fails to ensure we can recompile it
			cmd = tool + " " + cmd + " " + src.String() + " > /dev/null" +
				" || { cat " + log.String() + "; rm " + log.String() + "; exit 1; }"

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
			xsim_rules[log.String()] = true
		} else {
			// We handle header files separately from other source files
			if IsHeader(src.String()) {
				incs = append(incs, src)
			}
			// Add the file to the dependencies of the next one (including header files)
			deps = append(deps, src)
		}
	}

	return deps, incs
}

// xsimCompileIp compiles the IP dependencies and the source files and an IP.
func xsimCompileIp(ctx core.Context, rule Simulation, ip Ip,
	deps []core.Path, incs []core.Path) ([]core.Path, []core.Path) {
	for _, sub_ip := range ip.Ips() {
		deps, incs = xsimCompileIp(ctx, rule, sub_ip, deps, incs)
	}
	deps, incs = xsimCompileSrcs(ctx, rule, deps, incs, ip.Sources())

	return deps, incs
}

// xsimCompile compiles the IP dependencies and source files of a simulation rule.
func xsimCompile(ctx core.Context, rule Simulation) []core.Path {
	incs := []core.Path{}
	deps := []core.Path{}

	for _, ip := range rule.Ips {
		deps, incs = xsimCompileIp(ctx, rule, ip, deps, incs)
	}
	deps, incs = xsimCompileSrcs(ctx, rule, deps, incs, rule.Srcs)

	return deps
}

// elaborate creates and optimized version of the design optionally including
// coverage recording functionality. The optimized design unit can then conveniently
// be simulated using 'xsim'.
func elaborate(ctx core.Context, rule Simulation, deps []core.Path) {
	top := "board"
	if rule.Top != "" {
		top = rule.Top
	}

	log_file_suffix := "xelab.log"

	log_files := []core.OutPath{}
	targets := []string{}
	params := []string{}
	if rule.Params != nil {
		for key, _ := range rule.Params {
			log_files = append(log_files, rule.Path().WithSuffix("/"+key+"_"+log_file_suffix))
			targets = append(targets, rule.Name+key)
			params = append(params, key)
		}
	} else {
		log_files = append(log_files, rule.Path().WithSuffix("/"+log_file_suffix))
		targets = append(targets, rule.Name)
		params = append(params, "")
	}

	for i := range log_files {
		log_file := log_files[i]
		target := targets[i]
		param_set := params[i]

		// Skip if we already have a rule
		if xsim_rules[log_file.String()] {
			return
		}

		cmd := fmt.Sprintf("xelab --timescale 1ns/1ps --debug %s --log %s %s.%s -s %s",
			XelabDebug.Value(), log_file.String(), strings.ToLower(rule.Lib()), top, target)

		// Set up parameters
		if param_set != "" {
			// Check that the parameters exist
			if params, ok := rule.Params[param_set]; ok {
				// Add parameters for all generics
				for param, value := range params {
					cmd = fmt.Sprintf("%s -generic_top \"%s=%s\"", cmd, param, value)
				}
			} else {
				log.Fatal(fmt.Sprintf("parameter set '%s' not defined for Simulation target '%s'!",
					params, rule.Name))
			}
		}

		cmd = cmd + " > /dev/null || { cat " + log_file.String() +
			"; rm " + log_file.String() + "; exit 1; }"

		// Hack: Add testcase generator as an optional dependency
		if rule.TestCaseGenerator != nil {
			deps = append(deps, rule.TestCaseGenerator)
		}

		// Add the rule to run 'xelab'.
		ctx.AddBuildStep(core.BuildStep{
			Out:   log_file,
			Ins:   deps,
			Cmd:   cmd,
			Descr: fmt.Sprintf("xelab: %s %s", rule.Lib()+"."+top, target),
		})

		// Note that we created this rule
		xsim_rules[log_file.String()] = true
	}
}

// BuildXsim will compile and elaborate the source and IPs associated with the given
// rule.
func BuildXsim(ctx core.Context, rule Simulation) {
	// compile the code
	deps := xsimCompile(ctx, rule)

	// elaborate the code
	elaborate(ctx, rule, deps)
}

// xsimVerbosityLevelToFlag takes a verbosity level of none, low, medium or high and
// converts it to the corresponding DVM_ level.
func xsimVerbosityLevelToFlag(level string) (string, bool) {
	var verbosity_flag string
	var print_output bool
	switch level {
	case "none":
		verbosity_flag = " --testplusarg verbosity=DVM_VERB_NONE"
		print_output = false
	case "low":
		verbosity_flag = " --testplusarg verbosity=DVM_VERB_LOW"
		print_output = true
	case "medium":
		verbosity_flag = " --testplusarg verbosity=DVM_VERB_MED"
		print_output = true
	case "high":
		verbosity_flag = " --testplusarg verbosity=DVM_VERB_HIGH"
		print_output = true
	default:
		log.Fatal(fmt.Sprintf("invalid verbosity flag '%s', only 'low', 'medium',"+
			" 'high' or 'none' allowed!", level))
	}

	return verbosity_flag, print_output
}

// xsimCmd will create a command for starting 'vsim' on the compiled and optimized design with flags
// set in accordance with what is specified on the command line.
func xsimCmd(rule Simulation, args []string, gui bool, testcase string, params string) string {
	// Prefix the vsim command with this
	cmd_preamble := ""

	// Default log file
	log_file_suffix := "xsim.log"
	if testcase != "" {
		log_file_suffix = testcase + "_" + log_file_suffix
	}
	if params != "" {
		log_file_suffix = params + "_" + log_file_suffix
	}
	log_file := rule.Path().WithSuffix("/" + log_file_suffix)

	// Default flag values
	vsim_flags := " --onfinish quit --log " + log_file.String()
	seed_flag := " --sv_seed 1"
	verbosity_flag := " --testplusarg verbosity=DVM_VERB_NONE"
	mode_flag := ""
	plusargs_flag := ""

	// Collect do-files here
	var do_flags []string

	// Turn off output unless verbosity is activated
	print_output := false

	// Parse additional arguments
	for _, arg := range args {
		if strings.HasPrefix(arg, "-seed=") {
			// Define simulator seed
			var seed int64
			if _, err := fmt.Sscanf(arg, "-seed=%d", &seed); err == nil {
				seed_flag = fmt.Sprintf(" --sv_seed %d", seed)
			} else {
				log.Fatal("-seed expects an integer argument!")
			}
		} else if strings.HasPrefix(arg, "-verbosity=") {
			// Define verbosity level
			var level string
			if _, err := fmt.Sscanf(arg, "-verbosity=%s", &level); err == nil {
				verbosity_flag, print_output = xsimVerbosityLevelToFlag(level)
			} else {
				log.Fatal("-verbosity expects an argument of 'low', 'medium', 'high' or 'none'!")
			}
		} else if strings.HasPrefix(arg, "+") {
			// All '+' arguments go directly to the simulator
			plusargs_flag = plusargs_flag + " --testplusarg " + strings.TrimPrefix(arg, "+")
		}
	}

	// Create optional command preamble
	cmd_preamble, testcase = Preamble(rule, testcase)

	cmd_echo := ""
	if rule.Params != nil && params != "" {
		// Update coverage database name based on parameters. We cannot merge
		// different parameter sets, do we have to make a dedicated main database
		// for this parameter set.
		cmd_echo = "Testcase " + params

		// Update with testcase if specified
		if testcase != "" {
			cmd_echo = cmd_echo + "/" + testcase + ":"
			testcase = params + "_" + testcase
		} else {
			cmd_echo = cmd_echo + ":"
			testcase = params
		}
	} else {
		// Update coverage database name with testcase alone, main database stays
		// the same
		if testcase != "" {
			cmd_echo = "Testcase " + testcase + ":"
		} else {
			testcase = "default"
		}
	}

	cmd_postamble := ""
	if gui {
		mode_flag = " --gui"
		if rule.WaveformInit != nil {
			do_flags = append(do_flags, rule.WaveformInit.String())
		}
	} else {
		mode_flag = " --runall"
		cmd_newline := ":"
		if cmd_echo != "" {
			cmd_newline = "echo"
		}

		cmd_postamble = fmt.Sprintf("|| { %s; cat %s; exit 1; }", cmd_newline, log_file.String())
	}

	vsim_flags = vsim_flags + mode_flag + seed_flag +
		verbosity_flag + plusargs_flag + XsimFlags.Value()

	for _, do_flag := range do_flags {
		vsim_flags = vsim_flags + " --tclbatch " + do_flag
	}

	// Using this part of the command we send the stdout into a black hole to
	// keep the output clean
	cmd_devnull := ""
	if !print_output {
		cmd_devnull = "> /dev/null"
	}

	cmd := fmt.Sprintf("{ echo -n %s && xsim %s %s %s && "+
		"{ { ! grep -q FAILURE %s; } && echo PASS; } }",
		cmd_echo, vsim_flags, rule.Name+params, cmd_devnull, log_file.String())
	if cmd_preamble == "" {
		cmd = cmd + " " + cmd_postamble
	} else {
		cmd = "{ { " + cmd_preamble + " } && " + cmd + " } " + cmd_postamble
	}

	// Wrap command in another layer of {} to enable chaining
	cmd = "{ " + cmd + " }"

	return cmd
}

// simulateXsim will create a command to start 'vsim' on the compiled design
// with flags set in accordance with what is specified on the command line. It will
// optionally build a chain of commands in case the rule has parameters, but
// no parameters are specified on the command line
func simulateXsim(rule Simulation, args []string, gui bool) string {
	// Optional testcase goes here
	testcases := []string{}

	// Optional parameter set goes here
	params := []string{}

	// Parse additional arguments
	for _, arg := range args {
		if strings.HasPrefix(arg, "-testcases=") && rule.TestCaseGenerator != nil {
			var testcases_arg string
			if _, err := fmt.Sscanf(arg, "-testcases=%s", &testcases_arg); err != nil {
				log.Fatal(fmt.Sprintf("-testcases expects a string argument!"))
			}
			testcases = append(testcases, strings.Split(testcases_arg, ",")...)
		} else if strings.HasPrefix(arg, "-params=") && rule.Params != nil {
			var params_arg string
			if _, err := fmt.Sscanf(arg, "-params=%s", &params_arg); err != nil {
				log.Fatal(fmt.Sprintf("-params expects a string argument!"))
			} else {
				for _, param := range strings.Split(params_arg, ",") {
					if _, ok := rule.Params[param]; ok {
						params = append(params, param)
					}
				}
			}
		}
	}

	// If no parameters have been specified, simulate them all
	if rule.Params != nil && len(params) == 0 {
		for key := range rule.Params {
			params = append(params, key)
		}
	} else if len(params) == 0 {
		params = append(params, "")
	}

	// If no testcase has been specified, simulate them all
	if rule.TestCaseGenerator != nil && rule.TestCasesDir != nil && len(testcases) == 0 {
		// Loop through all defined testcases in directory
		if items, err := os.ReadDir(rule.TestCasesDir.String()); err == nil {
			for _, item := range items {
				testcases = append(testcases, item.Name())
			}
		} else {
			log.Fatal(err)
		}
	} else if len(testcases) == 0 {
		testcases = append(testcases, "")
	}

	// Final command
	cmd := "{ :; }"

	// Loop for all parameter sets
	for i := range params {
		// Loop for all test cases
		for j := range testcases {
			cmd = cmd + " && " + xsimCmd(rule, args, gui, testcases[j], params[i])
			// Only one testcase allowed in GUI mode
			if gui {
				break
			}
		}
		// Only one parameter set allowed in gui mode
		if gui {
			break
		}
	}

	return cmd
}

// Run will build the design and run a simulation in GUI mode.
func RunXsim(rule Simulation, args []string) string {
	return simulateXsim(rule, args, true)
}

// Test will build the design and run a simulation in batch mode.
func TestXsim(rule Simulation, args []string) string {
	return simulateXsim(rule, args, false)
}
