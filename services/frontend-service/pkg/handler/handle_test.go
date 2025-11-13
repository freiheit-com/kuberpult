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

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/assert"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
)

type mockReleaseTrainPrognosisServiceClient struct {
	request  *api.ReleaseTrainRequest
	response *api.GetReleaseTrainPrognosisResponse
}

func (c *mockReleaseTrainPrognosisServiceClient) GetReleaseTrainPrognosis(_ context.Context, in *api.ReleaseTrainRequest, _ ...grpc.CallOption) (*api.GetReleaseTrainPrognosisResponse, error) {
	c.request = in
	return c.response, nil
}

type mockVersionClient struct {
	request  *api.GetManifestsRequest
	response *api.GetManifestsResponse
}

func (c *mockVersionClient) GetManifests(_ context.Context, in *api.GetManifestsRequest, _ ...grpc.CallOption) (*api.GetManifestsResponse, error) {
	c.request = in
	return c.response, nil
}

// Not used, needs to be implemented
func (c *mockVersionClient) GetVersion(_ context.Context, in *api.GetVersionRequest, _ ...grpc.CallOption) (*api.GetVersionResponse, error) {
	return nil, nil
}

// The createTestForm function from the artifact
func createTestForm() (*multipart.Form, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	err := writer.WriteField("version", "1")
	if err != nil {
		return nil, err
	}
	fileWriter, err := writer.CreateFormFile("manifests[development]", "test.txt")
	if err != nil {
		return nil, err
	}
	_, err = io.WriteString(fileWriter, "This is the file content.")
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	err = req.ParseMultipartForm(32 << 20)
	if err != nil {
		return nil, err
	}
	return req.MultipartForm, nil
}

type HandleRequest struct {
	name                                 string
	req                                  *http.Request
	KeyRing                              openpgp.KeyRing
	signature                            string
	AzureAuthEnabled                     bool
	batchResponse                        *api.BatchResponse
	releaseTrainPrognosisResponse        *api.GetReleaseTrainPrognosisResponse
	versionClientResponse                *api.GetManifestsResponse
	expectedResp                         *http.Response
	expectedBody                         string
	expectedBatchRequest                 *api.BatchRequest
	expectedReleaseTrainPrognosisRequest *api.ReleaseTrainRequest
	expectedGetManifestRequest           *api.GetManifestsRequest
}

func generateCreateReleaseTestCasesForLengthChecks(tcName string, param string, form *multipart.Form) HandleRequest {
	requestParams := map[string][]string{
		"application": {"my-app"},
		"version":     {"1"},
		"revision":    {"1"},
	}

	requestParams[param] = []string{"value1", "value2"}

	return HandleRequest{
		name: tcName,
		req: &http.Request{
			Method: http.MethodPut,
			URL: &url.URL{
				Path: "/api/release",
			},
			MultipartForm: &multipart.Form{
				Value: requestParams,
				File:  form.File,
			},
		},
		batchResponse: &api.BatchResponse{
			Results: []*api.BatchResult{
				{
					Result: &api.BatchResult_CreateReleaseResponse{
						CreateReleaseResponse: &api.CreateReleaseResponse{
							Response: &api.CreateReleaseResponse_Success{
								Success: &api.CreateReleaseResponseSuccess{},
							},
						},
					},
				},
			},
		},
		expectedResp: &http.Response{
			StatusCode: http.StatusBadRequest,
		},
		expectedBody: fmt.Sprintf("Exact one '%s' parameter should be provided, 2 are given", param),
	}
}

func TestServer_Handle(t *testing.T) {
	exampleKey, err := openpgp.NewEntity("Test", "", "test@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	exampleKeyRing := openpgp.EntityList{exampleKey}
	exampleEnvironment := "development"
	exampleLockId := "test"
	lifeTime2h := "2h"
	lifeTimeEmpty := ""
	lifeTime3d := "3d"
	exampleConfig := `{
   "upstream":{
      "environment": "development"
   }
}`
	// workaround for *bool
	latestFlag := true
	starFlag := "*"

	signatureBuffer := bytes.Buffer{}
	err = openpgp.ArmoredDetachSign(&signatureBuffer, exampleKey, bytes.NewReader([]byte(exampleEnvironment)), nil)
	if err != nil {
		t.Fatal(err)
	}
	exampleSignature := signatureBuffer.String()
	signatureBuffer = bytes.Buffer{}
	err = openpgp.ArmoredDetachSign(&signatureBuffer, exampleKey, bytes.NewReader([]byte(exampleEnvironment+exampleLockId)), nil)
	if err != nil {
		t.Fatal(err)
	}
	exampleLockSignature := signatureBuffer.String()
	signatureBuffer = bytes.Buffer{}
	err = openpgp.ArmoredDetachSign(&signatureBuffer, exampleKey, bytes.NewReader([]byte(exampleConfig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	exampleConfigSignature := signatureBuffer.String()
	lockRequestJSON, _ := json.Marshal(putLockRequest{
		Message:           "test message",
		Signature:         exampleLockSignature,
		CiLink:            "www.test.com",
		SuggestedLifeTime: "3d",
	})
	releaseTrainRequestJSON, _ := json.Marshal(releaseTrainRequest{
		Signature: exampleSignature,
		CiLink:    "https://google.com",
	})

	// 1. Create a valid, parsable form using the function from the artifact
	form, err := createTestForm()
	if err != nil {
		t.Fatalf("Failed to create test form: %v", err)
	}

	tests := []HandleRequest{
		{
			name: "wrongly routed",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "unknown endpoint ''\n",
		},
		{
			name: "env but missing env",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "missing environment ID\n",
		},
		{
			name: "release train env group api",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/environment-groups/development/releasetrain",
				},
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainResponse{
								Target: "development",
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "{\"target\":\"development\"}",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainRequest{Target: "development", TargetType: api.ReleaseTrainRequest_ENVIRONMENTGROUP},
						},
					},
				},
			},
		},
		{
			name: "release train api",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/environments/development/releasetrain",
				},
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainResponse{
								Target: "development",
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "{\"target\":\"development\"}",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainRequest{Target: "development", TargetType: api.ReleaseTrainRequest_ENVIRONMENT},
						},
					},
				},
			},
		},
		{
			name: "release train api with CI Link",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/releasetrain",
				},
				Body: io.NopCloser(strings.NewReader(`{"cilink":"https://google.com"}`)),
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainResponse{
								Target: "development",
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "{\"target\":\"development\"}",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainRequest{Target: "development", CiLink: "https://google.com"},
						},
					},
				},
			},
		},
		{
			name:             "release train api with CI Link - Azure enabled",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/environments/development/releasetrain",
				},
				Body: io.NopCloser(bytes.NewReader(releaseTrainRequestJSON)),
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainResponse{
								Target: "development",
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "{\"target\":\"development\"}",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainRequest{Target: "development", TargetType: api.ReleaseTrainRequest_ENVIRONMENT, CiLink: "https://google.com"},
						},
					},
				},
			},
		},
		{
			name:             "release train api with CI Link - Azure enabled - missing signature",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/releasetrain",
				},
				Body: io.NopCloser(strings.NewReader(`{"cilink":"https://google.com"}`)),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "Missing signature in request body",
		},
		{
			name:             "release train api with CI Link - Azure enabled - invalid signature",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/releasetrain",
				},
				Body: io.NopCloser(strings.NewReader(`{"cilink":"https://google.com", "signature": "uncool"}`)),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusInternalServerError,
			},
			expectedBody: "Internal: Invalid Signature: EOF",
		},
		{
			name: "release train",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/releasetrain",
				},
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainResponse{
								Target: "development",
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "{\"target\":\"development\"}",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainRequest{Target: "development"},
						},
					},
				},
			},
		},
		{
			name: "create release",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/release",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"version":     {"1"},
						"application": {"my-app"},
						"revision":    {"1"},
					},
					File: form.File,
				},
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_CreateReleaseResponse{
							CreateReleaseResponse: &api.CreateReleaseResponse{
								Response: &api.CreateReleaseResponse_Success{
									Success: &api.CreateReleaseResponseSuccess{},
								},
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusCreated,
			},
			expectedBody: "{\"Success\":{}}\n",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateRelease{
							CreateRelease: &api.CreateReleaseRequest{Application: "my-app", Version: 1, Revision: 1, Manifests: map[string]string{
								"development": "This is the file content.",
							}},
						},
					},
				},
			},
		},
		{
			name: "create release api, no app name given",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/release",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"version":  {"1"},
						"revision": {"1"},
					},
					File: form.File,
				},
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_CreateReleaseResponse{
							CreateReleaseResponse: &api.CreateReleaseResponse{
								Response: &api.CreateReleaseResponse_Success{
									Success: &api.CreateReleaseResponseSuccess{},
								},
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "Exact one 'application' parameter should be provided, 0 are given",
		},
		generateCreateReleaseTestCasesForLengthChecks("create release api, too many app names", "application", form),
		generateCreateReleaseTestCasesForLengthChecks("create release api, too many team names", "team", form),
		generateCreateReleaseTestCasesForLengthChecks("create release api, too many source_commit_id", "source_commit_id", form),
		generateCreateReleaseTestCasesForLengthChecks("create release api, too many previous_commit_id", "previous_commit_id", form),
		generateCreateReleaseTestCasesForLengthChecks("create release api, too many source_author", "source_author", form),
		generateCreateReleaseTestCasesForLengthChecks("create release api, too many source_message", "source_message", form),
		generateCreateReleaseTestCasesForLengthChecks("create release api, too many versions", "version", form),
		generateCreateReleaseTestCasesForLengthChecks("create release api, too many display_version", "display_version", form),
		generateCreateReleaseTestCasesForLengthChecks("create release api, too many ci_link", "ci_link", form),
		generateCreateReleaseTestCasesForLengthChecks("create release api, too many is_prepublish", "is_prepublish", form),
		generateCreateReleaseTestCasesForLengthChecks("create release api, too many revision", "revision", form),
		{
			name: "create release, revision is not a number",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/release",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"version":     {"1"},
						"application": {"my-app"},
						"revision":    {"abcd"},
					},
					File: form.File,
				},
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_CreateReleaseResponse{
							CreateReleaseResponse: &api.CreateReleaseResponse{
								Response: &api.CreateReleaseResponse_Success{
									Success: &api.CreateReleaseResponseSuccess{},
								},
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "Invalid version: strconv.ParseUint: parsing \"abcd\": invalid syntax",
		},
		{
			name: "create release api, version is not a number",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/release",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"application": {"my-app"},
						"version":     {"abcd"},
						"revision":    {"1"},
					},
					File: form.File,
				},
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_CreateReleaseResponse{
							CreateReleaseResponse: &api.CreateReleaseResponse{
								Response: &api.CreateReleaseResponse_Success{
									Success: &api.CreateReleaseResponseSuccess{},
								},
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "Provided version 'abcd' is not valid: strconv.ParseUint: parsing \"abcd\": invalid syntax",
		},
		{
			name: "create release api, revision is not a number",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/release",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"application": {"my-app"},
						"version":     {"1"},
						"revision":    {"abcd"},
					},
					File: form.File,
				},
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_CreateReleaseResponse{
							CreateReleaseResponse: &api.CreateReleaseResponse{
								Response: &api.CreateReleaseResponse_Success{
									Success: &api.CreateReleaseResponseSuccess{},
								},
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "Provided revision 'abcd' is not valid: strconv.ParseUint: parsing \"abcd\": invalid syntax",
		},
		{
			name: "create release api, is_prepublish is not a boolean value",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/release",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"application":   {"my-app"},
						"version":       {"1"},
						"revision":      {"1"},
						"is_prepublish": {"abcd"},
					},
					File: form.File,
				},
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_CreateReleaseResponse{
							CreateReleaseResponse: &api.CreateReleaseResponse{
								Response: &api.CreateReleaseResponse_Success{
									Success: &api.CreateReleaseResponseSuccess{},
								},
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "Provided is_prepublish 'abcd' is not valid: strconv.ParseBool: parsing \"abcd\": invalid syntax",
		},
		{
			name: "Get manifests - full version",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/api/application/app/release/manifests/1.0",
				},
			},
			versionClientResponse: &api.GetManifestsResponse{
				Release: nil,
				Manifests: map[string]*api.Manifest{
					"development": {
						Environment: exampleEnvironment,
						Content:     "development manifest content",
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "{\"manifests\":{\"development\":{\"environment\":\"development\",\"content\":\"development manifest content\"}}}",
			expectedGetManifestRequest: &api.GetManifestsRequest{
				Application: "app",
				Release:     "1",
				Revision:    "0",
			},
		},
		{
			name: "Get manifests - only release number",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/api/application/app/release/manifests/1",
				},
			},
			versionClientResponse: &api.GetManifestsResponse{
				Release: nil,
				Manifests: map[string]*api.Manifest{
					"development": {
						Environment: exampleEnvironment,
						Content:     "development manifest content",
					},
				},
			},
			expectedGetManifestRequest: &api.GetManifestsRequest{
				Application: "app",
				Release:     "1",
				Revision:    "0",
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "{\"manifests\":{\"development\":{\"environment\":\"development\",\"content\":\"development manifest content\"}}}",
		},
		{
			name: "Get manifests - latest",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/api/application/app/release/manifests/latest",
				},
			},
			versionClientResponse: &api.GetManifestsResponse{
				Release: nil,
				Manifests: map[string]*api.Manifest{
					"development": {
						Environment: exampleEnvironment,
						Content:     "development manifest content",
					},
				},
			},
			expectedGetManifestRequest: &api.GetManifestsRequest{
				Application: "app",
				Release:     "latest",
				Revision:    "0",
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "{\"manifests\":{\"development\":{\"environment\":\"development\",\"content\":\"development manifest content\"}}}",
		},
		{
			name: "release train prognosis",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/api/environments/development/releasetrain/prognosis",
				},
			},
			releaseTrainPrognosisResponse: &api.GetReleaseTrainPrognosisResponse{
				EnvsPrognoses: map[string]*api.ReleaseTrainEnvPrognosis{
					"development": &api.ReleaseTrainEnvPrognosis{
						Outcome: &api.ReleaseTrainEnvPrognosis_AppsPrognoses{
							AppsPrognoses: &api.ReleaseTrainEnvPrognosis_AppsPrognosesWrapper{
								Prognoses: map[string]*api.ReleaseTrainAppPrognosis{
									"foo_app": &api.ReleaseTrainAppPrognosis{
										Outcome: &api.ReleaseTrainAppPrognosis_DeployedVersion{
											DeployedVersion: &api.ReleaseTrainPrognosisDeployedVersion{
												Version:  99,
												Revision: 0,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "{\"development\":{\"Outcome\":{\"AppsPrognoses\":{\"prognoses\":{\"foo_app\":{\"Outcome\":{\"DeployedVersion\":{\"version\":99}}}}}}}}",
			expectedReleaseTrainPrognosisRequest: &api.ReleaseTrainRequest{
				Target:     "development",
				Team:       "",
				CommitHash: "",
			},
		},
		{
			name: "release train but wrong method",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/environments/development/releasetrain",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusMethodNotAllowed,
			},
			expectedBody: "releasetrain only accepts method PUT, got: 'GET'\n",
		},
		{
			name: "release train prognosis with wrong method",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/environments/development/releasetrain/prognosis",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusMethodNotAllowed,
			},
			expectedBody: "releasetrain prognosis only accepts method GET, got: 'PUT'\n",
		},
		{
			name: "release train but additional path params",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/releasetrain/junk",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "release trains must be invoked via /releasetrain, but it was invoked via /releasetrain/junk\n",
		},
		{
			name: "release train prognosis but additional path params",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/api/environments/development/releasetrain/prognosis/junk",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "release trains must be invoked via /releasetrain/prognosis, but it was invoked via /releasetrain/prognosis/junk\n",
		},
		{
			name:             "release train - Azure enabled",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/releasetrain",
				},
				Body: io.NopCloser(strings.NewReader(exampleSignature)),
			},
			batchResponse: &api.BatchResponse{
				Results: []*api.BatchResult{
					{
						Result: &api.BatchResult_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainResponse{
								Target: "development",
							},
						},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "{\"target\":\"development\"}",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_ReleaseTrain{
							ReleaseTrain: &api.ReleaseTrainRequest{Target: "development"},
						},
					},
				},
			},
		},
		{
			name:             "release train - Azure enabled - missing signature",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/releasetrain",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "missing request body",
		},
		{
			name:             "release train - Azure enabled - invalid signature",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/releasetrain",
				},
				Body: io.NopCloser(strings.NewReader("uncool")),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusInternalServerError,
			},
			expectedBody: "Internal: Invalid Signature: EOF",
		},
		{
			name: "create environment -- dryrun",
			req: &http.Request{
				Method: http.MethodPost,
				Header: http.Header{
					"Content-Type": []string{"multipart/form-data"},
				},
				URL: &url.URL{
					Path:     "/api/environments/stg/",
					RawQuery: "dryrun=true",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"config": []string{exampleConfig},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironment{
							CreateEnvironment: &api.CreateEnvironmentRequest{
								Environment: "stg",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Environment: &exampleEnvironment,
									},
									Argocd:           nil,
									EnvironmentGroup: nil,
								},
								Dryrun: true,
							},
						},
					},
				},
			},
		},
		{
			name: "create environment",
			req: &http.Request{
				Method: http.MethodPost,
				Header: http.Header{
					"Content-Type": []string{"multipart/form-data"},
				},
				URL: &url.URL{
					Path: "/environments/stg/",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"config": []string{exampleConfig},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironment{
							CreateEnvironment: &api.CreateEnvironmentRequest{
								Environment: "stg",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Environment: &exampleEnvironment,
									},
									Argocd:           nil,
									EnvironmentGroup: nil,
								},
								Dryrun: false,
							},
						},
					},
				},
			},
		},
		{
			name: "create environment  - more data version",
			req: &http.Request{
				Method: http.MethodPost,
				Header: http.Header{
					"Content-Type": []string{"multipart/form-data"},
				},
				URL: &url.URL{
					Path: "/environments/stg/",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"config": []string{`
{
  "argocd": {
    "accessList": [],
    "applicationAnnotations": {},
    "destination": {
      "namespace": "*",
      "server": "https://example.com:443"
    }
  },
  "upstream": {
    "latest": true
  },
  "environmentGroup": "*"
}
`},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironment{
							CreateEnvironment: &api.CreateEnvironmentRequest{
								Environment: "stg",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: &latestFlag,
									},
									Argocd: &api.ArgoCDEnvironmentConfiguration{
										SyncWindows: nil,
										Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
											Server:    "https://example.com:443",
											Namespace: &starFlag,
										},
										AccessList:             []*api.ArgoCDEnvironmentConfiguration_AccessEntry{},
										ApplicationAnnotations: nil,
										IgnoreDifferences:      nil,
										SyncOptions:            nil,
									},
									EnvironmentGroup: &starFlag,
								},
								Dryrun: false,
							},
						},
					},
				},
			},
		},
		{
			name: "create environment but wrong method",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/environments/stg/",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "unknown function ''\n",
		},
		{
			name: "create environment but additional path params",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/stg/my-awesome-path",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "unknown function 'my-awesome-path'\n",
		},
		{
			name:             "create environment - Azure enabled",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/environments/stg/",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"config":    []string{exampleConfig},
						"signature": []string{exampleConfigSignature},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironment{
							CreateEnvironment: &api.CreateEnvironmentRequest{
								Environment: "stg",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Environment: &exampleEnvironment,
									},
									Argocd:           nil,
									EnvironmentGroup: nil,
								},
								Dryrun: false,
							},
						},
					},
				},
			},
		},
		{
			name:             "create environment - Azure enabled - missing signature",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/environments/stg/",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"config": []string{exampleConfig},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "Missing signature in request body",
		},
		{
			name:             "create environment - Azure enabled - invalid signature",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/environments/stg/",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"config":    []string{exampleConfig},
						"signature": []string{"my bad signature"},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusInternalServerError,
			},
			expectedBody: "Internal: Invalid Signature: EOF",
		},
		{
			name:    "create environment - Azure disabled - invalid signature",
			KeyRing: exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/environments/stg/",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"config":    []string{exampleConfig},
						"signature": []string{"my bad signature"},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusInternalServerError,
			},
			expectedBody: "Internal: Invalid Signature: EOF",
		},
		{
			name: "lock env",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/locks/test",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"message":"test message", "suggestedLifeTime": "2h"}`)),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironmentLock{
							CreateEnvironmentLock: &api.CreateEnvironmentLockRequest{
								Environment:       "development",
								LockId:            "test",
								Message:           "test message",
								SuggestedLifeTime: &lifeTime2h,
							},
						},
					},
				},
			},
		},
		{
			name: "lock env with CI Link",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/locks/test",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"message":"test message", "ciLink":"www.test.com"}`)),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironmentLock{
							CreateEnvironmentLock: &api.CreateEnvironmentLockRequest{
								Environment:       "development",
								LockId:            "test",
								Message:           "test message",
								CiLink:            "www.test.com",
								SuggestedLifeTime: &lifeTimeEmpty,
							},
						},
					},
				},
			},
		},
		{
			name:             "lock env - Azure Enabled",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/locks/test",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(bytes.NewReader(lockRequestJSON)),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironmentLock{
							CreateEnvironmentLock: &api.CreateEnvironmentLockRequest{
								Environment:       "development",
								LockId:            "test",
								Message:           "test message",
								CiLink:            "www.test.com",
								SuggestedLifeTime: &lifeTime3d,
							},
						},
					},
				},
			},
		},
		{
			name:             "lock env - Azure Enabled - wrong signature",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/staging/locks/test",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(bytes.NewReader(lockRequestJSON)),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusInternalServerError,
			},
			expectedBody: "Internal: Invalid Signature: openpgp: invalid signature: RSA verification failure",
		},
		{
			name: "lock env but missing lock ID",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/locks",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "missing ID for env lock\n",
		},
		{
			name: "lock env but additional path params",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/locks/test/junk",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "env locks does not accept additional path arguments after the lock ID, got: '/junk'\n",
		},
		{
			name: "lock env but wrong content type",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/locks/test",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusUnsupportedMediaType,
			},
			expectedBody: "body must be application/json, got: ''\n",
		},
		{
			name: "lock env but no content",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/locks/test",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(``)),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "Please provide lock message in body\n",
		},
		{
			name: "lock env but no message",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/locks/test",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{}`)),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "Please provide lock message in body\n",
		},
		{
			name: "lock env but empty message",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/locks/test",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"message":""}`)),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "Please provide lock message in body\n",
		},
		{
			name: "unlock env",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/environments/development/locks/test",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_DeleteEnvironmentLock{
							DeleteEnvironmentLock: &api.DeleteEnvironmentLockRequest{
								Environment: "development",
								LockId:      "test",
							},
						},
					},
				},
			},
		},
		{
			name: "unlock env but missing lock ID",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/environments/development/locks",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "missing ID for env lock\n",
		},
		{
			name: "unlock env but additional path params",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/environments/development/locks/test/junk",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "env locks does not accept additional path arguments after the lock ID, got: '/junk'\n",
		},
		{
			name: "lock env but wrong method",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/environments/development/locks/test",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusMethodNotAllowed,
			},
			expectedBody: "unsupported method 'GET'\n",
		},
		{
			name: "app but missing app",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/applications",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "missing application ID\n",
		},
		{
			name: "lock app",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/applications/service/locks/test",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"message":"test message", "ciLink":"www.test.com"}`)),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironmentApplicationLock{
							CreateEnvironmentApplicationLock: &api.CreateEnvironmentApplicationLockRequest{
								Environment:       "development",
								Application:       "service",
								LockId:            "test",
								Message:           "test message",
								CiLink:            "www.test.com",
								SuggestedLifeTime: &lifeTimeEmpty,
							},
						},
					},
				},
			},
		},
		{
			name: "lock app but missing lock ID",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/applications/service/locks",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "missing lock ID\n",
		},
		{
			name: "lock app but additional path params",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/applications/service/locks/test/junk",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "locks does not accept additional path arguments after the lock ID, got: /junk\n",
		},
		{
			name: "lock app but wrong content type",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/environments/development/applications/service/locks/test",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusUnsupportedMediaType,
			},
			expectedBody: "body must be application/json, got: ''\n",
		},
		{
			name: "unlock app",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/environments/development/applications/service/locks/test",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_DeleteEnvironmentApplicationLock{
							DeleteEnvironmentApplicationLock: &api.DeleteEnvironmentApplicationLockRequest{
								Environment: "development",
								Application: "service",
								LockId:      "test",
							},
						},
					},
				},
			},
		},
		{
			name: "unlock app but missing lock ID",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/environments/development/applications/service/locks",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "missing lock ID\n",
		},
		{
			name: "unlock app but additional path params",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/environments/development/applications/service/locks/test/junk",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "locks does not accept additional path arguments after the lock ID, got: /junk\n",
		},
		{
			name: "lock app but wrong method",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/environments/development/applications/service/locks/test",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusMethodNotAllowed,
			},
			expectedBody: "unsupported method 'GET'\n",
		},
		{
			name: "lock team",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/environments/development/lock/team/sre-team/test",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"message":"test message", "suggestedLifeTime": "3d"}`)),
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_CreateEnvironmentTeamLock{
							CreateEnvironmentTeamLock: &api.CreateEnvironmentTeamLockRequest{
								Environment:       "development",
								Team:              "sre-team",
								LockId:            "test",
								Message:           "test message",
								SuggestedLifeTime: &lifeTime3d,
							},
						},
					},
				},
			},
		},
		{
			name: "lock team but missing lock ID",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/environments/development/lock/team/sre-team/",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "Missing LockID\n",
		},
		{
			name: "lock team but additional path params",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/environments/development/lock/team/sre-team/test/junk",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "locks does not accept additional path arguments after the lock ID, got: /junk\n",
		},
		{
			name: "lock team but wrong content type",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/environments/development/lock/team/sre-team/test",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusUnsupportedMediaType,
			},
			expectedBody: "body must be application/json, got: ''\n",
		},
		{
			name: "unlock team",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/api/environments/development/lock/team/sre-team/test",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_DeleteEnvironmentTeamLock{
							DeleteEnvironmentTeamLock: &api.DeleteEnvironmentTeamLockRequest{
								Environment: "development",
								Team:        "sre-team",
								LockId:      "test",
							},
						},
					},
				},
			},
		},
		{
			name: "unlock team but missing lock ID",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/api/environments/development/lock/team/sre-team/",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "Missing LockID\n",
		},
		{
			name: "unlock team but additional path params",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/api/environments/development/lock/team/sre-team/test/junk",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "locks does not accept additional path arguments after the lock ID, got: /junk\n",
		},
		{
			name: "lock team but wrong method",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/api/environments/development/lock/team/sre-team/test",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusMethodNotAllowed,
			},
			expectedBody: "unsupported method 'GET'\n",
		},
		{
			name: "unlock env group",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/environment-groups/development/locks/test",
				},
			},
			batchResponse: &api.BatchResponse{Results: []*api.BatchResult{
				{},
			}},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "{\"Result\":null}",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_DeleteEnvironmentGroupLock{
							DeleteEnvironmentGroupLock: &api.DeleteEnvironmentGroupLockRequest{
								EnvironmentGroup: "development",
								LockId:           "test",
							},
						},
					},
				},
			},
		},
		{
			name: "unlock env group but missing lock ID",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/environment-groups/development/locks",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "missing ID for env group lock\n",
		},
		{
			name: "unlock env group but additional path params",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/environment-groups/development/locks/test/garbage",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "group locks does not accept additional path arguments after the lock ID, got: '/garbage'\n",
		},
		{
			name: "lock env group but wrong method",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/environment-groups/development/locks/test-lock",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusMethodNotAllowed,
			},
			expectedBody: "unsupported method 'GET'\n",
		},
		{
			name: "environment application commitInfo",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/api/environments/development/applications/testapp/commit",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: `{"author":"testauthor","commit_id":"testcommitId","commit_message":"testmessage"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchClient := &mockBatchClient{batchResponse: tt.batchResponse}
			releaseTrainPrognosisClient := &mockReleaseTrainPrognosisServiceClient{
				response: tt.releaseTrainPrognosisResponse,
			}
			versionClient := &mockVersionClient{
				response: tt.versionClientResponse,
			}
			commitInfoClient := &mockCommitDeploymentServiceClient{}
			s := Server{
				BatchClient:                 batchClient,
				ReleaseTrainPrognosisClient: releaseTrainPrognosisClient,
				VersionClient:               versionClient,
				CommitDeploymentsClient:     commitInfoClient,
				KeyRing:                     tt.KeyRing,
				AzureAuth:                   tt.AzureAuthEnabled,
				Config: config.ServerConfig{
					RevisionsEnabled: true,
				},
			}

			w := httptest.NewRecorder()
			if len(tt.req.URL.Path) >= 4 && tt.req.URL.Path[:4] == "/api" {
				s.HandleAPI(w, tt.req)
			} else {
				s.Handle(w, tt.req)
			}
			resp := w.Result()

			if d := cmp.Diff(tt.expectedResp, resp, cmpopts.IgnoreFields(http.Response{}, "Status", "Proto", "ProtoMajor", "ProtoMinor", "Header", "Body", "ContentLength")); d != "" {
				t.Errorf("response mismatch: %s", d)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("error reading response body: %s", err)
			}
			if d := cmp.Diff(tt.expectedBody, string(body)); d != "" {
				t.Errorf("response body mismatch:\ngot:  %s\nwant: %s\ndiff: \n%s", string(body), tt.expectedBody, d)
			}
			if d := cmp.Diff(tt.expectedBatchRequest, batchClient.batchRequest, protocmp.Transform()); d != "" {
				t.Errorf("create batch request mismatch: %s", d)
			}
			if d := cmp.Diff(tt.expectedGetManifestRequest, versionClient.request, protocmp.Transform()); d != "" {
				t.Errorf("get manifests request mismatch: %s", d)
			}
		})
	}
}

type mockRolloutClient struct {
	api.RolloutServiceClient
	request  *api.GetStatusRequest
	response *api.GetStatusResponse
}

func (m *mockRolloutClient) GetStatus(_ context.Context, in *api.GetStatusRequest, _ ...grpc.CallOption) (*api.GetStatusResponse, error) {
	m.request = in
	return m.response, nil
}

func TestServer_Rollout(t *testing.T) {
	tests := []struct {
		name                  string
		req                   *http.Request
		KeyRing               openpgp.KeyRing
		signature             string
		AzureAuthEnabled      bool
		Config                config.ServerConfig
		statusResponse        *api.GetStatusResponse
		expectedResp          *http.Response
		expectedBody          string
		expectedStatusRequest *api.GetStatusRequest
	}{
		{
			name: "propagates the environment group",
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/environment-groups/development/rollout-status",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{}`)),
			},
			statusResponse: &api.GetStatusResponse{},

			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: `{"status":"unknown","applications":[]}`,
			expectedStatusRequest: &api.GetStatusRequest{
				EnvironmentGroup: "development",
			},
		},
		{
			name: "propagates the team",
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/environment-groups/development/rollout-status",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"team":"foo"}`)),
			},
			statusResponse: &api.GetStatusResponse{},

			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: `{"status":"unknown","applications":[]}`,
			expectedStatusRequest: &api.GetStatusRequest{
				EnvironmentGroup: "development",
				Team:             "foo",
			},
		},
		{
			name: "propagates the wait time",
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/environment-groups/development/rollout-status",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"waitDuration":"1m"}`)),
			},
			statusResponse: &api.GetStatusResponse{},
			Config: config.ServerConfig{
				MaxWaitDuration: 2 * time.Minute,
			},

			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: `{"status":"unknown","applications":[]}`,
			expectedStatusRequest: &api.GetStatusRequest{
				EnvironmentGroup: "development",
				WaitSeconds:      60,
			},
		},
		{
			name: "rejects high wait time",
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/environment-groups/development/rollout-status",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"waitDuration":"10m"}`)),
			},
			statusResponse: &api.GetStatusResponse{},
			Config: config.ServerConfig{
				MaxWaitDuration: 2 * time.Minute,
			},

			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "waitDuration is too high: 10m - maximum is 2m0s\n",
		},
		{
			name: "rejects low wait time",
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/environment-groups/development/rollout-status",
				},
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"waitDuration":"1ns"}`)),
			},
			statusResponse: &api.GetStatusResponse{},
			Config: config.ServerConfig{
				MaxWaitDuration: 2 * time.Minute,
			},

			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "waitDuration is shorter than one second: 1ns\n",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			rolloutClient := &mockRolloutClient{response: tt.statusResponse}
			s := Server{
				RolloutClient: rolloutClient,
				KeyRing:       tt.KeyRing,
				AzureAuth:     tt.AzureAuthEnabled,
				Config:        tt.Config,
			}

			w := httptest.NewRecorder()
			if tt.req.URL.Path[:3] == "api" {
				s.HandleAPI(w, tt.req)
			} else {
				s.Handle(w, tt.req)
			}
			resp := w.Result()

			if d := cmp.Diff(tt.expectedResp, resp, cmpopts.IgnoreFields(http.Response{}, "Status", "Proto", "ProtoMajor", "ProtoMinor", "Header", "Body", "ContentLength")); d != "" {
				t.Errorf("response mismatch: %s", d)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("error reading response body: %s", err)
			}
			if d := cmp.Diff(tt.expectedBody, string(body)); d != "" {
				t.Errorf("response body mismatch:\ngot:  %s\nwant: %s\ndiff: \n%s", string(body), tt.expectedBody, d)

			}
			if d := cmp.Diff(tt.expectedStatusRequest, rolloutClient.request, protocmp.Transform()); d != "" {
				t.Errorf("create get status request mismatch: %s", d)
			}
		})
	}
}

type mockBatchClient struct {
	batchRequest  *api.BatchRequest
	batchResponse *api.BatchResponse
}

func (m *mockBatchClient) ProcessBatch(_ context.Context, in *api.BatchRequest, _ ...grpc.CallOption) (*api.BatchResponse, error) {
	m.batchRequest = in
	return m.batchResponse, nil
}

func TestWriteCorrespondingResponse(t *testing.T) {
	tests := []struct {
		name             string
		releaseResponse  *api.CreateReleaseResponse
		expectedStatus   int
		expectedResponse interface{}
	}{
		{
			name: "Success",
			releaseResponse: &api.CreateReleaseResponse{
				Response: &api.CreateReleaseResponse_Success{},
			},
			expectedStatus:   http.StatusCreated,
			expectedResponse: &api.CreateReleaseResponse_Success{},
		},
		{
			name: "AlreadyExistsSame",
			releaseResponse: &api.CreateReleaseResponse{
				Response: &api.CreateReleaseResponse_AlreadyExistsSame{},
			},
			expectedStatus:   http.StatusOK,
			expectedResponse: &api.CreateReleaseResponse_AlreadyExistsSame{},
		},
		{
			name: "AlreadyExistsDifferent",
			releaseResponse: &api.CreateReleaseResponse{
				Response: &api.CreateReleaseResponse_AlreadyExistsDifferent{},
			},
			expectedStatus:   http.StatusConflict,
			expectedResponse: &api.CreateReleaseResponse_AlreadyExistsDifferent{},
		},
		{
			name: "GeneralFailure",
			releaseResponse: &api.CreateReleaseResponse{
				Response: &api.CreateReleaseResponse_GeneralFailure{},
			},
			expectedStatus:   http.StatusInternalServerError,
			expectedResponse: &api.CreateReleaseResponse_GeneralFailure{},
		},
		{
			name: "TooOld",
			releaseResponse: &api.CreateReleaseResponse{
				Response: &api.CreateReleaseResponse_TooOld{},
			},
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: &api.CreateReleaseResponse_TooOld{},
		},
		{
			name: "TooLong",
			releaseResponse: &api.CreateReleaseResponse{
				Response: &api.CreateReleaseResponse_TooLong{},
			},
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: &api.CreateReleaseResponse_TooLong{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			req, err := http.NewRequest("GET", "/", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()

			writeCorrespondingResponse(ctx, rr, req, tt.releaseResponse, nil)

			if tt.expectedStatus != rr.Code {
				t.Errorf("expected status code %d, but got %d", tt.expectedStatus, rr.Code)
			}

			expectedJSON, err := json.Marshal(tt.expectedResponse)
			if err != nil {
				t.Fatalf("failed to marshal json: %v", err)
			}

			assert.JSONEq(t, string(expectedJSON), rr.Body.String())
		})
	}
}

func TestServer_HandleAAEnvironments(t *testing.T) {
	exampleKey, err := openpgp.NewEntity("Test", "", "test@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	exampleKeyRing := openpgp.EntityList{exampleKey}

	exampleConfig := `
{
	"accessList": [],
	"applicationAnnotations": {},
	"destination": {
  		"namespace": "*", 
  		"server": "https://example.com:443"
	}
}
`
	// workaround for *bool
	starFlag := "*"

	signatureBuffer := bytes.Buffer{}
	err = openpgp.ArmoredDetachSign(&signatureBuffer, exampleKey, bytes.NewReader([]byte(exampleConfig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	exampleConfigSignature := signatureBuffer.String()

	tests := []struct {
		name                 string
		req                  *http.Request
		KeyRing              openpgp.KeyRing
		signature            string
		AzureAuthEnabled     bool
		batchResponse        *api.BatchResponse
		expectedResp         *http.Response
		expectedBody         string
		expectedBatchRequest *api.BatchRequest
	}{
		{
			name: "create environment  - more data version",
			req: &http.Request{
				Method: http.MethodPost,
				Header: http.Header{
					"Content-Type": []string{"multipart/form-data"},
				},
				URL: &url.URL{
					Path: "/api/environments/envName/cluster",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"config": []string{exampleConfig},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_ExtendAaEnvironment{
							ExtendAaEnvironment: &api.ExtendAAEnvironmentRequest{
								EnvironmentName: "envName",
								ArgoCdConfiguration: &api.ArgoCDEnvironmentConfiguration{
									SyncWindows: nil,
									Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
										Server:    "https://example.com:443",
										Namespace: &starFlag,
									},
									AccessList:             []*api.ArgoCDEnvironmentConfiguration_AccessEntry{},
									ApplicationAnnotations: nil,
									IgnoreDifferences:      nil,
									SyncOptions:            nil,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "create environment but additional path params",
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/api/environments/envName/cluster/my-awesome-path",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "Extend Active/Active environment does not accept any extra arguments, got: '/my-awesome-path'\n",
		},
		{
			name:             "extend environment - Azure enabled",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/api/environments/envName/cluster",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"config":    {exampleConfig},
						"signature": {exampleConfigSignature},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody: "",
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_ExtendAaEnvironment{
							ExtendAaEnvironment: &api.ExtendAAEnvironmentRequest{
								EnvironmentName: "envName",
								ArgoCdConfiguration: &api.ArgoCDEnvironmentConfiguration{
									SyncWindows: nil,
									Destination: &api.ArgoCDEnvironmentConfiguration_Destination{
										Server:    "https://example.com:443",
										Namespace: &starFlag,
									},
									AccessList:             []*api.ArgoCDEnvironmentConfiguration_AccessEntry{},
									ApplicationAnnotations: nil,
									IgnoreDifferences:      nil,
									SyncOptions:            nil,
								},
							},
						},
					},
				},
			},
		},
		{
			name:             "create environment - Azure enabled - missing signature",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/api/environments/envName/cluster/",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"config": []string{exampleConfig},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			expectedBody: "Missing signature in request body",
		},
		{
			name:             "create environment - Azure enabled - invalid signature",
			AzureAuthEnabled: true,
			KeyRing:          exampleKeyRing,
			req: &http.Request{
				Method: http.MethodPost,
				URL: &url.URL{
					Path: "/api/environments/envName/cluster/",
				},
				MultipartForm: &multipart.Form{
					Value: map[string][]string{
						"config":    {exampleConfig},
						"signature": {"my bad signature"},
					},
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusUnauthorized,
			},
			expectedBody: "Invalid Signature: EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchClient := &mockBatchClient{batchResponse: tt.batchResponse}
			commitInfoClient := &mockCommitDeploymentServiceClient{}
			s := Server{
				BatchClient:             batchClient,
				CommitDeploymentsClient: commitInfoClient,
				KeyRing:                 tt.KeyRing,
				AzureAuth:               tt.AzureAuthEnabled,
				Config: config.ServerConfig{
					RevisionsEnabled: true,
				},
			}

			w := httptest.NewRecorder()

			s.HandleAPI(w, tt.req)

			resp := w.Result()

			if d := cmp.Diff(tt.expectedResp, resp, cmpopts.IgnoreFields(http.Response{}, "Status", "Proto", "ProtoMajor", "ProtoMinor", "Header", "Body", "ContentLength")); d != "" {
				t.Errorf("response mismatch: %s", d)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("error reading response body: %s", err)
			}
			if d := cmp.Diff(tt.expectedBody, string(body)); d != "" {
				t.Errorf("response body mismatch:\ngot:  %s\nwant: %s\ndiff: \n%s", string(body), tt.expectedBody, d)
			}
			if d := cmp.Diff(tt.expectedBatchRequest, batchClient.batchRequest, protocmp.Transform()); d != "" {
				t.Errorf("create batch request mismatch: %s", d)
			}
		})
	}
}

func TestServer_HandleDeleteAAEnvConfig(t *testing.T) {
	tests := []struct {
		name                 string
		req                  *http.Request
		batchResponse        *api.BatchResponse
		expectedResp         *http.Response
		expectedBatchRequest *api.BatchRequest
	}{
		{
			name: "create environment  - more data version",
			req: &http.Request{
				Method: http.MethodDelete,
				Header: http.Header{
					"Content-Type": []string{"multipart/form-data"},
				},
				URL: &url.URL{
					Path: "/api/environments/envName/cluster/test-1",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			},
			expectedBatchRequest: &api.BatchRequest{
				Actions: []*api.BatchAction{
					{
						Action: &api.BatchAction_DeleteAaEnvironmentConfig{
							DeleteAaEnvironmentConfig: &api.DeleteAAEnvironmentConfigRequest{
								ParentEnvironmentName:   "envName",
								ConcreteEnvironmentName: "test-1",
							},
						},
					},
				},
			},
		},
		{
			name: "Wrong Method",
			req: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Path: "/api/environments/envName/cluster/test-1",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewReader([]byte("cluster function does not support http method 'PUT'\n"))),
			},
		},
		{
			name: "Some extra arguments",
			req: &http.Request{
				Method: http.MethodDelete,
				URL: &url.URL{
					Path: "/api/environments/envName/cluster/test-1/this-should-not-be-here",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewReader([]byte("Delete Active/Active environment config does not accept any extra arguments, got: '/this-should-not-be-here'\n"))),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchClient := &mockBatchClient{batchResponse: tt.batchResponse}
			commitInfoClient := &mockCommitDeploymentServiceClient{}
			s := Server{
				BatchClient:             batchClient,
				CommitDeploymentsClient: commitInfoClient,
				Config: config.ServerConfig{
					RevisionsEnabled: true,
				},
			}

			w := httptest.NewRecorder()

			s.HandleAPI(w, tt.req)

			resp := w.Result()

			if d := cmp.Diff(tt.expectedResp, resp, cmpopts.IgnoreFields(http.Response{}, "Status", "Proto", "ProtoMajor", "ProtoMinor", "Header", "Body", "ContentLength")); d != "" {
				t.Errorf("response mismatch: %s", d)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("error reading response body: %s", err)
			}
			expectedBody, err := io.ReadAll(tt.expectedResp.Body)
			if err != nil {
				t.Errorf("error reading expected body: %s", err)
			}
			if d := cmp.Diff(string(expectedBody), string(body)); d != "" {
				t.Errorf("response body mismatch:\ngot:  %s\nwant: %s\ndiff: \n%s", string(body), string(expectedBody), d)
			}
			if d := cmp.Diff(tt.expectedBatchRequest, batchClient.batchRequest, protocmp.Transform()); d != "" {
				t.Errorf("create batch request mismatch: %s", d)
			}
		})
	}
}
