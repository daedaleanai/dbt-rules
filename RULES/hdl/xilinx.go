package hdl

import (
	"dbt-rules/RULES/core"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
)

type XciValue struct {
	Value string `json:"value"`
}

type XciProjectParameters struct {
	Architecture        []XciValue `json:"ARCHITECTURE"`
	BaseBoardPart       []XciValue `json:"BASE_BOARD_PART"`
	BoardConnections    []XciValue `json:"BOARD_CONNECTIONS"`
	Device              []XciValue `json:"DEVICE"`
	Package             []XciValue `json:"PACKAGE"`
	Prefhdl             []XciValue `json:"PREFHDL"`
	SiliconRevision     []XciValue `json:"SILICON_REVISION"`
	SimulatorLanguage   []XciValue `json:"SIMULATOR_LANGUAGE"`
	Speedgrade          []XciValue `json:"SPEEDGRADE"`
	StaticPower         []XciValue `json:"STATIC_POWER"`
	TemperatureGrade    []XciValue `json:"TEMPERATURE_GRADE"`
	UseRdiCustomization []XciValue `json:"USE_RDI_CUSTOMIZATION"`
	UseRdiGeneration    []XciValue `json:"USE_RDI_GENERATION"`
}

type XciParameters struct {
	ComponentParameters map[string]interface{} `json:"component_parameters"`
	ModelParameters     map[string]interface{} `json:"model_parameters"`
	ProjectParameters   XciProjectParameters   `json:"project_parameters"`
	RuntimeParameters   map[string]interface{} `json:"runtime_parameters"`
}

type XciIpInst struct {
	XciName            string                 `json:"xci_name"`
	ComponentReference string                 `json:"component_reference"`
	IpRevision         string                 `json:"ip_revision"`
	GenDirectory       string                 `json:"gen_directory"`
	Parameters         XciParameters          `json:"parameters"`
	Boundary           map[string]interface{} `json:"boundary"`
}

type Xci struct {
	Schema string    `json:"schema"`
	IpInst XciIpInst `json:"ip_inst"`
}

func ReadXci(path string) (Xci, error) {
	var result Xci

	xci_file, err := os.Open(path)
	if err == nil {
		// defer the closing of the file
		defer xci_file.Close()

		bytes, _ := ioutil.ReadAll(xci_file)

		err = json.Unmarshal([]byte(bytes), &result)
	}

	return result, err
}

type templateParams struct {
	Sources   []core.Path
	Simulator string
	Top       string
	Part      string
	Board     string
	Dir       string
	LibDir    string
	Defines   []string
	Options   []string
}

const export_ip_template = `
{{- range .Sources }}
{{- if or (hasSuffix .String ".xci") }}
set name [file tail {{ .String }}]
foreach xci [exec find .srcs -name "*.xci"] {
  if {[file tail $xci] == $name} {
    puts "Removing existing IP in $xci"
    file delete -force $xci
    break
  }
}
puts "Reading IP from {{ .String }}"
import_ip {{ .String }}
{{- end }}
{{- end }}
foreach ip [get_ips] {
  puts "Upgrade IP"
  upgrade_ip $ip
  puts "Generating IP"
  generate_target simulation $ip
	puts "Exporting IP to {{ .Dir }}"
  export_simulation -simulator {{ .Simulator }} -quiet -force -absolute_path -use_ip_compiled_libs -lib_map_path {{ .LibDir }} -of_objects $ip -step compile -directory {{ .Dir }}
}
`

const vivado_command = `#!/bin/env -S vivado -nojournal -nolog -mode batch -source`

const create_project_template = `
{{- if .Dir }}
if [file exists {{ .Dir }}] {
  file delete -force -- {{ .Dir }}
}
{{- end }}
create_project -in_memory -part {{ .Part }}
set_property target_language verilog [current_project]
set_property source_mgmt_mode All [current_project]
{{- if .Board }}
catch {set_property board_part {{ .Board }} [current_project]}
{{- end }}
`

const add_files_template = `
{{- /* add HDL source files */}}
catch {
  add_files -norecurse {
{{- range .Sources }}
{{- if or (hasSuffix .String ".v") }}
    {{ . }}
{{- end }}
{{- end }}
  }
}

{{- /* add utilities fileset */}}
if {[string equal [get_filesets -quiet utils_1] ""]} {
  create_fileset -constrset utils_1
}
catch {
  add_files -fileset utils_1 {
{{- range .Sources }}
{{- if hasSuffix .String ".tcl"}}
    {{ . }}
	{{- end }}
{{- end }}
  }
}

update_compile_order -fileset sources_1
`

const export_simulation_template = `
  set_property top {{ .Top }} [current_fileset -simset]
  export_simulation -simulator {{ .Simulator }}\
    -force -absolute_path\
    -use_ip_compiled_libs\
    -lib_map_path {{ .LibDir }}\
    -step compile\
{{- if .Defines }}
    -define [list\
{{- range .Defines }}
      { {{- . -}} }\
{{- end }}
    ]\
{{- end }}
{{- if .Options }}
    -more_options [list\
{{- range .Options }}
      { {{- $.Simulator }}.compile.{{ . -}} }\
{{- end }}
    ]\
{{- end }}
    -directory {{ .Dir }}\
`

const source_utils = `
foreach f [get_files -of [get_filesets utils_1]] {
  if {[string match *_pre_*.tcl $f] || [string match *_post_*.tcl $f]} {
    continue
  } else {
    puts "INFO: Sourcing utility file $f"
    source $f
  }
}
`

func ExportXilinxIpCheckpoint(ctx core.Context, rule Simulation, src core.Path, def DefineMap, flags FlagMap) core.Path {
	xci, err := ReadXci(src.String())
	if err != nil {
		log.Fatal(fmt.Sprintf("unable to read XCI file %s", src.Relative()))
	}

	if SimulatorLibDir.Value() == "" {
		log.Fatal("hdl-simulator-lib-dir must be set when compiling XCI files!")
	}

	part := xci.IpInst.Parameters.ProjectParameters.Device[0].Value + "-" +
		xci.IpInst.Parameters.ProjectParameters.Package[0].Value +
		xci.IpInst.Parameters.ProjectParameters.Speedgrade[0].Value

	if xci.IpInst.Parameters.ProjectParameters.TemperatureGrade[0].Value != "" {
		part = part + "-" + xci.IpInst.Parameters.ProjectParameters.TemperatureGrade[0].Value
	}

	defines := []string{"SIMULATION"}
	for key, value := range def {
		if value != "" {
			defines = append(defines, fmt.Sprintf("%s=%s", key, value))
		} else {
			defines = append(defines, key)
		}
	}

	options := []string{}
	for tool, option := range flags {
		options = append(options, fmt.Sprintf("%s:%s", tool, option))
	}

	// Determine name of .do file
	oldExt := path.Ext(src.Relative())
	newRel := strings.TrimSuffix(src.Relative(), oldExt)
	dir := core.BuildPath(path.Dir(src.Relative()))
	do := core.BuildPath(newRel).WithSuffix(fmt.Sprintf("/%s/compile.do", Simulator.Value()))

	// Template parameters are the direct and parent script sources.
	data := templateParams{
		Sources:   []core.Path{src},
		Dir:       dir.Absolute(),
		Part:      strings.ToLower(part),
		Simulator: Simulator.Value(),
		LibDir:    SimulatorLibDir.Value(),
		Defines:   defines,
		Options:   options,
	}

	ctx.AddBuildStep(core.BuildStep{
		Out:    do,
		In:     src,
		Script: core.CompileTemplate(vivado_command+create_project_template+export_ip_template, "export_ip", data),
		Descr:  fmt.Sprintf("export: %s", src.Relative()),
	})

	return do
}

type BlockDesign struct {
	Library
	Top   string
	Part  string
	Board string
}

func ExportBlockDesign(ctx core.Context, rule BlockDesign, def DefineMap, flags FlagMap) core.Path {
	// Get all Verilog sources files
	sources := rule.FilterSources(".tcl")
	sources = append(sources, rule.FilterSources(".v")...)

	// Select a suitable part
	part := PartName.Value()
	if rule.Part != "" {
		part = rule.Part
	}

	board := BoardName.Value()
	if rule.Board != "" {
		board = rule.Board
	}

	defines := []string{"SIMULATION"}
	for key, value := range def {
		if value != "" {
			defines = append(defines, fmt.Sprintf("%s=%s", key, value))
		} else {
			defines = append(defines, key)
		}
	}

	options := []string{}
	for tool, option := range flags {
		options = append(options, fmt.Sprintf("%s:%s", tool, option))
	}

	// Template parameters are the direct and parent script sources.
	data := templateParams{
		Sources:   sources,
		Dir:       ctx.Cwd().Absolute(),
		Top:       rule.Top,
		Part:      strings.ToLower(part),
		Board:     strings.ToLower(board),
		Simulator: Simulator.Value(),
		LibDir:    SimulatorLibDir.Value(),
		Defines:   defines,
		Options:   options,
	}

	do := ctx.Cwd().WithSuffix(fmt.Sprintf("/%s/compile.do", Simulator.Value()))

	ctx.AddBuildStep(core.BuildStep{
		Ins: sources,
		Out: do,
		Script: core.CompileTemplate(
			vivado_command+
				create_project_template+
				add_files_template+
				source_utils+
				rule.Top+
				export_simulation_template, "export_bd", data),
		Descr: fmt.Sprintf("export: %s", rule.Top),
	})

	return do
}
