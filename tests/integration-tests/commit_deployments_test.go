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
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	json "google.golang.org/protobuf/encoding/protojson"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
)

const (
	development = "development2"
	staging     = "staging"
)

func TestCommitDeployments(t *testing.T) {
	for _, tc := range []struct {
		name           string
		apps           []string
		releaseVersion string
		commitId       string
		expectedStatus map[string]api.CommitDeploymentStatus
	}{
		{
			name:           "Running commit deployments",
			apps:           []string{"commit-deployments-test-app-1", "commit-deployments-test-app-2"},
			releaseVersion: "1",
			commitId:       "1234567890123456789012345678901234567890",
			expectedStatus: map[string]api.CommitDeploymentStatus{
				development: api.CommitDeploymentStatus_DEPLOYED,
				staging:     api.CommitDeploymentStatus_PENDING,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Create the applications
			for _, app := range tc.apps {
				values := map[string]io.Reader{
					"application":      strings.NewReader(app),
					"version":          strings.NewReader(tc.releaseVersion),
					"source_commit_id": strings.NewReader(tc.commitId),
				}
				files := map[string]io.Reader{
					fmt.Sprintf("manifests[%s]", development): strings.NewReader("Test Manifest"),
					fmt.Sprintf("manifests[%s]", staging):     strings.NewReader("Test Manifest"),
				}
				statusCode, body, err := callRelease(values, files, "/api/release")
				if err != nil {
					t.Fatalf("Error creating app: %v", err)
				}
				if statusCode != http.StatusCreated {
					t.Fatalf("Error creating app: %s", body)
				}

				// Get deployment status for development
				status, err := getAppEnvDeploymentStatus(app, development, tc.commitId)
				if err != nil {
					t.Fatalf("Error getting deployments: %v", err)
				}
				if status != tc.expectedStatus[development] {
					t.Errorf("Deployment status for %s on %s is not as expected\nexpected: %v, got: %v", app, development, tc.expectedStatus[development], status)
				}
				// Get deployment status for staging
				status, err = getAppEnvDeploymentStatus(app, staging, tc.commitId)
				if err != nil {
					t.Fatalf("Error getting deployments: %v", err)
				}
				if status != tc.expectedStatus[staging] {
					t.Errorf("Deployment status for %s on %s is not as expected\nexpected: %v, got: %v", app, development, tc.expectedStatus[staging], status)
				}

				// Run release train to staging
				resp, err := runReleaseTrain(staging)
				if err != nil {
					t.Fatalf("Error running release train: %v", err)
				}
				if resp.StatusCode != http.StatusOK {
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						t.Fatalf("Release train failed with status %v", resp.StatusCode)
					}
					t.Fatalf("Release train failed with status %v:\n%v", resp.StatusCode, string(body))
				}
				// Staging should be deployed
				status, err = getAppEnvDeploymentStatus(app, staging, tc.commitId)
				if err != nil {
					t.Fatalf("Error getting deployments: %v", err)
				}
				if status != api.CommitDeploymentStatus_DEPLOYED {
					t.Errorf("Deployment status for %s on %s is not as expected\nexpected: %v, got: %v", app, staging, api.CommitDeploymentStatus_DEPLOYED, status)
				}
			}
		})
	}
}

func getAppEnvDeploymentStatus(app, env, commit string) (api.CommitDeploymentStatus, error) {
	resp, err := http.Get("http://localhost:8081/api/commit-deployments/" + commit)
	if err != nil {
		return api.CommitDeploymentStatus_UNKNOWN, fmt.Errorf("Error getting commit deployments: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return api.CommitDeploymentStatus_UNKNOWN, fmt.Errorf("Error reading response body: %v", err)
	}
	var status api.GetCommitDeploymentInfoResponse
	err = json.Unmarshal(body, &status)
	if err != nil {
		return api.CommitDeploymentStatus_UNKNOWN, fmt.Errorf("Error unmarshalling response body: %v", err)
	}
	appStatus, ok := status.DeploymentStatus[app]
	if !ok {
		return api.CommitDeploymentStatus_UNKNOWN, nil
	}
	appEnvStatus, ok := appStatus.DeploymentStatus[env]
	if !ok {
		return api.CommitDeploymentStatus_UNKNOWN, nil
	}
	return appEnvStatus, nil
}

func runReleaseTrain(env string) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:8081/api/environments/%s/releasetrain", env)
	request, err := http.NewRequestWithContext(context.Background(), "PUT", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %v", err)
	}
	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Error running release train: %v", err)
	}
	return resp, nil
}
