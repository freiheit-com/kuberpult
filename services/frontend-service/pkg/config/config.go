

package config

type ServerConfig struct {
	CdServer            string `default:"kuberpult-cd-service:8443"`
	HttpCdServer        string `default:"http://kuberpult-cd-service:80" split_words:"true"`
	GKEProjectNumber    string `default:"" split_words:"true"`
	GKEBackendServiceID string `default:"" split_words:"true"`
	EnableTracing       bool   `default:"false" split_words:"true"`
	ArgocdBaseUrl       string `default:"" split_words:"true"`
	PgpKeyRing          string `split_words:"true"`
	AzureEnableAuth     bool   `default:"false" split_words:"true"`
	AzureCloudInstance  string `default:"https://login.microsoftonline.com/" split_words:"true"`
	AzureClientId       string `default:"" split_words:"true"`
	AzureTenantId       string `default:"" split_words:"true"`
	AzureRedirectUrl    string `default:"" split_words:"true"`
	Version             string `default:""`
	SourceRepoUrl       string `default:"" split_words:"true"`
}

type FrontendConfig struct {
	ArgoCd           *ArgoCdConfig `json:"argocd"`
	Auth             *AuthConfig   `json:"auth"`
	KuberpultVersion string        `json:"version"`
	SourceRepoUrl    string        `json:"source"`
}

type ArgoCdConfig struct {
	BaseUrl string `json:"baseUrl"`
}

type AuthConfig struct {
	AzureAuth *AzureAuthConfig `json:"azureAuth"`
}

type AzureAuthConfig struct {
	Enabled       bool   `json:"enabled"`
	ClientId      string `json:"clientId"`
	TenantId      string `json:"tenantId"`
	CloudInstance string `json:"cloudInstance"`
	RedirectURL   string `json:"redirectURL"`
}
