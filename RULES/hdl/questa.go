package hdl

import (
	"fmt"
	"log"
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
var VoptFlags = core.StringFlag{
	Name: "questa-vopt-flags",
	DefaultFn: func() string {
		return "-fsmverbose"
	},
	Description: "Extra flags for the vopt command",
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

// Designfile enables the generation of a binary designfile for use with the visualizer
var Designfile = core.BoolFlag{
	Name: "questa-designfile",
	DefaultFn: func() bool {
		return false
	},
	Description: "Enable the creation of a binary designfile database for use with the visualizer",
}.Register()

// Access enables the user to control the accessibility in the compiled design for
// debugging purposes.
var Access = core.StringFlag{
	Name: "questa-access",
	DefaultFn: func() string {
		return "acc"
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

// Coverage enables the user to run the simulation with code coverage.
var DumpQwavedb = core.BoolFlag{
	Name: "questa-dump-qwavedb",
	DefaultFn: func() bool {
		return false
	},
	Description: "Enable waveform dumping to qwavedb file",
}.Register()

var DumpQwavedbScope = core.StringFlag{
	Name: "questa-dump-qwavedb-scope",
	DefaultFn: func() string {
		return "all"
	},
	Description:   "Control the scope of data dumped to qwavedb file",
	AllowedValues: []string{"all", "signals", "assertions", "memory", "queues"},
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
type ShFileParams struct {
	Name             string
	Work             string
	Path             string
	Visualizer       bool
	Target           string
	Params           []string
	LibFlags         string
	DbFlags          string
	Coverage         bool
	TestFiles        []string
	TestFilesDir     string
	TestFileGen      string
	TestFileGenFlags string
	TestFileElf      string
}

func (sh ShFileParams) AsArray(strs []string) string {
	return strings.Trim(strings.Join(strs, "\\n"), " ")
}

// Sh-file template
const sh_file_template = `#!/usr/bin/env bash
#================================================================
# HEADER
#================================================================
#% SYNOPSIS
#+    ${SCRIPT_NAME} [OPTION]...
#%
#% DESCRIPTION
#%    Run Questa Sim on {{ .Work }}/{{ .Name }}  
#%
#% OPTIONS
{{- if .TestFiles }}
#%    -f, --testfile=<string>       Select the test files to execute
{{ end }}
#%    -g, --gui                     Open GUI
#%    -h, --help                    Print this help
{{- if .Params }}
#%    -p, --params=<string>         Select parameter set to run 
{{ end }}
#%    -v, --verbosity=<string>      Control verbosity
#%
#% EXAMPLES
#%    ${SCRIPT_NAME}
#%        Runs all available parameter sets. 
#%
#%    ${SCRIPT_NAME} -p fast
#%        Runs only the fast parameter set.
#%
#================================================================
#- IMPLEMENTATION
#-    version         ${SCRIPT_NAME} (www.daedalean.ai) 1.0.1
#-    author          Niels Haandbaek
#-    copyright       Copyright (c) http://www.daedalean.ai
#-
#================================================================
#  DEBUG OPTION
#    set -n  # If you uncomment this line, the script will not execute,
#            # but the syntax will still be checked.
#    set -x  # If you uncomment this line the script will produce additional
#            # debug output to stdout.
#================================================================
# END_OF_HEADER
#================================================================

# Needed variables
SCRIPT_HEADSIZE=$(head -200 ${0} | grep -n "^# END_OF_HEADER" | cut -f1 -d:)
SCRIPT_NAME="$(basename ${0})"

#================================================================
# Utility functions
#================================================================

fecho() {
  _Type=${1} ; shift ;
  printf "[${_Type%[A-Z][A-Z]}] ${*}\n"
}

# Error management functions
info() { fecho INF "${*}"; }
warning() { fecho WRN "WARNING: ${*}" 1>&2 ; }
error() { fecho ERR "ERROR: ${*}" 1>&2 ; }
debug() { [[ ${flag_debug} -ne 0 ]] && fecho DBG "DEBUG: ${*}" 1>&2; }

# Usage functions
usage() {
  printf "Usage: "
  head -${SCRIPT_HEADSIZE:-99} ${0} | grep -e "^#+" | sed -e "s/^#+[ ]*//g" -e "s/\${SCRIPT_NAME}/${SCRIPT_NAME}/g"
}
usagefull() { head -${SCRIPT_HEADSIZE:-99} ${0} | grep -e "^#[%+-]" | sed -e "s/^#[%+-]//g" -e "s/\${SCRIPT_NAME}/${SCRIPT_NAME}/g"; }
scriptinfo() { head -${SCRIPT_HEADSIZE:-99} ${0} | grep -e "^#-" | sed -e "s/^#-//g" -e "s/\${SCRIPT_NAME}/${SCRIPT_NAME}/g"; }

# complain to STDERR and exit with error
die() {
  echo "$*" >&2
  exit 2
}

needs_arg() {
  if [[ -z "$OPTARG" ]]; then
    die "No arg for --$OPT option"
  fi
}

function ctrl_c() {
  cd - > /dev/null
  echo "Interrupted"
  exit 1
}

#================================================================
# Argument parsing
#================================================================

flag_debug=0
flag_gui=0
flag_quiet=0
{{- if .TestFiles }}
testfiles_dir={{ .TestFilesDir }}
all_testfiles=(\
  {{- range .TestFiles }}
  {{ . }}\
  {{- end }}
)
{{- else }}
testfiles_dir=""
all_testfiles=("_")
{{- end }}
{{- if .Params }}
all_params=(\
  {{- range .Params }}
  {{ . }}\
  {{- end }}
)
{{- else }}
all_params=("_")
{{- end }}
seed="-sv_seed random"
params=()
testfiles=()
otherargs=()

while getopts f:gho:p:qs:-: OPT; do
  # support long options: https://stackoverflow.com/a/28466267/519360
  if [[ "$OPT" = "-" ]]; then # long option: reformulate OPT and OPTARG
    OPT="${OPTARG%%=*}"       # extract long option name
    OPTARG="${OPTARG#$OPT}"   # extract long option argument (may be empty)
  fi
  case "$OPT" in
    f | testfile )   needs_arg; testfiles[${#testfiles[@]}]="$OPTARG" ;;
    g | gui )        flag_gui=1 ;;
    h | help | man ) usagefull; exit 0 ;;
    o | otherargs )  needs_arg; otherargs[${#otherargs[@]}]="$OPTARG" ;;
    p | params )     needs_arg; params[${#params[@]}]="$OPTARG" ;;
    q | quiet )      flag_quiet=1 ;;
    s | seed )       needs_arg; seed="-sv_seed $OPTARG" ;;
    ??* )            die "Illegal option --$OPT" ;;  # bad long option
    ? )              exit 2 ;;  # bad short option (error reported via getopts)
  esac
done
shift $((OPTIND-1)) # remove parsed options and args from $@ list

if [ ${#testfiles[@]} -eq 0 ]; then
  testfiles=("${all_testfiles[@]}")
fi

if [ ${#params[@]} -eq 0 ]; then
  params=("${all_params[@]}")
fi

# Store current directory 
cwd=${PWD}
cd {{ .Path }}

# Make sure we get back to where we started on Ctrl-C
trap ctrl_c INT

for p in ${params[@]}; do
  # Run for each parameter
  if [[ ${p} != "_" ]]; then
    target="${p}_{{ .Target }}"
    main_coverage_db="{{ .Name }}_${p}"
  else
    target="{{ .Target }}"
    main_coverage_db="{{ .Name }}"
  fi

  if [[ $flag_gui -eq 0 ]]; then
    mode="-batch -quiet"
  else
    {{- if .Visualizer }}
    mode="-visualizer=+designfile={{ .Path }}/{{ .Name }}/${target}.bin"
    {{- else }}
    mode="-gui"
    {{- end }}
  fi

  if [[ $flag_gui -eq 0 && $flag_quiet -eq 1 ]]; then
    nostdout="-nostdout"
  else
    nostdout=""
  fi

  for tf in ${testfiles[@]}; do
    # Run for each test file
    echo -n "Testcase "
    logfile=""

    if [[ ${p} != "_" ]]; then
      logfile="${logfile}${p}_"
      echo -n "${p}"
    fi

    if [[ ${tf} == "_" ]]; then
      testfile="default"
      coverage_db=${main_coverage_db}
      echo -n ":"
      {{- if .TestFileGen }}
      {{ .TestFileGen }} .
      {{- end }}
    else
      testfile="${tf##*/}"
      testfile="${testfile%.*}"
      coverage_db="${main_coverage_db}_${testfile}"
      if [[ ${p} != "" ]]; then 
        echo -n "/${testfile}:"
      else
        echo -n "${testfile}:"
      fi
      logfile="${logfile}${testfile}_"
      testgen_logfile="${logfile}testgen.log"
      {{- if .TestFileGen }}
      flags="{{ .TestFileGenFlags }}"
      if [[ $tf == *.json ]]; then
        flags="${flags} -test"
      fi
      {{ .TestFileGen }} $flags ${testfiles_dir}/${tf} -out {{ .TestFileElf }} > ${testgen_logfile} 2>&1
      {{- else }}
      ${testfiles_dir}/${tf} > ${testgen_logfile} 2>&1
      {{- end }}
      if [[ $? -ne 0 ]]; then
        cat ${testgen_logfile}; 
        cd ${cwd} > /dev/null
        exit 1;
      fi
    fi
  
    vsim_logfile="${logfile}vsim.log"

    # Run simulation
    vsim \
      -logfile ${vsim_logfile}\
      -modelsimini {{ .Path }}/{{ .Name }}/modelsim.ini\
      -onfinish final\
      $nostdout\
      $seed\
      {{- if .Coverage }}
      -coverage -assertdebug\
      -do "set coverage 1"\
      -do "set main_coverage_db ${main_coverage_db}"\
      -do "set coverage_db ${coverage_db}"\
      {{- end }}
      -do "set gui ${flag_gui}"\
      {{- if .LibFlags }}
      {{ .LibFlags }}\
      {{- end }}
      {{- if .DbFlags }}
      {{ .DbFlags }}\
      {{- end }}
      -do "set target ${target}"\
      -do "set testfile ${testfile}"\
      ${otherargs[@]}\
      -do {{ .Path }}/{{ .Name }}/vsim.do\
      -work {{ .Work }}\
      ${mode}\
      ${target}
    if [[ $? -eq 0 ]]; then
      echo -n "PASS"
    else
      if [[ ${flag_quiet} -eq 1 ]]; then
        cat ${vsim_logfile}
        echo
      fi
      echo "FAIL"
      cd ${cwd} > /dev/null
      exit 1
    fi
    {{- if .Coverage }}
    echo " Coverage: {{ .Path }}/{{ .Name }}/${main_coverage_db}.ucdb"
    {{- else }}
    echo ""
    {{- end }}
  done
done

# Return to original directory
cd ${cwd} > /dev/null
`

// Parameters of the do-file
type DoFileParams struct {
	WaveformInit string
	DumpVcdFile  string
	CovFiles     string
}

// Do-file template
const do_file_template = `# Disable warnings from standard packages
set StdArithNoWarnings 1
set NumericStdNoWarnings 1

{{ if .WaveformInit }}
if {[info exists gui] && $gui} {
	catch { source {{ .WaveformInit }} }
	assertion fail -action break
}
{{ end }}

if [info exists from] {
	run $from
}

{{ if .DumpVcdFile }}
vcd file {{ .DumpVcdFile }}
vcd add -r *
{{ end }}

if [info exists to] {
	run @$to
} else {
	run -all
}

{{ if .DumpVcdFile }}
vcd flush
{{ end }}

if [info exists coverage] {
	# Create coverage database
	coverage save -assert -directive -cvg -codeall -testname $testfile ${coverage_db}.ucdb
	# Optionally merge coverage databases
	if {$main_coverage_db != $coverage_db} {
		puts "Writing merged coverage database to [file normalize $main_coverage_db.ucdb]"
		vcover merge -testassociated -output ${main_coverage_db}.ucdb \
      ${main_coverage_db}.ucdb ${coverage_db}.ucdb
	}
	# Create HTML coverage report
	vcover report -html -output ${main_coverage_db}_covhtml \
		-testdetails -details -assert -directive -cvg -codeAll $main_coverage_db.ucdb
	# Create textual code coverage report
	{{ if .CovFiles }}
	vcover report -output ${main_coverage_db}_covcode.txt -srcfile={{ .CovFiles }}\
		-codeAll ${main_coverage_db}.ucdb
	{{ else }}
	vcover report -output ${main_coverage_db}_covcode.txt\
		-codeAll ${main_coverage_db}.ucdb
	{{ end }}
	# Create textual assertion coverage report
	puts "Writing coverage report to [file normalize ${main_coverage_db}_cover.txt]"
	vcover report -output ${main_coverage_db}_cover.txt -flat \
    -directive -cvg ${main_coverage_db}.ucdb
	# Create textural assertion report
	puts "Writing assertion report to [file normalize ${main_coverage_db}_cover.txt]"
	vcover report -output ${main_coverage_db}_assert.txt -flat\
    -assert ${main_coverage_db}.ucdb
}

if {![info exists gui] || !$gui} {
	# Report error in case status > 1 (WARNING)
	quit -code [expr [coverage attribute -name TESTSTATUS -concise] > 1]
}
`

func createModelsimIni(ctx core.Context, rule Simulation, deps []core.Path) []core.Path {
	log_file := rule.Path().WithSuffix("/vmap.log")

	cmds := []string{
		fmt.Sprintf("cd %s", rule.Path()),
		fmt.Sprintf("rm -f %s", log_file.String()),
		fmt.Sprintf("vlib %s >> %s 2>&1", rule.Path().WithSuffix("/"+rule.Lib()), log_file.String()),
		fmt.Sprintf("vmap %s %s >> %s 2>&1", rule.Lib(), rule.Path().WithSuffix("/"+rule.Lib()), log_file.String()),
	}

	if SimulatorLibDir.Value() != "" {
		for _, lib := range rule.Libs {
			cmds = append(cmds, fmt.Sprintf("vmap %s %s/%s", lib, SimulatorLibDir.Value(), lib))
		}
	}

	modelsim_ini := rule.Path().WithSuffix("/modelsim.ini")
	ctx.AddBuildStep(core.BuildStep{
		Out:   modelsim_ini,
		Cmd:   strings.Join(cmds, " && "),
		Descr: fmt.Sprintf("vmap: %s", modelsim_ini.Absolute()),
	})
	deps = append(deps, modelsim_ini)

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

			cmd := fmt.Sprintf("%s -work %s -logfile %s -modelsimini %s",
				common_flags, rule.Lib(), log.String(), rule.Path().WithSuffix("/modelsim.ini"))

			// tool will point to the tool to execute (also used for logging below)
			var tool string
			if IsVerilog(src.String()) {
				tool = "vlog"
				cmd += " " + VlogFlags.Value()
				cmd += "\\\n    -suppress 2583 -svinputport=net -define SIMULATION"
				if rule.libFlags() != "" {
					cmd += "\\\n    " + rule.libFlags()
				}
				cmd += "\\\n    +incdir+" + core.SourcePath("").String()
				seen_incs := make(map[string]struct{})
				for _, inc := range incs {
					inc_path := path.Dir(inc.Absolute())
					if _, ok := seen_incs[inc_path]; !ok {
						cmd += "\\\n    +incdir+" + inc_path
						seen_incs[inc_path] = struct{}{}
					}
				}
				if flags != nil {
					if vlog_flags, ok := flags["vlog"]; ok {
						cmd += " " + vlog_flags
					}
				}
				for key, value := range rule.Defines {
					cmd += "\\\n    -define " + key
					if value != "" {
						cmd += "=" + value
					}
				}
			} else if IsVhdl(src.String()) {
				tool = "vcom"
				cmd += " " + VcomFlags.Value()
				if flags != nil {
					if vcom_flags, ok := flags["vcom"]; ok {
						cmd += " " + vcom_flags
					}
				}
			}

			if Lint.Value() {
				cmd += "\\\n    -lint"
			}

			// Create plain compilation command
			cmd = tool + " " + cmd + "\\\n    " + src.String()

			// Add the source file to the dependencies
			deps = append(deps, src)

			script := "#!/usr/bin/env bash\n" + cmd + "\n"
			script += "if [ $? -ne 0 ]; then\n"
			script += "  rm " + log.String() + "\n"
			script += "  exit 1" + "\n"
			script += "fi"

			// Add the compilation command as a build step with the log file as the
			// generated output
			ctx.AddBuildStep(core.BuildStep{
				Out:    log,
				Ins:    deps,
				Script: script,
				Descr:  fmt.Sprintf("%s: %s", tool, src.Relative()),
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
			access_flag = "-debug"
		} else if Access.Value() == "livesim" {
			access_flag = "-debug,livesim"
		} else if Access.Value() == "acc" {
			access_flag = "+acc"
		} else if Access.Value() != "" {
			access_flag = fmt.Sprintf("+acc=%s", Access.Value())
		}

		// Generate designfile flag
		designfile_flag := ""
		if Designfile.Value() {
			designfile_flag = "-designfile " + rule.Path().WithSuffix("/"+target+".bin").String()
		}

		cmd := "vopt " + common_flags
		cmd += "\\\n    -modelsimini " + rule.Path().WithSuffix("/modelsim.ini").String()
		cmd += "\\\n    " + VoptFlags.Value()
		cmd += "\\\n    " + cover_flag
		cmd += "\\\n    " + access_flag
		cmd += "\\\n    " + designfile_flag
		cmd += "\\\n    -logfile " + log_file.String()
		cmd += "\\\n    -work " + rule.Lib()
		cmd += "\\\n    " + top
		cmd += "\\\n    " + additional_tops
		cmd += "\\\n    " + rule.libFlags()
		cmd += "\\\n    -o " + target
		cmd += "\\\n>> " + log_file.String() + " 2>&1"

		// Set up parameters
		if param_set != "" {
			// Check that the parameters exist
			if params, ok := rule.Params[param_set]; ok {
				// Add parameters for all generics
				for param, value := range params {
					cmd += fmt.Sprintf("\\\n    -g %s=%s", param, value)
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
			Out:    log_file,
			Ins:    deps,
			Script: cmd,
			Descr:  fmt.Sprintf("vopt: %s %s", rule.Lib()+"."+top, target),
		})

		// Note that we created this rule
		rules[log_file.String()] = true
	}
}

// Create a simulation script
func doFile(ctx core.Context, rule Simulation) {
	// Do-file script
	params := DoFileParams{
		CovFiles: strings.Join(rule.ReportCovFiles(), "+"),
	}

	if rule.WaveformInit != nil {
		params.WaveformInit = rule.WaveformInit.String()
	}

	if DumpVcd.Value() {
		params.DumpVcdFile = fmt.Sprintf("%s.vcd.gz", rule.Name)
	}

	doFile := rule.Path().WithSuffix("/" + "vsim.do")
	ctx.AddBuildStep(core.BuildStep{
		Out:   doFile,
		Data:  core.CompileTemplate(do_file_template, "do", params),
		Descr: fmt.Sprintf("vsim: %s", doFile.Relative()),
	})
}

// Create a script for starting the simulation
func simulationFile(ctx core.Context, rule Simulation) {
	var keys []string
	for k := range rule.Params {
		keys = append(keys, k)
	}

	params := ShFileParams{
		Name:             rule.Name,
		Work:             rule.Lib(),
		Target:           rule.Target(),
		Path:             core.BuildPath("").Absolute(),
		Params:           keys,
		Coverage:         Coverage.Value(),
		Visualizer:       Designfile.Value(),
		LibFlags:         rule.libFlags(),
		TestFiles:        rule.TestFiles(false),
		TestFileGenFlags: rule.TestCaseGeneratorFlags,
	}

	if rule.TestCaseGenerator != nil {
		params.TestFileGen = rule.TestCaseGenerator.String()
	}

	if rule.TestCaseElf != nil {
		params.TestFileElf = rule.TestCaseElf.String()
	}

	if rule.TestCasesDir != nil {
		params.TestFilesDir = rule.TestCasesDir.Absolute()
	}

	// Enable qwavedb dumping
	if DumpQwavedb.Value() {
		params.DbFlags = "-qwavedb="
		switch DumpQwavedbScope.Value() {
		case "signals":
			params.DbFlags += "+signal"
		case "assertions":
			params.DbFlags += "+signal+assertions=pass,atv"
		case "memory":
			params.DbFlags += "+signal+assertions=pass,atv+memory"
		case "queues":
			params.DbFlags += "+signal+assertions=pass,atv+memory+queues"
		case "all":
			params.DbFlags += "+signal+assertions=pass,atv+memory+queues+class+classmemory+classdynarray"
		}
		params.DbFlags += "+wavefile=" + rule.Target() + ".db"
	}

	shFile := rule.Path().WithSuffix("/" + "vsim.sh")
	ctx.AddBuildStep(core.BuildStep{
		Out:          shFile,
		Data:         core.CompileTemplate(sh_file_template, "sh", params),
		DataFileMode: 0755,
		Descr:        fmt.Sprintf("vsim: %s", shFile.Relative()),
	})
}

// BuildQuesta will compile and optimize the source and IPs associated with the given
// rule.
func BuildQuesta(ctx core.Context, rule Simulation) {
	// compile the code
	deps := compile(ctx, rule)

	// optimize the code
	optimize(ctx, rule, deps)

	// Create simulation command file
	simulationFile(ctx, rule)

	// Create simulation script
	doFile(ctx, rule)
}

// verbosityLevelToFlag takes a verbosity level of none, low, medium or high and
// converts it to the corresponding DVM_ level.
func verbosityLevelToFlag(level string) (string, bool) {
	var verbosity_flag string
	var print_output bool
	switch level {
	case "none":
		verbosity_flag = "+verbosity=DVM_VERB_NONE"
		print_output = false
	case "low":
		verbosity_flag = "+verbosity=DVM_VERB_LOW"
		print_output = true
	case "medium":
		verbosity_flag = "+verbosity=DVM_VERB_MED"
		print_output = true
	case "high":
		verbosity_flag = "+verbosity=DVM_VERB_HIGH"
		print_output = true
	case "all":
		verbosity_flag = "+verbosity=DVM_VERB_ALL"
		print_output = true
	default:
		log.Fatal(fmt.Sprintf("invalid verbosity flag '%s', only 'low', 'medium',"+
			" 'high', 'all'  or 'none' allowed!", level))
	}

	return verbosity_flag, print_output
}

// simulateQuesta will create a command to start 'vsim' on the compiled design
// with flags set in accordance with what is specified on the command line. It will
// optionally build a chain of commands in case the rule has parameters, but
// no parameters are specified on the command line
func simulateQuesta(rule Simulation, args []string, gui bool) string {
	// Control verbosity
	verbosity_flag := "+verbosity=DVM_VERB_NONE"
	// Control whether to print anything
	print_output := false

	// Optional testcase goes here
	testcases := []string{}

	// Optional parameter set goes here
	params := []string{}

	// Optional all other flags
	other_flags := []string{}

	// Parse additional arguments
	for _, arg := range args {
		if strings.HasPrefix(arg, "-seed=") {
			// Define simulator seed
			var seed int64
			if _, err := fmt.Sscanf(arg, "-seed=%d", &seed); err == nil {
				other_flags = append(other_flags, fmt.Sprintf("-s %d", seed))
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
				other_flags = append(other_flags, fmt.Sprintf("\"set from %s\"", from))
			} else {
				log.Fatal("-from expects an argument of '<timesteps>[<time units>]'!")
			}
		} else if strings.HasPrefix(arg, "-to=") {
			// Define how long to run
			var to string
			if _, err := fmt.Sscanf(arg, "-to=%s", &to); err == nil {
				other_flags = append(other_flags, fmt.Sprintf("\"set to %s\"", to))
			} else {
				log.Fatal("-to expects an argument of '<timesteps>[<time units>]'!")
			}
		} else if strings.HasPrefix(arg, "+") {
			// All '+' arguments go directly to the simulator
			other_flags = append(other_flags, arg)
		} else if strings.HasPrefix(arg, "-testcases=") && rule.TestCaseGenerator != nil {
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

	cmd := "-o " + verbosity_flag

	if len(testcases) > 0 {
		cmd += " -f " + strings.Join(testcases, " -f -")
	}

	if len(params) > 0 {
		cmd += " -p " + strings.Join(params, " -p ")
	}

	if len(other_flags) > 0 {
		cmd += " -o " + strings.Join(other_flags, " -o ") 
	}

	if !print_output {
		cmd += " -q"
	}

	return rule.Path().String() + "/vsim.sh " + cmd
}

// Run will build the design and run a simulation in GUI mode.
func RunQuesta(rule Simulation, args []string) string {
	return simulateQuesta(rule, args, true)
}

// Test will build the design and run a simulation in batch mode.
func TestQuesta(rule Simulation, args []string) string {
	return simulateQuesta(rule, args, false)
}
