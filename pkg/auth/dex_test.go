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

package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	jwtV5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

func TestNewDexAppClient(t *testing.T) {
	DEX_URL, _ := url.Parse(dexServiceURL)
	testCases := []struct {
		Name                            string
		clientID                        string
		clientSecret                    string
		baseURL                         string
		scopes                          []string
		useClusterInternalCommunication bool
		wantErr                         bool
		wantClientApp                   *DexAppClient
	}{
		{
			Name:                            "Creates the a new Dex App Client as expected",
			clientID:                        "test-client",
			clientSecret:                    "test-secret",
			baseURL:                         "www.test-url.com",
			scopes:                          []string{"scope1", "scope2"},
			useClusterInternalCommunication: false,
			wantErr:                         false,
			wantClientApp: &DexAppClient{
				ClientID:                        "test-client",
				ClientSecret:                    "test-secret",
				RedirectURI:                     "www.test-url.com/callback",
				IssuerURL:                       "www.test-url.com/dex",
				BaseURL:                         "www.test-url.com",
				UseClusterInternalCommunication: false,
				Scopes:                          []string{"scope1", "scope2"},
				Client: &http.Client{
					Transport: DexRewriteURLRoundTripper{
						DexURL: DEX_URL,
						T:      http.DefaultTransport,
					},
				},
			},
		},
		{
			Name:                            "Creates the a new Dex App Client as expected with internal communication",
			clientID:                        "test-client",
			clientSecret:                    "test-secret",
			baseURL:                         "www.test-url.com",
			scopes:                          []string{"scope1", "scope2"},
			useClusterInternalCommunication: true,
			wantErr:                         false,
			wantClientApp: &DexAppClient{
				ClientID:                        "test-client",
				ClientSecret:                    "test-secret",
				RedirectURI:                     "www.test-url.com/callback",
				IssuerURL:                       "www.test-url.com/dex",
				BaseURL:                         "www.test-url.com",
				UseClusterInternalCommunication: true,
				Scopes:                          []string{"scope1", "scope2"},
				Client: &http.Client{
					Transport: DexRewriteURLRoundTripper{
						DexURL: DEX_URL,
						T:      http.DefaultTransport,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			http.DefaultServeMux = http.NewServeMux()
			a, err := NewDexAppClient(tc.clientID, tc.clientSecret, tc.baseURL, tc.scopes, tc.useClusterInternalCommunication)
			if (err != nil) != tc.wantErr {
				t.Errorf("creating new dex client error = %v, wantErr %v", err, tc.wantErr)
			}
			if diff := cmp.Diff(a, tc.wantClientApp, cmpopts.IgnoreFields(DexRewriteURLRoundTripper{}, "T")); diff != "" {
				t.Errorf("got %v, want %v, diff (-want +got) %s", a, tc.wantClientApp, diff)
			}
		})
	}
}

func TestNewDexReverseProxy(t *testing.T) {
	testCases := []struct {
		Name           string
		mockDexServer  *httptest.Server
		wantStatusCode int
	}{
		{
			Name:           "Dex reverse proxy is working as expected on success",
			mockDexServer:  makeNewMockServer(http.StatusOK),
			wantStatusCode: http.StatusOK,
		},
		{
			Name:           "Dex reverse proxy is working as expected on error",
			mockDexServer:  makeNewMockServer(http.StatusInternalServerError),
			wantStatusCode: http.StatusInternalServerError,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// mock Dex server the app is being redirected to.
			mockDexServer := tc.mockDexServer
			defer mockDexServer.Close()
			server := httptest.NewServer(http.HandlerFunc(NewDexReverseProxy(mockDexServer.URL)))
			defer server.Close()
			resp, err := http.Get(server.URL)
			if err != nil {
				t.Errorf("could not create HTTP request: %s", err)
			}
			if diff := cmp.Diff(resp.StatusCode, tc.wantStatusCode); diff != "" {
				t.Errorf("got %v, want %v, diff (-want +got) %s", resp.StatusCode, tc.wantStatusCode, diff)
			}
		})
	}
}

func TestDexRoundTripper(t *testing.T) {
	testCases := []struct {
		Name           string
		mockDexServer  *httptest.Server
		wantStatusCode int
	}{
		{
			Name: "Round tripper works as expected",
			mockDexServer: httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.WriteHeader(http.StatusOK)
			})),
			wantStatusCode: http.StatusOK,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// mock Dex server the app is being redirected to.
			mockDexServer := tc.mockDexServer
			defer mockDexServer.Close()
			serverURL, _ := url.Parse(mockDexServer.URL)
			rt := DexRewriteURLRoundTripper{
				DexURL: serverURL,
				T:      http.DefaultTransport,
			}
			req, _ := http.NewRequest(http.MethodGet, "/", bytes.NewBuffer([]byte("")))
			rt.RoundTrip(req)
			target, _ := url.Parse(mockDexServer.URL)
			if diff := cmp.Diff(req.Host, target.Host); diff != "" {
				t.Errorf("got %v, want %v, diff (-want +got) %s", req.Host, target.Host, diff)
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	// Reset Handler registration. Avoids "panic: http: multiple registrations for <PATH>" error.
	http.DefaultServeMux = new(http.ServeMux)
	// Dex app client configuration variables.
	clientID := "test-client"
	clientSecret := "test-client"
	hostURL := "https://www.test.com"
	scopes := []string{"scope1", "scope2"}
	useClusterInternalCommunication := false
	appDex, _ := NewDexAppClient(clientID, clientSecret, hostURL, scopes, useClusterInternalCommunication)

	testCases := []struct {
		Name            string
		allowedAudience string
		dexApp          *DexAppClient
		wantErr         bool
	}{
		{
			Name:            "Token Verifier works as expected with the correct audience",
			dexApp:          appDex,
			allowedAudience: "test-client",
			wantErr:         false,
		},
		{
			Name:            "Token Verifier works as expected with the wrong audience",
			dexApp:          appDex,
			allowedAudience: "wrong-audience",
			wantErr:         true,
		},
	}
	for _, tc := range testCases {
		// Create a key set, private key and public key.
		keySet, jwkPrivateKey, _ := getJWKeySet()
		t.Run(tc.Name, func(t *testing.T) {
			// Mocks the OIDC server to retrieve the provider.
			oidcServer := MockOIDCTestServer(appDex.IssuerURL, keySet)
			defer oidcServer.Close()

			// Disable the TLS check to allow the test to run.
			dexURL, _ := url.Parse(oidcServer.URL)
			httpClient := &http.Client{
				Transport: DexRewriteURLRoundTripper{
					DexURL: dexURL,
					T: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					},
				},
			}
			claims := jwtV5.MapClaims{jwt.AudienceKey: tc.dexApp.ClientID, jwt.IssuerKey: tc.dexApp.IssuerURL}
			token, _ := GetSignedJwt(jwkPrivateKey, claims)

			ctx := oidc.ClientContext(context.Background(), httpClient)
			_, err := ValidateOIDCToken(ctx, appDex.IssuerURL, string(token), tc.allowedAudience, useClusterInternalCommunication)
			if (err != nil) != tc.wantErr {
				t.Errorf("creating new dex client error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestVerifyToken(t *testing.T) {
	// Reset Handler registration. Avoids "panic: http: multiple registrations for <PATH>" error.
	http.DefaultServeMux = new(http.ServeMux)
	// Dex app client configuration variables.
	clientID := "test-client"
	clientSecret := "test-client"
	hostURL := "https://www.test.com"
	scopes := []string{"scope1", "scope2"}
	useClusterInternalCommunication := false
	appDex, _ := NewDexAppClient(clientID, clientSecret, hostURL, scopes, useClusterInternalCommunication)

	testCases := []struct {
		Name      string
		claims    jwtV5.MapClaims
		wantErr   string
		wantClaim jwtV5.MapClaims
	}{
		{
			Name: "Token Verifier works as expected with the correct token value",
			claims: jwtV5.MapClaims{
				jwt.AudienceKey: clientID,
				jwt.IssuerKey:   appDex.IssuerURL,
				"name":          "User",
				"email":         "user@mail.com",
				"groups":        []string{"Developer"}},
			wantClaim: jwtV5.MapClaims{
				"email":  "user@mail.com",
				"name":   string("User"),
				"groups": []any{string("Developer")},
			},
		},
		{
			Name: "Token Verifier works as expected with no name",
			claims: jwtV5.MapClaims{
				jwt.AudienceKey: clientID,
				jwt.IssuerKey:   appDex.IssuerURL,
				"name":          ""},
			wantErr: "need required fields to determine group of user",
		},
	}
	for _, tc := range testCases {
		// Create a key set, private key and public key.
		t.Run(tc.Name, func(t *testing.T) {
			keySet, jwkPrivateKey, _ := getJWKeySet()
			idToken, _ := GetSignedJwt(jwkPrivateKey, tc.claims)

			// Mocks the OIDC server to retrieve the provider.
			oidcServer := MockOIDCTestServer(appDex.IssuerURL, keySet)
			defer oidcServer.Close()

			// Disable the TLS check to allow the test to run.
			dexURL, _ := url.Parse(oidcServer.URL)
			httpClient := &http.Client{
				Transport: DexRewriteURLRoundTripper{
					DexURL: dexURL,
					T: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					},
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			cookie := &http.Cookie{
				Name:  dexOAUTHTokenName,
				Value: string(idToken),
			}
			req.AddCookie(cookie)

			ctx := oidc.ClientContext(context.Background(), httpClient)
			u, err := VerifyToken(ctx, req, appDex.ClientID, hostURL, useClusterInternalCommunication)
			if err != nil {
				if diff := cmp.Diff(tc.wantErr, err.Error()); diff != "" {
					t.Errorf("Error mismatch (-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(u["groups"], tc.wantClaim["groups"]); diff != "" {
					t.Errorf("got %v, want %v, diff (-want +got) %s", u, tc.wantClaim, diff)
				}
			}
		})
	}
}

// Helper function to make a new mock server.
func makeNewMockServer(status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(status)
	}))
}

// Creates a signed JWT token with the respective claims.
func GetSignedJwt(signingKey any, claims jwtV5.MapClaims) ([]byte, error) {
	token := jwt.New()
	_ = token.Set(jwt.ExpirationKey, time.Now().Add(time.Hour*24).Unix())

	for key, value := range claims {
		_ = token.Set(key, value)
	}

	signedToken, _ := jwt.Sign(token, jwt.WithKey(jwa.RS256, signingKey))
	return signedToken, nil
}

// Generates and returns a key set, private key and public key.
func getJWKeySet() (keySet jwk.Set, jwkPrivateKey, jwkPublicKey jwk.Key) {
	rsaPrivate, rsaPublic := getRSAKeyPair()
	jwkPrivateKey, _ = jwk.FromRaw(rsaPrivate)
	jwkPublicKey, _ = jwk.FromRaw(rsaPublic)

	_ = jwkPrivateKey.Set(jwk.KeyIDKey, "my-unique-kid")
	_ = jwkPublicKey.Set(jwk.KeyIDKey, "my-unique-kid")

	keySet = jwk.NewSet()
	err := keySet.AddKey(jwkPublicKey)
	if err != nil {
		return nil, nil, nil
	}

	return keySet, jwkPrivateKey, jwkPublicKey
}

// Generates and returns a rsa key pair
func getRSAKeyPair() (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	publicKey := &privateKey.PublicKey
	return privateKey, publicKey
}

// Mocks the OIDC server to get all provider.
func MockOIDCTestServer(issuerURL string, keySet jwk.Set) *httptest.Server {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/dex/.well-known/openid-configuration":
			io.WriteString(w, fmt.Sprintf(`
{
  "issuer": "%[1]s",
  "authorization_endpoint": "%[1]s/auth",
  "token_endpoint": "%[1]s/token",
  "jwks_uri": "%[1]s/keys",
  "userinfo_endpoint": "%[1]s/userinfo",
  "device_authorization_endpoint": "%[1]s/device/code",
  "grant_types_supported": ["authorization_code"],
  "response_types_supported": ["code"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "code_challenge_methods_supported": ["S256", "plain"],
  "scopes_supported": ["openid"],
  "token_endpoint_auth_methods_supported": ["client_secret_basic", "client_secret_post"],
  "claims_supported": ["sub", "aud", "exp"]
}`, issuerURL))
		case "/dex/keys":
			out, _ := json.Marshal(keySet)
			_, _ = io.WriteString(w, string(out))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	return ts
}
