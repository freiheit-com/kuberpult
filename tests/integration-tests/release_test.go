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

package integration_tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/net/context"
)

const (
	devEnv       = "dev"
	stageEnv     = "staging"
	frontendPort = "8081"
)

// Used to compare two error message strings, needed because errors.Is(fmt.Errorf(text),fmt.Errorf(text)) == false
type errMatcher struct {
	msg string
}

func (e errMatcher) Error() string {
	return e.msg
}

func (e errMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}

func postWithForm(client *http.Client, url string, values map[string]io.Reader, files map[string]io.Reader) (*http.Response, error) {
	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer
	var err error
	multipartWriter := multipart.NewWriter(&b)
	for key, r := range values {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		if fw, err = multipartWriter.CreateFormField(key); err != nil {
			return nil, err
		}
		if _, err = io.Copy(fw, r); err != nil {
			return nil, err
		}
	}
	for key, r := range files {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		// Add a file
		if fw, err = multipartWriter.CreateFormFile(key, key); err != nil {
			return nil, err
		}
		if _, err = io.Copy(fw, r); err != nil {
			return nil, err
		}

	}
	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	err = multipartWriter.Close()
	if err != nil {
		return nil, err
	}

	// Now that you have a form, you can submit it to your handler.
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return nil, err
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	// Submit the request
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// calls the release endpoint with files for manifests
func callRelease(values map[string]io.Reader, files map[string]io.Reader, endpoint string) (int, string, error) {
	formResult, err := postWithForm(http.DefaultClient, "http://localhost:"+frontendPort+endpoint, values, files)
	if err != nil {
		return 0, "", err
	}
	defer formResult.Body.Close()
	resBody, err := io.ReadAll(formResult.Body)
	return formResult.StatusCode, string(resBody), err
}

// calls the release endpoint with files for manifests + signatures
func callCreateGroupLock(t *testing.T, envGroup, lockId string, requestBody *LockRequest) (int, string, error) {
	var buf bytes.Buffer
	jsonBytes, err := json.Marshal(&requestBody)
	if err != nil {
		return 0, "", err
	}
	buf.Write(jsonBytes)

	url := fmt.Sprintf("http://localhost:%s/environment-groups/%s/locks/%s", frontendPort, envGroup, lockId)
	req, err := http.NewRequest(http.MethodPut, url, &buf)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	responseBuf := new(strings.Builder)
	_, err = io.Copy(responseBuf, resp.Body)
	if err != nil {
		return 0, "", err
	}

	return resp.StatusCode, responseBuf.String(), err
}

func callEnvironmentLock(t *testing.T, environment, lockId string, requestBody *LockRequest, delete bool) (int, string, error) {
	var buf bytes.Buffer
	jsonBytes, err := json.Marshal(&requestBody)
	if err != nil {
		return 0, "", err
	}
	buf.Write(jsonBytes)

	url := fmt.Sprintf("http://localhost:%s/environments/%s/locks/%s", frontendPort, environment, lockId)
	method := http.MethodPut
	if delete {
		method = http.MethodDelete
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	responseBuf := new(strings.Builder)
	_, err = io.Copy(responseBuf, resp.Body)
	if err != nil {
		return 0, "", err
	}
	return resp.StatusCode, responseBuf.String(), err
}

func CalcSignature(t *testing.T, manifest string) string {
	cmd := exec.Command("gpg", "--keyring", "trustedkeys-kuberpult.gpg", "--local-user", "kuberpult-kind@example.com", "--detach", "--sign", "--armor")
	cmd.Stdin = strings.NewReader(manifest)
	theSignature, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(err.Error())
		t.Errorf("output: %s", string(theSignature))
		t.Fail()
	}
	return string(theSignature)
}

func TestReleaseCalls(t *testing.T) {
	theManifest := "I am a manifest\n- foo\nfoo"

	testCases := []struct {
		name               string
		inputApp           string
		inputManifest      string
		inputManifestEnv   string
		inputSignatureEnv  string // usually the same as inputManifestEnv
		inputVersion       string // actually an int, but for testing purposes it may be a string
		expectedStatusCode int
	}{
		{
			name:               "Simple invocation of /release endpoint",
			inputApp:           "my-app",
			inputManifest:      theManifest,
			inputManifestEnv:   devEnv,
			inputSignatureEnv:  devEnv,
			inputVersion:       "1",
			expectedStatusCode: 201,
		},
		{
			// Note that this test is not repeatable. Once the version exists, it cannot be overridden.
			// To repeat the test, we would have to reset the manifest repo.
			name:               "Simple invocation of /release endpoint with valid version should be new",
			inputApp:           "my-app-" + appSuffix,
			inputManifest:      theManifest,
			inputManifestEnv:   devEnv,
			inputSignatureEnv:  devEnv,
			inputVersion:       "99",
			expectedStatusCode: 201,
		},
		{
			// this is the same test, but this time we expect 200, because the release already exists:
			name:               "Simple invocation of /release endpoint with valid version should already exist",
			inputApp:           "my-app-" + appSuffix,
			inputManifest:      theManifest,
			inputManifestEnv:   devEnv,
			inputSignatureEnv:  devEnv,
			inputVersion:       "99",
			expectedStatusCode: 200,
		},
		{
			name:               "Simple invocation of /release endpoint with invalid version",
			inputApp:           "my-app",
			inputManifest:      theManifest,
			inputManifestEnv:   devEnv,
			inputSignatureEnv:  devEnv,
			inputVersion:       "notanumber",
			expectedStatusCode: 400,
		},
		{
			name:               "too long app name",
			inputApp:           "my-app-is-way-too-long-dont-you-think-so-too",
			inputManifest:      theManifest,
			inputManifestEnv:   devEnv,
			inputSignatureEnv:  devEnv,
			inputVersion:       "2",
			expectedStatusCode: 400,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			values := map[string]io.Reader{
				"application": strings.NewReader(tc.inputApp),
			}
			values["version"] = strings.NewReader(tc.inputVersion)
			files := map[string]io.Reader{
				"manifests[" + tc.inputManifestEnv + "]": strings.NewReader(tc.inputManifest),
			}

			actualStatusCode, body, err := callRelease(values, files, "/api/release")
			if err != nil {
				t.Fatalf("callRelease failed: %s", err.Error())
			}

			if actualStatusCode != tc.expectedStatusCode {
				t.Errorf("expected code %v but got %v. Body: %s", tc.expectedStatusCode, actualStatusCode, body)
			}
		})
	}
}

type LockRequest struct {
	Message   string `json:"message"`
	Signature string `json:"signature,omitempty"`
}

func TestGroupLock(t *testing.T) {
	testCases := []struct {
		name               string
		inputEnvGroup      string
		expectedStatusCode int
	}{
		{
			name:               "Simple invocation of group lock endpoint",
			inputEnvGroup:      "prod",
			expectedStatusCode: 201,
		},
	}

	for index, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			lockId := fmt.Sprintf("lockIdIntegration%d", index)
			inputSignature := CalcSignature(t, tc.inputEnvGroup+lockId)
			requestBody := &LockRequest{
				Message:   "hello world",
				Signature: inputSignature,
			}
			actualStatusCode, respBody, err := callCreateGroupLock(t, tc.inputEnvGroup, lockId, requestBody)
			if err != nil {
				t.Fatalf("callCreateGroupLock failed: %s", err.Error())
			}
			if actualStatusCode != tc.expectedStatusCode {
				t.Errorf("expected code %v but got %v. Body: '%s'", tc.expectedStatusCode, actualStatusCode, respBody)
			}
		})
	}
}

func TestEnvironmentLock(t *testing.T) {
	testCases := []struct {
		name                      string
		lockId                    string
		environment               string
		expectedStatusCodeLock    int
		expectedStatusCodeRelease int
		dbConfig                  db.DBConfig
		expectedBodyCreateRelease string
		appName                   string
	}{
		{
			name:                      "Create environment lock with endpoint",
			lockId:                    "A0",
			environment:               "development",
			expectedStatusCodeLock:    200,
			expectedStatusCodeRelease: 201,
			appName:                   "test-app",
			expectedBodyCreateRelease: "{\"Success\":{}}\n",
			dbConfig: db.DBConfig{
				DbName:       "kuberpult",
				DbUser:       "postgres",
				DbHost:       "localhost",
				DbPort:       "5432",
				DbPassword:   "mypassword",
				WriteEslOnly: false,
				DriverName:   "postgres",
				SSLMode:      "disable",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			lockId := tc.lockId
			environment := tc.environment
			appName := tc.appName
			inputSignature := CalcSignature(t, environment+lockId)
			requestBodyPut := &LockRequest{
				Message:   "Create environment lock",
				Signature: inputSignature,
			}
			requestBodyDelete := &LockRequest{
				Message:   "Delete environment lock",
				Signature: inputSignature,
			}
			ctx := testutil.MakeTestContext()
			dbHandler := connectToDB(t, tc.dbConfig, ctx)
			//Call the api to create Environment Lock in development
			actualStatusCode, respBody, err := callEnvironmentLock(t, environment, lockId, requestBodyPut, false)
			if err != nil {
				t.Fatalf("callEnvironmentLock failed %s", err.Error())
			}
			if actualStatusCode != tc.expectedStatusCodeLock {
				t.Errorf("expected code %v but got %v. Body: '%s'", tc.expectedStatusCodeLock, actualStatusCode, respBody)
			}
			// Check if the database was updated
			lock := callDBForLock(t, dbHandler, ctx, tc.environment, tc.lockId)
			if lock.Deleted {
				t.Errorf("expected active lock")
			}
			// Call the api to create a release
			values, files := createValuesFiles(t, appName, "1")
			actualStatusCode, actualBody, err := callRelease(values, files, "/api/release")
			if err != nil {
				t.Fatalf("callRelease failed %s", err.Error())
			}
			if actualStatusCode != tc.expectedStatusCodeRelease {
				t.Errorf("expected code %v but got %v. Body: '%s'", tc.expectedStatusCodeRelease, actualStatusCode, actualBody)
			}
			if diff := cmp.Diff(tc.expectedBodyCreateRelease, actualBody); diff != "" {
				t.Errorf("response body for create release mismatch (-want, +got):\n%s", diff)
			}

			release := callDBForReleases(t, dbHandler, ctx, appName)
			if len(release) != 1 {
				t.Errorf("expected 1 release but got %d", len(release))
			}

			// Call the db to see if the release was deployed
			deployment := callDBForDeployments(t, dbHandler, ctx, appName)
			if deployment.App != "" {
				t.Fatalf("expected no deployments")
			}

			// Call the api to delete the environment lock
			actualStatusCode, respBody, err = callEnvironmentLock(t, environment, lockId, requestBodyDelete, true)
			if err != nil {
				t.Fatalf("callEnvironmentLock failed %s", err.Error())
			}
			if actualStatusCode != tc.expectedStatusCodeLock {
				t.Errorf("expected code %v but got %v. Body: '%s'", tc.expectedStatusCodeLock, actualStatusCode, respBody)
			}

			lock = callDBForLock(t, dbHandler, ctx, environment, lockId)
			if !lock.Deleted {
				t.Errorf("expected deleted lock")
			}
			// Call the api to create a release this time with no environment lock
			values, files = createValuesFiles(t, appName, "2")
			actualStatusCode, actualBody, err = callRelease(values, files, "/api/release")
			if err != nil {
				t.Fatalf("callRelease failed %s", err.Error())
			}
			if actualStatusCode != tc.expectedStatusCodeRelease {
				t.Errorf("expected code %v but got %v. Body: '%s'", tc.expectedStatusCodeRelease, actualStatusCode, actualBody)
			}
			if diff := cmp.Diff(tc.expectedBodyCreateRelease, actualBody); diff != "" {
				t.Errorf("response body for create release mismatch (-want, +got):\n%s", diff)
			}

			release = callDBForReleases(t, dbHandler, ctx, appName)
			if len(release) != 2 {
				t.Errorf("expected 2 release but got %d", len(release))
			}

			deployment = callDBForDeployments(t, dbHandler, ctx, appName)
			if deployment.App != appName || deployment.Env != "development" {
				t.Fatalf("expected one deployment")
			}
		})
	}
}

func connectToDB(t *testing.T, dbConfig db.DBConfig, ctx context.Context) *db.DBHandler {
	dbHandler, err := db.Connect(ctx, dbConfig)
	if err != nil {
		t.Fatalf("DbConnect failed %s", err.Error())
	}
	pErr := dbHandler.DB.Ping()
	if pErr != nil {
		t.Fatalf("DbPint failed %s", pErr.Error())
	}
	return dbHandler
}

func callDBForLock(t *testing.T, dbHandler *db.DBHandler, ctx context.Context, environment, lockId string) *db.EnvironmentLock {
	lock, err := db.WithTransactionT(dbHandler, ctx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*db.EnvironmentLock, error) {
		return dbHandler.DBSelectEnvironmentLock(ctx, transaction, "development", lockId)
	})
	if err != nil {
		t.Errorf("DBSelectEnvionmentLock failed %s", err)
	}
	if lock.LockID != lockId {
		t.Errorf("expected LockId %s but got %s", lockId, lock.LockID)
	}
	if lock.Env != environment {
		t.Errorf("expected Environment %s but got %s", environment, lock.Env)
	}
	return lock
}

func callDBForReleases(t *testing.T, dbHandler *db.DBHandler, ctx context.Context, appName string) []*db.DBReleaseWithMetaData {
	release, err := db.WithTransactionMultipleEntriesT(dbHandler, ctx, true, func(ctx context.Context, transaction *sql.Tx) ([]*db.DBReleaseWithMetaData, error) {
		return dbHandler.DBSelectReleasesByAppLatestEslVersion(ctx, transaction, appName, true)
	})
	if err != nil {
		t.Fatalf("DBSelectReleasesByApp failed %s", err)
	}
	return release
}

func callDBForDeployments(t *testing.T, dbHandler *db.DBHandler, ctx context.Context, appName string) *db.Deployment {
	deployment, err := db.WithTransactionT(dbHandler, ctx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*db.Deployment, error) {
		return dbHandler.DBSelectLatestDeployment(ctx, transaction, appName, "development")
	})
	if err != nil {
		t.Fatalf("DBSelectLatestDeployment failed %s", err)
	}
	return deployment
}

func TestAppParameter(t *testing.T) {
	testCases := []struct {
		name                string
		inputNumberAppParam int
		inputVersion        int
		expectedStatusCode  int
		expectedError       error
		expectedBody        string
	}{
		{
			name:                "0 app names",
			inputNumberAppParam: 0,
			inputVersion:        98,
			expectedStatusCode:  400,
			expectedBody:        "Must provide application name",
		},
		{
			name:                "1 app name",
			inputNumberAppParam: 1,
			inputVersion:        99,
			expectedStatusCode:  201,
			expectedBody:        "{\"Success\":{}}\n",
		},
		// having multiple app names would be a bit harder to test
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			values := map[string]io.Reader{}
			for i := 0; i < tc.inputNumberAppParam; i++ {
				values["application"] = strings.NewReader("app1")
			}
			values["version"] = strings.NewReader(strconv.Itoa(tc.inputVersion))

			files := map[string]io.Reader{}
			files["manifests[dev]"] = strings.NewReader("manifest")

			actualStatusCode, actualBody, err := callRelease(values, files, "/api/release")
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			if actualStatusCode != tc.expectedStatusCode {
				t.Errorf("expected code %v but got %v", tc.expectedStatusCode, actualStatusCode)
			}
			if diff := cmp.Diff(tc.expectedBody, actualBody); diff != "" {
				t.Errorf("response body mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func createValuesFiles(t *testing.T, appName, version string) (map[string]io.Reader, map[string]io.Reader) {
	values := map[string]io.Reader{
		"application": strings.NewReader(appName),
		"version":     strings.NewReader(version),
	}
	files := map[string]io.Reader{
		"manifests[development]": strings.NewReader("Test Manifest"),
	}
	return values, files
}

func TestManifestParameterMissing(t *testing.T) {
	testCases := []struct {
		name               string
		expectedStatusCode int
		expectedBody       string
	}{
		{
			name:               "missing manifest",
			expectedStatusCode: 400,
			expectedBody:       "No manifest files provided",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			values := map[string]io.Reader{}
			values["application"] = strings.NewReader("app1")

			files := map[string]io.Reader{}

			actualStatusCode, actualBody, err := callRelease(values, files, "/api/release")

			if err != nil {
				t.Errorf("form error %s", err.Error())
			}

			if actualStatusCode != tc.expectedStatusCode {
				t.Errorf("expected code %v but got %v", tc.expectedStatusCode, actualStatusCode)
			}
			if diff := cmp.Diff(tc.expectedBody, actualBody); diff != "" {
				t.Errorf("response body mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestServeHttpInvalidInput(t *testing.T) {
	tcs := []struct {
		Name           string
		ExpectedStatus int
		ExpectedBody   string
		FormMetaData   string
	}{{
		Name:           "Error when no boundary provided",
		ExpectedStatus: 400,
		ExpectedBody:   "Invalid body: no multipart boundary param in Content-Type",
		FormMetaData:   "multipart/form-data;",
	}, {
		Name:           "Error when no content provided",
		ExpectedStatus: 400,
		ExpectedBody:   "Invalid body: multipart: NextPart: EOF",
		FormMetaData:   "multipart/form-data;boundary=nonExistantBoundary;",
	}}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			body := multipart.NewWriter(&buf)
			body.Close()

			if resp, err := http.Post("http://localhost:"+frontendPort+"/release", tc.FormMetaData, &buf); err != nil {
				t.Logf("response failure %s", err.Error())
				t.Fatal(err)
			} else {
				t.Logf("response: %v", resp.StatusCode)
				if resp.StatusCode != tc.ExpectedStatus {
					t.Fatalf("expected http status %d, received %d", tc.ExpectedStatus, resp.StatusCode)
				}
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(tc.ExpectedBody, string(bodyBytes)); diff != "" {
					t.Errorf("response body mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func TestServeHttpBasics(t *testing.T) {
	noCachingHeader := "no-cache,no-store,must-revalidate,max-age=0"
	yesCachingHeader := "max-age=604800"
	headerMapWithoutCaching := map[string]string{
		"Cache-Control": noCachingHeader,
	}
	headerMapWithCaching := map[string]string{
		"Cache-Control": yesCachingHeader,
	}

	var jsPath = ""
	var cssPath = ""
	{
		// find index.html to figure out what the name of the css and js files are:
		resp, err := http.Get("http://localhost:" + frontendPort + "/")
		if err != nil {
			t.Logf("response failure %s", err.Error())
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected http status %d, received %d", 200, resp.StatusCode)
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		bodyString := string(bodyBytes)

		prefixJs := "/static/js/main."
		afterJs1 := strings.SplitAfter(bodyString, prefixJs)
		afterJs2 := strings.SplitAfter(afterJs1[1], ".js")
		jsPath = prefixJs + afterJs2[0]

		prefixCss := "/static/css/main."
		afterCss1 := strings.SplitAfter(bodyString, prefixCss)
		afterCss2 := strings.SplitAfter(afterCss1[1], ".css")
		cssPath = prefixCss + afterCss2[0]
	}

	tcs := []struct {
		Name            string
		Endpoint        string
		ExpectedStatus  int
		ExpectedHeaders map[string]string
	}{
		{
			Name:            "Http works and returns caching headers for root",
			Endpoint:        "/",
			ExpectedStatus:  200,
			ExpectedHeaders: headerMapWithoutCaching,
		},
		{
			Name:            "Http works and returns caching headers for /index.html",
			Endpoint:        "/index.html",
			ExpectedStatus:  200,
			ExpectedHeaders: headerMapWithoutCaching,
		},
		{
			Name:            "Http works and returns caching headers for /ui",
			Endpoint:        "/ui",
			ExpectedStatus:  200,
			ExpectedHeaders: headerMapWithoutCaching,
		},
		{
			Name:            "Http works and returns correct headers for js",
			Endpoint:        jsPath,
			ExpectedStatus:  200,
			ExpectedHeaders: headerMapWithCaching,
		},
		{
			Name:            "Http works and returns correct headers for css",
			Endpoint:        cssPath,
			ExpectedStatus:  200,
			ExpectedHeaders: headerMapWithCaching,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			body := multipart.NewWriter(&buf)
			body.Close()

			if resp, err := http.Get("http://localhost:" + frontendPort + tc.Endpoint); err != nil {
				t.Logf("response failure %s", err.Error())
				t.Fatal(err)
			} else {
				t.Logf("response: %v", resp.StatusCode)
				if resp.StatusCode != tc.ExpectedStatus {
					t.Fatalf("expected http status %d, received %d", tc.ExpectedStatus, resp.StatusCode)
				}

				for key := range tc.ExpectedHeaders {
					expectedValue, _ := tc.ExpectedHeaders[key]
					actualValue := resp.Header.Get(key)
					if expectedValue != actualValue {
						t.Fatalf("Http header with key %v: Expected %v but got %v", key, expectedValue, actualValue)
					}
				}

				_, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestSignatures(t *testing.T) {
	theManifest := "I am a manifest\n- foo\nfoo"

	testCases := []struct {
		name               string
		inputApp           string
		inputManifest      string
		inputSignature     string
		inputManifestEnv   string
		inputSignatureEnv  string
		inputVersion       string
		expectedStatusCode int
	}{
		{
			name:               "Correct signature",
			inputApp:           "test-signatures-app" + appSuffix,
			inputManifest:      theManifest,
			inputSignature:     CalcSignature(t, theManifest),
			inputManifestEnv:   devEnv,
			inputSignatureEnv:  devEnv,
			inputVersion:       "1",
			expectedStatusCode: 201,
		},
		{
			name:               "invalid signature",
			inputApp:           "test-signatures-failing-app" + appSuffix,
			inputManifest:      theManifest,
			inputSignature:     "not valid!",
			inputManifestEnv:   devEnv,
			inputSignatureEnv:  devEnv,
			inputVersion:       "3",
			expectedStatusCode: 400,
		},
		{
			name:               "Valid signature, but at the wrong place",
			inputApp:           "test-signatures-failing-app" + appSuffix,
			inputManifest:      theManifest,
			inputSignature:     CalcSignature(t, theManifest),
			inputManifestEnv:   devEnv,
			inputSignatureEnv:  stageEnv, // !!
			inputVersion:       "4",
			expectedStatusCode: 400,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			values := map[string]io.Reader{
				"application": strings.NewReader(tc.inputApp),
			}
			values["version"] = strings.NewReader(tc.inputVersion)
			files := map[string]io.Reader{
				"manifests[" + tc.inputManifestEnv + "]":   strings.NewReader(tc.inputManifest),
				"signatures[" + tc.inputSignatureEnv + "]": strings.NewReader(tc.inputSignature),
			}

			actualStatusCode, body, err := callRelease(values, files, "/release")
			if err != nil {
				t.Fatalf("callRelease failed: %s", err.Error())
			}

			if actualStatusCode != tc.expectedStatusCode {
				t.Errorf("expected code %v but got %v. Body: %s", tc.expectedStatusCode, actualStatusCode, body)
			}
		})
	}
}
