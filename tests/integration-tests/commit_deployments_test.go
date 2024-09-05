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
	"github.com/freiheit-com/kuberpult/pkg/db"
)

const (
	development = "development2"
	staging     = "staging"
)

func TestCommitDeployments(t *testing.T) {
	ctx := context.Background()
	dbConfig := db.DBConfig{
		DbHost:         "localhost",
		DbPort:         "5432",
		DbUser:         "postgres",
		DbPassword:     "mypassword",
		DbName:         "kuberpult",
		DriverName:     "postgres",
		MigrationsPath: "../../database/migrations/postgres",
		SSLMode:        "disable",
	}
	dbHandler, err := db.Connect(ctx, dbConfig)
	if err != nil {
		t.Fatalf("Error establishing DB connection: %v", err)
	}
	pErr := dbHandler.DB.Ping()
	if pErr != nil {
		t.Fatalf("Error pinging database: %v", err)
	}

	for _, tc := range []struct {
		name           string
		apps           []string
		releaseVersion string
		commitId       string
		expectedStatus map[string]api.CommitDeploymentStatus
	}{
		{
			name:           "Running commit deployments",
			apps:           []string{"commit-deployments-test-app1", "commit-deployments-test-app2"},
			releaseVersion: "1",
			commitId:       "1234567890123456789012345678901234567890",
			expectedStatus: map[string]api.CommitDeploymentStatus{
				development: api.CommitDeploymentStatus_DEPLOYED,
				staging:     api.CommitDeploymentStatus_PENDING,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Create the application releases
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
			}
			for _, app := range tc.apps {
				status, err := getAppEnvDeploymentStatus(app, development, tc.commitId)
				if err != nil {
					t.Fatalf("Error getting deployments: %v", err)
				}
				if status != tc.expectedStatus[development] {
					t.Errorf("Deployment status for %s on %s is not as expected\nexpected: %v, got: %v", app, development, tc.expectedStatus[development], status)
				}
				status, err = getAppEnvDeploymentStatus(app, staging, tc.commitId)
				if err != nil {
					t.Fatalf("Error getting deployments: %v", err)
				}
				if status != tc.expectedStatus[staging] {
					t.Errorf("Deployment status for %s on %s is not as expected\nexpected: %v, got: %v", app, development, tc.expectedStatus[staging], status)
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
