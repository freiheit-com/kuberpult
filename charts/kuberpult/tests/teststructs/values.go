package teststructs

type ValuesDoc struct {
	Git            GitInfo        `yaml:"git"`
	DataDogTracing DatadogTracing `yaml:"datadogTracing"`
	ArgoCd         ArgoCdInfo     `yaml:"argocd"`
}
type ArgoCdInfo struct {
	BaseUrl       string `yaml:"baseUrl"`
	SendWebHooks  bool   `yaml:"sendWebHooks"`
	GenerateFiles bool   `yaml:"generateFiles"`
}

type GitInfo struct {
	Url                   string `yaml:"url"`
	WebUrl                string `yaml:"webUrl"`
	Branch                string `yaml:"branch"`
	ManifestRepoUrl       string `yaml:"manifestRepoUrl"`
	SourceRepoUrl         string `yaml:"sourceRepoUrl"`
	MaximumCommitsPerPush int    `yaml:"maximumCommitsPerPush"`
}

type DatadogTracing struct {
	Enabled     bool   `yaml:"enabled"`
	Debugging   bool   `yaml:"debugging"`
	environment string `yaml:"environment"`
}
