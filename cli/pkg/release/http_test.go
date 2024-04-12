/*
This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com
*/

package release

import (
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type mockHttpServer struct {
	response      int
	multipartForm *multipart.Form
}

func (s *mockHttpServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	err := req.ParseMultipartForm(MAXIMUM_MULTIPART_SIZE)
	if err != nil {
		panic(err)
	}
	s.multipartForm = req.MultipartForm

	w.WriteHeader(s.response)
}

const MAXIMUM_MULTIPART_SIZE = 12 * 1024 * 1024 // = 12Mi, taken from environments.go

func TestRequestCreation(t *testing.T) {
	// simplified version of multipart.FileHeader
	type simpleMultipartFormFileHeader struct {
		filename string
		content  string
	}

	type testCase struct {
		name                       string
		params                     *ReleaseParameters
		expectedMultipartFormValue map[string][]string
		expectedMultupartFormFile  map[string][]simpleMultipartFormFileHeader
		expectedErrorMsg           string
		responseCode               int
	}

	tcs := []testCase{
		{
			name: "no manifests",
			params: &ReleaseParameters{
				Application: "potato",
			},
			expectedMultipartFormValue: map[string][]string{
				"application": {"potato"},
			},
			expectedMultupartFormFile: map[string][]simpleMultipartFormFileHeader{
				
			},
			responseCode: http.StatusOK,
		},
		{
			name: "one environment manifest",
			params: &ReleaseParameters{
				Application: "potato",
				Manifests: map[string]string{
					"development": "some development manifest",
				},
			},
			expectedMultipartFormValue: map[string][]string{
				"application":            {"potato"},
			},
			expectedMultupartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content: "some development manifest",
					},
				},
			},

			responseCode: http.StatusOK,
		},
		{
			name: "multiple environment manifests",
			params: &ReleaseParameters{
				Application: "potato",
				Manifests: map[string]string{
					"development": "some development manifest",
					"production":  "some production manifest",
				},
			},
			expectedMultipartFormValue: map[string][]string{
				"application": {"potato"},
			},
			expectedMultupartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content: "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content: "some production manifest",
					},
				},
			},
			responseCode: http.StatusOK,
		},
		{
			name: "multiple environment manifests with response code BadRequest",
			params: &ReleaseParameters{
				Application: "potato",
				Manifests: map[string]string{
					"development": "some development manifest",
					"production":  "some production manifest",
				},
			},
			expectedMultipartFormValue: map[string][]string{
				"application": {"potato"},
			},
			expectedMultupartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content: "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content: "some production manifest",
					},
				},
			},
			expectedErrorMsg: "error while issuing HTTP request, error: response was not OK or Accepted, response code: 400",
			responseCode:     http.StatusBadRequest,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mockServer := &mockHttpServer{
				response: tc.responseCode,
			}
			server := httptest.NewServer(mockServer)

			// check errors
			err := Release(server.URL, tc.params)
			if err != nil && err.Error() != tc.expectedErrorMsg {
				t.Fatalf("error messages mismatched, expected %s, received %s", tc.expectedErrorMsg, err.Error())
			}
			if err == nil && tc.expectedErrorMsg != "" {
				t.Fatalf("expected error %v, but no error was raised", tc.expectedErrorMsg)
			}

			// check multipart form values
			if !cmp.Equal(mockServer.multipartForm.Value, tc.expectedMultipartFormValue) {
				t.Fatalf("request multipart forms are different, expected %v, received %v", tc.expectedMultipartFormValue, mockServer.multipartForm)
			}

			// check multipart form files
			fileHeaders := make(map[string][]simpleMultipartFormFileHeader)
			for key, val := range mockServer.multipartForm.File {
				simpleHeaders := make([]simpleMultipartFormFileHeader, 0)
				for _, header := range val {
					file, err := header.Open()
					if err != nil {
						t.Fatalf("error encountered while opening the multipart file header for key \"%s\" file \"%s\", error: %v", key, header.Filename, err)
					}
					bytes := make([]byte, MAXIMUM_MULTIPART_SIZE)
					n, err := file.Read(bytes)
					if err != nil {
						t.Fatalf("error encountered while reading the multipart file header for key \"%s\" file \"%s\", error: %v", key, header.Filename, err)
					}
					bytes = bytes[:n]
					content := string(bytes)
					simpleHeader := simpleMultipartFormFileHeader{
						filename: header.Filename,
						content: content,
					}

					simpleHeaders = append(simpleHeaders, simpleHeader)
				}
				fileHeaders[key] = simpleHeaders
			}
			if !cmp.Equal(fileHeaders, tc.expectedMultupartFormFile, cmp.AllowUnexported(simpleMultipartFormFileHeader{})) {
				t.Fatalf("request multipart forms are different, expected %v, received %v", tc.expectedMultupartFormFile, fileHeaders)
			}
		})
	}
}
