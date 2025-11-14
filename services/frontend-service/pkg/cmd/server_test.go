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
	"bytes"
	"context"
	"github.com/freiheit-com/kuberpult/pkg/errorMatcher"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
	"github.com/google/go-cmp/cmp/cmpopts"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
)

func TestServerHeader(t *testing.T) {
	tcs := []struct {
		Name           string
		RequestPath    string
		RequestMethod  string
		RequestHeaders http.Header
		Environment    map[string]string

		ExpectedHeaders http.Header
	}{
		{
			Name:        "simple case",
			RequestPath: "/",

			ExpectedHeaders: http.Header{
				"Accept-Ranges": {"bytes"},
				"Content-Type":  {"text/html; charset=utf-8"},
				"Content-Security-Policy": {
					"default-src 'self'; style-src-elem 'self' fonts.googleapis.com 'unsafe-inline'; font-src fonts.gstatic.com; connect-src 'self' login.microsoftonline.com; child-src 'none'",
				},
				"Permission-Policy": {
					"accelerometer=(), ambient-light-sensor=(), autoplay=(), battery=(), camera=(), cross-origin-isolated=(), display-capture=(), document-domain=(), encrypted-media=(), execution-while-not-rendered=(), execution-while-out-of-viewport=(), fullscreen=(), geolocation=(), gyroscope=(), keyboard-map=(), magnetometer=(), microphone=(), midi=(), navigation-override=(), payment=(), picture-in-picture=(), publickey-credentials-get=(), screen-wake-lock=(), sync-xhr=(), usb=(), web-share=(), xr-spatial-tracking=(), clipboard-read=(), clipboard-write=(), gamepad=(), speaker-selection=()",
				},
				"Referrer-Policy":           {"no-referrer"},
				"Strict-Transport-Security": {"max-age=31536000; includeSubDomains;"},
				"X-Content-Type-Options":    {"nosniff"},
				"X-Frame-Options":           {"DENY"},
			},
		},
		{

			Name:          "cors",
			RequestMethod: "OPTIONS",
			RequestHeaders: http.Header{
				"Origin": {"https://something.else"},
			},
			Environment: map[string]string{
				"KUBERPULT_ALLOWED_ORIGINS": "https://kuberpult.fdc",
			},

			ExpectedHeaders: http.Header{
				"Accept-Ranges":                    {"bytes"},
				"Access-Control-Allow-Credentials": {"true"},
				"Access-Control-Allow-Origin":      {"https://kuberpult.fdc"},
				"Content-Type":                     {"text/html; charset=utf-8"},
				"Content-Security-Policy":          {"default-src 'self'; style-src-elem 'self' fonts.googleapis.com 'unsafe-inline'; font-src fonts.gstatic.com; connect-src 'self' login.microsoftonline.com; child-src 'none'"},

				"Permission-Policy": {
					"accelerometer=(), ambient-light-sensor=(), autoplay=(), battery=(), camera=(), cross-origin-isolated=(), display-capture=(), document-domain=(), encrypted-media=(), execution-while-not-rendered=(), execution-while-out-of-viewport=(), fullscreen=(), geolocation=(), gyroscope=(), keyboard-map=(), magnetometer=(), microphone=(), midi=(), navigation-override=(), payment=(), picture-in-picture=(), publickey-credentials-get=(), screen-wake-lock=(), sync-xhr=(), usb=(), web-share=(), xr-spatial-tracking=(), clipboard-read=(), clipboard-write=(), gamepad=(), speaker-selection=()",
				},
				"Referrer-Policy":           {"no-referrer"},
				"Strict-Transport-Security": {"max-age=31536000; includeSubDomains;"},
				"X-Content-Type-Options":    {"nosniff"},
				"X-Frame-Options":           {"DENY"},
			},
		},
		{

			Name:          "cors preflight",
			RequestMethod: "OPTIONS",
			RequestHeaders: http.Header{
				"Origin":                        {"https://something.else"},
				"Access-Control-Request-Method": {"POST"},
			},
			Environment: map[string]string{
				"KUBERPULT_ALLOWED_ORIGINS": "https://kuberpult.fdc",
			},

			ExpectedHeaders: http.Header{
				"Access-Control-Allow-Credentials": {"true"},
				"Access-Control-Allow-Headers":     {"content-type,x-grpc-web,authorization"},
				"Access-Control-Allow-Methods":     {"POST"},
				"Access-Control-Allow-Origin":      {"https://kuberpult.fdc"},
				"Access-Control-Max-Age":           {"0"},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			var wg sync.WaitGroup
			ctx, cancel := context.WithCancel(context.Background())
			wg.Add(1)
			go func(t *testing.T) {
				defer wg.Done()
				defer cancel()
				for {
					res, err := http.Get("http://localhost:8081/healthz")
					if err != nil {
						t.Logf("unhealthy: %q", err)
						<-time.After(1 * time.Second)
						continue
					}
					if res.StatusCode != 200 {
						t.Logf("status: %q", res.StatusCode)
						<-time.After(1 * time.Second)
						_ = res.Body.Close()
						continue
					}
					_ = res.Body.Close()
					break
				}
				//
				path, err := url.JoinPath("http://localhost:8081/", tc.RequestPath)
				if err != nil {
					panic(err)
				}
				req, err := http.NewRequest(tc.RequestMethod, path, nil)
				if err != nil {
					t.Errorf("expected no error but got %q", err)
				}
				defer func() {
					if req.Body != nil {
						_ = req.Body.Close()
					}
				}()
				req.Header = tc.RequestHeaders
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Errorf("expected no error but got %q", err)
				}
				t.Logf("%v %q", res.StatusCode, err)
				// Delete three headers that are hard to test.
				hdrs := res.Header.Clone()
				hdrs.Del("Content-Length")
				hdrs.Del("Date")
				hdrs.Del("Last-Modified")
				hdrs.Del("Cache-Control") // for caching tests see TestServeHttpBasics
				body, _ := io.ReadAll(res.Body)
				t.Logf("body: %q", body)
				if !cmp.Equal(tc.ExpectedHeaders, hdrs) {
					t.Errorf("expected no diff for headers but got %s", cmp.Diff(tc.ExpectedHeaders, hdrs))
				}

			}(t)
			for k, v := range tc.Environment {
				t.Setenv(k, v)
			}
			td := t.TempDir()
			err := os.Mkdir(filepath.Join(td, "build"), 0755)
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(filepath.Join(td, "build", "index.html"), ([]byte)(`<!doctype html><html lang="en"></html>`), 0755)
			if err != nil {
				t.Fatal(err)
			}
			err = os.Chdir(td)
			if err != nil {
				t.Fatal(err)
			}
			err = os.Setenv("KUBERPULT_GIT_AUTHOR_EMAIL", "mail2")
			if err != nil {
				t.Fatalf("expected no error, but got %q", err)
			}
			err = os.Setenv("KUBERPULT_GIT_AUTHOR_NAME", "name1")
			if err != nil {
				t.Fatalf("expected no error, but got %q", err)
			}
			err = runServer(ctx)
			if err != nil {
				t.Fatalf("expected no error, but got %q", err)
			}
			wg.Wait()
		})
	}
}

func TestGrpcForwardHeader(t *testing.T) {
	tcs := []struct {
		Name        string
		Environment map[string]string

		RequestPath string
		Body        proto.Message

		ExpectedHttpStatusCode int
	}{
		{
			Name:                   "rollout server unimplemented",
			RequestPath:            "/api.v1.RolloutService/StreamStatus",
			Body:                   &api.StreamStatusRequest{},
			ExpectedHttpStatusCode: 200,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			var wg sync.WaitGroup
			ctx, cancel := context.WithCancel(context.Background())
			wg.Add(1)
			go func(t *testing.T) {
				defer wg.Done()
				defer cancel()
				for {
					res, err := http.Get("http://localhost:8081/healthz")
					if err != nil {
						t.Logf("unhealthy: %q", err)
						<-time.After(1 * time.Second)
						continue
					}
					if res.StatusCode != 200 {
						t.Logf("status: %q", res.StatusCode)
						<-time.After(1 * time.Second)
						continue
					}
					break
				}
				path, err := url.JoinPath("http://localhost:8081/", tc.RequestPath)
				if err != nil {
					t.Errorf("error joining url: %s", err)
				}
				body, err := proto.Marshal(tc.Body)
				if err != nil {
					t.Errorf("expected no error while calling Marshal but got %q", err)
				}
				req, err := http.NewRequest("POST", path, bytes.NewReader(body))
				if err != nil {
					t.Errorf("expected no error but got %q", err)
				}
				req.Header.Add("Content-Type", "application/grpc-web")
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Errorf("expected no error but got %q", err)
				}
				_, _ = io.ReadAll(res.Body)
				if tc.ExpectedHttpStatusCode != res.StatusCode {
					t.Errorf("unexpected http status code, expected %d, got %d", tc.ExpectedHttpStatusCode, res.StatusCode)
				}
				// TODO(HVG): test the grpc status
			}(t)
			for k, v := range tc.Environment {
				t.Setenv(k, v)
			}
			err := os.Setenv("KUBERPULT_GIT_AUTHOR_EMAIL", "mail2")
			if err != nil {
				t.Fatalf("expected no error, but got %q", err)
			}
			err = os.Setenv("KUBERPULT_GIT_AUTHOR_NAME", "name1")
			if err != nil {
				t.Fatalf("expected no error, but got %q", err)
			}
			t.Logf("env var: %s", os.Getenv("KUBERPULT_GIT_AUTHOR_EMAIL"))
			err = runServer(ctx)
			if err != nil {
				t.Fatalf("expected no error, but got %q", err)
			}
			wg.Wait()
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
		{
			Name: "minimal set of env vars to not get an error",
			Environment: map[string]string{
				"KUBERPULT_GIT_AUTHOR_NAME":  "git-name1",
				"KUBERPULT_GIT_AUTHOR_EMAIL": "git-email2",
			},
			ExpectedConfiguration: &config.ServerConfig{
				CdServer:               "kuberpult-cd-service:8443",
				ManifestExportServer:   "kuberpult-manifest-repo-export-service:8443",
				ArgocdNamespace:        "tools",
				AzureCloudInstance:     "https://login.microsoftonline.com/",
				AzureEnableAuth:        false,
				DexFullNameOverride:    "kuberpult-dex",
				BatchClientTimeout:     2 * time.Minute,
				MaxWaitDuration:        10 * time.Minute,
				ApiEnableDespiteNoAuth: false,
				IapEnabled:             false,
				GrpcMaxRecvMsgSize:     4,
				RevisionsEnabled:       false,
				GitAuthorName:          "git-name1",
				GitAuthorEmail:         "git-email2",
			},
			ExpectedError: nil,
		},
		{
			Name: "all values overwritten",
			Environment: map[string]string{
				"KUBERPULT_CDSERVER":             "cd:8443",
				"KUBERPULT_MANIFESTEXPORTSERVER": "mani:8443",
				"KUBERPULT_CD_SERVER_SECURE":     "true",
				"KUBERPULT_ROLLOUTSERVER":        "rollout",

				"KUBERPULT_GKE_PROJECT_NUMBER":       "proj",
				"KUBERPULT_GKE_BACKEND_SERVICE_ID":   "backend",
				"KUBERPULT_GKE_BACKEND_SERVICE_NAME": "serv-name",

				"KUBERPULT_ENABLE_TRACING":   "true",
				"KUBERPULT_ARGOCD_BASE_URL":  "argo-base",
				"KUBERPULT_ARGOCD_NAMESPACE": "argocd",

				"KUBERPULT_PGP_KEY_RING_PATH": "pgp",

				"KUBERPULT_AZURE_CLOUD_INSTANCE": "www.example.com",
				"KUBERPULT_AZURE_CLIENT_ID":      "client id",
				"KUBERPULT_AZURE_TENANT_ID":      "tenant",
				"KUBERPULT_AZURE_REDIRECT_URL":   "redirect",

				"KUBERPULT_DEX_CLIENT_ID":                          "dex client id",
				"KUBERPULT_DEX_CLIENT_SECRET":                      "dex secret",
				"KUBERPULT_DEX_RBAC_POLICY_PATH":                   "dex policy",
				"KUBERPULT_DEX_BASE_URL":                           "dex base",
				"KUBERPULT_DEX_FULL_NAME_OVERRIDE":                 "dex-kuberpult-123",
				"KUBERPULT_DEX_SCOPES":                             "dex scope",
				"KUBERPULT_DEX_USE_CLUSTER_INTERNAL_COMMUNICATION": "true",

				"KUBERPULT_VERSION":           "1.2.3",
				"KUBERPULT_SOURCE_REPO_URL":   "example.com/repo",
				"KUBERPULT_MANIFEST_REPO_URL": "example.com/manifest-repo",
				"KUBERPULT_GIT_BRANCH":        "mainOfTheUniverse",
				"KUBERPULT_ALLOWED_ORIGINS":   "localhost",

				"KUBERPULT_GIT_AUTHOR_NAME":  "git name",
				"KUBERPULT_GIT_AUTHOR_EMAIL": "git mail",

				"KUBERPULT_BATCH_CLIENT_TIMEOUT":       "22m",
				"KUBERPULT_MAX_WAIT_DURATION":          "33m",
				"KUBERPULT_API_ENABLE_DESPITE_NO_AUTH": "true",
				"KUBERPULT_IAP_ENABLED":                "true",
				"KUBERPULT_GRPC_MAX_RECV_MSG_SIZE":     "50",
				"KUBERPULT_REVISIONS_ENABLED":          "true",
			},
			ExpectedConfiguration: &config.ServerConfig{
				CdServer:              "cd:8443",
				ManifestExportServer:  "mani:8443",
				CdServerSecure:        true,
				RolloutServer:         "rollout",
				GKEProjectNumber:      "proj",
				GKEBackendServiceID:   "backend",
				GKEBackendServiceName: "serv-name",
				EnableTracing:         true,
				ArgocdBaseUrl:         "argo-base",
				ArgocdNamespace:       "argocd",
				PgpKeyRingPath:        "pgp",

				AzureEnableAuth:    false,
				AzureCloudInstance: "www.example.com",
				AzureClientId:      "client id",
				AzureTenantId:      "tenant",
				AzureRedirectUrl:   "redirect",

				DexEnabled:                         false,
				DexClientId:                        "dex client id",
				DexClientSecret:                    "dex secret",
				DexRbacPolicyPath:                  "dex policy",
				DexBaseURL:                         "dex base",
				DexFullNameOverride:                "dex-kuberpult-123",
				DexScopes:                          "dex scope",
				DexUseClusterInternalCommunication: true,

				Version:         "1.2.3",
				SourceRepoUrl:   "example.com/repo",
				ManifestRepoUrl: "example.com/manifest-repo",
				GitBranch:       "mainOfTheUniverse",
				AllowedOrigins:  "localhost",

				GitAuthorName:  "git name",
				GitAuthorEmail: "git mail",

				BatchClientTimeout:     22 * time.Minute,
				MaxWaitDuration:        33 * time.Minute,
				ApiEnableDespiteNoAuth: true,
				IapEnabled:             true,
				GrpcMaxRecvMsgSize:     50,
				RevisionsEnabled:       true,
			},
			ExpectedError: nil,
		},
		{
			Name: "invalid value for wait duration",
			Environment: map[string]string{
				"KUBERPULT_GIT_AUTHOR_NAME":  "git-name1",
				"KUBERPULT_GIT_AUTHOR_EMAIL": "git-email2",

				"KUBERPULT_MAX_WAIT_DURATION": "33",
			},
			ExpectedConfiguration: nil,
			ExpectedError: errorMatcher.ContainsErrMatcher{
				Messages: []string{"KUBERPULT_MAX_WAIT_DURATION", "33"},
			},
		},
		{
			Name: "invalid value for batch client timeout",
			Environment: map[string]string{
				"KUBERPULT_GIT_AUTHOR_NAME":  "git-name1",
				"KUBERPULT_GIT_AUTHOR_EMAIL": "git-email2",

				"KUBERPULT_BATCH_CLIENT_TIMEOUT": "44",
			},
			ExpectedConfiguration: nil,
			ExpectedError: errorMatcher.ContainsErrMatcher{
				Messages: []string{"KUBERPULT_BATCH_CLIENT_TIMEOUT", "44"},
			},
		},
		{
			Name: "invalid value for grpc max msg",
			Environment: map[string]string{
				"KUBERPULT_GIT_AUTHOR_NAME":  "git-name1",
				"KUBERPULT_GIT_AUTHOR_EMAIL": "git-email2",

				"KUBERPULT_GRPC_MAX_RECV_MSG_SIZE": "not-a-number",
			},
			ExpectedConfiguration: nil,
			ExpectedError: errorMatcher.ContainsErrMatcher{
				Messages: []string{"KUBERPULT_GRPC_MAX_RECV_MSG_SIZE", "not-a-number"},
			},
		},
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

			for _, useOldParser := range []bool{true, false} {
				var actual *config.ServerConfig
				var err error
				actual, err = EnvVarWrapper(useOldParser)
				// check errors
				if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch for case 'useOldParser=%v' (-want, +got):\n%s", useOldParser, diff)
				}

				if diff := cmp.Diff(actual, tc.ExpectedConfiguration); diff != "" {
					t.Logf("actual configuration: %v", actual)
					t.Logf("expected configuration: %v", tc.ExpectedConfiguration)
					t.Errorf("expected args for case 'useOldParser=%v':\n  %v\ngot:\n  %v\ndiff:\n  %s\n", useOldParser, actual, tc.ExpectedConfiguration, diff)
				}
			}
		})
	}
}
