package hdl

import (
	"strings"
)

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
		strings.HasSuffix(path, ".svh")
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
