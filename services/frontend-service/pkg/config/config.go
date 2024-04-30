/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com*/

package config

import "time"

type ServerConfig struct {
	CdServer               string        `default:"kuberpult-cd-service:8443"`
	CdServerSecure         bool          `default:"false" split_words:"true"`
	RolloutServer          string        `default:""`
	HttpCdServer           string        `default:"http://kuberpult-cd-service:80" split_words:"true"`
	GKEProjectNumber       string        `default:"" split_words:"true"`
	GKEBackendServiceID    string        `default:"" split_words:"true"`
	GKEBackendServiceName  string        `default:"" split_words:"true"`
	EnableTracing          bool          `default:"false" split_words:"true"`
	ArgocdBaseUrl          string        `default:"" split_words:"true"`
	ArgocdNamespace        string        `default:"tools" split_words:"true"`
	PgpKeyRingPath         string        `split_words:"true"`
	AzureEnableAuth        bool          `default:"false" split_words:"true"`
	AzureCloudInstance     string        `default:"https://login.microsoftonline.com/" split_words:"true"`
	AzureClientId          string        `default:"" split_words:"true"`
	AzureTenantId          string        `default:"" split_words:"true"`
	AzureRedirectUrl       string        `default:"" split_words:"true"`
	DexEnabled             bool          `default:"false" split_words:"true"`
	DexClientId            string        `default:"" split_words:"true"`
	DexClientSecret        string        `default:"" split_words:"true"`
	DexRbacPolicyPath      string        `split_words:"true"`
	DexBaseURL             string        `default:"" split_words:"true"`
	DexScopes              string        `default:"" split_words:"true"`
	Version                string        `default:""`
	SourceRepoUrl          string        `default:"" split_words:"true"`
	ManifestRepoUrl        string        `default:"" split_words:"true"`
	GitBranch              string        `default:"" split_words:"true"`
	AllowedOrigins         string        `default:"" split_words:"true"`
	GitAuthorName          string        `default:"" split_words:"true"`
	GitAuthorEmail         string        `default:"" split_words:"true"`
	BatchClientTimeout     time.Duration `default:"2m" split_words:"true"`
	MaxWaitDuration        time.Duration `default:"10m" split_words:"true"`
	ApiEnableDespiteNoAuth bool          `default:"false" split_words:"true"`
	IapEnabled             bool          `default:"false" split_words:"true"`
}

type FrontendConfig struct {
	ArgoCd           *ArgoCdConfig `json:"argocd"`
	Auth             *AuthConfig   `json:"auth"`
	KuberpultVersion string        `json:"version"`
	SourceRepoUrl    string        `json:"source"`
	ManifestRepoUrl  string        `json:"manifestRepoUrl"`
	Branch           string        `json:"branch"`
}

type ArgoCdConfig struct {
	BaseUrl   string `json:"baseUrl"`
	Namespace string `json:"namespace"`
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
