package hdl

import (
	"fmt"
	"path"
	"strings"
	"log"

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

type SimulationQuesta struct {
	Name    string
	Srcs    []core.Path
	Ips     []Ip
	Libs    []string
	Params  []string
	Top     string
}

// Lib returns the standard Questa library name defined for this rule.
func (rule SimulationQuesta) Lib() string {
	return rule.Name + "Lib"
}

// Target returns the optimization target name defined for this rule.
func (rule SimulationQuesta) Target() string {
	if Coverage.Value() {
		return rule.Name + "Cov"
	} else {
		return rule.Name
	}
}

// Path returns the default root path for log files defined for this rule.
func (rule SimulationQuesta) Path() core.Path {
	return core.BuildPath("/" + rule.Name)
}

// rules holds a map of all defined rules to prevent defining the same rule
// multiple times.
var rules = make(map[string]bool)

// common_flags holds common flags used for the 'vlog', 'vcom', 'vopt' and 'vsim' commands.
const common_flags = "-nologo -quiet"

// CompileSrcs compiles a list of sources using the specified context ctx, rule,
// dependencies and include paths. It returns the resulting dependencies and include paths
// that result from compiling the source files.
func CompileSrcs(ctx core.Context, rule SimulationQuesta, 
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
				Ins:   deps,
				Cmd:   cmd,
				Descr: fmt.Sprintf("%s: %s", tool, path.Base(src.String())),
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
func CompileIp(ctx core.Context, rule SimulationQuesta, ip Ip, 
	             deps []core.Path, incs []core.Path) ([]core.Path, []core.Path) {
	for _, sub_ip := range ip.Ips() {
		deps, incs = CompileIp(ctx, rule, sub_ip, deps, incs)
	}
	deps, incs = CompileSrcs(ctx, rule, deps, incs, ip.Sources())

	return deps, incs
}

// Compile compiles the IP dependencies and source files of a simulation rule.
func Compile(ctx core.Context, rule SimulationQuesta) []core.Path {
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
func Optimize(ctx core.Context, rule SimulationQuesta, deps []core.Path) {
	top := "board"
	if rule.Top != "" {
		top = rule.Top
	}
	
	log := rule.Path().WithSuffix("/vopt.log")
	cover_flag := ""
	if Coverage.Value() {
		log = rule.Path().WithSuffix("/vopt_cover.log")
		cover_flag = "+cover"
	}

	// Skip if we already have a rule
	if rules[log.String()] {
		return
	}

	cmd := fmt.Sprintf("vopt %s %s +acc=%s -l %s -work %s %s -o %s", 
	                   common_flags, cover_flag, Access.Value(),
	                   log.String(), rule.Lib(), top, rule.Target())

	// Add parameters for all generics
	for _, param := range rule.Params {
		cmd = cmd + " -G " + param
	}
	
	// Add the rule to run 'vopt'.
	ctx.AddBuildStep(core.BuildStep{
		Out:   log,
		Ins:   deps,
		Cmd:   cmd,
		Descr: fmt.Sprintf("vopt: %s", rule.Lib() + "." + top),
	})

	// Note that we created this rule
	rules[log.String()] = true
}

// CreateLib creates a Questa simulation library named after the rule with the
// suffix "Lib" added. It also adds the base name where source code will be
// compiled to the global list of created rules after it has added the
// build step for the library.
func CreateLib(ctx core.Context, rule SimulationQuesta) {
	path := rule.Path()

	// Skip if we already know this rule
	if rules[path.String()] {
		return
	}

	// Add the rule to run 'vlib'
	ctx.AddBuildStep(core.BuildStep{
		Out:   path.WithSuffix("Lib"),
		Cmd:   fmt.Sprintf("vlib %s", rule.Lib()),
		Descr: fmt.Sprintf("vlib: %s", rule.Lib()),
	})

	// Remember that we created this rule
	rules[path.String()] = true
}

// Build will compile and optimize the source and IPs associated with the given
// rule.
func (rule SimulationQuesta) Build(ctx core.Context) {
	// Create the library
	CreateLib(ctx, rule)

	// Compile the code
	deps := Compile(ctx, rule)

	// Optimize the code
	Optimize(ctx, rule, deps)
}

// Simulate will start 'vsim' on the compiled design with flags set in accordance
// with what is specified on the command line.
func Simulate(args []string, gui bool, rule SimulationQuesta) string {
	vsim_flags := ""
	seed_flag := " -sv_seed random"
	verbosity_flag := " +verbosity=DVM_VERB_NONE"
	testcases_flag := " +testcases=__all__"
	for _, arg := range args {
		if strings.HasPrefix(arg, "-seed=") {
			var seed int64
			if _, err := fmt.Sscanf(arg, "-seed=%d", &seed); err == nil {
				seed_flag = fmt.Sprintf(" -sv_seed %d", seed)
			} else {
				log.Fatal("-seed expects an integer argument!")
			}
		} else if strings.HasPrefix(arg, "-verbosity=") {
			var level string
			if _, err := fmt.Sscanf(arg, "-verbosity=%s", &level); err == nil {
				switch level {
				case "none":   verbosity_flag = " +verbosity=DVM_VERB_NONE"
				case "low":    verbosity_flag = " +verbosity=DVM_VERB_LOW"
				case "medium": verbosity_flag = " +verbosity=DVM_VERB_MED"
				case "high":   verbosity_flag = " +verbosity=DVM_VERB_HIGH"
				}
			} else {
				log.Fatal("-verbosity expects an argument of 'low', 'medium', 'high' or 'none'!")
			}
		}	else if strings.HasPrefix(arg, "-testcases=") {
			var testcases string
			if _, err := fmt.Sscanf(arg, "-testcases=%s", &testcases); err == nil {
				testcases_flag = " +testcases=" + testcases		
			} else {
				log.Fatal("-testcases expects a string argument!")
			}
		}
	}

	log := rule.Path().WithSuffix("/vsim.log")
	vsim_flags = vsim_flags + seed_flag + verbosity_flag + testcases_flag + " -onfinish stop" +
	             " -l " + log.String()

	cmd_extra := "" 
	if gui {
		vsim_flags = vsim_flags + " -gui"
	} else {
		vsim_flags = vsim_flags + " -batch -nostdout -quiet -do \"run -all; quit -code [coverage attribute -name TESTSTATUS -concise]\""
		cmd_extra = fmt.Sprintf("|| { cat %s; exit 1; }", log.String())
	}

	return fmt.Sprintf("vsim %s -work %s %s %s", vsim_flags, rule.Lib(), 
	                    rule.Target(), cmd_extra)
}

// Run will build the design and run a simulation in GUI mode.
func (rule SimulationQuesta) Run(args []string) string {
	return Simulate(args, true, rule)
}

// Test will build the design and run a simulation in batch mode.
func (rule SimulationQuesta) Test(args []string) string {
	return Simulate(args, false, rule)
}
