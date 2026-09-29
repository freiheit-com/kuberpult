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
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	jwxjwt "github.com/lestrrat-go/jwx/v2/jwt"
	"google.golang.org/protobuf/proto"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/errorMatcher"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
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
				CdServer:                     "kuberpult-cd-service:8443",
				ManifestExportServer:         "kuberpult-manifest-repo-export-service:8443",
				ArgocdNamespace:              "tools",
				AzureCloudInstance:           "https://login.microsoftonline.com/",
				AzureEnableAuth:              false,
				DexFullNameOverride:          "kuberpult-dex",
				BatchClientTimeout:           2 * time.Minute,
				MaxWaitDuration:              10 * time.Minute,
				ApiEnableDespiteNoAuth:       false,
				IapEnabled:                   false,
				GrpcMaxRecvMsgSize:           4,
				RevisionsEnabled:             false,
				ReleaseYamlValidationEnabled: true,
				GitAuthorName:                "git-name1",
				GitAuthorEmail:               "git-email2",
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

				"KUBERPULT_ROOT_APPS_POINT_TO_BRACKETS": "true",
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

				BatchClientTimeout:           22 * time.Minute,
				MaxWaitDuration:              33 * time.Minute,
				ApiEnableDespiteNoAuth:       true,
				IapEnabled:                   true,
				GrpcMaxRecvMsgSize:           50,
				RevisionsEnabled:             true,
				ReleaseYamlValidationEnabled: true,
				RootAppsPointToBrackets:      true,
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

			var actual *config.ServerConfig
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

func TestAuthServeHTTPInner(t *testing.T) {
	const (
		azureClientId = "testClientId"
		azureTenantId = "testTenantId"
		azureName     = "Azure User"
		azureEmail    = "azure.user@example.com"
		dexEmail      = "dex-user@example.com"
		dexName       = "Dex User"
	)

	azureJWKS, err := makeAzureTestJWKS()
	if err != nil {
		t.Fatalf("failed to create test JWKS: %v", err)
	}
	azureToken, err := makeAzureTestToken(azureClientId, azureTenantId, azureName, azureEmail)
	if err != nil {
		t.Fatalf("failed to create test token: %v", err)
	}

	tcs := []struct {
		Name          string
		ServerConfig  *config.ServerConfig
		Request       *http.Request
		DexClaims     map[string]any // nil = no Dex cookie attached at all
		AttachAzure   bool
		Policy        *auth.RBACPolicies
		ExpectedError string
		ExpectedUser  *auth.User // nil = don't check (e.g. the error case)
		ClientEmail   string     // sent by the client
		ClientName    string     // sent by the client
	}{{
		Name:         "server config nil",
		ServerConfig: nil,
		Request: &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/test"},
			Header: http.Header{},
		},
		ExpectedError: "serverConfig is nil in Auth middleware",
	},
		{
			Name: "Dex login succeeds: combined user is the Dex user, not the default",
			ServerConfig: &config.ServerConfig{
				//exhaustruct:ignore
				DexEnabled:  true,
				DexClientId: "test-client",
				// unused when the request has no Dex cookie/token to validate against a live
				// server, but here it must point at our mock OIDC server for discovery to succeed.
				DexUseClusterInternalCommunication: false,
			},
			DexClaims: map[string]any{
				"email": dexEmail,
				"name":  dexName,
			},
			Policy: &auth.RBACPolicies{},
			ExpectedUser: &auth.User{
				Email:          dexEmail,
				Name:           dexName,
				DexAuthContext: &auth.DexAuthContext{},
			},
		},
		{
			Name: "Dex login succeeds and policy arrives in claims",
			ServerConfig: &config.ServerConfig{
				//exhaustruct:ignore
				DexEnabled:  true,
				DexClientId: "test-client",
				// unused when the request has no Dex cookie/token to validate against a live
				// server, but here it must point at our mock OIDC server for discovery to succeed.
				DexUseClusterInternalCommunication: false,
			},
			DexClaims: map[string]any{
				"email": dexEmail,
				"name":  dexName,
			},
			Policy: &auth.RBACPolicies{Groups: map[string]auth.RBACGroup{
				"x": {Group: dexEmail, Role: "Developer_ROLE"},
			}},
			ExpectedUser: &auth.User{
				Email:          dexEmail,
				Name:           dexName,
				DexAuthContext: &auth.DexAuthContext{Role: []string{"Developer_ROLE"}},
			},
		},
		{
			Name: "Dex login succeeds without a name claim: email is used, name falls back to email",
			ServerConfig: &config.ServerConfig{
				DexEnabled:  true,
				DexClientId: "test-client",
			},
			DexClaims: map[string]any{
				"email": dexEmail,
			},
			Policy: &auth.RBACPolicies{},
			ExpectedUser: &auth.User{
				Email:          dexEmail,
				Name:           dexEmail,
				DexAuthContext: &auth.DexAuthContext{},
			},
		},
		{
			Name: "Dex enabled, no token at all: falls back to the default user",
			ServerConfig: &config.ServerConfig{
				//exhaustruct:ignore
				DexEnabled:  true,
				DexClientId: "test-client",
			},
			DexClaims: nil,
			Policy:    &auth.RBACPolicies{},
			ExpectedUser: &auth.User{
				Email: "default@example.com",
				Name:  "default",
			},
		},

		{
			Name: "Azure succeeds, Dex enabled but no Dex session: Azure identity must be preserved",
			ServerConfig: &config.ServerConfig{
				//exhaustruct:ignore
				AzureEnableAuth: true,
				AzureClientId:   azureClientId,
				AzureTenantId:   azureTenantId,
				DexEnabled:      true,
				DexClientId:     "test-client",
			},
			AttachAzure: true,
			DexClaims:   nil,
			Policy:      &auth.RBACPolicies{},
			ExpectedUser: &auth.User{
				Email: azureEmail,
				Name:  azureName,
			},
		},
		{
			Name: "Dex token with groups but no email, no other source: default identity, roles from groups",
			ServerConfig: &config.ServerConfig{
				DexEnabled:  true,
				DexClientId: "test-client",
			},
			DexClaims: map[string]any{
				"groups": []string{"group1"},
			},
			Policy: &auth.RBACPolicies{Groups: map[string]auth.RBACGroup{
				"x": {Group: "group1", Role: "Group1_ROLE"},
			}},
			ExpectedUser: &auth.User{
				Email:          "default@example.com",
				Name:           "default",
				DexAuthContext: &auth.DexAuthContext{Role: []string{"Group1_ROLE"}},
			},
		},
		{
			Name: "Azure succeeds and Dex token has groups but no email: Azure identity, roles from groups",
			ServerConfig: &config.ServerConfig{
				AzureEnableAuth: true,
				AzureClientId:   azureClientId,
				AzureTenantId:   azureTenantId,
				DexEnabled:      true,
				DexClientId:     "test-client",
			},
			AttachAzure: true,
			DexClaims: map[string]any{
				"groups": []string{"group1"},
			},
			Policy: &auth.RBACPolicies{Groups: map[string]auth.RBACGroup{
				"x": {Group: "group1", Role: "Group1_ROLE"},
			}},
			ExpectedUser: &auth.User{
				Email:          azureEmail,
				Name:           azureName,
				DexAuthContext: &auth.DexAuthContext{Role: []string{"Group1_ROLE"}},
			},
		},
		{
			Name: "Azure and Dex both succeed with an email: az identity is kept and dex roles win",
			ServerConfig: &config.ServerConfig{
				AzureEnableAuth: true,
				AzureClientId:   azureClientId,
				AzureTenantId:   azureTenantId,
				DexEnabled:      true,
				DexClientId:     "test-client",
			},
			AttachAzure: true,
			DexClaims: map[string]any{
				"email": dexEmail,
				"name":  dexName,
			},
			Policy: &auth.RBACPolicies{Groups: map[string]auth.RBACGroup{
				"x": {Group: dexEmail, Role: "Developer_ROLE"},
			}},
			ExpectedUser: &auth.User{
				Email:          azureEmail,
				Name:           azureName,
				DexAuthContext: &auth.DexAuthContext{Role: []string{"Developer_ROLE"}},
			},
		},
		{
			Name: "Dex token with groups but no email ignores client-supplied author headers",
			ServerConfig: &config.ServerConfig{
				DexEnabled:  true,
				DexClientId: "test-client",
			},
			DexClaims: map[string]any{
				"groups": []string{"group1"},
			},
			Policy: &auth.RBACPolicies{Groups: map[string]auth.RBACGroup{
				"x": {Group: "group1", Role: "Group1_ROLE"},
			}},
			ClientEmail: "spoofed@evil.example.com",
			ClientName:  "Spoofed User",
			ExpectedUser: &auth.User{
				Email:          "default@example.com",
				Name:           "default",
				DexAuthContext: &auth.DexAuthContext{Role: []string{"Group1_ROLE"}},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			req := tc.Request
			if req == nil {
				req = httptest.NewRequest(http.MethodPut, "/test", nil)
			}
			if tc.AttachAzure {
				req.Header.Set("authorization", azureToken)
			}
			if tc.ClientEmail != "" {
				req.Header.Set(auth.HeaderUserEmail, auth.Encode64(tc.ClientEmail))
			}
			if tc.ClientName != "" {
				req.Header.Set(auth.HeaderUserName, auth.Encode64(tc.ClientName))
			}
			if tc.ServerConfig != nil && tc.ServerConfig.DexEnabled {
				if tc.DexClaims != nil {
					keySet, privateKey, err := makeDexKeySet()
					if err != nil {
						t.Fatalf("failed to create dex key set: %v", err)
					}
					oidcServer := makeDexOIDCServer(keySet)
					defer oidcServer.Close()
					tc.ServerConfig.DexBaseURL = oidcServer.URL

					claims := map[string]any{"aud": tc.ServerConfig.DexClientId, "iss": oidcServer.URL + "/dex"}
					for k, v := range tc.DexClaims {
						claims[k] = v
					}
					token, err := signDexToken(privateKey, claims)
					if err != nil {
						t.Fatalf("failed to sign dex token: %v", err)
					}
					req.AddCookie(&http.Cookie{Name: "kuberpult.oauth", Value: token})
				} else {
					// No cookie/token attached: point DexBaseURL somewhere that is never
					// dialed (Dex verification fails before any network call, see VerifyToken).
					tc.ServerConfig.DexBaseURL = "http://dex.invalid"
				}
			}
			var capturedRequest *http.Request
			mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
			})
			authMw := &Auth{
				HttpServer: mockHandler,
				DefaultUser: auth.User{
					Name:  "default",
					Email: "default@example.com",
				},
				Policy:       tc.Policy,
				AzureJWKS:    azureJWKS,
				serverConfig: tc.ServerConfig,
			}
			w := &mockResponseWriter{}

			err := authMw.serveHTTPInner(context.Background(), w, req)

			if tc.ExpectedError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.ExpectedError) {
					t.Errorf("expected error to contain %q, got %v", tc.ExpectedError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if tc.ExpectedUser != nil {
				gotUser, err := auth.ReadUserFromContext(capturedRequest.Context())
				if err != nil {
					t.Fatalf("could not read user from context: %v", err)
				}
				if diff := cmp.Diff(tc.ExpectedUser, gotUser); diff != "" {
					t.Errorf("user mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func makeAzureTestJWKS() (*keyfunc.JWKS, error) {
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(`-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC/oyqURHIPNzx4vcKrUUZYr6Bx
q2OSD44a63zeIDA1oZkR+sactmkub+8NI49GqrbssWf944v3ZLp8KXMh6i+U9pkS
dDfvKcQUProQ+Tlm/m0SFXa6h7vq6iVD1uawzN9aQaR7WiKV1TuPGUgE86/l+XTv
LZ/MbKh0tz9j8JtY4QIDAQAB
-----END PUBLIC KEY-----`))
	if err != nil {
		return nil, err
	}
	givenKey := keyfunc.NewGivenRSA(publicKey, keyfunc.GivenKeyOptions{})
	return keyfunc.NewGiven(map[string]keyfunc.GivenKey{"testKid": givenKey}), nil
}

func makeAzureTestToken(clientId, tenantId, name, email string) (string, error) {
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXQIBAAKBgQC/oyqURHIPNzx4vcKrUUZYr6Bxq2OSD44a63zeIDA1oZkR+sac
tmkub+8NI49GqrbssWf944v3ZLp8KXMh6i+U9pkSdDfvKcQUProQ+Tlm/m0SFXa6
h7vq6iVD1uawzN9aQaR7WiKV1TuPGUgE86/l+XTvLZ/MbKh0tz9j8JtY4QIDAQAB
AoGBAICNeROq8oSIfjVUvlDkHXeCoPN/kDS74IzoaYQsPYrMk30/J5qatuYiyk6b
CxLRlBIlU+g5i3vygzKlL4mRqkZuCM4xPbpuW9sdZp61TxWZk7Tm+SYBTStYSGkT
tPmvnKsYWkUh1WDSkeLJqHkRbQXAZJkAKRMYgLu2F29fWOZBAkEA8P31nm/AiDiD
dkGSGp4GVQ5BBry3XdP3c6rfzmW8sMElxqoj2watdia72+grf8eVo8vtsTiOrVUD
ZoS5C5GKKQJBAMuSXXQZrBa4qB7YkGi5ysQRQZoegdYZa44q9L9oBE/iEl/ejR1l
EKZi+v2greoIruqczGAD7VbEiwT50+npH/kCQQDJgpGvOaK0RQ0oBQw2VYzV8mVN
TN/HBUcU4PzjiQ6OffMoe3wf2SWSdjD/YNN+tVTa8dp/Jdun9D4zqydQFRKBAkBV
zlPl5AxNZ3g1yELWYbm9+ygTtlgzznMvcZvIMiffJANqtXv1r+vctkvlLB0iUJap
/X2H2x/nOuD+L+/K4KDBAkAHcO3Gv7VZsSHfnd/JfDzxtL0MFWerGZyGlaNFmX27
1dWRXvcS5A0zPMgiBWfvHFx2DpSiceffqnis+UryeE+L
-----END RSA PRIVATE KEY-----`))
	if err != nil {
		return "", fmt.Errorf("could not parse RSA private key: %w", err)
	}
	claims := jwt.MapClaims{
		"aud":   clientId,
		"tid":   tenantId,
		"name":  name,
		"email": email,
		"exp":   time.Now().Add(10 * time.Minute).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "testKid"
	return tok.SignedString(privateKey)
}

func TestGetRequestAuthorFromAzure(t *testing.T) {
	const (
		testClientId = "testClientId"
		testTenantId = "testTenantId"
		testName     = "Test User"
		testEmail    = "test.user@example.com"
	)

	jwks, err := makeAzureTestJWKS()
	if err != nil {
		t.Fatalf("failed to create test JWKS: %v", err)
	}
	validToken, err := makeAzureTestToken(testClientId, testTenantId, testName, testEmail)
	if err != nil {
		t.Fatalf("failed to create test token: %v", err)
	}

	tcs := []struct {
		Name          string
		Authorization string // value for "authorization" header
		AuthorName    string // base64-encoded value for "author-name" header
		AuthorEmail   string // base64-encoded value for "author-email" header
		JWKSNil       bool
		ClientId      string
		TenantId      string
		ExpectedUser  *auth.User
	}{
		{
			Name:         "no headers returns nil",
			ClientId:     testClientId,
			TenantId:     testTenantId,
			ExpectedUser: nil,
		},
		{
			Name:        "author headers only returns user from headers",
			AuthorName:  auth.Encode64("ci-bot"),
			AuthorEmail: auth.Encode64("ci@example.com"),
			ClientId:    testClientId,
			TenantId:    testTenantId,
			ExpectedUser: &auth.User{
				Name:           "ci-bot",
				Email:          "ci@example.com",
				DexAuthContext: &auth.DexAuthContext{Role: []string{""}},
			},
		},
		{
			Name:          "valid JWT returns user from JWT claims",
			Authorization: validToken,
			ClientId:      testClientId,
			TenantId:      testTenantId,
			ExpectedUser:  &auth.User{Name: testName, Email: testEmail},
		},
		{
			Name:          "valid JWT takes priority over author headers",
			Authorization: validToken,
			AuthorName:    auth.Encode64("ci-bot"),
			AuthorEmail:   auth.Encode64("ci@example.com"),
			ClientId:      testClientId,
			TenantId:      testTenantId,
			ExpectedUser:  &auth.User{Name: testName, Email: testEmail},
		},
		{
			Name:          "nil JWKS skips JWT validation and falls back to author headers",
			Authorization: validToken,
			AuthorName:    auth.Encode64("ci-bot"),
			AuthorEmail:   auth.Encode64("ci@example.com"),
			JWKSNil:       true,
			ClientId:      testClientId,
			TenantId:      testTenantId,
			ExpectedUser: &auth.User{
				Name:           "ci-bot",
				Email:          "ci@example.com",
				DexAuthContext: &auth.DexAuthContext{Role: []string{""}},
			},
		},
		{
			Name:          "invalid JWT falls back to author headers",
			Authorization: "not.a.validtoken",
			AuthorName:    auth.Encode64("ci-bot"),
			AuthorEmail:   auth.Encode64("ci@example.com"),
			ClientId:      testClientId,
			TenantId:      testTenantId,
			ExpectedUser: &auth.User{
				Name:           "ci-bot",
				Email:          "ci@example.com",
				DexAuthContext: &auth.DexAuthContext{Role: []string{""}},
			},
		},
		{
			Name:          "JWT with wrong clientId falls back to author headers",
			Authorization: validToken,
			AuthorName:    auth.Encode64("ci-bot"),
			AuthorEmail:   auth.Encode64("ci@example.com"),
			ClientId:      "wrongClientId",
			TenantId:      testTenantId,
			ExpectedUser: &auth.User{
				Name:           "ci-bot",
				Email:          "ci@example.com",
				DexAuthContext: &auth.DexAuthContext{Role: []string{""}},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequest(http.MethodPost, "/api/release", nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			if tc.Authorization != "" {
				req.Header.Set("authorization", tc.Authorization)
			}
			if tc.AuthorName != "" {
				req.Header.Set(auth.HeaderUserName, tc.AuthorName)
			}
			if tc.AuthorEmail != "" {
				req.Header.Set(auth.HeaderUserEmail, tc.AuthorEmail)
			}

			testJWKS := jwks
			if tc.JWKSNil {
				testJWKS = nil
			}

			got, err := getRequestAuthorFromAzure(context.Background(), req, testJWKS, tc.ClientId, tc.TenantId)
			if diff := cmp.Diff(nil, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("unexpected error (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.ExpectedUser, got); diff != "" {
				t.Errorf("user mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

// makeTestPgpKeyringFile creates a temporary armored PGP public-key file and returns its path.
// Required when running the server with AzureEnableAuth=true.
func makeTestPgpKeyringFile(t *testing.T) string {
	t.Helper()
	entity, err := openpgp.NewEntity("Test", "", "test@example.com", nil)
	if err != nil {
		t.Fatalf("failed to create PGP entity: %v", err)
	}
	path := filepath.Join(t.TempDir(), "keyring.asc")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create PGP keyring file: %v", err)
	}
	defer f.Close()
	w, err := armor.Encode(f, openpgp.PublicKeyType, nil)
	if err != nil {
		t.Fatalf("failed to create armor encoder: %v", err)
	}
	if err := entity.Serialize(w); err != nil {
		t.Fatalf("failed to serialize PGP entity: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close armor writer: %v", err)
	}
	return path
}

func TestServerApiEnableDespiteNoAuthWithAzure(t *testing.T) {
	tcs := []struct {
		Name                   string
		AzureEnableAuth        bool
		ApiEnableDespiteNoAuth bool
		ExpectedUnauthorized   bool
	}{
		{
			// Bug: the Azure middleware rejects /api/release before ApiEnableDespiteNoAuth is checked.
			// This case should NOT return 401 once the bug is fixed.
			Name:                   "Azure=true, ApiNoAuth=true - should pass through Azure middleware",
			AzureEnableAuth:        true,
			ApiEnableDespiteNoAuth: true,
			ExpectedUnauthorized:   false,
		},
		{
			Name:                   "Azure=true, ApiNoAuth=false - Azure blocks unauthenticated request",
			AzureEnableAuth:        true,
			ApiEnableDespiteNoAuth: false,
			ExpectedUnauthorized:   true,
		},
		{
			Name:                   "Azure=false, ApiNoAuth=true - request reaches handler",
			AzureEnableAuth:        false,
			ApiEnableDespiteNoAuth: true,
			ExpectedUnauthorized:   false,
		},
		{
			Name:                   "Azure=false, ApiNoAuth=false - /api unavailable without auth method",
			AzureEnableAuth:        false,
			ApiEnableDespiteNoAuth: false,
			ExpectedUnauthorized:   true,
		},
	}

	for _, tc := range tcs {
		tc := tc
		// NOTE: subtests must NOT run in parallel — they share port 8081 and jwksInitAzure.
		t.Run(tc.Name, func(t *testing.T) {
			if tc.AzureEnableAuth {
				orig := jwksInitAzure
				jwksInitAzure = func(_ context.Context) (*keyfunc.JWKS, error) {
					return makeAzureTestJWKS()
				}
				defer func() { jwksInitAzure = orig }()

				t.Setenv("KUBERPULT_AZURE_ENABLE_AUTH", "true")
				t.Setenv("KUBERPULT_AZURE_CLIENT_ID", "testClientId")
				t.Setenv("KUBERPULT_AZURE_TENANT_ID", "testTenantId")
				t.Setenv("KUBERPULT_PGP_KEY_RING_PATH", makeTestPgpKeyringFile(t))
			}
			if tc.ApiEnableDespiteNoAuth {
				t.Setenv("KUBERPULT_API_ENABLE_DESPITE_NO_AUTH", "true")
			}

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
						<-time.After(1 * time.Second)
						_ = res.Body.Close()
						continue
					}
					_ = res.Body.Close()
					break
				}

				req, err := http.NewRequest(http.MethodPost, "http://localhost:8081/api/release", nil)
				if err != nil {
					t.Errorf("failed to create request: %v", err)
					return
				}
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Errorf("failed to do request: %v", err)
					return
				}
				defer res.Body.Close()
				body, _ := io.ReadAll(res.Body)
				t.Logf("status: %d, body: %q", res.StatusCode, body)

				if tc.ExpectedUnauthorized {
					if res.StatusCode != http.StatusUnauthorized {
						t.Errorf("expected 401 Unauthorized but got %d (body: %q)", res.StatusCode, body)
					}
				} else {
					if res.StatusCode == http.StatusUnauthorized {
						t.Errorf("expected request to pass auth but got 401 Unauthorized (body: %q)", body)
					}
				}
			}(t)

			td := t.TempDir()
			if err := os.Mkdir(filepath.Join(td, "build"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(td, "build", "index.html"), []byte(`<!doctype html><html lang="en"></html>`), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(td); err != nil {
				t.Fatal(err)
			}
			t.Setenv("KUBERPULT_GIT_AUTHOR_EMAIL", "mail2")
			t.Setenv("KUBERPULT_GIT_AUTHOR_NAME", "name1")
			t.Setenv("KUBERPULT_GRPC_MAX_RECV_MSG_SIZE", "4")

			if err := runServer(ctx); err != nil {
				t.Fatalf("runServer returned unexpected error: %q", err)
			}
			wg.Wait()
		})
	}
}

type mockResponseWriter struct {
	header http.Header
}

func (m *mockResponseWriter) Header() http.Header {
	if m.header == nil {
		m.header = http.Header{}
	}
	return m.header
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	// no-op
}

// makeDexKeySet generates an RSA keypair wrapped as a JWK set, for signing/verifying test Dex tokens.
func makeDexKeySet() (jwk.Set, jwk.Key, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	jwkPrivateKey, err := jwk.FromRaw(privateKey)
	if err != nil {
		return nil, nil, err
	}
	jwkPublicKey, err := jwk.FromRaw(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	_ = jwkPrivateKey.Set(jwk.KeyIDKey, "dex-test-kid")
	_ = jwkPublicKey.Set(jwk.KeyIDKey, "dex-test-kid")
	keySet := jwk.NewSet()
	if err := keySet.AddKey(jwkPublicKey); err != nil {
		return nil, nil, err
	}
	return keySet, jwkPrivateKey, nil
}

// signDexToken signs claims (plus a 1h expiry) as an RS256 JWT, mimicking a Dex ID token.
func signDexToken(privateKey jwk.Key, claims map[string]any) (string, error) {
	token := jwxjwt.New()
	_ = token.Set(jwxjwt.ExpirationKey, time.Now().Add(time.Hour).Unix())
	for k, v := range claims {
		_ = token.Set(k, v)
	}
	signed, err := jwxjwt.Sign(token, jwxjwt.WithKey(jwa.RS256, privateKey))
	if err != nil {
		return "", err
	}
	return string(signed), nil
}

// makeDexOIDCServer serves the OIDC discovery doc and JWKS endpoint that ValidateOIDCToken
// discovers via DexBaseURL+"/dex" (see pkg/auth/dex.go).
func makeDexOIDCServer(keySet jwk.Set) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/dex/.well-known/openid-configuration":
			_, _ = fmt.Fprintf(w, `{
    "issuer": "%[1]s/dex",
    "authorization_endpoint": "%[1]s/dex/auth",
    "token_endpoint": "%[1]s/dex/token",
    "jwks_uri": "%[1]s/dex/keys",
    "response_types_supported": ["code"],
    "subject_types_supported": ["public"],
    "id_token_signing_alg_values_supported": ["RS256"]
  }`, ts.URL)
		case "/dex/keys":
			out, _ := json.Marshal(keySet)
			_, _ = w.Write(out)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	return ts
}
