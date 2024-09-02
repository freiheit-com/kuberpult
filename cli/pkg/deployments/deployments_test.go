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

package deployments

import (
	"net/http"
	"net/url"
	"testing"

	kutil "github.com/freiheit-com/kuberpult/cli/pkg/kuberpult_utils"
)

var iapToken string = "iap_token"
var dexToken string = "dex_token"

func TestCreateHttpRequest(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		authParams      kutil.AuthenticationParameters
		parameters      *CommitDeploymentsParameters
		expectedRequest *http.Request
	}{
		{
			name: "valid url",
			url:  "http://example.com",
			authParams: kutil.AuthenticationParameters{
				IapToken: nil,
				DexToken: nil,
			},
			parameters: &CommitDeploymentsParameters{
				CommitId: "123",
				OutFile:  "",
			},
			expectedRequest: &http.Request{
				Method: http.MethodGet,
				URL:    &url.URL{Path: "/api/commit-deployments/123"},
			},
		},
		{
			name: "valid url with IAP token",
			url:  "http://example.com",
			authParams: kutil.AuthenticationParameters{
				IapToken: &iapToken,
				DexToken: nil,
			},
			parameters: &CommitDeploymentsParameters{
				CommitId: "123",
				OutFile:  "",
			},
			expectedRequest: &http.Request{
				Method: http.MethodGet,
				URL:    &url.URL{Path: "/api/commit-deployments/123"},
				Header: http.Header{
					"Proxy-Authorization": []string{"Bearer " + iapToken},
				},
			},
		},
		{
			name: "valid url with DEX token",
			url:  "http://example.com",
			authParams: kutil.AuthenticationParameters{
				IapToken: nil,
				DexToken: &dexToken,
			},
			parameters: &CommitDeploymentsParameters{
				CommitId: "123",
				OutFile:  "",
			},
			expectedRequest: &http.Request{
				Method: http.MethodGet,
				URL:    &url.URL{Path: "/api/commit-deployments/123"},
				Header: http.Header{
					"Authorization": []string{"Bearer " + dexToken},
				},
			},
		},
		{
			name: "valid url with IAP and DEX token",
			url:  "http://example.com",
			authParams: kutil.AuthenticationParameters{
				IapToken: &iapToken,
				DexToken: &dexToken,
			},
			parameters: &CommitDeploymentsParameters{
				CommitId: "123",
				OutFile:  "",
			},
			expectedRequest: &http.Request{
				Method: http.MethodGet,
				URL:    &url.URL{Path: "/api/commit-deployments/123"},
				Header: http.Header{
					"Proxy-Authorization": []string{"Bearer " + iapToken},
					"Authorization":       []string{"Bearer " + dexToken},
				},
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual, err := createHttpRequest(tc.url, tc.authParams, tc.parameters)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if actual.Method != tc.expectedRequest.Method {
				t.Errorf("expected method %s, got %s", tc.expectedRequest.Method, actual.Method)
			}
			if actual.URL.Path != tc.expectedRequest.URL.Path {
				t.Errorf("expected path %s, got %s", tc.expectedRequest.URL.Path, actual.URL.Path)
			}
			if len(actual.Header) != len(tc.expectedRequest.Header) {
				t.Errorf("expected %d headers, got %d", len(tc.expectedRequest.Header), len(actual.Header))
			}
			for key, value := range tc.expectedRequest.Header {
				if actual.Header.Get(key) != value[0] {
					t.Errorf("expected header %s: %s, got %s", key, value[0], actual.Header.Get(key))
				}
			}
		})
	}
}
