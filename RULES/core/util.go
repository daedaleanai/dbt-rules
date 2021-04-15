package core

import (
	"fmt"
	"os"
)

const fileMode = 0755

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
		fatal("cannot use build directory before all flag values are known")
	}
	return buildDirPrefix() + buildDirSuffix
}

func fatal(format string, a ...interface{}) {
	if mode() == "completion" {
		return
	}

	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stderr, "Error: %s.\n", msg)
	os.Exit(1)
}
