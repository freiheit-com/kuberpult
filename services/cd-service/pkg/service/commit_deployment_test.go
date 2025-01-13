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

package service

import (
	"reflect"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
)

func TestGetCommitReleaseNumber(t *testing.T) {
	tcs := []struct {
		name      string
		eventJson []byte
		expected  uint64
	}{
		{
			name:      "ReleaseVersion doesn't exist in metadata",
			eventJson: []byte(`{"EventData":{"Environments":{"development":{},"staging":{}}},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000000","EventType":"new-release"}}`),
			expected:  0,
		},
		{
			name:      "ReleaseVersion exists in metadata",
			eventJson: []byte(`{"EventData":{"Environments":{"development":{},"staging":{}}},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000000","EventType":"new-release","ReleaseVersion":12}}`),
			expected:  12,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			releaseVersion, err := getCommitReleaseNumber(tc.eventJson)
			if err != nil {
				t.Fatalf("Error getting release version: %v", err)
			}
			if releaseVersion != tc.expected {
				t.Fatalf("Expected %d, got %d", tc.expected, releaseVersion)
			}
		})
	}
}

func TestGetCommitStatus(t *testing.T) {
	tcs := []struct {
		name                string
		releaseNumber       uint64
		allEnvironments     []string
		environmentReleases map[string]uint64
		expectedStatus      CommitStatus
	}{
		{
			name:                "One environment with newer release",
			releaseNumber:       1,
			allEnvironments:     []string{"dev"},
			environmentReleases: map[string]uint64{"dev": 2},
			expectedStatus: CommitStatus{
				"dev": api.CommitDeploymentStatus_DEPLOYED,
			},
		},
		{
			name:                "One environment with older release",
			releaseNumber:       2,
			allEnvironments:     []string{"dev"},
			environmentReleases: map[string]uint64{"dev": 1},
			expectedStatus: CommitStatus{
				"dev": api.CommitDeploymentStatus_PENDING,
			},
		},
		{
			name:                "One environment with same release",
			releaseNumber:       1,
			allEnvironments:     []string{"dev"},
			environmentReleases: map[string]uint64{"dev": 1},
			expectedStatus: CommitStatus{
				"dev": api.CommitDeploymentStatus_DEPLOYED,
			},
		},
		{
			name:                "Multiple environments with different releases",
			releaseNumber:       2,
			allEnvironments:     []string{"dev", "staging", "prod"},
			environmentReleases: map[string]uint64{"dev": 3, "staging": 2, "prod": 1},
			expectedStatus: CommitStatus{
				"dev":     api.CommitDeploymentStatus_DEPLOYED,
				"staging": api.CommitDeploymentStatus_DEPLOYED,
				"prod":    api.CommitDeploymentStatus_PENDING,
			},
		},
		{
			name:                "Commit not deployed to all environments",
			releaseNumber:       2,
			allEnvironments:     []string{"dev", "staging", "prod", "qa"},
			environmentReleases: map[string]uint64{"dev": 3, "staging": 2, "prod": 1},
			expectedStatus: CommitStatus{
				"dev":     api.CommitDeploymentStatus_DEPLOYED,
				"staging": api.CommitDeploymentStatus_DEPLOYED,
				"prod":    api.CommitDeploymentStatus_PENDING,
				"qa":      api.CommitDeploymentStatus_PENDING,
			},
		},
		{
			name:                "Commit is not deployed anywhere",
			releaseNumber:       2,
			allEnvironments:     []string{"dev", "staging", "prod", "qa"},
			environmentReleases: map[string]uint64{},
			expectedStatus: CommitStatus{
				"dev":     api.CommitDeploymentStatus_PENDING,
				"staging": api.CommitDeploymentStatus_PENDING,
				"prod":    api.CommitDeploymentStatus_PENDING,
				"qa":      api.CommitDeploymentStatus_PENDING,
			},
		},
		{
			name:                "Release number is 0",
			releaseNumber:       0,
			allEnvironments:     []string{"dev", "staging", "prod", "qa"},
			environmentReleases: map[string]uint64{"dev": 3, "staging": 2, "prod": 1},
			expectedStatus: CommitStatus{
				"dev":     api.CommitDeploymentStatus_UNKNOWN,
				"staging": api.CommitDeploymentStatus_UNKNOWN,
				"prod":    api.CommitDeploymentStatus_UNKNOWN,
				"qa":      api.CommitDeploymentStatus_UNKNOWN,
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status := getCommitStatus(tc.releaseNumber, tc.environmentReleases, tc.allEnvironments)
			if !reflect.DeepEqual(status, tc.expectedStatus) {
				t.Fatalf("Expected %v, got %v", tc.expectedStatus, status)
			}
		})
	}
}
