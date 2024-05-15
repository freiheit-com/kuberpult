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

package setup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/metrics"
	"github.com/google/go-cmp/cmp"
)

func TestBasicAuthHandler(t *testing.T) {
	tcs := []struct {
		desc            string
		basicAuthServer *BasicAuth
		requestUser     string
		requestPassword string

		expectedResponseCode     int
		expectedChainHandlerCall bool
	}{
		{
			desc:                     "returns 401 on wrong auth, wrong username",
			basicAuthServer:          &BasicAuth{"test", "666"},
			requestUser:              "foo",
			requestPassword:          "666",
			expectedResponseCode:     401,
			expectedChainHandlerCall: false,
		},
		{
			desc:                     "returns 401 on wrong auth, wrong password",
			basicAuthServer:          &BasicAuth{"test", "666"},
			requestUser:              "test",
			requestPassword:          "888",
			expectedResponseCode:     401,
			expectedChainHandlerCall: false,
		},
		{
			desc:                     "passes request true, if auth ok",
			basicAuthServer:          &BasicAuth{"test", "666"},
			requestUser:              "test",
			requestPassword:          "666",
			expectedResponseCode:     200,
			expectedChainHandlerCall: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			testChainHandler := &testChainHandler{}

			testRequest := httptest.NewRequest("GET", "http://example.com/", nil)
			testRequest.SetBasicAuth(tc.requestUser, tc.requestPassword)

			testResponse := httptest.NewRecorder()

			handler := NewBasicAuthHandler(tc.basicAuthServer, testChainHandler)
			handler.ServeHTTP(testResponse, testRequest)

			if tc.expectedChainHandlerCall != testChainHandler.called {
				t.Errorf("expectedChainHandlerCall %t, got %t", tc.expectedChainHandlerCall, testChainHandler.called)
			}
			if tc.expectedResponseCode != testResponse.Code {
				t.Errorf("expectedResponseCode %d, got %d", tc.expectedResponseCode, testResponse.Code)
			}
		})
	}
}

func TestGracefulShutdown(t *testing.T) {
	tcs := []struct {
		desc   string
		port   string
		termFn func()
		cancel bool
	}{
		{
			desc: "Cleans up on shutdown triggered by the OS",
			port: "8383",
			termFn: func() {
				osSignalChannel <- syscall.SIGTERM
			},
		},
		{
			desc:   "Cleans up on cancelled context",
			port:   "8282",
			cancel: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {

			fakeServer := make(chan interface{}, 1)
			backServeHTTP := serveHTTP
			defer func() {
				serveHTTP = backServeHTTP
			}()
			serveHTTP = func(ctx context.Context, httpS *http.Server, port string, cancel context.CancelFunc) {
				for range fakeServer {
				}
			}

			backShutdownHTTP := shutdownHTTP
			defer func() {
				shutdownHTTP = backShutdownHTTP
			}()
			shutdownHTTP = func(ctx context.Context, httpS *http.Server) error {
				close(fakeServer)
				return nil
			}

			backOsSignalChannel := osSignalChannel
			osSignalChannel = make(chan os.Signal, 1)
			defer func() {
				osSignalChannel = backOsSignalChannel
			}()

			cleanShutdownCh := make(chan bool, 1)

			cfg := ServerConfig{
				HTTP: []HTTPConfig{
					{
						Port:      tc.port,
						Register:  func(*http.ServeMux) {},
						BasicAuth: nil,
						Shutdown: func(ctx context.Context) error {
							<-time.After(200 * time.Millisecond) // Releasing resources (time consuming task)
							cleanShutdownCh <- true
							return nil
						},
					},
				},
			}

			mainExited := make(chan bool, 1)
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				Run(ctx, cfg)
				mainExited <- true
			}()
			if tc.cancel {
				cancel()
			}
			if tc.termFn != nil {
				tc.termFn()
			}
			select {
			case <-mainExited:
				t.Errorf("Main goroutine finished before resource cleanup")
			case <-time.After(10 * time.Second):
				t.Errorf("Program didn't finish on shutdown signal")
			case <-cleanShutdownCh: // That's what we expect
			}
		})
	}

}

func TestMetrics(t *testing.T) {
	tcs := []struct {
		desc string
		port string
	}{
		{
			desc: "registers metrics server automatically",
			port: "8384",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			metricAdded := make(chan struct{})
			cfg := ServerConfig{
				HTTP: []HTTPConfig{
					{
						Port:     tc.port,
						Register: func(*http.ServeMux) {},
					},
				},
				Background: []BackgroundTaskConfig{
					{
						Name: "something",
						Run: func(ctx context.Context, hr *HealthReporter) error {
							pv := metrics.FromContext(ctx)
							counter, _ := pv.Meter("something").Int64Counter("something")
							counter.Add(ctx, 1)
							metricAdded <- struct{}{}
							<-ctx.Done()
							return nil
						},
					},
				},
			}

			mainExited := make(chan bool, 1)
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				Run(ctx, cfg)
				mainExited <- true
			}()
			<-metricAdded
			var response *http.Response
			for i := 0; i < 10; i = i + 1 {
				res, err := http.Get(fmt.Sprintf("http://localhost:%s/metrics", tc.port))
				if err != nil {
					if i == 9 {
						t.Errorf("error getting metrics: %s", err)
					}
					continue
				}
				response = res
				time.After(time.Second)
			}
			body, _ := io.ReadAll(response.Body)
			expectedBody := `# HELP background_job_ready 
# TYPE background_job_ready gauge
background_job_ready{name="something"} 0
# HELP something_total 
# TYPE something_total counter
something_total 1
`
			if string(body) != expectedBody {
				t.Errorf("got wrong metric body, diff %s", cmp.Diff(string(body), expectedBody))
			}
			cancel()
			<-mainExited
		})
	}
}

//helper

type testChainHandler struct {
	called bool
}

func (h *testChainHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.called = true
}
