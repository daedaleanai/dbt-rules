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
	Targets   map[string]targetInfo
	Flags     map[string]flagInfo
	BuildDir  string
}

func GeneratorMain(vars map[string]interface{}) {
	output := generatorOutput{"", map[string]targetInfo{}, map[string]flagInfo{}, ""}

	output.Flags = lockAndGetFlags()
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

	// Create build.ninja file.
	if mode() == "ninja" {
		ctx := newContext(vars)
		for name, target := range output.Targets {
			ctx.handleTarget(name, target.build)
		}

		output.NinjaFile = ctx.ninjaFile.String()
	}

	// Serialize generator output.
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fatal("failed to marshall generator output: %s", err)
	}
	err = ioutil.WriteFile(outputFileName, data, fileMode)
	if err != nil {
		fatal("failed to write generator output: %s", err)
	}
}
