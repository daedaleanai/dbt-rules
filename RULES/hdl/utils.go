package hdl

import (
  "encoding/json"
  "io/ioutil"
  "os"
	"strings"
)

type XciValue struct {
  Value string `json:"value"`
}

type XciProjectParameters struct {
  Architecture []XciValue `json:"ARCHITECTURE"`
  BaseBoardPart []XciValue `json:"BASE_BOARD_PART"`
  BoardConnections []XciValue `json:"BOARD_CONNECTIONS"`
  Device []XciValue `json:"DEVICE"`
  Package []XciValue `json:"PACKAGE"`
  Prefhdl []XciValue `json:"PREFHDL"`
  SiliconRevision []XciValue `json:"SILICON_REVISION"`
  SimulatorLanguage []XciValue `json:"SIMULATOR_LANGUAGE"`
  Speedgrade []XciValue `json:"SPEEDGRADE"`
  StaticPower []XciValue `json:"STATIC_POWER"`
  TemperatureGrade []XciValue `json:"TEMPERATURE_GRADE"`
  UseRdiCustomization []XciValue `json:"USE_RDI_CUSTOMIZATION"`
  UseRdiGeneration []XciValue `json:"USE_RDI_GENERATION"`
}

type XciParameters struct {
  ComponentParameters map[string]interface{} `json:"component_parameters"`
  ModelParameters map[string]interface{} `json:"model_parameters"`
  ProjectParameters XciProjectParameters `json:"project_parameters"`
  RuntimeParameters map[string]interface{} `json:"runtime_parameters"`
}

type XciIpInst struct {
  XciName string `json:"xci_name"`
  ComponentReference string `json:"component_reference"`
  IpRevision string `json:"ip_revision"`
  GenDirectory string `json:"gen_directory"`
  Parameters XciParameters `json:"parameters"`
  Boundary map[string]interface{} `json:"boundary"`
}

type Xci struct {
  Schema string `json:"schema"`
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

func IsRtl(path string) bool {
	return strings.HasSuffix(path, ".v") ||
		strings.HasSuffix(path, ".sv") ||
		strings.HasSuffix(path, ".vhdl") ||
		strings.HasSuffix(path, ".vhd")
}

func IsVerilog(path string) bool {
	return strings.HasSuffix(path, ".v") ||
		strings.HasSuffix(path, ".sv")
}

func IsSystemVerilog(path string) bool {
	return strings.HasSuffix(path, ".sv")
}

func IsVhdl(path string) bool {
	return strings.HasSuffix(path, ".vhdl") ||
		strings.HasSuffix(path, ".vhd")
}

func IsHeader(path string) bool {
	return strings.HasSuffix(path, ".vh") ||
		strings.HasSuffix(path, ".svh") ||
		strings.HasSuffix(path, ".svp")
}

func IsConstraint(path string) bool {
	return strings.HasSuffix(path, ".xdc")
}

func IsXilinxIpCheckpoint(path string) bool {
	return strings.HasSuffix(path, ".xci")
}

func IsSimulationArchive(path string) bool {
	return strings.HasSuffix(path, ".sim.tar.gz")
}
