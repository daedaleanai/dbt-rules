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
	Report      bool
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
	SelectedTargets []string
}

type generatorOutput struct {
	NinjaFile   string
	Targets     map[string]targetInfo
	Flags       map[string]flagInfo
	CompDbRules []string
}

var input = loadInput()

func checkHasAnySelectedTargetsOtherThanReports(vars map[string]interface{}) bool {
	for _, targetPath := range input.SelectedTargets {
		tgt := vars[targetPath]
		if _, ok := tgt.(coverageReportInterface); ok {
			continue
		}
		if _, ok := tgt.(analyzerReportInterface); ok {
			continue
		}
		return true
	}
	return false
}

func isTargetSelected(targetPath string) bool {
	for _, path := range input.SelectedTargets {
		if path == targetPath {
			return true
		}
	}
	return false
}

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

	hasAnySelectedTargetsOtherThanReports := checkHasAnySelectedTargetsOtherThanReports(vars)

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
		var targetsForAnalyze = []AnalyzeInterface{}

		for _, targetPath := range targetPaths {
			tgt := vars[targetPath]
			if cov, ok := tgt.(CoverageInterface); ok {
				if !hasAnySelectedTargetsOtherThanReports || isTargetSelected(targetPath) {
					targetsForCoverage = append(targetsForCoverage, cov)
				}
			}
			if sa, ok := tgt.(AnalyzeInterface); ok {
				if !hasAnySelectedTargetsOtherThanReports || isTargetSelected(targetPath) {
					targetsForAnalyze = append(targetsForAnalyze, sa)
				}
			}
		}

		for _, targetPath := range targetPaths {
			tgt := vars[targetPath]
			if build, ok := tgt.(coverageReportInterface); ok {
				tgt = build.CoverageReport(targetsForCoverage)
			}
			if build, ok := tgt.(analyzerReportInterface); ok {
				tgt = build.AnalyzerReport(targetsForAnalyze)
			}

			if build, ok := tgt.(buildInterface); ok {
				ctx.handleTarget(targetPath, build)
			}
		}
		output.NinjaFile = ctx.ninjaFile()

		output.CompDbRules = []string{}
		for name := range ctx.compDbBuildRules {
			output.CompDbRules = append(output.CompDbRules, name)
		}
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
