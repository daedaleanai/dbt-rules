package core

import (
	"bytes"
	"fmt"
	"os"
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
	t, err := template.New(name).Parse(tmpl)
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
