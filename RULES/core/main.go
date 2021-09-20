package core

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"unicode"
)

const buildProtocolVersion = 2
const inputFileName = "input.json"
const outputFileName = "output.json"

type targetInfo struct {
	Description string
}

type generatorInput struct {
	Version         uint
	SourceDir       string
	WorkingDir      string
	BuildDirPrefix  string
	BuildFlags      map[string]string
	CompletionsOnly bool
}

type generatorOutput struct {
	Version   uint
	NinjaFile string
	Targets   map[string]targetInfo
	Flags     map[string]flagInfo
	BuildDir  string
}

var input generatorInput

func GeneratorMain(vars map[string]interface{}) {
	output := generatorOutput{
		Version:  buildProtocolVersion,
		Targets:  map[string]targetInfo{},
		Flags:    lockAndGetFlags(),
		BuildDir: buildDir(),
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
		output.Targets[targetPath] = info
	}

	// Create build files.
	if !completionsOnly() {
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
