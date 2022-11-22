package hdl

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"dbt-rules/RULES/core"
)

// VlogFlags enables the user to specify additional flags for the 'vlog' command.
var VlogFlags = core.StringFlag{
	Name: "questa-vlog-flags",
	DefaultFn: func() string {
		return ""
	},
	Description: "Extra flags for the vlog command",
}.Register()

// VcomFlags enables the user to specify additional flags for the 'vcom' command.
var VcomFlags = core.StringFlag{
	Name: "questa-vcom-flags",
	DefaultFn: func() string {
		return ""
	},
	Description: "Extra flags for the vcom command",
}.Register()

// VsimFlags enables the user to specify additional flags for the 'vsim' command.
var VsimFlags = core.StringFlag{
	Name: "questa-vsim-flags",
	DefaultFn: func() string {
		return ""
	},
	Description: "Extra flags for the vsim command",
}.Register()

// Lint enables additional linting information during compilation.
var Lint = core.BoolFlag{
	Name: "questa-lint",
	DefaultFn: func() bool {
		return false
	},
	Description: "Enable additional lint information during compilation",
}.Register()

// Access enables the user to control the accessibility in the compiled design for
// debugging purposes.
var Access = core.StringFlag{
	Name: "questa-access",
	DefaultFn: func() string {
		return "debug"
	},
	Description: "Control access to simulation objects for debugging purposes",
}.Register()

// Coverage enables the user to run the simulation with code coverage.
var Coverage = core.BoolFlag{
	Name: "questa-coverage",
	DefaultFn: func() bool {
		return false
	},
	Description: "Enable code-coverage database generation",
}.Register()

// Target returns the optimization target name defined for this rule.
func (rule Simulation) Target() string {
	if Coverage.Value() {
		return "vopt_cover"
	} else {
		return "vopt"
	}
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

// libFlags returns the flags needed to configure the extra libraries for this rule
func (rule Simulation) libFlags() string {
	flags := ""
	if SimulatorLibDir.Value() != "" {
		for _, lib := range rule.Libs {
			flags += fmt.Sprintf(" -L %s/%s", SimulatorLibDir.Value(), lib)
		}
	}

	return flags
}

// rules holds a map of all defined rules to prevent defining the same rule
// multiple times.
var rules = make(map[string]bool)

// common_flags holds common flags used for the 'vlog', 'vcom', and 'vopt' commands.
const common_flags = "-nologo -quiet"

// Parameters of the do-file
type DoFileParams struct {
	Lib          string
	WaveformInit string
	DumpVcd      bool
	DumpVcdFile  string
	CovFiles     string
}

// Do-file template
const do_file_template = `
proc reload {} {
	global target
	vsim -work {{ .Lib }} $target
	{{ if .WaveformInit }}
		source {{ .WaveformInit }}
	{{ end }}

}

set StdArithNoWarnings 1
set NumericStdNoWarnings 1

{{ if .WaveformInit }}
if [info exists gui] {
	source {{ .WaveformInit }}
	assertion fail -action break
}
{{ end }}

if [info exists from] {
	run $from
}

{{ if .DumpVcd }}
vcd file {{ .DumpVcdFile }}
vcd add -r *
{{ end }}

if [info exists to] {
	run @$to
} else {
	run -all
}

{{ if .DumpVcd }}
vcd flush
{{ end }}

if [info exists coverage] {
	# Create coverage database
	coverage save -assert -directive -cvg -codeall -testname $testcase $coverage_db.ucdb
	# Optionally merge coverage databases
	if {$main_coverage_db != $coverage_db} {
		puts "Writing merged coverage database to [pwd]/$main_coverage_db.ucdb"
		vcover merge -testassociated -output $main_coverage_db.ucdb $main_coverage_db.ucdb $coverage_db.ucdb
	}
	# Create HTML coverage report
	vcover report -html -output ${main_coverage_db}_covhtml \
		-testdetails -details -assert -directive -cvg -codeAll $main_coverage_db.ucdb
	# Create textual code coverage report
	{{ if .CovFiles }}
	vcover report -output ${main_coverage_db}_covcode.txt -srcfile={{ .CovFiles }}\
		-codeAll $main_coverage_db.ucdb
	{{ else }}
	vcover report -output ${main_coverage_db}_covcode.txt\
		-codeAll $main_coverage_db.ucdb
	{{ end }}
	# Create textual assertion coverage report
	puts "Writing coverage report to [pwd]/${main_coverage_db}_cover.txt"
	vcover report -output ${main_coverage_db}_cover.txt -flat -directive -cvg $main_coverage_db.ucdb
	# Create textural assertion report
	puts "Writing assertion report to [pwd]/${main_coverage_db}_cover.txt"
	vcover report -output ${main_coverage_db}_assert.txt -flat -assert $main_coverage_db.ucdb
}

if ![info exists gui] {
	quit -code [coverage attribute -name TESTSTATUS -concise]
}
`

func createModelsimIni(ctx core.Context, rule Simulation, deps []core.Path) []core.Path {
	if SimulatorLibDir.Value() != "" {
		cmds := []string{}
		for _, lib := range rule.Libs {
			cmds = append(cmds, fmt.Sprintf("vmap %s %s/%s", lib, SimulatorLibDir.Value(), lib))
		}

		if len(cmds) > 0 {
			modelsim_ini := rule.Path().WithSuffix("/modelsim.ini")
			ctx.AddBuildStep(core.BuildStep{
				Out:   modelsim_ini,
				Cmd:   strings.Join(cmds, " && "),
				Descr: fmt.Sprintf("vmap: %s", modelsim_ini.Absolute()),
			})
			deps = append(deps, modelsim_ini)
		}
	}
	return deps
}

// compileSrcs compiles a list of sources using the specified context ctx, rule,
// dependencies and include paths. It returns the resulting dependencies and include paths
// that result from compiling the source files.
func compileSrcs(ctx core.Context, rule Simulation,
	deps []core.Path, incs []core.Path, srcs []core.Path, flags FlagMap) ([]core.Path, []core.Path) {
	for _, src := range srcs {
		if IsRtl(src.String()) {
			// log will point to the log file to be generated when compiling the code
			log := rule.Path().WithSuffix("/" + src.Relative() + ".log")

			// If we already have a rule for this file, skip it.
			if rules[log.String()] {
				continue
			}

			cmd := fmt.Sprintf("%s -work %s -l %s", common_flags, rule.Lib(), log.String())

			// tool will point to the tool to execute (also used for logging below)
			var tool string
			if IsVerilog(src.String()) {
				tool = "vlog"
				cmd = cmd + " " + VlogFlags.Value()
				cmd = cmd + " -suppress 2583 -svinputport=net -define SIMULATION"
				cmd = cmd + fmt.Sprintf(" %s", rule.libFlags())
				cmd = cmd + fmt.Sprintf(" +incdir+%s", core.SourcePath("").String())
				for _, inc := range incs {
					cmd = cmd + fmt.Sprintf(" +incdir+%s", path.Dir(inc.Absolute()))
				}
				if flags != nil {
					if vlog_flags, ok := flags["vlog"]; ok {
						cmd = cmd + " " + vlog_flags
					}
				}
				for key, value := range rule.Defines {
					cmd = cmd + fmt.Sprintf(" -define %s", key)
					if value != "" {
						cmd = cmd + fmt.Sprintf("=%s", value)
					}
				}
			} else if IsVhdl(src.String()) {
				tool = "vcom"
				cmd = cmd + " " + VcomFlags.Value()
				if flags != nil {
					if vcom_flags, ok := flags["vcom"]; ok {
						cmd = cmd + " " + vcom_flags
					}
				}
			}

			if Lint.Value() {
				cmd = cmd + " -lint"
			}

			// Create plain compilation command
			cmd = tool + " " + cmd + " " + src.String()

			// Remove the log file if the command fails to ensure we can recompile it
			cmd = cmd + " || { rm " + log.String() + " && exit 1; }"

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
			rules[log.String()] = true
		} else {
			// We handle header files separately from other source files
			if IsHeader(src.String()) {
				incs = append(incs, src)
			}

			// Just add the file to the dependencies of the next one (including header files)
			deps = append(deps, src)
		}
	}

	return deps, incs
}

// compileIp compiles the IP dependencies and the source files and an IP.
func compileIp(ctx core.Context, rule Simulation, ip Ip,
	deps []core.Path, incs []core.Path) ([]core.Path, []core.Path) {
	for _, sub_ip := range ip.Ips() {
		deps, incs = compileIp(ctx, rule, sub_ip, deps, incs)
	}
	deps, incs = compileSrcs(ctx, rule, deps, incs, ip.Sources(), ip.Flags())

	return deps, incs
}

// compile compiles the IP dependencies and source files of a simulation rule.
func compile(ctx core.Context, rule Simulation) []core.Path {
	incs := []core.Path{}
	deps := []core.Path{}

	deps = createModelsimIni(ctx, rule, deps)

	for _, ip := range rule.Ips {
		deps, incs = compileIp(ctx, rule, ip, deps, incs)
	}
	deps, incs = compileSrcs(ctx, rule, deps, incs, rule.Srcs, rule.ToolFlags)

	return deps
}

// optimize creates and optimized version of the design optionally including
// coverage recording functionality. The optimized design unit can then conveniently
// be simulated using 'vsim'.
func optimize(ctx core.Context, rule Simulation, deps []core.Path) {
	top := "board"
	additional_tops := ""

	if rule.Top != "" && len(rule.Tops) > 0 {
		log.Fatal(fmt.Sprintf("only one of Top or Tops allowed!"))
	}

	if rule.Top != "" {
		top = rule.Top
	}

	if len(rule.Tops) > 0 {
		top = rule.Tops[0]
		if len(rule.Tops) > 1 {
			additional_tops = strings.Join(rule.Tops[1:], " ")
		}
	}

	cover_flag := ""
	log_file_suffix := "vopt.log"
	if Coverage.Value() {
		cover_flag = "+cover"
		log_file_suffix = "vopt_cover.log"
	}

	log_files := []core.OutPath{}
	targets := []string{}
	params := []string{}
	if rule.Params != nil {
		for key, _ := range rule.Params {
			log_files = append(log_files, rule.Path().WithSuffix("/"+key+"_"+log_file_suffix))
			targets = append(targets, key+"_"+rule.Target())
			params = append(params, key)
		}
	} else {
		log_files = append(log_files, rule.Path().WithSuffix("/"+log_file_suffix))
		targets = append(targets, rule.Target())
		params = append(params, "")
	}

	for i := range log_files {
		log_file := log_files[i]
		target := targets[i]
		param_set := params[i]

		// Skip if we already have a rule
		if rules[log_file.String()] {
			return
		}

		// Generate access flag
		access_flag := ""
		if Access.Value() == "debug" {
			access_flag = "+acc"
		} else if Access.Value() != "" {
			access_flag = fmt.Sprintf("+acc=%s", Access.Value())
		}

		cmd := fmt.Sprintf("vopt %s %s %s -l %s -work %s %s %s -o %s %s",
			common_flags, cover_flag, access_flag,
			log_file.String(), rule.Lib(), top, additional_tops, target, rule.libFlags())

		// Set up parameters
		if param_set != "" {
			// Check that the parameters exist
			if params, ok := rule.Params[param_set]; ok {
				// Add parameters for all generics
				for param, value := range params {
					cmd = fmt.Sprintf("%s -g %s=%s", cmd, param, value)
				}
			}
		}

		// Add any extra flags specified with the rule
		if rule.ToolFlags != nil {
			if vopt_flags, ok := rule.ToolFlags["vopt"]; ok {
				cmd = cmd + " " + vopt_flags
			}
		}

		if rule.TestCaseGenerator != nil {
			deps = append(deps, rule.TestCaseGenerator)
		}

		// Add the rule to run 'vopt'.
		ctx.AddBuildStep(core.BuildStep{
			Out:   log_file,
			Ins:   deps,
			Cmd:   cmd,
			Descr: fmt.Sprintf("vopt: %s %s", rule.Lib()+"."+top, target),
		})

		// Note that we created this rule
		rules[log_file.String()] = true
	}
}

// Create a simulation script
func doFile(ctx core.Context, rule Simulation) {
	// Do-file script
	params := DoFileParams{
		Lib: rule.Lib(),
		DumpVcd: DumpVcd.Value(),
		DumpVcdFile: fmt.Sprintf("%s.vcd.gz", rule.Name),
		CovFiles: strings.Join(rule.ReportCovFiles(), "+"),
	}

	if rule.WaveformInit != nil {
		params.WaveformInit = rule.WaveformInit.String()
	}

	doFile := rule.Path().WithSuffix("/" + "vsim.do")
	ctx.AddBuildStep(core.BuildStep{
		Out:   doFile,
		Data:  core.CompileTemplate(do_file_template, "do", params),
		Descr: fmt.Sprintf("vsim: %s", doFile.Relative()),
	})
}

// BuildQuesta will compile and optimize the source and IPs associated with the given
// rule.
func BuildQuesta(ctx core.Context, rule Simulation) {
	// compile the code
	deps := compile(ctx, rule)

	// optimize the code
	optimize(ctx, rule, deps)

	// Create script
	doFile(ctx, rule)
}

// verbosityLevelToFlag takes a verbosity level of none, low, medium or high and
// converts it to the corresponding DVM_ level.
func verbosityLevelToFlag(level string) (string, bool) {
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
	case "all":
		verbosity_flag = " +verbosity=DVM_VERB_ALL"
		print_output = true
	default:
		log.Fatal(fmt.Sprintf("invalid verbosity flag '%s', only 'low', 'medium',"+
			" 'high', 'all'  or 'none' allowed!", level))
	}

	return verbosity_flag, print_output
}

// questaCmd will create a command for starting 'vsim' on the compiled and optimized design with flags
// set in accordance with what is specified on the command line.
func questaCmd(rule Simulation, args []string, gui bool, testcase string, params string) string {
	// Prefix the vsim command with this
	cmd_preamble := ""

	// Default log file
	log_file_suffix := "vsim.log"
	if testcase != "" {
		log_file_suffix = testcase + "_" + log_file_suffix
	}
	if params != "" {
		log_file_suffix = params + "_" + log_file_suffix
	}
	log_file := rule.Path().WithSuffix("/" + log_file_suffix)

	// Script to execute
	do_file := rule.Path().WithSuffix("/" + "vsim.do")

	// Collect do-files and commands here
	var do_flags []string

	// Default flag values
	vsim_flags := " -onfinish final -l " + log_file.String() + rule.libFlags()

	seed_flag := " -sv_seed random"
	verbosity_flag := " +verbosity=DVM_VERB_NONE"
	mode_flag := " -batch -quiet"
	plusargs_flag := ""

	// Default database name for simulation
	var target string
	if len(params) > 0 {
		target = params + "_" + rule.Target()
	} else {
		target = rule.Target()
	}

	// Enable coverage in simulator
	coverage_flag := ""
	if Coverage.Value() {
		coverage_flag = " -coverage -assertdebug"
		do_flags = append(do_flags, "\"set coverage 1\"")
	}

	// Determine the names of the coverage databases, this one will hold merged
	// data from multiple testcases
	main_coverage_db := rule.Name

	// This will be the name of the database created by the current run
	coverage_db := rule.Name

	// Turn off output unless verbosity is activated
	print_output := false

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
				verbosity_flag, print_output = verbosityLevelToFlag(level)
			} else {
				log.Fatal("-verbosity expects an argument of 'low', 'medium', 'high' or 'none'!")
			}
		} else if strings.HasPrefix(arg, "-from=") {
			// Define how long to run
			var from string
			if _, err := fmt.Sscanf(arg, "-from=%s", &from); err == nil {
				do_flags = append(do_flags, fmt.Sprintf("\"set from %s\"", from))
			} else {
				log.Fatal("-from expects an argument of '<timesteps>[<time units>]'!")
			}
		} else if strings.HasPrefix(arg, "-to=") {
			// Define how long to run
			var to string
			if _, err := fmt.Sscanf(arg, "-to=%s", &to); err == nil {
				do_flags = append(do_flags, fmt.Sprintf("\"set to %s\"", to))
			} else {
				log.Fatal("-to expects an argument of '<timesteps>[<time units>]'!")
			}
		} else if strings.HasPrefix(arg, "+") {
			// All '+' arguments go directly to the simulator
			plusargs_flag = plusargs_flag + " " + arg
		}
	}

	// Create optional command preamble
	cmd_preamble, testcase = Preamble(rule, testcase)

	cmd_echo := ""
	if rule.Params != nil && params != "" {
		// Update coverage database name based on parameters. Since we cannot merge
		// different parameter sets, we have to make a dedicated main database
		// for this parameter set.
		main_coverage_db = main_coverage_db + "_" + params
		coverage_db = coverage_db + "_" + params
		cmd_echo = "Testcase " + params

		// Update with testcase if specified
		if testcase != "" {
			coverage_db = coverage_db + "_" + testcase
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
			coverage_db = coverage_db + "_" + testcase
			cmd_echo = "Testcase " + testcase + ":"
		} else {
			testcase = "default"
		}
	}

	do_flags = append(do_flags, fmt.Sprintf("\"set target %s\"", target))
	do_flags = append(do_flags, fmt.Sprintf("\"set testcase %s\"", testcase))
	do_flags = append(do_flags, fmt.Sprintf("\"set main_coverage_db %s\"", main_coverage_db))
	do_flags = append(do_flags, fmt.Sprintf("\"set coverage_db %s\"", coverage_db))

	cmd_postamble := ""
	cmd_pass := "PASS"
	cmd_fail := "FAIL"
	if gui {
		mode_flag = " -gui"
		do_flags = append(do_flags, "\"set gui 1\"")
	}

	if !print_output && !gui {
		mode_flag = mode_flag + " -nostdout"
	}

	if Coverage.Value() {
		cmd_pass = cmd_pass + fmt.Sprintf(" Coverage: $$(pwd)/%s.ucdb", main_coverage_db)
		cmd_fail = cmd_fail + fmt.Sprintf(" Coverage: $$(pwd)/%s.ucdb", main_coverage_db)
	}

	cmd_newline := ":"
	if cmd_echo != "" {
		cmd_newline = "echo"
	}

	if !print_output {
		cmd_postamble = fmt.Sprintf("|| { %s; cat %s; echo %s; exit 1; }", cmd_newline, log_file.String(), cmd_fail)
	}

	vsim_flags = vsim_flags + mode_flag + seed_flag + coverage_flag +
		verbosity_flag + plusargs_flag + " " + VsimFlags.Value()

	// Add any extra flags specified with the rule
	if rule.ToolFlags != nil {
		if extra_flags, ok := rule.ToolFlags["vsim"]; ok {
			vsim_flags = vsim_flags + " " + extra_flags
		}
	}

	for _, do_flag := range do_flags {
		vsim_flags = vsim_flags + " -do " + do_flag
	}

	// Add the file as the last argument
	vsim_flags = vsim_flags + " -do " + do_file.String()

	cmd := fmt.Sprintf("{ echo -n %s && vsim %s -work %s %s && echo %s; }", cmd_echo, vsim_flags, rule.Lib(), target, cmd_pass)
	if cmd_preamble == "" {
		cmd = cmd + " " + cmd_postamble
	} else {
		cmd = "{ { " + cmd_preamble + " } && " + cmd + " } " + cmd_postamble
	}

	// Wrap command in another layer of {} to enable chaining
	cmd = "{ " + cmd + " }"

	return cmd
}

// simulateQuesta will create a command to start 'vsim' on the compiled design
// with flags set in accordance with what is specified on the command line. It will
// optionally build a chain of commands in case the rule has parameters, but
// no parameters are specified on the command line
func simulateQuesta(rule Simulation, args []string, gui bool) string {
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
			cmd = cmd + " && " + questaCmd(rule, args, gui, testcases[j], params[i])
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
func RunQuesta(rule Simulation, args []string) string {
	return simulateQuesta(rule, args, true)
}

// Test will build the design and run a simulation in batch mode.
func TestQuesta(rule Simulation, args []string) string {
	return simulateQuesta(rule, args, false)
}
