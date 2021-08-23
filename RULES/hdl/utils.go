package hdl

import (
	"strings"
)

func IsRtl(path string) bool {
	return strings.HasSuffix(path, ".v") || strings.HasSuffix(path, ".sv") || strings.HasSuffix(path, ".vhd")
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
