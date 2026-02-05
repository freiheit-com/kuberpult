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

Copyright freiheit.com*/

package cmd

import (
	"os"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/errorMatcher"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestCheckReleaseVersionLimit(t *testing.T) {
	for _, test := range []struct {
		name          string
		config        Config
		expectedError error
	}{
		{
			name: "versions limit equals the minimum allowed",
			config: Config{
				ReleaseVersionsLimit: minReleaseVersionsLimit,
			},
			expectedError: nil,
		},
		{
			name: "versions limit equals the maximum allowed",
			config: Config{
				ReleaseVersionsLimit: maxReleaseVersionsLimit,
			},
			expectedError: nil,
		},
		{
			name: "default versions limit",
			config: Config{
				ReleaseVersionsLimit: 20,
			},
			expectedError: nil,
		},
		{
			name: "versions limit below minimum",
			config: Config{
				ReleaseVersionsLimit: 3,
			},
			expectedError: releaseVersionsLimitError{limit: 3},
		},
		{
			name: "versions limit above maximum",
			config: Config{
				ReleaseVersionsLimit: 45,
			},
			expectedError: releaseVersionsLimitError{limit: 45},
		},
	} {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := checkReleaseVersionLimit(tc.config.ReleaseVersionsLimit)
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestEnvVarParsing(t *testing.T) {
	tcs := []struct {
		Name        string
		Environment map[string]string

		ExpectedConfiguration *config.ServerConfig
		ExpectedError         error
	}{
		{
			Name:                  "default values only - no env vars set",
			Environment:           map[string]string{},
			ExpectedConfiguration: nil,
			ExpectedError: errorMatcher.ContainsErrMatcher{
				Messages: []string{"KUBERPULT_GIT_AUTHOR_NAME", "could not read"},
			},
		},
		// {
		// 	Name: "minimal set of env vars to not get an error",
		// 	Environment: map[string]string{
		// 		"KUBERPULT_GIT_AUTHOR_NAME":  "git-name1",
		// 		"KUBERPULT_GIT_AUTHOR_EMAIL": "git-email2",
		// 	},
		// 	ExpectedConfiguration: &config.ServerConfig{
		// 		CdServer:               "kuberpult-cd-service:8443",
		// 		ManifestExportServer:   "kuberpult-manifest-repo-export-service:8443",
		// 		ArgocdNamespace:        "tools",
		// 		AzureCloudInstance:     "https://login.microsoftonline.com/",
		// 		AzureEnableAuth:        false,
		// 		DexFullNameOverride:    "kuberpult-dex",
		// 		BatchClientTimeout:     2 * time.Minute,
		// 		MaxWaitDuration:        10 * time.Minute,
		// 		ApiEnableDespiteNoAuth: false,
		// 		IapEnabled:             false,
		// 		GrpcMaxRecvMsgSize:     4,
		// 		RevisionsEnabled:       false,
		// 		GitAuthorName:          "git-name1",
		// 		GitAuthorEmail:         "git-email2",
		// 	},
		// 	ExpectedError: nil,
		// },
		// {
		// 	Name: "all values overwritten",
		// 	Environment: map[string]string{
		// 		"KUBERPULT_CDSERVER":             "cd:8443",
		// 		"KUBERPULT_MANIFESTEXPORTSERVER": "mani:8443",
		// 		"KUBERPULT_CD_SERVER_SECURE":     "true",
		// 		"KUBERPULT_ROLLOUTSERVER":        "rollout",

		// 		"KUBERPULT_GKE_PROJECT_NUMBER":       "proj",
		// 		"KUBERPULT_GKE_BACKEND_SERVICE_ID":   "backend",
		// 		"KUBERPULT_GKE_BACKEND_SERVICE_NAME": "serv-name",

		// 		"KUBERPULT_ENABLE_TRACING":   "true",
		// 		"KUBERPULT_ARGOCD_BASE_URL":  "argo-base",
		// 		"KUBERPULT_ARGOCD_NAMESPACE": "argocd",

		// 		"KUBERPULT_PGP_KEY_RING_PATH": "pgp",

		// 		"KUBERPULT_AZURE_CLOUD_INSTANCE": "www.example.com",
		// 		"KUBERPULT_AZURE_CLIENT_ID":      "client id",
		// 		"KUBERPULT_AZURE_TENANT_ID":      "tenant",
		// 		"KUBERPULT_AZURE_REDIRECT_URL":   "redirect",

		// 		"KUBERPULT_DEX_CLIENT_ID":                          "dex client id",
		// 		"KUBERPULT_DEX_CLIENT_SECRET":                      "dex secret",
		// 		"KUBERPULT_DEX_RBAC_POLICY_PATH":                   "dex policy",
		// 		"KUBERPULT_DEX_BASE_URL":                           "dex base",
		// 		"KUBERPULT_DEX_FULL_NAME_OVERRIDE":                 "dex-kuberpult-123",
		// 		"KUBERPULT_DEX_SCOPES":                             "dex scope",
		// 		"KUBERPULT_DEX_USE_CLUSTER_INTERNAL_COMMUNICATION": "true",

		// 		"KUBERPULT_VERSION":           "1.2.3",
		// 		"KUBERPULT_SOURCE_REPO_URL":   "example.com/repo",
		// 		"KUBERPULT_MANIFEST_REPO_URL": "example.com/manifest-repo",
		// 		"KUBERPULT_GIT_BRANCH":        "mainOfTheUniverse",
		// 		"KUBERPULT_ALLOWED_ORIGINS":   "localhost",

		// 		"KUBERPULT_GIT_AUTHOR_NAME":  "git name",
		// 		"KUBERPULT_GIT_AUTHOR_EMAIL": "git mail",

		// 		"KUBERPULT_BATCH_CLIENT_TIMEOUT":       "22m",
		// 		"KUBERPULT_MAX_WAIT_DURATION":          "33m",
		// 		"KUBERPULT_API_ENABLE_DESPITE_NO_AUTH": "true",
		// 		"KUBERPULT_IAP_ENABLED":                "true",
		// 		"KUBERPULT_GRPC_MAX_RECV_MSG_SIZE":     "50",
		// 		"KUBERPULT_REVISIONS_ENABLED":          "true",
		// 	},
		// 	ExpectedConfiguration: &config.ServerConfig{
		// 		CdServer:              "cd:8443",
		// 		ManifestExportServer:  "mani:8443",
		// 		CdServerSecure:        true,
		// 		RolloutServer:         "rollout",
		// 		GKEProjectNumber:      "proj",
		// 		GKEBackendServiceID:   "backend",
		// 		GKEBackendServiceName: "serv-name",
		// 		EnableTracing:         true,
		// 		ArgocdBaseUrl:         "argo-base",
		// 		ArgocdNamespace:       "argocd",
		// 		PgpKeyRingPath:        "pgp",

		// 		AzureEnableAuth:    false,
		// 		AzureCloudInstance: "www.example.com",
		// 		AzureClientId:      "client id",
		// 		AzureTenantId:      "tenant",
		// 		AzureRedirectUrl:   "redirect",

		// 		DexEnabled:                         false,
		// 		DexClientId:                        "dex client id",
		// 		DexClientSecret:                    "dex secret",
		// 		DexRbacPolicyPath:                  "dex policy",
		// 		DexBaseURL:                         "dex base",
		// 		DexFullNameOverride:                "dex-kuberpult-123",
		// 		DexScopes:                          "dex scope",
		// 		DexUseClusterInternalCommunication: true,

		// 		Version:         "1.2.3",
		// 		SourceRepoUrl:   "example.com/repo",
		// 		ManifestRepoUrl: "example.com/manifest-repo",
		// 		GitBranch:       "mainOfTheUniverse",
		// 		AllowedOrigins:  "localhost",

		// 		GitAuthorName:  "git name",
		// 		GitAuthorEmail: "git mail",

		// 		BatchClientTimeout:     22 * time.Minute,
		// 		MaxWaitDuration:        33 * time.Minute,
		// 		ApiEnableDespiteNoAuth: true,
		// 		IapEnabled:             true,
		// 		GrpcMaxRecvMsgSize:     50,
		// 		RevisionsEnabled:       true,
		// 	},
		// 	ExpectedError: nil,
		// },
		// {
		// 	Name: "invalid value for wait duration",
		// 	Environment: map[string]string{
		// 		"KUBERPULT_GIT_AUTHOR_NAME":  "git-name1",
		// 		"KUBERPULT_GIT_AUTHOR_EMAIL": "git-email2",

		// 		"KUBERPULT_MAX_WAIT_DURATION": "33",
		// 	},
		// 	ExpectedConfiguration: nil,
		// 	ExpectedError: errorMatcher.ContainsErrMatcher{
		// 		Messages: []string{"KUBERPULT_MAX_WAIT_DURATION", "33"},
		// 	},
		// },
		// {
		// 	Name: "invalid value for batch client timeout",
		// 	Environment: map[string]string{
		// 		"KUBERPULT_GIT_AUTHOR_NAME":  "git-name1",
		// 		"KUBERPULT_GIT_AUTHOR_EMAIL": "git-email2",

		// 		"KUBERPULT_BATCH_CLIENT_TIMEOUT": "44",
		// 	},
		// 	ExpectedConfiguration: nil,
		// 	ExpectedError: errorMatcher.ContainsErrMatcher{
		// 		Messages: []string{"KUBERPULT_BATCH_CLIENT_TIMEOUT", "44"},
		// 	},
		// },
		// {
		// 	Name: "invalid value for grpc max msg",
		// 	Environment: map[string]string{
		// 		"KUBERPULT_GIT_AUTHOR_NAME":  "git-name1",
		// 		"KUBERPULT_GIT_AUTHOR_EMAIL": "git-email2",

		// 		"KUBERPULT_GRPC_MAX_RECV_MSG_SIZE": "not-a-number",
		// 	},
		// 	ExpectedConfiguration: nil,
		// 	ExpectedError: errorMatcher.ContainsErrMatcher{
		// 		Messages: []string{"KUBERPULT_GRPC_MAX_RECV_MSG_SIZE", "not-a-number"},
		// 	},
		// },
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			os.Clearenv()
			for key, value := range tc.Environment {
				err := os.Setenv(key, value)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				t.Logf("set %s=%s", key, value)
			}

			var actual Config
			var err error
			actual, err = parseEnvVars()
			// check errors
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(actual, tc.ExpectedConfiguration); diff != "" {
				t.Logf("actual configuration: %v", actual)
				t.Logf("expected configuration: %v", tc.ExpectedConfiguration)
				t.Errorf("expected args:\n  %v\ngot:\n  %v\ndiff:\n  %s\n", actual, tc.ExpectedConfiguration, diff)
			}
		})
	}
}
