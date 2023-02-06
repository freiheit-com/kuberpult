
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/openpgp"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestServer_Handle(t *testing.T) {
	exampleKey, err := openpgp.NewEntity("Test", "", "test@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	exampleKeyRing := openpgp.EntityList{exampleKey}
	exampleEnvironment := "development"
	exampleLockId := "test"
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
	lockRequestJSON, _ := json.Marshal(putLockRequest{
		Message:   "test message",
		Signature: exampleLockSignature,
	})

	tests := []struct {
		name                                            string
		req                                             *http.Request
		KeyRing                                         openpgp.KeyRing
		signature                                       string
		AzureAuthEnabled                                bool
		expectedResp                                    *http.Response
		expectedBody                                    string
		expectedDeployRequest                           *api.DeployRequest
		expectedReleaseTrainRequest                     *api.ReleaseTrainRequest
		expectedCreateEnvironmentLockRequest            *api.CreateEnvironmentLockRequest
		expectedDeleteEnvironmentLockRequest            *api.DeleteEnvironmentLockRequest
		expectedCreateEnvironmentApplicationLockRequest *api.CreateEnvironmentApplicationLockRequest
		expectedDeleteEnvironmentApplicationLockRequest *api.DeleteEnvironmentApplicationLockRequest
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
			expectedBody: "unknown group ''\n",
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
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody:                "{\"upstream\":\"production\",\"targetEnv\":\"development\"}",
			expectedReleaseTrainRequest: &api.ReleaseTrainRequest{Environment: "development"},
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
			expectedBody: "releasetrain does not accept additional path arguments, got: '/junk'\n",
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
			expectedResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedBody:                "{\"upstream\":\"production\",\"targetEnv\":\"development\"}",
			expectedReleaseTrainRequest: &api.ReleaseTrainRequest{Environment: "development"},
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
			expectedCreateEnvironmentLockRequest: &api.CreateEnvironmentLockRequest{
				Environment: "development",
				LockId:      "test",
				Message:     "test message",
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
			expectedCreateEnvironmentLockRequest: &api.CreateEnvironmentLockRequest{
				Environment: "development",
				LockId:      "test",
				Message:     "test message",
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
			expectedBody: "Internal: Invalid Signature: openpgp: invalid signature: hash tag doesn't match",
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
			expectedBody: "missing lock ID\n",
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
			expectedBody: "locks does not accept additional path arguments after the lock ID, got: '/junk'\n",
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
			expectedDeleteEnvironmentLockRequest: &api.DeleteEnvironmentLockRequest{
				Environment: "development",
				LockId:      "test",
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
			expectedBody: "missing lock ID\n",
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
			expectedBody: "locks does not accept additional path arguments after the lock ID, got: '/junk'\n",
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
			expectedCreateEnvironmentApplicationLockRequest: &api.CreateEnvironmentApplicationLockRequest{
				Environment: "development",
				Application: "service",
				LockId:      "test",
				Message:     "test message",
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
			expectedDeleteEnvironmentApplicationLockRequest: &api.DeleteEnvironmentApplicationLockRequest{
				Environment: "development",
				Application: "service",
				LockId:      "test",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployClient := &mockDeployClient{}
			lockClient := &mockLockClient{}
			s := Server{
				DeployClient: deployClient,
				LockClient:   lockClient,
				KeyRing:      tt.KeyRing,
				AzureAuth:    tt.AzureAuthEnabled,
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
				t.Errorf("response body mismatch: %s", d)
			}
			if d := cmp.Diff(tt.expectedDeployRequest, deployClient.deployRequest, protocmp.Transform()); d != "" {
				t.Errorf("deploy request mismatch: %s", d)
			}
			if d := cmp.Diff(tt.expectedReleaseTrainRequest, deployClient.releaseTrainRequest, protocmp.Transform()); d != "" {
				t.Errorf("release train request mismatch: %s", d)
			}
			if d := cmp.Diff(tt.expectedCreateEnvironmentLockRequest, lockClient.createEnvironmentLockRequest, protocmp.Transform()); d != "" {
				t.Errorf("create environment lock request mismatch: %s", d)
			}
			if d := cmp.Diff(tt.expectedDeleteEnvironmentLockRequest, lockClient.deleteEnvironmentLockRequest, protocmp.Transform()); d != "" {
				t.Errorf("delete environment lock request mismatch: %s", d)
			}
			if d := cmp.Diff(tt.expectedCreateEnvironmentApplicationLockRequest, lockClient.createEnvironmentApplicationLockRequest, protocmp.Transform()); d != "" {
				t.Errorf("create environment application lock request mismatch: %s", d)
			}
			if d := cmp.Diff(tt.expectedDeleteEnvironmentApplicationLockRequest, lockClient.deleteEnvironmentApplicationLockRequest, protocmp.Transform()); d != "" {
				t.Errorf("delete environment application lock request mismatch: %s", d)
			}
		})
	}
}

type mockDeployClient struct {
	deployRequest       *api.DeployRequest
	releaseTrainRequest *api.ReleaseTrainRequest
}

func (m *mockDeployClient) Deploy(_ context.Context, in *api.DeployRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	m.deployRequest = in
	return &emptypb.Empty{}, nil
}

func (m *mockDeployClient) ReleaseTrain(_ context.Context, in *api.ReleaseTrainRequest, _ ...grpc.CallOption) (*api.ReleaseTrainResponse, error) {
	m.releaseTrainRequest = in
	return &api.ReleaseTrainResponse{TargetEnv: in.Environment, Upstream: "production"}, nil
}

type mockLockClient struct {
	createEnvironmentLockRequest            *api.CreateEnvironmentLockRequest
	deleteEnvironmentLockRequest            *api.DeleteEnvironmentLockRequest
	createEnvironmentApplicationLockRequest *api.CreateEnvironmentApplicationLockRequest
	deleteEnvironmentApplicationLockRequest *api.DeleteEnvironmentApplicationLockRequest
}

func (m *mockLockClient) CreateEnvironmentLock(_ context.Context, in *api.CreateEnvironmentLockRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	m.createEnvironmentLockRequest = in
	return &emptypb.Empty{}, nil
}

func (m *mockLockClient) DeleteEnvironmentLock(_ context.Context, in *api.DeleteEnvironmentLockRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	m.deleteEnvironmentLockRequest = in
	return &emptypb.Empty{}, nil
}

func (m *mockLockClient) CreateEnvironmentApplicationLock(_ context.Context, in *api.CreateEnvironmentApplicationLockRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	m.createEnvironmentApplicationLockRequest = in
	return &emptypb.Empty{}, nil
}

func (m *mockLockClient) DeleteEnvironmentApplicationLock(_ context.Context, in *api.DeleteEnvironmentApplicationLockRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	m.deleteEnvironmentApplicationLockRequest = in
	return &emptypb.Empty{}, nil
}
