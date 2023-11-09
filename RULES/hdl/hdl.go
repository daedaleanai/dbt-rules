package hdl

import (
	"dbt-rules/RULES/core"
	"strings"
)

var BoardName = core.StringFlag{
	Name: "board",
	DefaultFn: func() string {
		return ""
	},
}.Register()

var PartName = core.StringFlag{
	Name: "part",
	DefaultFn: func() string {
		return "xczu3eg-sbva484-1-e"
	},
}.Register()

type FlagMap map[string]string

type Ip interface {
	Sources() []core.Path
	Data() []core.Path
	Ips() []Ip
	Flags() FlagMap
	AllSources() []core.Path
	FilterSources(string) []core.Path
	filterSources(map[string]bool, []core.Path, string) (map[string]bool, []core.Path)
}

type Library struct {
	Srcs      []core.Path
	DataFiles []core.Path
	IpDeps    []Ip
	ToolFlags FlagMap
}

func (lib Library) Sources() []core.Path {
	return lib.Srcs
}

func (lib Library) Data() []core.Path {
	return lib.DataFiles
}

func (lib Library) Ips() []Ip {
	return lib.IpDeps
}

func (lib Library) Flags() FlagMap {
	return lib.ToolFlags
}

// Get all sources from a target, including listed IPs.
func (lib Library) AllSources() []core.Path {
	return lib.FilterSources("")
}

// Get all sources from a target that match a filter pattern, including listed IPs.
func (lib Library) FilterSources(suffix string) []core.Path {
	_, sources := lib.filterSources(map[string]bool{}, []core.Path{}, suffix)
	return sources
}

// Get sources from a target, including listed IPs recursively.
// Takes a slice of sources and a map of sources we have already seen and adds everything new from the current rule.
func (lib Library) filterSources(seen map[string]bool, sources []core.Path, suffix string) (map[string]bool, []core.Path) {
	// Add sources from dependent IPs
	for _, ipDep := range lib.Ips() {
		seen, sources = ipDep.filterSources(seen, sources, suffix)
	}

	// Add sources
	for _, source := range lib.Sources() {
		if (suffix != "") && strings.HasSuffix(source.String(), suffix) {
			if _, ok := seen[source.String()]; !ok {
				seen[source.String()] = true
				sources = append(sources, source)
			}
		}
	}

	return seen, sources
}
