package core

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"text/template"
)

const fileMode = 0755

var currentTarget = ""

func mode() string {
	return os.Args[1]
}

func sourceDir() string {
	return os.Args[2]
}

func buildDirPrefix() string {
	return os.Args[3]
}

func workingDir() string {
	return os.Args[4]
}

func otherArgs() []string {
	return os.Args[5:]
}

var buildDirSuffix = ""

func buildDir() string {
	if !flagsLocked {
		Fatal("cannot use build directory before all flag values are known")
	}
	return buildDirPrefix() + buildDirSuffix
}

func Fatal(format string, a ...interface{}) {
	if mode() == "completion" {
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
