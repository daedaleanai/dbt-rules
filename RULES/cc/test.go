package cc

import (
	"dbt-rules/RULES/core"
	"fmt"
)

type Test struct {
	Lib Library
}

func (test *Test) Build(ctx core.Context) core.BuildOutput {
	out := ctx.Cwd().WithFilename("test")

	ctx.SetBuildOption(ToolchainParam, TestToolchainA)
	x1 := ctx.Build(&test.Lib).Output()
	ctx.SetBuildOption(ToolchainParam, TestToolchainB)
	x2 := ctx.Build(&test.Lib).Output()

	ctx.AddBuildStep(core.BuildStep{
		Out:   out,
		Ins:   []core.Path{x1, x2},
		Cmd:   fmt.Sprintf("cat %q %q > %q", x1.Absolute(), x2.Absolute(), out.Absolute()),
		Descr: "Test",
	})

	return out
}
