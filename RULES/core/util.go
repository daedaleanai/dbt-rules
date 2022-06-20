package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"text/template"
)

const fileMode = 0755

var currentTarget = ""

func loadInput() generatorInput {
	data, err := ioutil.ReadFile(inputFileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not read DBT input file: %s.\n", err)
		os.Exit(1)
	}
	var input generatorInput
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not parse DBT input: %s.\n", err)
		os.Exit(1)
	}
	if input.Version != buildProtocolVersion {
		fmt.Fprintf(os.Stderr, "Error: Unexpected version of DBT input: %d. Expected %d.\n", input.Version, buildProtocolVersion)
		os.Exit(1)
	}
	return input
}

func Fatal(format string, a ...interface{}) {
	if input.CompletionsOnly {
		return
	}

	msg := fmt.Sprintf(format, a...)
	if currentTarget == "" {
		fmt.Fprintf(os.Stderr, "Error: %s.\n", msg)
	} else {
		fmt.Fprintf(os.Stderr, "Error while processing target '%s': %s.\n", currentTarget, msg)
	}
	os.Exit(1)
}

// Compile a go text template, execute it, and return the result as a string
func CompileTemplate(tmpl, name string, data interface{}) string {
	t, err := template.New(name).Funcs(template.FuncMap{
		"hasSuffix": strings.HasSuffix,
	}).Parse(tmpl)

	if err != nil {
		Fatal("Cannot parse the IP generator template: %s", err)
	}

	var buff bytes.Buffer
	err = t.Execute(&buff, data)
	if err != nil {
		Fatal("Cannot execute the IP generator template: %s", err)
	}
	return buff.String()
}

// Compile a go text template from a file, execute it, and return the result as a string
func CompileTemplateFile(tmplFile string, data interface{}) string {
	t, err := template.New(path.Base(tmplFile)).Funcs(template.FuncMap{
		"hasSuffix": strings.HasSuffix,
	}).ParseFiles(tmplFile)

	if err != nil {
		Fatal("Cannot parse the IP generator template: %s", err)
	}

	var buff bytes.Buffer
	err = t.Execute(&buff, data)
	if err != nil {
		Fatal("Cannot execute the IP generator template: %s", err)
	}
	return buff.String()
}

// Get paths from a path map sorted by key
func GetSortedPaths(pathMap map[string]Path) []Path {
	keys := []string{}
	for k, _ := range pathMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	paths := []Path{}
	for _, k := range keys {
		path := pathMap[k]
		paths = append(paths, path)
	}
	return paths
}

// Get paths from a path map sorted by key
func GetSortedOutPaths(pathMap map[string]OutPath) []OutPath {
	keys := []string{}
	for k, _ := range pathMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	paths := []OutPath{}
	for _, k := range keys {
		path := pathMap[k]
		paths = append(paths, path)
	}
	return paths
}

type StringPath struct {
	Key   string
	Value Path
}

type StringString struct {
	Key   string
	Value string
}
