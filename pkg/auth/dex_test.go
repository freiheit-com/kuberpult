package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNewDexAppClient(t *testing.T) {
	DEX_URL, _ := url.Parse(dexServiceURL)
	testCases := []struct {
		Name          string
		clientID      string
		clientSecret  string
		baseURL       string
		scopes        []string
		wantErr       bool
		wantClientApp *DexAppClient
	}{
		{
			Name:         "Creates the a new Dex App Client as expected",
			clientID:     "test-client",
			clientSecret: "test-secret",
			baseURL:      "www.test-url.com",
			scopes:       []string{"scope1", "scope2"},
			wantErr:      false,
			wantClientApp: &DexAppClient{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				RedirectURI:  "www.test-url.com/callback",
				IssuerURL:    "www.test-url.com/dex",
				BaseURL:      "www.test-url.com",
				Scopes:       []string{"scope1", "scope2"},
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
			a, err := NewDexAppClient(tc.clientID, tc.clientSecret, tc.baseURL, tc.scopes)
			if (err != nil) != tc.wantErr {
				t.Errorf("build dependency map error = %v, wantErr %v", err, tc.wantErr)
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
		wantErr        bool
		wantStatusCode int
	}{
		{
			Name: "Dex reverse proxy is working as expected on success",
			mockDexServer: httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.WriteHeader(http.StatusOK)
			})),
			wantStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			Name: "Dex reverse proxy is working as expected on error",
			mockDexServer: httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.WriteHeader(http.StatusInternalServerError)
			})),
			wantStatusCode: http.StatusInternalServerError,
			wantErr:        false,
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
			if (err != nil) != tc.wantErr {
				t.Errorf("build dependency map error = %v, wantErr %v", err, tc.wantErr)
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
