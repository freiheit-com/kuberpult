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

package release

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/freiheit-com/kuberpult/cli/pkg/kuberpult_utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type mockHttpServer struct {
	response      int
	multipartForm *multipart.Form
}

func (s *mockHttpServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	err := req.ParseMultipartForm(MAXIMUM_MULTIPART_SIZE)
	if err != nil {
		panic(fmt.Errorf("error while parsing the multipart form in the mock HTTP server, error: %w", err))
	}
	s.multipartForm = req.MultipartForm

	w.WriteHeader(s.response)
}

const MAXIMUM_MULTIPART_SIZE = 12 * 1024 * 1024 // = 12Mi, taken from environments.go

func strPtr(s string) *string {
	return &s
}

func intPtr(n uint64) *uint64 {
	return &n
}

func TestRequestCreation(t *testing.T) {
	// simplified version of multipart.FileHeader
	type simpleMultipartFormFileHeader struct {
		filename string
		content  string
	}

	type testCase struct {
		name                       string
		params                     ReleaseParameters
		expectedMultipartFormValue map[string][]string
		expectedMultipartFormFile  map[string][]simpleMultipartFormFileHeader
		expectedErrorMsg           string
		responseCode               int
	}

	tcs := []testCase{
		{
			name: "no manifests",
			params: ReleaseParameters{
				Application: "potato",
			},
			expectedMultipartFormValue: map[string][]string{
				"application": {"potato"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{},
			responseCode:              http.StatusOK,
		},
		{
			name: "one environment manifest",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
				},
			},
			expectedMultipartFormValue: map[string][]string{
				"application": {"potato"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
			},

			responseCode: http.StatusOK,
		},
		{
			name: "one environment manifest with signature",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
				},
				Signatures: map[string][]byte{
					"development": []byte("some development signature"),
				},
			},
			expectedMultipartFormValue: map[string][]string{
				"application": {"potato"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"signatures[development]": {
					{
						filename: "development-signature",
						content: "some development signature",
					},
				},
			},
			responseCode: http.StatusOK,
		},
		{
			name: "multiple environment manifests",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
					"production": []byte("some production manifest"),
				},
			},
			expectedMultipartFormValue: map[string][]string{
				"application": {"potato"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content:  "some production manifest",
					},
				},
			},
			responseCode: http.StatusOK,
		},
		{
			name: "multiple environment manifests with signatures",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
					"production": []byte("some production manifest"),
				},
				Signatures: map[string][]byte{
					"development": []byte("some development signature"),
					"production": []byte("some production signature"),
				},
			},
			expectedMultipartFormValue: map[string][]string{
				"application": {"potato"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content:  "some production manifest",
					},
				},
				"signatures[development]": {
					{
						filename: "development-signature",
						content: "some development signature",
					},
				},
				"signatures[production]": {
					{
						filename: "production-signature",
						content: "some production signature",
					},
				},
				
			},
			responseCode: http.StatusOK,
		},
		{
			name: "multiple environment manifests with response code BadRequest",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
					"production": []byte("some production manifest"),
				},
			},
			expectedMultipartFormValue: map[string][]string{
				"application": {"potato"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content:  "some production manifest",
					},
				},
			},
			expectedErrorMsg: "error while issuing HTTP request, error: response was not OK or Accepted\nresponse code: 400\nresponse body:\n   ",
			responseCode:     http.StatusBadRequest,
		},
		{
			name: "multiple environment manifests with teams set",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
					"production": []byte("some production manifest"),
				},
				Team: strPtr("potato-team"),
			},
			expectedMultipartFormValue: map[string][]string{
				"application": {"potato"},
				"team":        {"potato-team"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content:  "some production manifest",
					},
				},
			},
			responseCode: http.StatusOK,
		},
		{
			name: "source commit ID is set",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
					"production": []byte("some production manifest"),
				},
				Team:           strPtr("potato-team"),
				SourceCommitId: strPtr("0123abcdef0123abcdef0123abcdef0123abcdef"),
			},
			expectedMultipartFormValue: map[string][]string{
				"application":      {"potato"},
				"team":             {"potato-team"},
				"source_commit_id": {"0123abcdef0123abcdef0123abcdef0123abcdef"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content:  "some production manifest",
					},
				},
			},
			responseCode: http.StatusOK,
		},
		{
			name: "previous commit ID is set",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
					"production": []byte("some production manifest"),
				},
				Team:             strPtr("potato-team"),
				SourceCommitId:   strPtr("0123abcdef0123abcdef0123abcdef0123abcdef"),
				PreviousCommitId: strPtr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			},
			expectedMultipartFormValue: map[string][]string{
				"application":        {"potato"},
				"team":               {"potato-team"},
				"source_commit_id":   {"0123abcdef0123abcdef0123abcdef0123abcdef"},
				"previous_commit_id": {"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content:  "some production manifest",
					},
				},
			},
			responseCode: http.StatusOK,
		},
		{
			name: "source_author is set",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
					"production": []byte("some production manifest"),
				},
				Team:             strPtr("potato-team"),
				SourceCommitId:   strPtr("0123abcdef0123abcdef0123abcdef0123abcdef"),
				PreviousCommitId: strPtr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				SourceAuthor:     strPtr("potato@tomato.com"),
			},
			expectedMultipartFormValue: map[string][]string{
				"application":        {"potato"},
				"team":               {"potato-team"},
				"source_commit_id":   {"0123abcdef0123abcdef0123abcdef0123abcdef"},
				"previous_commit_id": {"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				"source_author":      {"potato@tomato.com"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content:  "some production manifest",
					},
				},
			},
			responseCode: http.StatusOK,
		},
		{
			name: "source_message is set",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
					"production": []byte("some production manifest"),
				},
				Team:             strPtr("potato-team"),
				SourceCommitId:   strPtr("0123abcdef0123abcdef0123abcdef0123abcdef"),
				PreviousCommitId: strPtr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				SourceAuthor:     strPtr("potato@tomato.com"),
				SourceMessage:    strPtr("test source message"),
			},
			expectedMultipartFormValue: map[string][]string{
				"application":        {"potato"},
				"team":               {"potato-team"},
				"source_commit_id":   {"0123abcdef0123abcdef0123abcdef0123abcdef"},
				"previous_commit_id": {"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				"source_author":      {"potato@tomato.com"},
				"source_message":     {"test source message"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content:  "some production manifest",
					},
				},
			},
			responseCode: http.StatusOK,
		},
		{
			name: "source_message is set with newlines",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
					"production": []byte("some production manifest"),
				},
				Team:             strPtr("potato-team"),
				SourceCommitId:   strPtr("0123abcdef0123abcdef0123abcdef0123abcdef"),
				PreviousCommitId: strPtr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				SourceAuthor:     strPtr("potato@tomato.com"),
				SourceMessage:    strPtr("test\nsource\nmessage"),
			},
			expectedMultipartFormValue: map[string][]string{
				"application":        {"potato"},
				"team":               {"potato-team"},
				"source_commit_id":   {"0123abcdef0123abcdef0123abcdef0123abcdef"},
				"previous_commit_id": {"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				"source_author":      {"potato@tomato.com"},
				"source_message":     {"test\nsource\nmessage"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content:  "some production manifest",
					},
				},
			},
			responseCode: http.StatusOK,
		},
		{
			name: "version is set",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
					"production": []byte("some production manifest"),
				},
				Team:             strPtr("potato-team"),
				SourceCommitId:   strPtr("0123abcdef0123abcdef0123abcdef0123abcdef"),
				PreviousCommitId: strPtr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				SourceAuthor:     strPtr("potato@tomato.com"),
				SourceMessage:    strPtr("test\nsource\nmessage"),
				Version:          intPtr(123123),
			},
			expectedMultipartFormValue: map[string][]string{
				"application":        {"potato"},
				"team":               {"potato-team"},
				"source_commit_id":   {"0123abcdef0123abcdef0123abcdef0123abcdef"},
				"previous_commit_id": {"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				"source_author":      {"potato@tomato.com"},
				"source_message":     {"test\nsource\nmessage"},
				"version":            {"123123"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content:  "some production manifest",
					},
				},
			},
			responseCode: http.StatusOK,
		},
		{
			name: "display_version is set",
			params: ReleaseParameters{
				Application: "potato",
				Manifests: map[string][]byte{
					"development": []byte("some development manifest"),
					"production": []byte("some production manifest"),
				},
				Team:             strPtr("potato-team"),
				SourceCommitId:   strPtr("0123abcdef0123abcdef0123abcdef0123abcdef"),
				PreviousCommitId: strPtr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				SourceAuthor:     strPtr("potato@tomato.com"),
				SourceMessage:    strPtr("test\nsource\nmessage"),
				Version:          intPtr(123123),
				DisplayVersion:   strPtr("1.23.4"),
			},
			expectedMultipartFormValue: map[string][]string{
				"application":        {"potato"},
				"team":               {"potato-team"},
				"source_commit_id":   {"0123abcdef0123abcdef0123abcdef0123abcdef"},
				"previous_commit_id": {"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				"source_author":      {"potato@tomato.com"},
				"source_message":     {"test\nsource\nmessage"},
				"version":            {"123123"},
				"display_version":    {"1.23.4"},
			},
			expectedMultipartFormFile: map[string][]simpleMultipartFormFileHeader{
				"manifests[development]": {
					{
						filename: "development-manifest",
						content:  "some development manifest",
					},
				},
				"manifests[production]": {
					{
						filename: "production-manifest",
						content:  "some production manifest",
					},
				},
			},
			responseCode: http.StatusOK,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockServer := &mockHttpServer{
				response: tc.responseCode,
			}
			server := httptest.NewServer(mockServer)
			
			authParams := kuberpult_utils.AuthenticationParameters {}
			err := Release(server.URL, authParams, tc.params)
			// check errors
			if diff := cmp.Diff(errMatcher{tc.expectedErrorMsg}, err, cmpopts.EquateErrors()); !(err == nil && tc.expectedErrorMsg == "") && diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			// check multipart form values
			if diff := cmp.Diff(mockServer.multipartForm.Value, tc.expectedMultipartFormValue); diff != "" {
				t.Fatalf("request multipart forms are different\nexpected:\n  %v\nreceived:\n  %v\ndiff:\n  %s", tc.expectedMultipartFormValue, mockServer.multipartForm, diff)
			}

			// check multipart form files
			fileHeaders := make(map[string][]simpleMultipartFormFileHeader)
			// Note: we do not need to sort the map before iterating over it, because we just use `cmp.Equal` which handles the undefined order
			for key, val := range mockServer.multipartForm.File {
				simpleHeaders := make([]simpleMultipartFormFileHeader, 0)
				for _, header := range val {
					file, err := header.Open()
					if err != nil {
						t.Fatalf("error encountered while opening the multipart file header for key \"%s\" file \"%s\", error: %v", key, header.Filename, err)
					}
					defer file.Close()

					bytes := make([]byte, MAXIMUM_MULTIPART_SIZE)
					n, err := file.Read(bytes)
					if err != nil {
						t.Fatalf("error encountered while reading the multipart file header for key \"%s\" file \"%s\", error: %v", key, header.Filename, err)
					}
					bytes = bytes[:n]
					content := string(bytes)
					simpleHeader := simpleMultipartFormFileHeader{
						filename: header.Filename,
						content:  content,
					}

					simpleHeaders = append(simpleHeaders, simpleHeader)
				}
				fileHeaders[key] = simpleHeaders
			}
			if diff := cmp.Diff(fileHeaders, tc.expectedMultipartFormFile, cmp.AllowUnexported(simpleMultipartFormFileHeader{})); diff != "" {
				t.Fatalf("request multipart forms are different\nexpected:\n  %v\nreceived:\n  %v\ndiff:\n  %s\n", tc.expectedMultipartFormFile, fileHeaders, diff)
			}
		})
	}
}
