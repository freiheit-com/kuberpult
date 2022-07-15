package config

type FrontendConfig struct {
	ArgoCd *ArgoCdConfig `json:"argocd"`
}

type ArgoCdConfig struct {
	BaseUrl string `json:"baseUrl"`
}
