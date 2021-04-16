package core

type TargetGroup []interface{}

func (group TargetGroup) Build(ctx Context) {
	for i := range group {
		ctx.addTargetDependency(group[i])
	}
}
