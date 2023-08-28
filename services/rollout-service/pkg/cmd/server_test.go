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

package cmd

import (
	"context"
	"testing"
)

func TestService(t *testing.T) {
	tcs := []struct {
		Name          string
		ExpectedError string
		Config        Config
	}{
		{
			Name:          "simple case",
			ExpectedError: "invalid argocd server url: parse \"\": empty url",
		},
		{
			Name:          "invalid argocd url",
			ExpectedError: "invalid argocd server url: parse \"not a http address\": invalid URI for request",
			Config: Config{
				ArgocdServer: "not a http address",
			},
		},
		{
			Name:          "valid http argocd url",
			ExpectedError: "connecting to argocd version: dial tcp 127.0.0.1:32761: connect: connection refused",
			Config: Config{
				ArgocdServer: "http://localhost:32761",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			err := runServer(ctx, tc.Config)
			if err != nil {
				if err.Error() != tc.ExpectedError {
					t.Errorf("expected error %q but got %q", tc.ExpectedError, err)
				}
			} else if tc.ExpectedError != "" {
				t.Errorf("expected error %q but got <nil>", tc.ExpectedError)
			}
		})
	}
}

func TestClientConfig(t *testing.T) {
	tcs := []struct {
		Name   string
		Config Config

		ExpectedError      string
		ExpectedServerAddr string
		ExpectedPlainText  bool
	}{
		{
			Name: "simple plaintext",
			Config: Config{
				ArgocdServer: "http://foo:80",
			},
			ExpectedServerAddr: "foo:80",
			ExpectedPlainText:  true,
		},
		{
			Name: "simple tls",
			Config: Config{
				ArgocdServer: "tls://foo:80",
			},
			ExpectedServerAddr: "foo:80",
			ExpectedPlainText:  false,
		},
		{
			Name: "simple tls",
			Config: Config{
				ArgocdServer: "not a url",
			},
			ExpectedError: "invalid argocd server url: parse \"not a url\": invalid URI for request",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			clientConfig, err := tc.Config.ClientConfig()
			if err != nil {
				if err.Error() != tc.ExpectedError {
					t.Errorf("expected error %q but got %q", tc.ExpectedError, err)
				}
			} else if tc.ExpectedError != "" {
				t.Errorf("expected error %q but got <nil>", tc.ExpectedError)
			}
			if clientConfig.ServerAddr != tc.ExpectedServerAddr {
				t.Errorf("mismatched ServerAddr, expected %q, got %q", tc.ExpectedServerAddr, clientConfig.ServerAddr)
			}
			if clientConfig.PlainText != tc.ExpectedPlainText {
				t.Errorf("mismatched PlainText, expected %t, got %t", tc.ExpectedPlainText, clientConfig.PlainText)
			}
		})
	}
}
