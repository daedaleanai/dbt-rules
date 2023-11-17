package core

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"sort"
	"unicode"
	"regexp"
	"fmt"
)

const inputFileName = "input.json"
const outputFileName = "output.json"

type mode uint

const (
	modeBuild mode = iota
	modeList
	modeRun
	modeTest
	modeFlags
)

type targetInfo struct {
	Description string
	Runnable    bool
	Testable    bool
	Report      bool
	Selected	bool
}

type generatorInput struct {
	DbtVersion           version
	SourceDir            string
	WorkingDir           string
	OutputDir            string
	CmdlineFlags         map[string]string
	WorkspaceFlags       map[string]string
	CompletionsOnly      bool
	RunArgs              []string
	TestArgs             []string
	Layout               string
	SelectedTargets      []string
	PersistFlags         bool
	PositivePatterns []string
	NegativePatterns []string
	Mode				  mode
}

type generatorOutput struct {
	NinjaFile   string
	Targets     map[string]targetInfo
	Flags       map[string]flagInfo
	CompDbRules []string
	SelectedTargets []string
}

var input = loadInput()


func GeneratorMain(vars map[string]interface{}) {
	output := generatorOutput{
		Targets: map[string]targetInfo{},
		Flags:   lockAndGetFlags(input.PersistFlags),
	}

	// Determine the set of targets to be built.
	positiveRegexps := []*regexp.Regexp{}
	for _, pattern := range input.PositivePatterns {
		re, err := regexp.Compile(fmt.Sprintf("^%s$", pattern))
		if err != nil {
			Fatal("Positive target pattern '%s' is not a valid regular expression: %s.\n", pattern, err)
		}
		positiveRegexps = append(positiveRegexps, re)
	}

	negativeRegexps := []*regexp.Regexp{}
	for _, pattern := range input.NegativePatterns {
		re, err := regexp.Compile(fmt.Sprintf("^%s$", pattern))
		if err != nil {
			Fatal("Negative target pattern '%s' is not a valid regular expression: %s.\n", pattern, err)
		}
		negativeRegexps = append(negativeRegexps, re)
	}

	var selectedTargets = []interface{}{}

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
		if _, ok := variable.(reportInterface); ok {
			info.Report = true
		}

		if input.Mode == modeRun && !info.Runnable {
			continue
		}

		if input.Mode == modeTest && !info.Testable {
			continue
		}

		info.Selected = false

		// Negative patterns have precedence
		matchesNegativePattern := false
		for _, re := range negativeRegexps {
			if re.MatchString(targetPath) {
				matchesNegativePattern = true
				break
			}
		}

		if !matchesNegativePattern {	
			for idx, re := range positiveRegexps {
				selected := false
				if info.Report {
					selected = input.PositivePatterns[idx] == targetPath
				} else {
					selected = re.MatchString(targetPath)
				}
				if selected {
					info.Selected = true
					selectedTargets = append(selectedTargets, variable)

					if _, ok := variable.(buildInterface); ok {
						output.SelectedTargets = append(output.SelectedTargets, targetPath)
					}

					break
				}
			}
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

		var allTargets = []interface{}{}
		var reportTargets = []reportInterface{}

		for _, targetPath := range targetPaths {
			tgt := vars[targetPath]
			allTargets = append(allTargets, tgt)
			
			if rep, ok := tgt.(reportInterface); ok {
				reportTargets = append(reportTargets, rep)
			}
		}

		for _, targetPath := range targetPaths {
			tgt := vars[targetPath]
			if rep, ok := tgt.(reportInterface); ok {
				if info,iok := output.Targets[targetPath]; iok {
					if !info.Selected {
						continue
					}
				} else {
					continue
				}

				tgt = rep.Report(allTargets, selectedTargets)
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
		sort.Strings(output.CompDbRules)
	}

	// Serialize generator output.
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		Fatal("failed to marshal generator output: %s", err)
	}
	err = ioutil.WriteFile(outputFileName, data, fileMode)
	if err != nil {
		Fatal("failed to write generator output: %s", err)
	}
}
