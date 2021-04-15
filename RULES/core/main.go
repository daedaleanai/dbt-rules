package core

import (
	"encoding/json"
	"io/ioutil"
)

const outputFileMode = 0755
const outputFileName = "output.json"

type target struct {
	Description string
	build buildInterface
}

type flag struct {
	Type string
	Alias string
	AllowedValues []string
	Value string
}

type generatorOutput struct {
	NinjaFile string
	Targets   map[string]target
	Flags     map[string]flag
}

func GeneratorMain(vars map[string]interface{}) {
	output := generatorOutput{"", map[string]target{}, map[string]flag{}}

	for name, variable := range vars {
		if buildIface, ok := variable.(buildInterface); ok {
			description := ""
			if descriptionIface, ok := variable.(descriptionInterface); ok {
				description = descriptionIface.Description()
			}
			output.Targets[name] = target{description, buildIface}
		}
	}
	
	if mode() == "ninja" {
		ctx := newContext()
		for name, target := range output.Targets {
			ctx.handleTarget(name, target.build)
		}
		
		output.NinjaFile = ctx.ninjaFile.String()
	}

	data, err := json.MarshalIndent(output, "", "  ")
	Assert(err == nil, "failed to marshall generator output: %s", err)
	err = ioutil.WriteFile(outputFileName, data, outputFileMode)
	Assert(err == nil, "failed to write generator output: %s", err)
}
