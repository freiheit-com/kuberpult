package teststructs

type OutputDoc struct {
	Metadata Metadata
	Kind     string
	Spec     Spec
}
type Metadata struct {
	Name string
}

type Spec struct {
	Replicas int
	Template Template
}

type Template struct {
	Spec TemplateSpec
}

type TemplateSpec struct {
	Containers []Container
}

type Container struct {
	Name string
	Env  []EnvVar
}

type EnvVar struct {
	Name  string
	Value string
}

func (t *OutputDoc) CheckForEnv(target EnvVar, strict bool) bool {
	for _, container := range t.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == target.Name && (!strict || env.Value == target.Value) {
				return true
			}
		}
	}
	return false
}
