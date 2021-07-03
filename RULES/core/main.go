package core

import (
	"encoding/json"
	"io/ioutil"
)

const outputFileName = "output.json"

type targetInfo struct {
	Description string
	build       buildInterface
}

type generatorOutput struct {
	NinjaFile string
	BashFile  string
	Targets   map[string]targetInfo
	Flags     map[string]flagInfo
	BuildDir  string
}

func GeneratorMain(vars map[string]interface{}) {
	output := generatorOutput{"", "", map[string]targetInfo{}, map[string]flagInfo{}, ""}

	output.Flags = initializeFlags()
	output.BuildDir = buildDir()

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
	if mode() == "buildFiles" {
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
