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

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"

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

func TestServer_Handle(t *testing.T) {
	exampleKey, err := openpgp.NewEntity("Test", "", "test@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	exampleKeyRing := openpgp.EntityList{exampleKey}
	exampleEnvironment := "development"
	exampleLockId := "test"
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
		Message:   "test message",
		Signature: exampleLockSignature,
	})

	tests := []struct {
		name                                 string
		req                                  *http.Request
		KeyRing                              openpgp.KeyRing
		signature                            string
		AzureAuthEnabled                     bool
		batchResponse                        *api.BatchResponse
		releaseTrainPrognosisResponse        *api.GetReleaseTrainPrognosisResponse
		expectedResp                         *http.Response
		expectedBody                         string
		expectedBatchRequest                 *api.BatchRequest
		expectedReleaseTrainPrognosisRequest *api.ReleaseTrainRequest
	}{
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
			name: "release train prognosis",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/environments/development/releasetrain/prognosis",
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
											DeployedVersion: 99,
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
			expectedBody: "{\"development\":{\"Outcome\":{\"AppsPrognoses\":{\"prognoses\":{\"foo_app\":{\"Outcome\":{\"DeployedVersion\":99}}}}}}}",
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
					Path: "/environments/development/releasetrain/prognosis",
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
			expectedBody: "release trains must be invoked via either /releasetrain or /releasetrain/prognosis, but it was invoked via /releasetrain/junk\n",
		},
		{
			name: "release train but additional path params",
			req: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path: "/environments/development/releasetrain/prognosis/junk",
				},
			},
			expectedResp: &http.Response{
				StatusCode: http.StatusNotFound,
			},
			expectedBody: "release trains must be invoked via either /releasetrain or /releasetrain/prognosis, but it was invoked via /releasetrain/prognosis/junk\n",
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
  "environment_group": "*"
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
									Argocd: &api.EnvironmentConfig_ArgoCD{
										SyncWindows: nil,
										Destination: &api.EnvironmentConfig_ArgoCD_Destination{
											Server:    "https://example.com:443",
											Namespace: &starFlag,
										},
										AccessList:             []*api.EnvironmentConfig_ArgoCD_AccessEntry{},
										ApplicationAnnotations: nil,
										IgnoreDifferences:      nil,
										SyncOptions:            nil,
									},
									EnvironmentGroup: &starFlag,
								},
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
				Body: io.NopCloser(strings.NewReader(`{"message":"test message"}`)),
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
								Environment: "development",
								LockId:      "test",
								Message:     "test message",
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
								Environment: "development",
								LockId:      "test",
								Message:     "test message",
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
				Body: io.NopCloser(strings.NewReader(`{"message":"test message"}`)),
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
								Environment: "development",
								Application: "service",
								LockId:      "test",
								Message:     "test message",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchClient := &mockBatchClient{batchResponse: tt.batchResponse}
			releaseTrainPrognosisClient := &mockReleaseTrainPrognosisServiceClient{
				response: tt.releaseTrainPrognosisResponse,
			}
			s := Server{
				BatchClient:                 batchClient,
				ReleaseTrainPrognosisClient: releaseTrainPrognosisClient,
				KeyRing:                     tt.KeyRing,
				AzureAuth:                   tt.AzureAuthEnabled,
			}

			w := httptest.NewRecorder()
			s.Handle(w, tt.req)
			resp := w.Result()

			if d := cmp.Diff(tt.expectedResp, resp, cmpopts.IgnoreFields(http.Response{}, "Status", "Proto", "ProtoMajor", "ProtoMinor", "Header", "Body", "ContentLength")); d != "" {
				t.Errorf("response mismatch: %s", d)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("error reading response body: %s", err)
			}
			if d := cmp.Diff(tt.expectedBody, string(body)); d != "" {
				//t.Errorf("response body mismatch: %s", d)
				t.Errorf("response body mismatch:\ngot:  %s\nwant: %s\ndiff: \n%s", string(body), tt.expectedBody, d)

			}
			if d := cmp.Diff(tt.expectedBatchRequest, batchClient.batchRequest, protocmp.Transform()); d != "" {
				t.Errorf("create batch request mismatch: %s", d)
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
			s.Handle(w, tt.req)
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
