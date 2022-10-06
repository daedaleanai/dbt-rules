package hdl

import (
	"reflect"

	"dbt-rules/RULES/core"
)

var BoardName = core.StringFlag{
	Name: "board",
	DefaultFn: func() string {
		return "em.avnet.com:ultra96v2:part0:1.0"
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
	ReportCovFiles() []string
}

type Library struct {
	Lib       string
	Srcs      []core.Path
	DataFiles []core.Path
	IpDeps    []Ip
	ToolFlags FlagMap
	ReportCov bool
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

func (lib Library) ReportCovFiles() []string {
	files := []string{}
	if lib.ReportCov {
		for _, src := range lib.Srcs {
			files = append(files, src.Absolute())
		}
	}
	for _, ip := range lib.IpDeps {
		files = append(files, ip.ReportCovFiles()...)
	}

	// Remove duplicates
	set := make(map[string]bool)
	for _, file := range files {
		set[file] = true
	}

	// Convert back to string list
	keys := reflect.ValueOf(set).MapKeys()
	str_keys := make([]string, len(keys))
	for i := 0; i < len(keys); i++ {
		str_keys[i] = keys[i].String()
	}

	return str_keys
}