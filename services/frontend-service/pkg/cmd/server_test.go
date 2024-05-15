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
					t.Fatalf("error joining url: %s", err)
				}
				body, err := proto.Marshal(tc.Body)
				req, err := http.NewRequest("POST", path, bytes.NewReader(body))
				if err != nil {
					t.Fatalf("expected no error but got %q", err)
				}
				req.Header.Add("Content-Type", "application/grpc-web")
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("expected no error but got %q", err)
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
