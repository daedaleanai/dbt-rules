package core

import (
	"encoding/json"
	"io/ioutil"
)

const buildProtocolVersion = 2
const inputFileName = "input.json"
const outputFileName = "output.json"

type targetInfo struct {
	Description string
	build       buildInterface
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
	BashFile  string
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

	for name, variable := range vars {
		if buildIface, ok := variable.(buildInterface); ok {
			description := ""
			if descriptionIface, ok := variable.(descriptionInterface); ok {
				description = descriptionIface.Description()
			}
			output.Targets[name] = targetInfo{description, buildIface}
		}
	}

	// Create build files.
	if !completionsOnly() {
		ctx := newContext(vars)
		for name, target := range output.Targets {
			ctx.handleTarget(name, target.build)
		}
		ctx.finish()
		output.NinjaFile = ctx.ninjaFile.String()
		output.BashFile = ctx.bashFile.String()
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
