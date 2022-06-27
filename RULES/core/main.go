package core

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"unicode"
)

const inputFileName = "input.json"
const outputFileName = "output.json"

type targetInfo struct {
	Description string
	Runnable    bool
	Testable    bool
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
		output.Targets[targetPath] = info
	}

	// Create build files.
	if !input.CompletionsOnly {
		ctx := newContext(vars)
		for targetPath, variable := range vars {
			if build, ok := variable.(buildInterface); ok {
				ctx.handleTarget(targetPath, build)
			}
		}
		output.NinjaFile = ctx.ninjaFile.String()
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
