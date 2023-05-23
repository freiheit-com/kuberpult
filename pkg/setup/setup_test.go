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

package setup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"
	"time"
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

//helper

type testChainHandler struct {
	called bool
}

func (h *testChainHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.called = true
}
