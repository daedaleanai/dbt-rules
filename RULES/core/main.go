package core

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"sort"
	"unicode"
)

const inputFileName = "input.json"
const outputFileName = "output.json"

type targetInfo struct {
	Description string
	Runnable    bool
	Testable    bool
	Report		bool
}

type generatorInput struct {
	DbtVersion      version
	SourceDir       string
	WorkingDir      string
	OutputDir       string
	CmdlineFlags    map[string]string
	WorkspaceFlags  map[string]string
	CompletionsOnly bool
	RunArgs         []string
	TestArgs        []string
	Layout          string
	SelectedTargets        []string
}

type generatorOutput struct {
	NinjaFile string
	Targets   map[string]targetInfo
	Flags     map[string]flagInfo
}

var input = loadInput()



func GeneratorMain(vars map[string]interface{}) {
	output := generatorOutput{
		Targets: map[string]targetInfo{},
		Flags:   lockAndGetFlags(),
	}

	for targetPath, variable := range vars {
		targetName := path.Base(targetPath)
		if !unicode.IsUpper([]rune(targetName)[0]) {
			continue
		}
		if _, ok := variable.(buildInterface); !ok {
			continue
		}
		info := targetInfo{}
		if descriptionIface, ok := variable.(descriptionInterface); ok {
			info.Description = descriptionIface.Description()
		}
		if _, ok := variable.(runInterface); ok {
			info.Runnable = true
		}
		if _, ok := variable.(testInterface); ok {
			info.Testable = true
		}
		if _, ok := variable.(coverageReportInterface); ok {
			info.Report = true
		}
		output.Targets[targetPath] = info
	}

	// Create build files.
	if !input.CompletionsOnly {
		ctx := newContext(vars)

		// Making sure targets are processed in a deterministic order
		targetPaths := []string{}
		for targetPath := range vars {
			targetPaths = append(targetPaths, targetPath)
		}
		sort.Strings(targetPaths)

		var targetsForCoverage = []CoverageInterface{}

		for _, targetPath := range targetPaths {
			tgt := vars[targetPath]
			if cov, ok := tgt.(CoverageInterface); ok {
				var selected = false
				for _,path := range input.SelectedTargets {
					if path == targetPath {
						selected = true
						break
					}
				}
				if !selected {
					continue
				}
				targetsForCoverage = append(targetsForCoverage, cov)
			}
		}

		for _, targetPath := range targetPaths {
			tgt := vars[targetPath]
			if build, ok := tgt.(coverageReportInterface); ok {
				tgt = build.CoverageReport(targetsForCoverage)
			}

			if build, ok := tgt.(buildInterface); ok {
				ctx.handleTarget(targetPath, build)
			}
		}
		output.NinjaFile = ctx.ninjaFile()
	}

	// Serialize generator output.
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		Fatal("failed to marshall generator output: %s", err)
	}
	err = ioutil.WriteFile(outputFileName, data, fileMode)
	if err != nil {
		Fatal("failed to write generator output: %s", err)
	}
}
