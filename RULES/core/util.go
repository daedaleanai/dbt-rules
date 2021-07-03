package core

import (
	"fmt"
	"os"
)

const fileMode = 0755

var currentTarget = ""

func mode() string {
	return os.Args[1]
}

func sourceDir() string {
	return os.Args[2]
}

func buildDir() string {
	return os.Args[3]
}

func workingDir() string {
	return os.Args[4]
}

func otherArgs() []string {
	return os.Args[5:]
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
