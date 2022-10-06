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

// Parameters of the do-file
type tclFileParams struct {
	DumpVcd      bool
	DumpVcdFile  string
}

// Do-file template
const xsim_tcl_file_template = `
if [info exists ::env(from)] {
	run $::env(from)
}

{{ if .DumpVcd }}
open_vcd {{ .DumpVcdFile }}
log_vcd [get_objects -r]
{{ end }}

if [info exists ::env(duration)] {
	run $::env(duration)
} else {
	run -all
}

{{ if .DumpVcd }}
close_vcd
{{ end }}
`

type prjFile struct {
	Rule   Simulation
	Macros []string
	Incs   []string
	Deps   []core.Path
	Data   []string
}

func addToPrjFile(ctx core.Context, prj prjFile, ips []Ip, srcs []core.Path) prjFile {
	for _, ip := range ips {
		prj = addToPrjFile(ctx, prj, ip.Ips(), ip.Sources())
	}

	for _, src := range srcs {
		if IsHeader(src.String()) {
			new_path := path.Dir(src.Absolute())
			gotit := false
			for _, old_path := range prj.Incs {
				if new_path == old_path {
					gotit = true
					break
				}
			}
			if !gotit {
				prj.Incs = append(prj.Incs, new_path)
			}
		} else if IsRtl(src.String()) {
			prefix := ""
			if IsSystemVerilog(src.String()) {
				prefix = "sv"
			} else if IsVerilog(src.String()) {
				prefix  = "verilog"
			} else if IsVhdl(src.String()) {
				prefix  = "vhdl"
			}

			entry := fmt.Sprintf("%s %s %s", prefix, strings.ToLower(prj.Rule.Lib()), src.String())

			for _, inc_path := range prj.Incs {
				entry = entry + " -i " + inc_path
			}

			if len(prj.Macros) > 0 {
				entry = entry + " -d " + strings.Join(prj.Macros, " -d ")
			}

			prj.Data = append(prj.Data, entry)
		}

		prj.Deps = append(prj.Deps, src)
	}

	return prj
}

func createPrjFile(ctx core.Context, rule Simulation) core.Path {
	macros := []string{"SIMULATION"}
	for key, value := range rule.Defines {
		macro := key
		if value != "" {
			macro = fmt.Sprintf("%s=%s", key, value)
		}
		macros = append(macros, macro)
	}
	prjFilePath := rule.Path().WithSuffix("/" + "xsim.prj")
	prjFileContents := addToPrjFile(
		ctx,
		prjFile{
			Rule: rule,
			Macros: macros,
			Incs: []string{core.SourcePath("").String()},
		}, rule.Ips, rule.Srcs)
	ctx.AddBuildStep(core.BuildStep{
		Out:   prjFilePath,
		Ins:   prjFileContents.Deps,
		Data:  strings.Join(prjFileContents.Data, "\n"),
		Descr: fmt.Sprintf("xsim project: %s", prjFilePath.Relative()),
	})

	return prjFilePath
}

// Create a simulation script
func tclFile(ctx core.Context, rule Simulation) {
	// Do-file script
	params := tclFileParams{
		DumpVcd: DumpVcd.Value(),
		DumpVcdFile: rule.Path().WithSuffix(fmt.Sprintf("/%s.vcd", rule.Name)).String(),
	}

	tclFile := rule.Path().WithSuffix("/xsim.tcl")
	ctx.AddBuildStep(core.BuildStep{
		Out:   tclFile,
		Data:  core.CompileTemplate(xsim_tcl_file_template, "tcl", params),
		Descr: fmt.Sprintf("xsim: %s", tclFile.Relative()),
	})
}

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
				cmd = cmd + " --sv --define SIMULATION --define XSIM" + XvlogFlags.Value()
				cmd = cmd + fmt.Sprintf(" -i %s", core.SourcePath("").String())
				for _, inc := range incs {
					cmd = cmd + fmt.Sprintf(" -i %s", path.Dir(inc.Absolute()))
				}
				for key, value := range rule.Defines {
					cmd = cmd + fmt.Sprintf(" --define %s", key)
					if value != "" {
						cmd = cmd + fmt.Sprintf("=%s", value)
					}
				}
			} else if IsVhdl(src.String()) {
				tool = "xvhdl"
				cmd = cmd + " " + XvhdlFlags.Value()
			}

			// Remove the log file if the command fails to ensure we can recompile it
			cmd = tool + " " + cmd + " " + src.String() + " > /dev/null" +
				" || { cat " + log.String() + "; rm " + log.String() + "; exit 1; }"

			// Add the source file to the dependencies
			deps = append(deps, src)

			// Add the compilation command as a build step with the log file as the
			// generated output
			ctx.AddBuildStep(core.BuildStep{
				Out:   log,
				Ins:   deps,
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
func elaborate(ctx core.Context, rule Simulation, prj_file core.Path) {
	xelab_base_cmd := []string{
		"xelab",
		"--timescale",
		"1ns/1ps",
		"--debug", XelabDebug.Value(),
		"--prj", prj_file.String(),
	}

	for _, lib := range rule.Libs {
		xelab_base_cmd = append(xelab_base_cmd, "--lib", lib)
	}

	tops := []string{"board"}
	if rule.Top != "" {
		tops = []string{rule.Top}
	} else if len(rule.Tops) > 0 {
		tops = rule.Tops
	}

	for _, top := range tops {
		xelab_base_cmd = append(xelab_base_cmd, strings.ToLower(rule.Lib()) + "." + top)
	}

	log_file_suffix := "xelab.log"
	log_files := []core.OutPath{}
	targets := []string{}
	params := []string{}
	if rule.Params != nil {
		for key, _ := range rule.Params {
			log_files = append(log_files, rule.Path().WithSuffix("/" + key + "_" + log_file_suffix))
			targets = append(targets, rule.Name + "_" + key)
			params = append(params, key)
		}
	} else {
		log_files = append(log_files, rule.Path().WithSuffix("/" + log_file_suffix))
		targets = append(targets, rule.Name)
		params = append(params, "")
	}

	for i := range log_files {
		log_file := log_files[i]
		target := targets[i]
		param_set := params[i]

		// Build up command using base command plus additional variable arguments
		xelab_cmd := append(xelab_base_cmd, "--log", log_file.String(), "--snapshot", target)

		// Set up parameters
		if param_set != "" {
			// Check that the parameters exist
			if params, ok := rule.Params[param_set]; ok {
				// Add parameters for all generics
				for param, value := range params {
					xelab_cmd = append(xelab_cmd, "-generic_top", fmt.Sprintf("\"%s=%s\"", param, value))
				}
			} else {
				log.Fatal(fmt.Sprintf("parameter set '%s' not defined for Simulation target '%s'!",
					params, rule.Name))
			}
		}

		cmd := strings.Join(xelab_cmd, " ") + " > /dev/null || { cat " + log_file.String() +
			"; rm " + log_file.String() + "; exit 1; }"

		// Hack: Add testcase generator as an optional dependency
		deps := []core.Path{prj_file}
		if rule.TestCaseGenerator != nil {
			deps = append(deps, rule.TestCaseGenerator)
		}

		// Add the rule to run 'xelab'.
		ctx.AddBuildStep(core.BuildStep{
			Out:   log_file,
			Ins:   deps,
			Cmd:   cmd,
			Descr: fmt.Sprintf("xelab: %s %s", strings.Join(tops, " "), target),
		})
	}
}

// BuildXsim will compile and elaborate the source and IPs associated with the given
// rule.
func BuildXsim(ctx core.Context, rule Simulation) {
	prj := createPrjFile(ctx, rule)

	// compile and elaborate the code
	elaborate(ctx, rule, prj)

	// Create simulation script
	tclFile(ctx, rule)
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

	// Script to execute
	do_file := rule.Path().WithSuffix("/" + "xsim.tcl")

	// Default flag values
	seed := int64(1)
	xsim_cmd := []string{"xsim", "--log", log_file.String(), "--tclbatch", do_file.String(), XsimFlags.Value()}
	verbosity_level := "none"

	// Parse additional arguments
	for _, arg := range args {
		if strings.HasPrefix(arg, "-seed=") {
			// Define simulator seed
			var seed_flag int64
			if _, err := fmt.Sscanf(arg, "-seed=%d", &seed_flag); err == nil {
				seed = seed_flag
			} else {
				log.Fatal("-seed expects an integer argument!")
			}
		} else if strings.HasPrefix(arg, "-from=") {
			// Define how long to run
			var from string
			if _, err := fmt.Sscanf(arg, "-from=%s", &from); err == nil {
				xsim_cmd = append([]string{fmt.Sprintf("export from=%s &&", from)}, xsim_cmd...)
			} else {
				log.Fatal("-from expects an argument of '<timesteps>[<time units>]'!")
			}
		} else if strings.HasPrefix(arg, "-duration=") {
			// Define how long to run
			var to string
			if _, err := fmt.Sscanf(arg, "-duration=%s", &to); err == nil {
				xsim_cmd = append([]string{fmt.Sprintf("export duration=%s &&", to)}, xsim_cmd...)
			} else {
				log.Fatal("-duration expects an argument of '<timesteps>[<time units>]'!")
			}
		} else if strings.HasPrefix(arg, "-verbosity=") {
			// Define verbosity level
			var level string
			if _, err := fmt.Sscanf(arg, "-verbosity=%s", &level); err == nil {
				verbosity_level = level
			} else {
				log.Fatal("-verbosity expects an argument of 'low', 'medium', 'high' or 'none'!")
			}
		} else if strings.HasPrefix(arg, "+") {
			// All '+' arguments go directly to the simulator
			xsim_cmd = append(xsim_cmd, "--testplusarg", strings.TrimPrefix(arg, "+"))
		}
	}

	// Add seed flag
	xsim_cmd = append(xsim_cmd, "--sv_seed", fmt.Sprintf("%d", seed))

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
		xsim_cmd = append(xsim_cmd, "--gui")
		if rule.WaveformInit != nil && strings.HasSuffix(rule.WaveformInit.String(), ".tcl") {
			xsim_cmd = append(xsim_cmd, "--tclbatch", rule.WaveformInit.String())
		}
	} else {
		xsim_cmd = append(xsim_cmd, "--onfinish quit")
		cmd_newline := ":"
		if cmd_echo != "" {
			cmd_newline = "echo"
		}

		cmd_postamble = fmt.Sprintf("|| { %s; cat %s; exit 1; }", cmd_newline, log_file.String())
	}

	// Convert verbosity flag to string and append to command
	verbosity_flag, print_output := xsimVerbosityLevelToFlag(verbosity_level)
	xsim_cmd = append(xsim_cmd, verbosity_flag)

	//Finally, add the snapshot to the command as the last element
	snapshot := rule.Name
	if params != "" {
		snapshot = snapshot + "_" + params
	}
	xsim_cmd = append(xsim_cmd, snapshot)

	// Using this part of the command we send the stdout into a black hole to
	// keep the output clean
	cmd_devnull := ""
	if !print_output {
		cmd_devnull = "> /dev/null"
	}

	cmd := fmt.Sprintf("{ echo -n %s && %s %s && "+
		"{ { ! grep -q FAILURE %s; } && echo PASS; } }",
		cmd_echo, strings.Join(xsim_cmd, " "), cmd_devnull, log_file.String())
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
