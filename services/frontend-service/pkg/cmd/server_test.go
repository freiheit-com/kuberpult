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
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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
				"Content-Type": {"text/plain; charset=utf-8"}, "Content-Security-Policy": {
					"default-src 'self'; style-src-elem 'self' fonts.googleapis.com; font-src fonts.gstatic.com; connect-src 'self' login.microsoft.com; child-src 'none'",
				},
				"Strict-Transport-Security": {"max-age=31536000; includeSubDomains;"},
				"X-Content-Type-Options":    {"nosniff"},
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
				"Access-Control-Allow-Credentials": {"true"},
				"Access-Control-Allow-Origin":      {"https://kuberpult.fdc"},
				"Allow":                            {"OPTIONS, GET, HEAD"},
				"Content-Security-Policy":          {"default-src 'self'; style-src-elem 'self' fonts.googleapis.com; font-src fonts.gstatic.com; connect-src 'self' login.microsoft.com; child-src 'none'"},
				"Strict-Transport-Security":        {"max-age=31536000; includeSubDomains;"},
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
					res, err := http.Get("http://localhost:8081/health")
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
				//
				path, err := url.JoinPath("http://localhost:8081/", tc.RequestPath)
				if err != nil {
					panic(err)
				}
				req, err := http.NewRequest(tc.RequestMethod, path, nil)
				if err != nil {
					t.Fatalf("expected no error but got %q", err)
				}
				req.Header = tc.RequestHeaders
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("expected no error but got %q", err)
				}
				t.Logf("%v %q", res.StatusCode, err)
				// Delete two headers that are hard to test.
				hdrs := res.Header.Clone()
				hdrs.Del("Content-Length")
				hdrs.Del("Date")
				if !cmp.Equal(tc.ExpectedHeaders, hdrs) {
					t.Errorf("expected no diff for headers but got %s", cmp.Diff(tc.ExpectedHeaders, hdrs))
				}

			}(t)
			for k, v := range tc.Environment {
				t.Setenv(k, v)
			}
			err := runServer(ctx)
			if err != nil {
				t.Fatalf("expected no error, but got %q", err)
			}
			wg.Wait()
		})
	}
}
