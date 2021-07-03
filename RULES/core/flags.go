package core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strings"
)

const configFilePath = "../FLAGS.json"

var flags = map[string]*Flag{}
var flagsLocked = false

type Flag struct {
	Name           string
	Description    string
	AllowedValues  []string
	DefaultValueFn func() string
	Value          string
}

func RegisterFlag(name string, description string, allowedValues []string, defaultValueFn func() string) *Flag {
	if flagsLocked {
		Fatal("flags are locked")
	}
	if _, exists := flags[name]; exists {
		Fatal("flag '%s' already exists", name)
	}
	flag := &Flag{
		Name:           name,
		Description:    description,
		AllowedValues:  allowedValues,
		DefaultValueFn: defaultValueFn,
	}
	flags[name] = flag
	return flag
}

var buildParams = map[string]*BuildParam{}

type buildOptionName interface {
	Name() string
}

type BuildParam struct {
	Name    string
	Type    reflect.Type
	Default interface{}
	Options map[string]interface{}
	flag    *Flag
}

func RegisterBuildParam(param interface{}, description string) *BuildParam {
	paramType := reflect.TypeOf(param).Elem()
	paramName := fmt.Sprintf("%s.%s", path.Base(paramType.PkgPath()), paramType.Name())
	if !paramType.Implements(reflect.TypeOf((*buildOptionName)(nil)).Elem()) {
		Fatal("build param '%s' does not implement a Name() function", paramName)
	}
	if _, exists := buildParams[paramName]; exists {
		Fatal("build param '%s' already exists", paramName)
	}
	buildParam := &BuildParam{
		Name:    paramName,
		Type:    paramType,
		Options: map[string]interface{}{},
		flag:    RegisterFlag(paramName, description, []string{}, nil),
	}
	buildParams[paramName] = buildParam
	return buildParam
}

func (param *BuildParam) AddDefaultOption(option buildOptionName) bool {
	if reflect.TypeOf(option) != param.Type && !reflect.TypeOf(option).Implements(param.Type) {
		Fatal("default option for build param '%s' has incorrect type", param.Name)
	}
	if param.Default != nil {
		Fatal("default option for build param '%s' already exists", param.Name)
	}
	param.Default = option
	param.AddOption(option)
	param.flag.DefaultValueFn = func() string { return option.Name() }
	return true
}

func (param *BuildParam) AddOption(option buildOptionName) bool {
	if reflect.TypeOf(option) != param.Type && !reflect.TypeOf(option).Implements(param.Type) {
		Fatal("option for build param '%s' has incorrect type", param.Name)
	}
	if _, exists := param.Options[option.Name()]; exists {
		Fatal("option '%s' for build param '%s' already exists", option.Name(), param.Name)
	}
	param.flag.AllowedValues = append(param.flag.AllowedValues, option.Name())
	param.Options[option.Name()] = option
	return true
}

type flagInfo struct {
	Description   string
	Type          string
	AllowedValues []string
	Value         string
}

func initializeFlags() map[string]flagInfo {
	flagsLocked = true

	cmdlineFlags := getCmdlineFlags()
	configFileFlags := getConfigFileFlags()

	info := map[string]flagInfo{}
	for name, flag := range flags {
		if value, exists := cmdlineFlags[name]; exists {
			flag.Value = value
		} else if value, exists := configFileFlags[name]; exists {
			flag.Value = value
		} else if flag.DefaultValueFn != nil {
			flag.Value = flag.DefaultValueFn()
		} else {
			Fatal("flag '%s' has no value", name)
		}
		if flag.AllowedValues != nil {
			found := false
			for _, value := range flag.AllowedValues {
				if value == flag.Value {
					found = true
					break
				}
			}
			if !found {
				Fatal("value '%s' is not allowed for flag '%s'", flag.Value, name)
			}
		}
		info[name] = flagInfo{
			flag.Description,
			"string",
			flag.AllowedValues,
			flag.Value,
		}
	}
	return info
}

func getCmdlineFlags() map[string]string {
	flags := map[string]string{}
	for _, arg := range otherArgs() {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) > 1 {
			flags[parts[0]] = parts[1]
		} else {
			flags[parts[0]] = "true"
		}
	}
	return flags
}

func getConfigFileFlags() map[string]string {
	flags := map[string]string{}

	data, err := ioutil.ReadFile(configFilePath)
	if os.IsNotExist(err) {
		return flags
	}
	if err != nil {
		Fatal("failed to read config flags: %s", err)
	}
	err = json.Unmarshal(data, &flags)
	if err != nil {
		Fatal("failed to unmarshall config flags: %s", err)
	}

	return flags
}
