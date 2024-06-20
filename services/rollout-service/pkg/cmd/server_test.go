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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// Used to compare two error message strings, needed because errors.Is(fmt.Errorf(text),fmt.Errorf(text)) == false
type errMatcher struct {
	msg string
}

func (e errMatcher) Error() string {
	return e.msg
}

func (e errMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}

func TestService(t *testing.T) {
	tcs := []struct {
		Name          string
		ExpectedError error
		Config        Config
	}{
		{
			Name:          "simple case",
			ExpectedError: errMatcher{"invalid argocd server url: parse \"\": empty url"},
		},
		{
			Name:          "invalid argocd url",
			ExpectedError: errMatcher{"invalid argocd server url: parse \"not a http address\": invalid URI for request"},
			Config: Config{
				ArgocdServer: "not a http address",
			},
		},
		{
			Name:          "valid http argocd url",
			ExpectedError: errMatcher{"connecting to argocd version: dial tcp 127.0.0.1:32761: connect: connection refused"},
			Config: Config{
				ArgocdServer: "http://127.0.0.1:32761",
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			err := runServer(ctx, tc.Config)
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestClientConfig(t *testing.T) {
	tcs := []struct {
		Name   string
		Config Config

		ExpectedError      error
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
			ExpectedError: errMatcher{"invalid argocd server url: parse \"not a url\": invalid URI for request"},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			clientConfig, err := tc.Config.ClientConfig()
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
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
