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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestServerHeader(t *testing.T) {
	tcs := []struct {
		Name        string
		RequestPath string

		ExpectedHeaders http.Header
	}{
		{
			Name:        "simple case",
			RequestPath: "/",

			ExpectedHeaders: http.Header{
				"Content-Type":              {"text/plain; charset=utf-8"},
				"Strict-Transport-Security": {"max-age=31536000; includeSubDomains;"},
				"X-Content-Type-Options":    {"nosniff"},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			go func(t *testing.T) {
				defer cancel()
				for {
					res, err := http.Get("http://localhost:8081/health")
					if err != nil {
						<-time.After(1 * time.Second)
						continue
					}
					if res.StatusCode != 200 {
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
				res, err := http.Get(path)
				if err != nil {
					t.Fatalf("expected no error but got %q", err)
				}
				hdrs := res.Header.Clone()
				hdrs.Del("Content-Length")
				hdrs.Del("Date")
				if !cmp.Equal(tc.ExpectedHeaders, hdrs) {
					t.Errorf("expected no diff for headers but got %s", cmp.Diff(tc.ExpectedHeaders, hdrs))
				}

			}(t)
			err := runServer(ctx)
			if err != nil {
				t.Fatalf("expected no error, but got %q", err)
			}
		})
	}
}
