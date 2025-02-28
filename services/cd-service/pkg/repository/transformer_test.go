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

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/freiheit-com/kuberpult/pkg/testutil"
	time2 "github.com/freiheit-com/kuberpult/pkg/time"

	"github.com/freiheit-com/kuberpult/pkg/db"

	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/DataDog/datadog-go/v5/statsd"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/testfs"
	"github.com/google/go-cmp/cmp"
)

const (
	envAcceptance      = "acceptance"
	envProduction      = "production"
	additionalVersions = 7
)

var timeNowOld = time.Date(1999, 01, 02, 03, 04, 05, 0, time.UTC)

func TestUndeployApplicationErrors(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedError     *TransformerBatchApplyError
		expectedCommitMsg string
	}{
		{
			Name: "Delete non-existent application",
			Transformers: []Transformer{
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            0,
				TransformerError: errMatcher{"UndeployApplication: error cannot undeploy non-existing application 'app1'"},
			},
			expectedCommitMsg: "",
		},
		{
			Name: "Success",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedCommitMsg: "application 'app1' was deleted successfully",
		},
		{
			Name: "Create un-deploy Version for un-deployed application should not work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&UndeployApplication{
					Application: "app1",
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            4,
				TransformerError: errMatcher{"cannot undeploy non-existing application 'app1'"},
			},
			expectedCommitMsg: "",
		},
		{
			Name: "Undeploy application where there is an application lock should not work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateEnvironmentApplicationLock{
					Environment: "acceptance",
					Application: "app1",
					LockId:      "22133",
					Message:     "test",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedCommitMsg: "application 'app1' was deleted successfully",
		},
		{
			Name: "Undeploy application where there is an application lock created after the un-deploy version creation should",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateEnvironmentApplicationLock{
					Environment: "acceptance",
					Application: "app1",
					LockId:      "22133",
					Message:     "test",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedCommitMsg: "application 'app1' was deleted successfully",
		},
		{
			Name: "Undeploy application where there current releases are not undeploy shouldn't work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateEnvironmentLock{
					Environment: "acceptance",
					LockId:      "22133",
					Message:     "test",
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            4,
				TransformerError: errMatcher{"UndeployApplication(db): error cannot un-deploy application 'app1' the current release 'acceptance' is not un-deployed"},
			},
			expectedCommitMsg: "",
		},
		{
			Name: "Undeploy application where the app does not have a release in all envs must work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironment{
					Environment: "production",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedCommitMsg: "application 'app1' was deleted successfully",
		},
		{
			Name: "Undeploy application where there is an environment lock should work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateEnvironmentLock{
					Environment: "acceptance",
					LockId:      "22133",
					Message:     "test",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedCommitMsg: "application 'app1' was deleted successfully",
		},
		{
			Name: "Undeploy application where the last release is not Undeploy shouldn't work",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateApplicationVersion{
					Application:     "app1",
					Manifests:       nil,
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					WriteCommitData: true,
					Version:         3,
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            3,
				TransformerError: errMatcher{"UndeployApplication: error last release is not un-deployed application version of 'app1'"},
			},
			expectedCommitMsg: "",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			commitMsgPtr, _ := db.WithTransactionT(repo.State().DBHandler, ctx, 0, false, func(ctx context.Context, transaction *sql.Tx) (*[]string, error) {
				commitMsg, _, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
					return nil, nil
				}
				return &commitMsg, nil
			})
			commitMsg := *commitMsgPtr
			actualMsg := ""
			if len(commitMsg) > 0 {
				actualMsg = commitMsg[len(commitMsg)-1]
			}

			if diff := cmp.Diff(tc.expectedCommitMsg, actualMsg); diff != "" {
				t.Errorf("commit message mismatch (-want, +got):\n%s", diff)
			}

		})
	}
}

func TestCreateApplicationVersionIdempotency(t *testing.T) {
	tcs := []struct {
		Name          string
		Transformers  []Transformer
		expectedError *TransformerBatchApplyError
	}{
		{
			Name: "recreate same version with idempotence",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Version:     10000,
					Manifests: map[string]string{
						envAcceptance: "{}",
					},
					WriteCommitData: true,
				},
				&CreateApplicationVersion{
					Application: "app1",
					Version:     10000,
					Manifests: map[string]string{
						envAcceptance: "{}",
					},
					WriteCommitData: true,
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            2,
				TransformerError: errMatcher{"already_exists_same:{}"},
			},
		},
		{
			Name: "recreate same version without idempotence",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Version:     10000,
					Manifests: map[string]string{
						envAcceptance: `{}`,
					},
					WriteCommitData: true,
				},
				&CreateApplicationVersion{
					Application: "app1",
					Version:     10000,
					Manifests: map[string]string{
						envAcceptance: `{ "different": "yes" }`,
					},
					WriteCommitData: true,
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index: 2,
				TransformerError: &CreateReleaseError{
					response: api.CreateReleaseResponse{
						Response: &api.CreateReleaseResponse_AlreadyExistsDifferent{
							AlreadyExistsDifferent: &api.CreateReleaseResponseAlreadyExistsDifferent{
								FirstDifferingField: api.DifferingField_MANIFESTS,
								Diff:                "--- acceptance-existing\n+++ acceptance-request\n@@ -1 +1 @@\n-{}\n\\ No newline at end of file\n+{ \"different\": \"yes\" }\n\\ No newline at end of file\n",
							},
						},
					},
				},
			},
		},
		{
			Name: "recreate same version with idempotence, but different formatting of yaml",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Version:     10000,
					Manifests: map[string]string{
						envAcceptance: `{ "different":                  "yes" }`,
					},
					WriteCommitData: true,
				},
				&CreateApplicationVersion{
					Application: "app1",
					Version:     10000,
					Manifests: map[string]string{
						envAcceptance: `{ "different": "yes" }`,
					},
					WriteCommitData: true,
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index: 2,
				TransformerError: &CreateReleaseError{
					response: api.CreateReleaseResponse{
						Response: &api.CreateReleaseResponse_AlreadyExistsSame{
							AlreadyExistsSame: &api.CreateReleaseResponseAlreadyExistsSame{},
						},
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctxWithTime := time2.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			t.Parallel()

			// optimization: no need to set up the repository if this fails
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err := repo.ApplyTransformersInternal(ctxWithTime, transaction, tc.Transformers...)
				return err
			})

			if err == nil {
				t.Fatalf("expected error, got none.")
			}
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestApplicationDeploymentEvent(t *testing.T) {
	type TestCase struct {
		Name             string
		Transformers     []Transformer
		db               bool
		expectedDBEvents []db.EventRow // the events that the last transformer created
	}

	tcs := []TestCase{
		{
			Name: "Create a single application version without deploying it",
			// no need to bother with environments here
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[string]string{
						"staging": "doesn't matter",
					},
					WriteCommitData: true,
					Version:         1,
				},
			},
			expectedDBEvents: []db.EventRow{
				{
					Uuid:          "00000000-0000-0000-0000-000000000001",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					EventType:     "new-release",
					EventJson:     "{}",
					TransformerID: 1,
				},
			},
		},
		{
			Name: "Create a single application version and deploy it",
			// no need to bother with environments here
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[string]string{
						"staging": "doesn't matter",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Application:     "app",
					Environment:     "staging",
					WriteCommitData: true,
					Version:         1,
				},
			},
			expectedDBEvents: []db.EventRow{
				{
					Uuid:          "00000000-0000-0000-0000-000000000002",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					EventType:     "deployment",
					EventJson:     "{}",
					TransformerID: 2,
				},
			},
		},
		{
			Name: "Trigger a deployment via a release train with environment target",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "staging",
						},
					},
				},
				&CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "staging",
							Latest:      true,
						},
					},
				},
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					Manifests: map[string]string{
						"production": "some production manifest 2",
						"staging":    "some staging manifest 2",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:     "staging",
					Application:     "app",
					Version:         1,
					WriteCommitData: true,
				},
				&ReleaseTrain{
					Target:          "production",
					WriteCommitData: true,
				},
			},
			expectedDBEvents: []db.EventRow{
				{
					Uuid:       "00000000-0000-0000-0000-000000000005",
					CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					EventType:  "deployment",
					//EventJson:     "{}",
					TransformerID: 5,
				},
			},
		},
		{
			Name: "Trigger a deployment via a release train with environment group target without lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "staging",
						},
						EnvironmentGroup: conversion.FromString("production-group"),
					},
				},
				&CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "staging",
							Latest:      true,
						},
					},
				},
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					Manifests: map[string]string{
						"production": "some production manifest 2",
						"staging":    "some staging manifest 2",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:     "staging",
					Application:     "app",
					Version:         1,
					WriteCommitData: true,
				},
				&ReleaseTrain{
					Target:          "production-group",
					WriteCommitData: true,
				},
			},
			expectedDBEvents: []db.EventRow{
				{
					Uuid:          "00000000-0000-0000-0000-000000000005",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					EventType:     "deployment",
					EventJson:     "{}",
					TransformerID: 5,
				},
			},
		},
		{
			Name: "Trigger a deployment via a release train with environment group target with lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "staging",
						},
						EnvironmentGroup: conversion.FromString("production-group"),
					},
				},
				&CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "staging",
							Latest:      true,
						},
					},
				},
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					Manifests: map[string]string{
						"production": "some production manifest 2",
						"staging":    "some staging manifest 2",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:     "staging",
					Application:     "app",
					Version:         1,
					WriteCommitData: true,
				},
				&CreateEnvironmentLock{
					Environment: "production",
					LockId:      "lock id 1",
					Message:     "lock msg 1",
				},
				&ReleaseTrain{
					Target:          "production-group",
					WriteCommitData: true,
				},
			},
			expectedDBEvents: []db.EventRow{
				{
					Uuid:          "00000000-0000-0000-0000-000000000005",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					EventType:     "lock-prevented-deployment",
					EventJson:     "{}",
					TransformerID: 6,
				},
			},
		},
		{
			Name: "Block deployments using env lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "dev",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&CreateEnvironmentLock{
					Environment: "staging",
					LockId:      "lock1",
					Message:     "lock staging",
				},
				&CreateApplicationVersion{
					Application:    "myapp",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					Manifests: map[string]string{
						"dev":     "some dev manifest",
						"staging": "some staging manifest",
					},
					WriteCommitData: true,
					Version:         3,
				},
			},
			expectedDBEvents: []db.EventRow{
				{
					Uuid:          "00000000-0000-0000-0000-000000000001",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					EventType:     "new-release",
					EventJson:     "{}",
					TransformerID: 4,
				},
				{
					Uuid:          "00000000-0000-0000-0000-000000000002",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					EventType:     "deployment",
					EventJson:     "{}",
					TransformerID: 4,
				},
				{
					Uuid:          "00000000-0000-0000-0000-000000000003",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					EventType:     "lock-prevented-deployment",
					EventJson:     "{}",
					TransformerID: 4,
				},
			},
		},
		{
			Name: "Block deployments using app lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "dev",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				//				&CreateEnvironmentLock{
				//					Environment: "staging",
				//					LockId:      "lock1",
				//					Message:     "lock staging",
				//				},
				&CreateEnvironmentApplicationLock{
					Environment: "staging",
					Application: "myapp",
					LockId:      "lock2",
					Message:     "lock myapp",
				},
				&CreateApplicationVersion{
					Application:    "myapp",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					Manifests: map[string]string{
						"dev":     "some dev manifest",
						"staging": "some staging manifest",
					},
					WriteCommitData: true,
					Version:         4,
				},
			},
			expectedDBEvents: []db.EventRow{
				{
					Uuid:          "00000000-0000-0000-0000-000000000001",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					EventType:     "new-release",
					EventJson:     "{}",
					TransformerID: 4,
				},
				{
					Uuid:          "00000000-0000-0000-0000-000000000002",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					EventType:     "deployment",
					EventJson:     "{}",
					TransformerID: 4,
				},
				{
					Uuid:          "00000000-0000-0000-0000-000000000003",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					EventType:     "lock-prevented-deployment",
					EventJson:     "{}",
					TransformerID: 4,
				},
			},
		},
		{
			Name: "Block deployments using Team lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "dev",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&CreateApplicationVersion{ //Create the team
					Application:    "someapp",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaf",
					Manifests: map[string]string{
						"dev": "some dev manifest",
					},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         5,
				},
				&CreateEnvironmentTeamLock{
					Environment: "dev",
					Team:        "sre-team",
					LockId:      "lock2",
					Message:     "lock sreteam",
				},
				&CreateApplicationVersion{
					Application:    "myapp",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					Manifests: map[string]string{
						"dev": "some dev manifest",
					},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         6,
				},
			},
			expectedDBEvents: []db.EventRow{
				{
					Uuid:          "00000000-0000-0000-0000-000000000004",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					EventType:     "new-release",
					EventJson:     "{}",
					TransformerID: 4,
				},
				{
					Uuid:          "00000000-0000-0000-0000-000000000005",
					CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
					EventType:     "lock-prevented-deployment",
					EventJson:     "{}",
					TransformerID: 4,
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			tc := tc
			t.Parallel()

			fakeGen := testutil.NewIncrementalUUIDGenerator()
			ctx := testutil.MakeTestContext()
			ctx = AddGeneratorToContext(ctx, fakeGen)
			var err error = nil
			repo, dbHandler := SetupRepositoryTestWithDBOptions(t, false)
			var lastTransformerId db.TransformerID = -1
			for index, transformer := range tc.Transformers {
				_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
					var batchError *TransformerBatchApplyError = nil
					_, _, _, batchError = repo.ApplyTransformersInternal(ctx, tx, transformer)
					if batchError != nil {
						t.Fatalf("encountered error but no error is expected here: %d '%v'", index, batchError)
					}
					lastTransformerId = transformer.GetEslVersion()
					return nil
				})
			}

			t.Logf("last Transformer id: %v", lastTransformerId)

			commitEvents, _ := db.WithTransactionT[[]db.EventRow](dbHandler, ctx, 0, true, func(ctx context.Context, tx *sql.Tx) (*[]db.EventRow, error) {
				events, err := dbHandler.DBSelectAllCommitEventsForTransformerID(ctx, tx, lastTransformerId)
				if err != nil {
					t.Fatalf("2 encountered error but no error is expected here: '%v'", err)
				}
				return &events, nil
			})

			if err != nil {
				t.Fatalf("encountered error but no error is expected here: '%v'", err)
			}

			if diff := cmp.Diff(tc.expectedDBEvents, *commitEvents, cmpopts.IgnoreFields(db.EventRow{}, "Timestamp", "EventJson")); diff != "" {
				t.Errorf("result mismatch (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(len(tc.expectedDBEvents), len(*commitEvents)); diff != "" {
				t.Errorf("result mismatch in number of events (-want, +got):\n%s", diff)
			}

			//fs := updatedState.Filesystem
			//if err := verifyContent(fs, tc.expectedContent); err != nil {
			//	t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(fs), "\n"))
			//}
		})
	}
}

// Tests various error cases in the prepare-Undeploy endpoint, specifically the error messages returned.
func TestUndeployErrors(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedError     *TransformerBatchApplyError
		expectedCommitMsg string
	}{
		{
			Name: "Access non-existent application",
			Transformers: []Transformer{
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            0,
				TransformerError: errMatcher{"cannot undeploy non-existing application 'app1'"},
			},
			expectedCommitMsg: "",
		},
		{
			Name: "Success",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
			},
			expectedCommitMsg: "created undeploy-version 2 of 'app1'",
		},
		{
			Name: "Deploy after Undeploy should work",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateApplicationVersion{
					Application:     "app1",
					Manifests:       nil,
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					WriteCommitData: true,
					Version:         3,
				},
			},
			expectedCommitMsg: "created version 3 of \"app1\"",
		},
		{
			Name: "Undeploy twice should succeed",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
			},
			expectedCommitMsg: "created undeploy-version 3 of 'app1'",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			_ = r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				commitMsg, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}

				actualMsg := ""
				if len(commitMsg) > 0 {
					actualMsg = commitMsg[len(commitMsg)-1]
				}
				if diff := cmp.Diff(tc.expectedCommitMsg, actualMsg); diff != "" {
					t.Errorf("commit message mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})

		})
	}
}

// Tests various error cases in the release train, specifically the error messages returned.
func TestReleaseTrainErrors(t *testing.T) {
	tcs := []struct {
		Name              string
		Setup             []Transformer
		ReleaseTrain      ReleaseTrain
		expectedError     *TransformerBatchApplyError
		expectedPrognosis ReleaseTrainPrognosis
		expectedCommitMsg string
	}{
		{
			Name:  "Access non-existent environment",
			Setup: []Transformer{},
			ReleaseTrain: ReleaseTrain{
				Target: "doesnotexistenvironment",
			},
			expectedError: &TransformerBatchApplyError{
				Index: 0,
				TransformerError: status.Error(
					codes.InvalidArgument,
					"error: could not find environment group or environment configs for 'doesnotexistenvironment'",
				),
			},
			expectedPrognosis: ReleaseTrainPrognosis{
				Error: status.Error(
					codes.InvalidArgument,
					"error: could not find environment group or environment configs for 'doesnotexistenvironment'",
				),
				EnvironmentPrognoses: nil,
			},
			expectedCommitMsg: "",
		},
		{
			Name: "Environment is locked - but train continues in other env",
			Setup: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance + "-de",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						EnvironmentGroup: conversion.FromString(envAcceptance),
					},
				},
				&CreateEnvironment{
					Environment: envAcceptance + "-ca",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						EnvironmentGroup: conversion.FromString(envAcceptance),
					},
				},
				&CreateEnvironmentLock{
					Environment: envAcceptance + "-ca",
					Message:     "mA",
					LockId:      "IdA",
				},
				&CreateEnvironmentLock{
					Environment: envAcceptance + "-de",
					Message:     "mB",
					LockId:      "IdB",
				},
			},
			ReleaseTrain: ReleaseTrain{
				Target: envAcceptance,
			},
			expectedPrognosis: ReleaseTrainPrognosis{
				Error: nil,
				EnvironmentPrognoses: map[string]ReleaseTrainEnvironmentPrognosis{
					"acceptance-ca": {
						SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
							SkipCause: api.ReleaseTrainEnvSkipCause_ENV_IS_LOCKED,
						},
						Error:         nil,
						AppsPrognoses: map[string]ReleaseTrainApplicationPrognosis{},
						Locks: []*api.Lock{
							{
								Message:   "mA",
								LockId:    "IdA",
								CreatedAt: timestamppb.Now(),
								CreatedBy: &api.Actor{
									Email: "testmail@example.com",
									Name:  "test tester",
								},
							},
						},
					},
					"acceptance-de": {
						SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
							SkipCause: api.ReleaseTrainEnvSkipCause_ENV_IS_LOCKED,
						},
						Error:         nil,
						AppsPrognoses: map[string]ReleaseTrainApplicationPrognosis{},
						Locks: []*api.Lock{
							{
								Message:   "mB",
								LockId:    "IdB",
								CreatedAt: timestamppb.Now(),
								CreatedBy: &api.Actor{
									Email: "testmail@example.com",
									Name:  "test tester",
								},
							},
						},
					},
				},
			},
			expectedCommitMsg: `Release Train to environment/environment group 'acceptance':

Target Environment 'acceptance-ca' is locked - skipping.
Target Environment 'acceptance-de' is locked - skipping.`,
		},
		{
			Name: "Environment has no upstream - but train continues in other env",
			Setup: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance + "-ca",
					Config: config.EnvironmentConfig{
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString(envAcceptance),
					},
				},
				&CreateEnvironment{
					Environment: envAcceptance + "-de",
					Config: config.EnvironmentConfig{
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString(envAcceptance),
					},
				},
			},
			ReleaseTrain: ReleaseTrain{
				Target: envAcceptance,
			},
			expectedPrognosis: ReleaseTrainPrognosis{
				Error: nil,
				EnvironmentPrognoses: map[string]ReleaseTrainEnvironmentPrognosis{
					"acceptance-ca": ReleaseTrainEnvironmentPrognosis{
						SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
							SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM,
						},
						Error:         nil,
						AppsPrognoses: nil,
					},
					"acceptance-de": ReleaseTrainEnvironmentPrognosis{
						SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
							SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM,
						},
						Error:         nil,
						AppsPrognoses: nil,
					},
				},
			},
			expectedCommitMsg: `Release Train to environment/environment group 'acceptance':

Environment '"acceptance-ca"' does not have upstream configured - skipping.
Environment '"acceptance-de"' does not have upstream configured - skipping.`,
		},
		{
			Name: "Environment has no upstream.latest or env - but train continues in other env",
			Setup: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance + "-ca",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "",
							Latest:      false,
						},
						EnvironmentGroup: conversion.FromString(envAcceptance),
					},
				},
				&CreateEnvironment{
					Environment: envAcceptance + "-de",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "",
							Latest:      false,
						},
						EnvironmentGroup: conversion.FromString(envAcceptance),
					},
				},
			},
			ReleaseTrain: ReleaseTrain{
				Target: envAcceptance,
			},
			expectedPrognosis: ReleaseTrainPrognosis{
				Error: nil,
				EnvironmentPrognoses: map[string]ReleaseTrainEnvironmentPrognosis{
					"acceptance-ca": ReleaseTrainEnvironmentPrognosis{
						SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
							SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM_LATEST_OR_UPSTREAM_ENV,
						},
						Error:         nil,
						AppsPrognoses: nil,
					},
					"acceptance-de": ReleaseTrainEnvironmentPrognosis{
						SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
							SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_NO_UPSTREAM_LATEST_OR_UPSTREAM_ENV,
						},
						Error:         nil,
						AppsPrognoses: nil,
					},
				},
			},
			expectedCommitMsg: `Release Train to environment/environment group 'acceptance':

Environment "acceptance-ca" does not have upstream.latest or upstream.environment configured - skipping.
Environment "acceptance-de" does not have upstream.latest or upstream.environment configured - skipping.`,
		},
		{
			Name: "Environment has both upstream.latest and env - but train continues in other env",
			Setup: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance + "-ca",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "dev",
							Latest:      true,
						},
						EnvironmentGroup: conversion.FromString(envAcceptance),
					},
				},
				&CreateEnvironment{
					Environment: envAcceptance + "-de",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "dev",
							Latest:      true,
						},
						EnvironmentGroup: conversion.FromString(envAcceptance),
					},
				},
			},
			expectedPrognosis: ReleaseTrainPrognosis{
				Error: nil,
				EnvironmentPrognoses: map[string]ReleaseTrainEnvironmentPrognosis{
					"acceptance-ca": {
						SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
							SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV,
						},
						Error:         nil,
						Locks:         nil,
						AppsPrognoses: nil,
					},
					"acceptance-de": {
						SkipCause: &api.ReleaseTrainEnvPrognosis_SkipCause{
							SkipCause: api.ReleaseTrainEnvSkipCause_ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV,
						},
						Error:         nil,
						Locks:         nil,
						AppsPrognoses: nil,
					},
				},
			},
			ReleaseTrain: ReleaseTrain{
				Target: envAcceptance,
			},
			expectedCommitMsg: `Release Train to environment/environment group 'acceptance':

Environment "acceptance-ca" has both upstream.latest and upstream.environment configured - skipping.
Environment "acceptance-de" has both upstream.latest and upstream.environment configured - skipping.`,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)

			err := repo.Apply(ctx, tc.Setup...)
			if err != nil {
				t.Fatalf("error encountered during setup, but none was expected here, error: %v", err)
			}

			_ = r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				configs, _ := repo.State().GetAllEnvironmentConfigs(ctx, transaction)
				prognosis := tc.ReleaseTrain.Prognosis(ctx, repo.State(), transaction, configs)
				if diff := cmp.Diff(prognosis.EnvironmentPrognoses, tc.expectedPrognosis.EnvironmentPrognoses, protocmp.Transform(), protocmp.IgnoreFields(&api.Lock{}, "created_at")); diff != "" {
					t.Fatalf("release train prognosis is wrong, wanted the result \n%v\n got\n%v\ndiff:\n%s", tc.expectedPrognosis.EnvironmentPrognoses, prognosis.EnvironmentPrognoses, diff)
				}
				if !cmp.Equal(prognosis.Error, tc.expectedPrognosis.Error, cmpopts.EquateErrors()) {
					t.Fatalf("release train prognosis is wrong, wanted the error %v, got %v", tc.expectedPrognosis.Error, prognosis.Error)
				}

				commitMsg, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, []Transformer{&tc.ReleaseTrain}...)

				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				// note that we only check the LAST error here:
				actualMsg := ""
				if len(commitMsg) > 0 {
					actualMsg = commitMsg[len(commitMsg)-1]
				}
				if diff := cmp.Diff(tc.expectedCommitMsg, actualMsg); diff != "" {
					t.Errorf("got \n%s\n, want \n%s\n, diff (-want +got)\n%s\n", actualMsg, tc.expectedCommitMsg, diff)
				}
				return nil
			})
		})
	}
}

func TestTransformerChanges(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedCommitMsg string
		expectedChanges   *TransformerResult
	}{
		{
			Name: "Deploy 1 app, another app locked by app lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      testutil.MakeEnvConfigUpstream(envAcceptance, nil),
				},
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
				&CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						envProduction: envProduction,
						envAcceptance: envAcceptance,
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateEnvironmentApplicationLock{
					Environment: envProduction,
					Application: "foo",
					LockId:      "foo-id",
					Message:     "foo",
				},
				&CreateApplicationVersion{
					Application: "bar",
					Manifests: map[string]string{
						envProduction: envProduction,
						envAcceptance: envAcceptance,
					},
					WriteCommitData: true,
					Version:         2,
				},
				&ReleaseTrain{
					Target: envProduction,
				},
			},
			expectedChanges: &TransformerResult{
				ChangedApps: []AppEnv{
					// foo is locked, so it should not appear here
					{
						App: "bar",
						Env: envProduction,
					},
				},
			},
		},
		{
			Name: "env lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      testutil.MakeEnvConfigUpstream(envAcceptance, nil),
				},
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
				&CreateEnvironmentLock{
					Environment: envProduction,
					LockId:      "foo-id",
					Message:     "foo",
				},
				&CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						envProduction: envProduction,
						envAcceptance: envAcceptance,
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateApplicationVersion{
					Application: "bar",
					Manifests: map[string]string{
						envProduction: envProduction,
						envAcceptance: envAcceptance,
					},
					WriteCommitData: true,
					Version:         2,
				},
				&ReleaseTrain{
					Target: envProduction,
				},
			},
			expectedChanges: &TransformerResult{
				ChangedApps: nil,
			},
		},
		{
			Name: "team lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      testutil.MakeEnvConfigUpstream(envAcceptance, nil),
				},
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
				&CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						envProduction: envProduction,
						envAcceptance: envAcceptance,
					},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         1,
				},
				&CreateEnvironmentTeamLock{ //team lock always needs to come after some release creation
					Environment: envProduction,
					Team:        "sre-team",
					LockId:      "foo-id",
					Message:     "foo",
				},
				&CreateApplicationVersion{
					Application: "bar",
					Manifests: map[string]string{
						envProduction: envProduction,
						envAcceptance: envAcceptance,
					},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         1,
				},
				&ReleaseTrain{
					Target: envProduction,
				},
			},
			expectedChanges: &TransformerResult{
				ChangedApps: nil,
			},
		},
		{
			Name: "create env lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      testutil.MakeEnvConfigUpstream(envAcceptance, nil),
				},
				&CreateEnvironmentLock{
					Environment: envProduction,
					LockId:      "foo-id",
					Message:     "foo",
				},
			},
			expectedChanges: &TransformerResult{
				ChangedApps: nil,
			},
		},
		{
			Name: "create env",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      testutil.MakeEnvConfigUpstream(envAcceptance, nil),
				},
			},
			expectedChanges: &TransformerResult{
				ChangedApps: nil,
			},
		},
		{
			Name: "delete env from app",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
				&CreateEnvironment{
					Environment: envProduction,
					Config:      testutil.MakeEnvConfigUpstream(envAcceptance, nil),
				},
				&CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						envProduction: envProduction,
						envAcceptance: envAcceptance,
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeleteEnvFromApp{
					Application: "foo",
					Environment: envAcceptance,
				},
			},
			expectedChanges: &TransformerResult{
				ChangedApps: []AppEnv{
					{
						App: "foo",
						Env: envAcceptance,
					},
				},
				DeletedRootApps: []RootApp{
					{
						Env: envAcceptance,
					},
				},
			},
		},
		{
			Name: "deploy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
				&CreateEnvironment{
					Environment: envProduction,
					Config:      testutil.MakeEnvConfigUpstream(envAcceptance, nil),
				},
				&CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						envProduction: envProduction,
						envAcceptance: envAcceptance,
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Authentication: Authentication{},
					Environment:    envProduction,
					Application:    "foo",
					Version:        1,
				},
			},
			expectedChanges: &TransformerResult{
				ChangedApps: []AppEnv{
					{
						App: "foo",
						Env: envProduction,
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			//repo := setupRepositoryTest(t)
			repo := SetupRepositoryTestWithDB(t)

			dbHandler := repo.State().DBHandler

			_ = dbHandler.WithTransaction(testutil.MakeTestContext(), false, func(ctx context.Context, transaction *sql.Tx) error {
				msgs, _, actualChanges, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				// note that we only check the LAST error here:
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
				}
				// we only diff the changes from the last transformer here:
				lastChanges := actualChanges[len(actualChanges)-1]
				if diff := cmp.Diff(lastChanges, tc.expectedChanges); diff != "" {
					t.Log("Commit message:\n", msgs[len(msgs)-1])
					t.Errorf("got %v, want %v, diff (-want +got) %s", lastChanges, tc.expectedChanges, diff)
				}
				return nil
			})

		})
	}
}

func TestRbacTransformerTest(t *testing.T) {
	envGroupProduction := "production"
	fixtureWrapTransformError := func(err error) *TransformerBatchApplyError {
		return &TransformerBatchApplyError{
			Index:            0,
			TransformerError: err,
		}
	}
	fixtureWrapGeneralFailure := func(err error) *CreateReleaseError {
		return &CreateReleaseError{
			response: api.CreateReleaseResponse{
				Response: &api.CreateReleaseResponse_GeneralFailure{
					GeneralFailure: &api.CreateReleaseResponseGeneralFailure{
						Message: err.Error(),
					},
				},
			},
		}
	}
	tcs := []struct {
		Name          string
		ctx           context.Context
		Transformers  []Transformer
		ExpectedError error
	}{
		{
			Name: "able to undeploy application with team permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "staging",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironment{
					Environment:    "production",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "staging"}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						"production": "production",
						"staging":    "staging",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team",
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application:    "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&UndeployApplication{
					Application: "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeployUndeploy,staging:*,app1,allow":    {Role: "developer"},
						"p,role:developer,DeployUndeploy,production:*,app1,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
		},
		{
			Name: "able to undeploy application with permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "staging",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironment{
					Environment:    "production",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "staging"}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						"production": "production",
						"staging":    "staging",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application:    "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&UndeployApplication{
					Application: "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeployUndeploy,staging:*,app1,allow":    {Role: "developer"},
						"p,role:developer,DeployUndeploy,production:*,app1,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
		},
		{
			Name: "unable to undeploy application without team permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "staging",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironment{
					Environment:    "production",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "staging"}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						"production": "production",
						"staging":    "staging",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team-1",
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application:    "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&UndeployApplication{
					Application: "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeployUndeploy,staging:*,app1,allow":    {Role: "developer"},
						"p,role:developer,DeployUndeploy,production:*,app1,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.TeamPermissionError{
				User:   "test tester",
				Email:  "testmail@example.com",
				Action: "DeployUndeploy",
				Team:   "team-1",
			}),
		},
		{
			Name: "unable to undeploy application without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "staging",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironment{
					Environment:    "production",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "staging"}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						"production": "production",
						"staging":    "staging",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application:    "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&UndeployApplication{
					Application: "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeployUndeploy,production:*,app1,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "DeployUndeploy",
				Environment: "*",
			}),
		},
		{
			Name: "able to create environment with permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production-p1",
					Config:      config.EnvironmentConfig{EnvironmentGroup: nil},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateEnvironment,*:*,*,allow": {Role: "developer"}}}}}},
			},
		},
		{
			Name: "able to create environment inside environment group with permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production-p2",
					Config:      config.EnvironmentConfig{EnvironmentGroup: &envGroupProduction},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateEnvironment,production:*,*,allow": {Role: "developer"}}}}}},
			},
		},
		{
			Name: "unable to create environment without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production-p2",
					Config:         config.EnvironmentConfig{EnvironmentGroup: &envGroupProduction},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{}}}}},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "CreateEnvironment",
				Environment: "*",
			}),
		},
		{

			Name: "able to create undeploy with team permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "staging",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironment{
					Environment:    "production",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "staging"}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						"production": "production",
						"staging":    "staging",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team",
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateUndeploy,production:*,app1,allow": {Role: "developer"},
						"p,role:developer,CreateUndeploy,staging:*,app1,allow":    {Role: "developer"},
						"p,role:developer,DeployRelease,staging:*,app1,allow":     {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
		},
		{
			Name: "able to create undeploy with permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "staging",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironment{
					Environment:    "production",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "staging"}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						"production": "production",
						"staging":    "staging",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateUndeploy,production:*,app1,allow": {Role: "developer"},
						"p,role:developer,CreateUndeploy,staging:*,app1,allow":    {Role: "developer"},
						"p,role:developer,DeployRelease,staging:*,app1,allow":     {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
		},
		{

			Name: "unable to create undeploy without team permissions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "staging",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironment{
					Environment:    "production",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "staging"}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						"production": "production",
						"staging":    "staging",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team-1",
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateUndeploy,production:*,app1,allow": {Role: "developer"},
						"p,role:developer,CreateUndeploy,staging:*,app1,allow":    {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.TeamPermissionError{
				User:   "test tester",
				Email:  "testmail@example.com",
				Action: "CreateUndeploy",
				Team:   "team-1",
			}),
		},
		{
			Name: "unable to create undeploy without permissions policy: Missing DeployRelease permission",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "staging",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironment{
					Environment:    "production",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "staging"}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						"production": "production",
						"staging":    "staging",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateUndeploy,production:*,app1,allow": {Role: "developer"},
						"p,role:developer,CreateUndeploy,staging:*,app1,allow":    {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "DeployRelease",
				Environment: "staging",
			}),
		},
		{
			Name: "unable to create undeploy without permissions policy: Missing CreateUndeploy permission",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "staging",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironment{
					Environment:    "production",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "staging"}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						"production": "production",
						"staging":    "staging",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateUndeploy,production:*,app1,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "CreateUndeploy",
				Environment: "*",
			}),
		},
		{
			Name: "able to create release train with permissions policy",
			Transformers: ReleaseTrainTestSetup(&ReleaseTrain{
				Target: envProduction,
				Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
					"p,role:developer,DeployReleaseTrain,production:production,*,allow": {Role: "developer"},
					"p,role:developer,DeployRelease,production:*,test,allow":            {Role: "developer"},
				}},
					Team: &auth.RBACTeams{Permissions: map[string][]string{
						"testmail@example.com": []string{"*"},
					}}}},
			}),
		},
		{
			Name: "unable to create release train without permissions policy: Missing DeployRelease permission",
			Transformers: ReleaseTrainTestSetup(&ReleaseTrain{
				Target: envProduction,
				Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
					"p,role:developer,DeployReleaseTrain,production:production,*,allow": {Role: "developer"},
				}},
					Team: &auth.RBACTeams{Permissions: map[string][]string{
						"testmail@example.com": []string{"*"},
					}}}},
			}),
			ExpectedError: fixtureWrapTransformError(status.Error(codes.Internal, "internal error")),
		},
		{
			Name: "unable to create release train without permissions policy: Missing ReleaseTrain permission",
			Transformers: ReleaseTrainTestSetup(&ReleaseTrain{
				Target:         envProduction,
				Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{}}}},
			}),
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "DeployReleaseTrain",
				Environment: "production",
			}),
		},
		{
			Name: "able to create application version with team permissions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "acceptance",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1-testing",
					Manifests: map[string]string{
						envAcceptance: "acceptance", // not empty
					},
					Team: "team",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateRelease,acceptance:*,app1-testing,allow": {Role: "developer"},
						"p,role:developer,DeployRelease,acceptance:*,app1-testing,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
					WriteCommitData: true,
					Version:         1,
				},
			},
		},
		{
			Name: "able to create application version with permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "acceptance",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1-testing",
					Manifests: map[string]string{
						envAcceptance: "acceptance", // not empty
					},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateRelease,acceptance:*,app1-testing,allow": {Role: "developer"},
						"p,role:developer,DeployRelease,acceptance:*,app1-testing,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
					WriteCommitData: true,
					Version:         1,
				},
			},
		},
		{
			Name: "unable to create application version with team permissions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "acceptance",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1-testing",
					Manifests: map[string]string{
						envAcceptance: "acceptance", // not empty
					},
					Team: "team-1",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateRelease,acceptance:*,app1-testing,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
					WriteCommitData: true,
					Version:         1,
				},
			},
			ExpectedError: fixtureWrapTransformError(
				auth.TeamPermissionError{
					User:   "test tester",
					Email:  "testmail@example.com",
					Action: "CreateRelease",
					Team:   "team-1",
				},
			),
		},
		{
			Name: "unable to create application version with permissions policy: Missing DeployRelease permission",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "acceptance",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1-testing",
					Manifests: map[string]string{
						envAcceptance: "acceptance", // not empty
					},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateRelease,acceptance:*,app1-testing,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
					WriteCommitData: true,
					Version:         1,
				},
			},
			ExpectedError: fixtureWrapTransformError(
				fixtureWrapGeneralFailure(
					auth.PermissionError{
						User:        "test tester",
						Role:        "developer",
						Action:      "DeployRelease",
						Environment: "acceptance",
					},
				),
			),
		},
		{
			Name: "unable to create application version without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "acceptance",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance", // not empty
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{}}}},
					WriteCommitData: true,
					Version:         1,
				},
			},
			ExpectedError: fixtureWrapTransformError(
				auth.PermissionError{
					User:        "test tester",
					Role:        "developer",
					Action:      "CreateRelease",
					Environment: "*",
				},
			),
		},
		{
			Name: "able to deploy application with team permissions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "acceptance",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance", // not empty
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team",
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:   envAcceptance,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_FAIL,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeployRelease,acceptance:acceptance,*,allow": {Role: "developer"}}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
		},
		{
			Name: "able to deploy application with permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "acceptance",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance", // not empty
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:   envAcceptance,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_FAIL,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeployRelease,acceptance:acceptance,*,allow": {Role: "developer"}}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
		},
		{
			Name: "unable to deploy application with team permissions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "acceptance",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance", // not empty
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team-1",
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:   envAcceptance,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_FAIL,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeployRelease,acceptance:acceptance,*,allow": {Role: "developer"}}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.TeamPermissionError{
				User:   "test tester",
				Email:  "testmail@example.com",
				Action: "DeployRelease",
				Team:   "team-1",
			}),
		},
		{
			Name: "unable to deploy application with permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "acceptance",
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance", // not empty
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:    envAcceptance,
					Application:    "app1",
					Version:        1,
					LockBehaviour:  api.LockBehavior_FAIL,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "DeployRelease",
				Environment: "acceptance",
			}),
		},
		{
			Name: "able to create environment lock with permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironmentLock{
					Environment: "production",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "developer"}}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
		},
		{
			Name: "able to create environment lock with permissions policy: different user",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironmentLock{
					Environment: "production",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:releaseManager,CreateLock,production:production,*,allow": {Role: "releaseManager"}}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
			ctx: testutil.MakeTestContextDexEnabledUser("releaseManager"),
		},
		{
			Name: "unable to create environment lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironmentLock{
					Environment:    "production",
					Message:        "don't",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "CreateLock",
				Environment: "production",
			}),
		},
		{
			Name: "unable to delete environment lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironmentLock{
					Environment: "production",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "developer"}}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
				&DeleteEnvironmentLock{
					Environment:    "production",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "DeleteLock",
				Environment: "production",
			}),
		},
		{
			Name: "able to delete environment lock with permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateEnvironmentLock{
					Environment: "production",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "developer"}}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
				&DeleteEnvironmentLock{
					Environment: "production",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeployRelease,production:production,*,allow": {Role: "developer"},
						"p,role:developer,CreateLock,production:production,*,allow":    {Role: "developer"},
						"p,role:developer,DeleteLock,production:production,*,allow":    {Role: "developer"}}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
		},
		{
			Name: "able to create environment application lock with team permissions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team",
					Version:         1,
				},
				&CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "Developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
		},
		{
			Name: "unable to create environment application lock with team permissions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team-1",
					Version:         1,
				},
				&CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "Developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.TeamPermissionError{
				User:   "test tester",
				Email:  "testmail@example.com",
				Action: "CreateLock",
				Team:   "team-1",
			}),
		},
		{
			Name: "unable to create environment application lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateEnvironmentApplicationLock{
					Environment:    "production",
					Application:    "test",
					Message:        "don't",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "CreateLock",
				Environment: "production",
			}),
		},
		{
			Name: "able to create environment application lock with correct permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "Developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
		},
		{
			Name: "able to delete environment application lock with team permissions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team",
					Version:         1,
				},
				&CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
				&DeleteEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeleteLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
		},
		{
			Name: "unable to delete environment application lock with team permissions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team-1",
					Version:         1,
				},
				&CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
				&DeleteEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeleteLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.TeamPermissionError{
				User:   "test tester",
				Email:  "testmail@example.com",
				Action: "DeleteLock",
				Team:   "team-1",
			}),
		},
		{
			Name: "unable to delete environment application lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateEnvironmentApplicationLock{
					Environment:    "production",
					Application:    "test",
					Message:        "don't",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&DeleteEnvironmentApplicationLock{
					Environment:    "production",
					Application:    "test",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "DeleteLock",
				Environment: "production",
			}),
		},
		{
			Name: "able to delete environment application lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
				&DeleteEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeleteLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
		},
		{
			Name: "able to create environment team lock with team permissions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         1,
				},
				&CreateEnvironmentTeamLock{
					Environment: "production",
					Team:        "sre-team",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "Developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"sre-team"},
						}}}},
				},
			},
		},
		{
			Name: "unable to create environment team lock with team permissions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "sre-team-1",
					Version:         1,
				},
				&CreateEnvironmentTeamLock{
					Environment: "production",
					Team:        "sre-team-1",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "Developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"sre-team"},
						}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.TeamPermissionError{
				User:   "test tester",
				Email:  "testmail@example.com",
				Action: "CreateLock",
				Team:   "sre-team-1",
			}),
		},
		{
			Name: "unable to create environment team lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         1,
				},
				&CreateEnvironmentTeamLock{
					Environment:    "production",
					Team:           "sre-team",
					Message:        "don't",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "CreateLock",
				Environment: "production",
				Team:        "sre-team",
			}),
		},
		{
			Name: "able to create environment team lock with correct permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         1,
				},
				&CreateEnvironmentTeamLock{
					Environment: "production",
					Team:        "sre-team",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "Developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
		},
		{
			Name: "able to create environment team lock with correct permissions policy - sre-team",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         1,
				},
				&CreateEnvironmentTeamLock{
					Environment: "production",
					Team:        "sre-team",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "Developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
		},
		{
			Name: "able to delete environment team lock with team permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         1,
				},
				&CreateEnvironmentTeamLock{
					Environment: "production",
					Team:        "sre-team",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"sre-team"},
						}}}},
				},
				&DeleteEnvironmentTeamLock{
					Environment: "production",
					Team:        "sre-team",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeleteLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"sre-team"},
						}}}},
				},
			},
		},
		{
			Name: "unable to delete environment team lock with team permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         1,
				},
				&CreateEnvironmentTeamLock{
					Environment: "production",
					Team:        "sre-team",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"sre-team"},
						}}}},
				},
				&DeleteEnvironmentTeamLock{
					Environment: "production",
					Team:        "sre-team",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeleteLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"sre-team-1"},
						}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.TeamPermissionError{
				User:   "test tester",
				Action: "DeleteLock",
				Email:  "testmail@example.com",
				Team:   "sre-team",
			}),
		},
		{
			Name: "unable to delete environment team lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         1,
				},
				&CreateEnvironmentTeamLock{
					Environment:    "production",
					Team:           "sre-team",
					Message:        "don't",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&DeleteEnvironmentTeamLock{
					Environment:    "production",
					Team:           "sre-team",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "DeleteLock",
				Environment: "production",
				Team:        "sre-team",
			}),
		},
		{
			Name: "able to delete environment team lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    "production",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "sre-team",
					Version:         1,
				},
				&CreateEnvironmentTeamLock{
					Environment: "production",
					Team:        "sre-team",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,CreateLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
				&DeleteEnvironmentTeamLock{
					Environment: "production",
					Team:        "sre-team",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeleteLock,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
		},
		{
			Name: "able to delete environment application with team permission policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    envProduction,
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team",
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:    envProduction,
					Application:    "app1",
					Version:        1,
					LockBehaviour:  api.LockBehavior_FAIL,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&DeleteEnvFromApp{
					Application: "app1",
					Environment: envProduction,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeleteEnvironmentApplication,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
		},
		{
			Name: "unable to delete environment application without team permission policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    envProduction,
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Team:            "team-1",
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:    envProduction,
					Application:    "app1",
					Version:        1,
					LockBehaviour:  api.LockBehavior_FAIL,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&DeleteEnvFromApp{
					Application: "app1",
					Environment: envProduction,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeleteEnvironmentApplication,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"team"},
						}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.TeamPermissionError{
				User:   "test tester",
				Email:  "testmail@example.com",
				Action: "DeleteEnvironmentApplication",
				Team:   "team-1",
			}),
		},
		{
			Name: "unable to delete environment application without permission policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    envProduction,
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:    envProduction,
					Application:    "app1",
					Version:        1,
					LockBehaviour:  api.LockBehavior_FAIL,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&DeleteEnvFromApp{
					Application:    "app1",
					Environment:    envProduction,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{}}}},
				},
			},
			ExpectedError: fixtureWrapTransformError(auth.PermissionError{
				User:        "test tester",
				Role:        "developer",
				Action:      "DeleteEnvironmentApplication",
				Environment: "production",
			}),
		},
		{
			Name: "able to delete environment application without permission policy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:    envProduction,
					Config:         config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					Authentication:  Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:    envProduction,
					Application:    "app1",
					Version:        1,
					LockBehaviour:  api.LockBehavior_FAIL,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				&DeleteEnvFromApp{
					Application: "app1",
					Environment: envProduction,
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: &auth.RBACPolicies{Permissions: map[string]auth.Permission{
						"p,role:developer,DeleteEnvironmentApplication,production:production,*,allow": {Role: "developer"},
					}},
						Team: &auth.RBACTeams{Permissions: map[string][]string{
							"testmail@example.com": []string{"*"},
						}}}},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()
			ctx := testutil.MakeTestContextDexEnabled()
			if tc.ctx != nil {
				ctx = tc.ctx
			}
			repo := SetupRepositoryTestWithDB(t)
			r := repo.(*repository)
			var err error
			for _, tf := range tc.Transformers {
				err = r.Apply(ctx, tf)
				if err != nil {
					break
				}
			}
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

// Helper method to setup release train unit tests.
func ReleaseTrainTestSetup(releaseTrainTransformer Transformer) []Transformer {
	return append([]Transformer{
		&CreateEnvironment{
			Environment: envProduction,
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: envAcceptance, // train drives from acceptance to production
				},
			},
		},
		&CreateEnvironment{
			Environment: envAcceptance,
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Latest: true,
				},
			},
		},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				envProduction: "productionmanifest",
				envAcceptance: "acceptancenmanifest",
			},
			WriteCommitData: true,
			Team:            "team-1",
			Version:         1,
		},
		&DeployApplicationVersion{
			Environment: envProduction,
			Application: "test",
			Version:     1,
		},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				envProduction: "productionmanifest",
				envAcceptance: "acceptancenmanifest",
			},
			WriteCommitData: true,
			Version:         2,
		},
		&DeployApplicationVersion{
			Environment: envAcceptance,
			Application: "test",
			Version:     1,
		},
		&DeployApplicationVersion{
			Environment: envAcceptance,
			Application: "test",
			Version:     2,
		}}, releaseTrainTransformer)
}

func SetupRepositoryTestWithDB(t *testing.T) Repository {
	r, _ := SetupRepositoryTestWithDBOptions(t, false)
	return r
}

func SetupRepositoryTestWithDBOptions(t *testing.T, writeEslOnly bool) (Repository, *db.DBHandler) {
	ctx := context.Background()
	migrationsPath, err := testutil.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dbConfig := &db.DBConfig{
		DriverName:     "sqlite3",
		MigrationsPath: migrationsPath,
		WriteEslOnly:   writeEslOnly,
	}

	dir := t.TempDir()
	// remoteDir := path.Join(dir, "remote")
	// localDir := path.Join(dir, "local")
	// cmd := exec.Command("git", "init", "--bare", remoteDir)
	// err = cmd.Start()
	// if err != nil {
	// 	t.Fatalf("error starting %v", err)
	// 	return nil, nil
	// }
	// err = cmd.Wait()
	// if err != nil {
	// 	t.Fatalf("error waiting %v", err)
	// 	return nil, nil
	// }
	// t.Logf("test created dir: %s", localDir)

	repoCfg := RepositoryConfig{
		// URL:                 remoteDir,
		// Path: localDir,
		// CommitterEmail:      "kuberpult@freiheit.com",
		// CommitterName:       "kuberpult",
		ArgoCdGenerateFiles: true,
	}
	dbConfig.DbHost = dir

	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	repoCfg.DBHandler = dbHandler

	repo, err := New(
		testutil.MakeTestContext(),
		repoCfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, dbHandler
}

func setupRepositoryTest(t *testing.T) Repository {
	repo, _ := setupRepositoryTestWithPath(t)
	return repo
}

func setupRepositoryTestWithPath(t *testing.T) (Repository, string) {
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	err := cmd.Start()
	if err != nil {
		t.Errorf("could not start git init")
		return nil, ""
	}
	err = cmd.Wait()
	if err != nil {
		t.Errorf("could not wait for git init to finish")
		return nil, ""
	}
	repo, err := New(
		testutil.MakeTestContext(),
		RepositoryConfig{
			URL:                   remoteDir,
			Path:                  localDir,
			CommitterEmail:        "kuberpult@freiheit.com",
			CommitterName:         "kuberpult",
			WriteCommitData:       true,
			MaximumCommitsPerPush: 5,
			ArgoCdGenerateFiles:   true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, remoteDir
}

// Injects an error in the filesystem of the state
type injectErr struct {
	Transformer
	collector *testfs.UsageCollector
	operation testfs.Operation
	filename  string
	err       error
}

func (i *injectErr) Transform(ctx context.Context, state *State, t TransformerContext, transaction *sql.Tx) (string, error) {
	s, err := i.Transformer.Transform(ctx, state, t, transaction)
	return s, err
}

func mockSendMetrics(repo Repository, interval time.Duration) <-chan bool {
	ch := make(chan bool, 1)
	go RegularlySendDatadogMetrics(repo, interval, func(repo Repository, even bool) { ch <- true })
	return ch
}

func TestSendRegularlyDatadogMetrics(t *testing.T) {
	tcs := []struct {
		Name          string
		shouldSucceed bool
	}{
		{
			Name: "Testing ticker",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo := SetupRepositoryTestWithDB(t)

			select {
			case <-mockSendMetrics(repo, 1):
			case <-time.After(4 * time.Second):
				t.Fatal("An error occurred during the go routine")
			}

		})
	}
}

type Gauge struct {
	Name  string
	Value float64
	Tags  []string
	Rate  float64
}

type MockClient struct {
	events []*statsd.Event
	gauges []Gauge
	statsd.ClientInterface
}

func (c *MockClient) Event(e *statsd.Event) error {
	if c == nil {
		return errors.New("no client provided")
	}
	c.events = append(c.events, e)
	return nil
}

var i = 0

func (c *MockClient) Gauge(name string, value float64, tags []string, rate float64) error {
	i = i + 1
	c.gauges = append(c.gauges, Gauge{
		Name:  name,
		Value: value,
		Tags:  tags,
		Rate:  rate,
	})
	return nil
}

// Verify that MockClient implements the ClientInterface.
// https://golang.org/doc/faq#guarantee_satisfies_interface
var _ statsd.ClientInterface = &MockClient{}

func TestDatadogQueueMetric(t *testing.T) {
	tcs := []struct {
		Name           string
		changes        *TransformerResult
		transformers   []Transformer
		expectedGauges int
	}{
		{
			Name: "Changes are sent as one event",
			transformers: []Transformer{
				&CreateEnvironment{
					Environment: "envA",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						"envA": "envA-manifest-1",
					},
					WriteCommitData: false,
					Version:         1,
				},
				&CreateApplicationVersion{
					Application: "app2",
					Manifests: map[string]string{
						"envA": "envA-manifest-2",
					},
					WriteCommitData: false,
					Version:         2,
				},
			},
			expectedGauges: 1,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			//t.Parallel() // do not run in parallel because of the global var `ddMetrics`!
			ctx := time2.WithTimeNow(testutil.MakeTestContext(), time.Unix(0, 0))
			var mockClient = &MockClient{}
			var client statsd.ClientInterface = mockClient
			repo := SetupRepositoryTestWithDB(t)
			ddMetrics = client

			err := repo.Apply(ctx, tc.transformers...)

			if err != nil {
				t.Fatalf("Expected no error: %v", err)
			}

			if tc.expectedGauges != len(mockClient.gauges) {
				// Don't compare the value of the gauge, only the number of gauges,
				// because we cannot be sure at this point what the size of the queue was during measurement
				msg := fmt.Sprintf("expected %d gauges but got %d\n",
					tc.expectedGauges, len(mockClient.gauges))
				t.Fatalf(msg)
			}
		})
	}
}

func TestDeleteEnvFromApp(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedError     *TransformerBatchApplyError
		expectedCommitMsg string
		shouldSucceed     bool
	}{
		{
			Name: "Success",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:   envProduction,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_FAIL,
				},
				&DeleteEnvFromApp{
					Application: "app1",
					Environment: envProduction,
				},
			},
			expectedCommitMsg: "Environment 'production' was removed from application 'app1' successfully.",
			shouldSucceed:     true,
		},
		{
			Name: "Success Double Delete",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:   envProduction,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_FAIL,
				},
				&DeleteEnvFromApp{
					Application: "app1",
					Environment: envProduction,
				},
				&DeleteEnvFromApp{
					Application: "app1",
					Environment: envProduction,
				},
			},
			expectedCommitMsg: "Environment 'production' was removed from application 'app1' successfully.",
			shouldSucceed:     true,
		},
		{
			Name: "fail to provide app name",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:   envProduction,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_FAIL,
				},
				&DeleteEnvFromApp{
					Environment: envProduction,
				},
			},
			expectedCommitMsg: "Environment 'production' was removed from application '' successfully.",
			shouldSucceed:     false,
		},
		{
			Name: "fail to provide env name",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment:   envProduction,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_FAIL,
				},
				&DeleteEnvFromApp{
					Application: "app1",
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            3,
				TransformerError: errMatcher{"Attempting to delete an environment that doesn't exist in the environments table"},
			},
			expectedCommitMsg: "",
			shouldSucceed:     false,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := SetupRepositoryTestWithDBOptions(t, false)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			_ = r.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				commitMsg, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				actualMsg := ""
				// note that we only check the LAST error here:
				if len(commitMsg) > 0 {
					actualMsg = commitMsg[len(commitMsg)-1]
				}
				if diff := cmp.Diff(tc.expectedCommitMsg, actualMsg); diff != "" {
					t.Errorf("commit message mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
		})
	}
}

func TestDeleteLocks(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedError     *TransformerBatchApplyError
		expectedCommitMsg string
		shouldSucceed     bool
	}{
		{
			Name: "Success delete env lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironmentLock{
					Environment: envProduction,
					LockId:      "l123",
				},
				&DeleteEnvironmentLock{
					Environment: envProduction,
					LockId:      "l123",
				},
			},
			expectedCommitMsg: "Deleted lock \"l123\" on environment \"production\"",
			shouldSucceed:     true,
		},
		{
			Name: "Success delete app lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironmentApplicationLock{
					Environment: envProduction,
					Application: "app1",
					LockId:      "l123",
					Message:     "none",
				},
				&DeleteEnvironmentApplicationLock{
					Environment: envProduction,
					Application: "app1",
					LockId:      "l123",
				},
			},
			expectedCommitMsg: "Deleted lock \"l123\" on environment \"production\" for application \"app1\"",
			shouldSucceed:     true,
		},
		{
			Name: "Success create env lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironmentLock{
					Environment: envProduction,
					LockId:      "l123",
					Message:     "my lock",
				},
			},
			expectedCommitMsg: "Created lock \"l123\" on environment \"production\"",
			shouldSucceed:     true,
		},
		{
			Name: "Success create app lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironmentApplicationLock{
					Environment: envProduction,
					Application: "app1",
					LockId:      "l123",
					Message:     "my lock",
				},
			},
			expectedCommitMsg: "Created lock \"l123\" on environment \"production\" for application \"app1\"",
			shouldSucceed:     true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, _ := SetupRepositoryTestWithDBOptions(t, false)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			_ = r.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				commitMsg, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				actualMsg := ""
				// note that we only check the LAST error here:
				if len(commitMsg) > 0 {
					actualMsg = commitMsg[len(commitMsg)-1]
				}
				if diff := cmp.Diff(tc.expectedCommitMsg, actualMsg); diff != "" {
					t.Errorf("commit message mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
		})
	}
}

func TestEnvironmentGroupLocks(t *testing.T) {
	group := conversion.FromString("prod")
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedError     *TransformerBatchApplyError
		expectedCommitMsg string
		shouldSucceed     bool
	}{
		{
			Name: "Success create env group lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "prod-ca",
					Config:      testutil.MakeEnvConfigLatestWithGroup(nil, group),
				},
				&CreateEnvironment{
					Environment: "prod-de",
					Config:      testutil.MakeEnvConfigLatestWithGroup(nil, group),
				},
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("another-group")),
				},
				&CreateEnvironmentGroupLock{
					Authentication:   Authentication{},
					EnvironmentGroup: *group,
					LockId:           "my-lock",
					Message:          "my-message",
				},
			},
			expectedCommitMsg: "Creating locks 'my-lock' for environment group 'prod':\nCreated lock \"my-lock\" on environment \"prod-ca\"\nCreated lock \"my-lock\" on environment \"prod-de\"",
			shouldSucceed:     true,
		},
		{
			Name: "Success delete env group lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "prod-ca",
					Config:      testutil.MakeEnvConfigLatestWithGroup(nil, group),
				},
				&CreateEnvironment{
					Environment: "prod-de",
					Config:      testutil.MakeEnvConfigLatestWithGroup(nil, group),
				},
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("another-group")),
				},
				&CreateEnvironmentGroupLock{
					Authentication:   Authentication{},
					EnvironmentGroup: *group,
					LockId:           "my-lock",
					Message:          "my-message",
				},
				&DeleteEnvironmentGroupLock{
					Authentication:   Authentication{},
					EnvironmentGroup: *group,
					LockId:           "my-lock",
				},
			},
			expectedCommitMsg: "Deleting locks 'my-lock' for environment group 'prod':\nDeleted lock \"my-lock\" on environment \"prod-ca\"\nDeleted lock \"my-lock\" on environment \"prod-de\"",
			shouldSucceed:     true,
		},
		{
			Name: "Success delete env group that was created as env lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "prod-ca",
					Config:      testutil.MakeEnvConfigLatestWithGroup(nil, group),
				},
				&CreateEnvironmentLock{
					Authentication: Authentication{},
					Environment:    "prod-ca",
					LockId:         "my-lock",
					Message:        "my-message",
				},
				&DeleteEnvironmentGroupLock{
					Authentication:   Authentication{},
					EnvironmentGroup: *group,
					LockId:           "my-lock",
				},
			},
			expectedCommitMsg: "Deleting locks 'my-lock' for environment group 'prod':\nDeleted lock \"my-lock\" on environment \"prod-ca\"",
			shouldSucceed:     true,
		},
		{
			Name: "Success delete env lock that was created as env group lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "prod-ca",
					Config:      testutil.MakeEnvConfigLatestWithGroup(nil, group),
				},
				&CreateEnvironmentGroupLock{
					Authentication:   Authentication{},
					EnvironmentGroup: *group,
					LockId:           "my-lock",
					Message:          "my-message",
				},
				&DeleteEnvironmentLock{
					Authentication: Authentication{},
					Environment:    "prod-ca",
					LockId:         "my-lock",
				},
			},
			expectedCommitMsg: "Deleted lock \"my-lock\" on environment \"prod-ca\"",
			shouldSucceed:     true,
		},
		{
			Name: "Failure create env group lock - no envs found",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "prod-ca",
					Config:      testutil.MakeEnvConfigLatestWithGroup(nil, group),
				},
				&CreateEnvironment{
					Environment: "prod-de",
					Config:      testutil.MakeEnvConfigLatestWithGroup(nil, group),
				},
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("another-group")),
				},
				&CreateEnvironmentGroupLock{
					Authentication:   Authentication{},
					EnvironmentGroup: "dev",
					LockId:           "my-lock",
					Message:          "my-message",
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            3,
				TransformerError: status.Error(codes.InvalidArgument, "error: No environment found with given group 'dev'"),
			},
			expectedCommitMsg: "",
			shouldSucceed:     false,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := SetupRepositoryTestWithDBOptions(t, false)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			_ = r.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				commitMsg, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				actualMsg := ""
				// note that we only check the LAST error here:
				if len(commitMsg) > 0 {
					actualMsg = commitMsg[len(commitMsg)-1]
				}
				if diff := cmp.Diff(tc.expectedCommitMsg, actualMsg); diff != "" {
					t.Errorf("commit message mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
		})
	}
}

func TestReleaseTrainsWithCommitHash(t *testing.T) {
	appName := "app"
	groupName := "prodgroup"
	versionOne := int64(1)
	versionTwo := int64(2)

	tcs := []struct {
		Name                string
		SetupStages         [][]Transformer
		CommitHashIndex     uint
		ReleaseTrain        ReleaseTrain
		ExpectedDeployments []db.Deployment
	}{
		{
			Name: "Trigger a deployment with a release train with a commit hash",
			SetupStages: [][]Transformer{
				{
					&CreateEnvironment{
						Environment: "production",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
							},
						},
					},
					&CreateEnvironment{
						Environment: "staging",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
								Latest:      true,
							},
						},
					},
					&CreateApplicationVersion{
						Application:    appName,
						SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
						Manifests: map[string]string{
							"production": "some production manifest 2",
							"staging":    "some staging manifest 2",
						},
						WriteCommitData: true,
						Version:         uint64(versionOne),
					},
					&DeployApplicationVersion{
						Environment:     "staging",
						Application:     appName,
						Version:         uint64(versionOne),
						WriteCommitData: true,
					},
				},
			},
			CommitHashIndex: 0,
			ReleaseTrain: ReleaseTrain{
				Target:          "production",
				WriteCommitData: true,
			},
			ExpectedDeployments: []db.Deployment{
				{
					App:           "app",
					Env:           "production",
					Version:       &versionOne,
					TransformerID: 5,
				},
				{
					App:           "app",
					Env:           "staging",
					Version:       &versionOne,
					TransformerID: 4,
				},
			},
		},
		{
			Name: "Trigger a deployment with a release train with an older commit hash",
			SetupStages: [][]Transformer{
				{
					&CreateEnvironment{
						Environment: "production",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
							},
						},
					},
					&CreateEnvironment{
						Environment: "staging",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
								Latest:      true,
							},
						},
					},
					&CreateApplicationVersion{
						Application:    appName,
						SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
						Manifests: map[string]string{
							"production": "some production manifest 2",
							"staging":    "some staging manifest 2",
						},
						WriteCommitData: true,
						Version:         uint64(versionOne),
					},
					&DeployApplicationVersion{
						Environment:     "staging",
						Application:     appName,
						Version:         uint64(versionOne),
						WriteCommitData: true,
					},
				},
				{
					&CreateApplicationVersion{
						Application:    appName,
						SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaac",
						Manifests: map[string]string{
							"production": "some production manifest 2",
							"staging":    "some staging manifest 2",
						},
						WriteCommitData: true,
						Version:         uint64(versionTwo),
					},
					&DeployApplicationVersion{
						Environment:     "staging",
						Application:     appName,
						Version:         uint64(versionTwo),
						WriteCommitData: true,
					},
				},
			},
			CommitHashIndex: 0,
			ReleaseTrain: ReleaseTrain{
				Target:          "production",
				WriteCommitData: true,
			},
			ExpectedDeployments: []db.Deployment{
				{
					App:           "app",
					Env:           "production",
					Version:       &versionOne,
					TransformerID: 7,
				},
				{
					App:           "app",
					Env:           "staging",
					Version:       &versionTwo,
					TransformerID: 6,
				},
			},
		},
		{
			Name: "Trigger a deployment with a release train with a newer commit hash",
			SetupStages: [][]Transformer{
				{
					&CreateEnvironment{
						Environment: "production",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
							},
						},
					},
					&CreateEnvironment{
						Environment: "staging",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
								Latest:      true,
							},
						},
					},
					&CreateApplicationVersion{
						Application:    appName,
						SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
						Manifests: map[string]string{
							"production": "some production manifest 2",
							"staging":    "some staging manifest 2",
						},
						WriteCommitData: true,
						Version:         uint64(versionOne),
					},
					&DeployApplicationVersion{
						Environment:     "staging",
						Application:     appName,
						Version:         uint64(versionOne),
						WriteCommitData: true,
					},
				},
				{
					&CreateApplicationVersion{
						Application:    appName,
						SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaac",
						Manifests: map[string]string{
							"production": "some production manifest 2",
							"staging":    "some staging manifest 2",
						},
						WriteCommitData: true,
						Version:         uint64(versionTwo),
					},
					&DeployApplicationVersion{
						Environment:     "staging",
						Application:     appName,
						Version:         uint64(versionTwo),
						WriteCommitData: true,
					},
				},
			},
			CommitHashIndex: 1,
			ReleaseTrain: ReleaseTrain{
				Target:          "production",
				WriteCommitData: true,
			},
			ExpectedDeployments: []db.Deployment{
				{
					App:           "app",
					Env:           "production",
					Version:       &versionTwo,
					TransformerID: 7,
				},
				{
					App:           "app",
					Env:           "staging",
					Version:       &versionTwo,
					TransformerID: 6,
				},
			},
		},
		{
			Name: "Trigger no deployments with a release train with a commit hash",
			SetupStages: [][]Transformer{
				{
					&CreateEnvironment{
						Environment: "production",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
							},
						},
					},
					&CreateEnvironment{
						Environment: "staging",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "development",
							},
						},
					},
					&CreateEnvironment{
						Environment: "development",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "development",
								Latest:      true,
							},
						},
					},
					&CreateApplicationVersion{
						Application:    appName,
						SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
						Manifests: map[string]string{
							"production":  "some production manifest 2",
							"staging":     "some staging manifest 2",
							"development": "some development manifest 2",
						},
						WriteCommitData: true,
						Version:         uint64(versionOne),
					},
					&DeployApplicationVersion{
						Environment:     "development",
						Application:     appName,
						Version:         uint64(versionOne),
						WriteCommitData: true,
					},
				},
				{
					&ReleaseTrain{
						Target:          "staging",
						WriteCommitData: true,
					},
				},
			},
			CommitHashIndex: 0,
			ReleaseTrain: ReleaseTrain{
				Target:          "production",
				WriteCommitData: true,
			},
			ExpectedDeployments: []db.Deployment{
				{
					App:           "app",
					Env:           "development",
					Version:       &versionOne,
					TransformerID: 5,
				},
				{
					App:           "app",
					Env:           "staging",
					Version:       &versionOne,
					TransformerID: 6,
				},
			},
		},
		{
			Name: "Trigger no deployments with a release train with a commit hash due to locks created afterwards",
			SetupStages: [][]Transformer{
				{
					&CreateEnvironment{
						Environment: "production",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
							},
						},
					},
					&CreateEnvironment{
						Environment: "staging",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
								Latest:      true,
							},
						},
					},
					&CreateApplicationVersion{
						Application:    appName,
						SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
						Manifests: map[string]string{
							"production": "some production manifest 2",
							"staging":    "some staging manifest 2",
						},
						WriteCommitData: true,
						Version:         uint64(versionOne),
					},
					&DeployApplicationVersion{
						Environment:     "staging",
						Application:     appName,
						Version:         uint64(versionOne),
						WriteCommitData: true,
					},
				},
				{
					&CreateEnvironmentApplicationLock{
						Environment: "production",
						Application: appName,
						LockId:      "22133",
						Message:     "test",
					},
				},
			},
			CommitHashIndex: 0,
			ReleaseTrain: ReleaseTrain{
				Target:          "production",
				WriteCommitData: true,
			},
			ExpectedDeployments: []db.Deployment{
				{
					App:           "app",
					Env:           "staging",
					Version:       &versionOne,
					TransformerID: 4,
				},
			},
		},
		{
			Name: "Trigger deployments with a release train with a commit hash that had locks deleted afterwards",
			SetupStages: [][]Transformer{
				{
					&CreateEnvironment{
						Environment: "production",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
							},
						},
					},
					&CreateEnvironment{
						Environment: "staging",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
								Latest:      true,
							},
						},
					},
					&CreateApplicationVersion{
						Application:    appName,
						SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
						Manifests: map[string]string{
							"production": "some production manifest 2",
							"staging":    "some staging manifest 2",
						},
						WriteCommitData: true,
						Version:         uint64(versionOne),
					},
					&DeployApplicationVersion{
						Environment:     "staging",
						Application:     appName,
						Version:         uint64(versionOne),
						WriteCommitData: true,
					},
					&CreateEnvironmentApplicationLock{
						Environment: "production",
						Application: appName,
						LockId:      "22133",
						Message:     "test",
					},
				},
				{
					&DeleteEnvironmentApplicationLock{
						Environment: "production",
						Application: appName,
						LockId:      "22133",
					},
				},
			},
			CommitHashIndex: 0,
			ReleaseTrain: ReleaseTrain{
				Target:          "production",
				WriteCommitData: true,
			},
			ExpectedDeployments: []db.Deployment{
				{
					App:           "app",
					Env:           "staging",
					Version:       &versionOne,
					TransformerID: 4,
				},
				{
					App:           "app",
					Env:           "production",
					Version:       &versionOne,
					TransformerID: 7,
				},
			},
		},
		{
			Name: "Trigger a deployment with a release train for a group with a commit hash",
			SetupStages: [][]Transformer{
				{
					&CreateEnvironment{
						Environment: "production1",
						Config: config.EnvironmentConfig{
							EnvironmentGroup: &groupName,
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
							},
						},
					},
					&CreateEnvironment{
						Environment: "production2",
						Config: config.EnvironmentConfig{
							EnvironmentGroup: &groupName,
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
							},
						},
					},
					&CreateEnvironment{
						Environment: "staging",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
								Latest:      true,
							},
						},
					},
					&CreateApplicationVersion{
						Application:    appName,
						SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
						Manifests: map[string]string{
							"production1": "some production manifest 2",
							"production2": "some production manifest 2",
							"staging":     "some staging manifest 2",
						},
						WriteCommitData: true,
						Version:         uint64(versionOne),
					},
					&DeployApplicationVersion{
						Environment:     "staging",
						Application:     appName,
						Version:         uint64(versionOne),
						WriteCommitData: true,
					},
				},
			},
			CommitHashIndex: 0,
			ReleaseTrain: ReleaseTrain{
				Target:          groupName,
				WriteCommitData: true,
			},
			ExpectedDeployments: []db.Deployment{
				{
					App:           "app",
					Env:           "production1",
					Version:       &versionOne,
					TransformerID: 6,
				},
				{
					App:           "app",
					Env:           "production2",
					Version:       &versionOne,
					TransformerID: 6,
				},
				{
					App:           "app",
					Env:           "staging",
					Version:       &versionOne,
					TransformerID: 5,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			tc := tc
			t.Parallel()

			fakeGen := testutil.NewIncrementalUUIDGenerator()
			ctx := testutil.MakeTestContext()
			ctx = AddGeneratorToContext(ctx, fakeGen)
			var err error = nil
			repo, dbHandler := SetupRepositoryTestWithDBOptions(t, false)

			var commitHashes []string
			for idx, steps := range tc.SetupStages {
				err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					//Apply the setup transformers
					for _, transformer := range steps {
						_, _, _, err := repo.ApplyTransformersInternal(ctx, transaction, transformer)
						if err != nil {
							return err
						}
					}

					ts, err := dbHandler.DBReadTransactionTimestamp(ctx, transaction)
					if err != nil {
						return err
					}

					currentCommitHash := strings.Repeat(strconv.Itoa(idx), 40)
					commitHashes = append(commitHashes, currentCommitHash)

					//Register these timestamps. Essentially we are 'tagging' every step so that we can use them in the release train
					err = dbHandler.DBWriteCommitTransactionTimestamp(ctx, transaction, currentCommitHash, ts.UTC())
					return err
				})

				if err != nil {
					t.Fatalf("Error applying transformers step %d: %v", idx, err)
				}

				time.Sleep(1000 * time.Millisecond) //This is here so that timestamps on sqlite do not collide when multiple stages are involved.
			}

			// Run the Release Train
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				releaseTrain := tc.ReleaseTrain
				releaseTrain.CommitHash = commitHashes[tc.CommitHashIndex]
				_, _, _, err := repo.ApplyTransformersInternal(ctx, transaction, &releaseTrain)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error applying the release train transformer: '%v'", err)
			}

			deployments, err := db.WithTransactionT[[]db.Deployment](dbHandler, ctx, 0, true, func(ctx context.Context, tx *sql.Tx) (*[]db.Deployment, error) {
				var deployments []db.Deployment
				latestDeployments, err := dbHandler.DBSelectAllLatestDeploymentsForApplication(ctx, tx, appName)
				if err != nil {
					return nil, err
				}

				for _, deployment := range latestDeployments {
					deployments = append(deployments, deployment)
				}

				return &deployments, nil
			})
			if err != nil {
				t.Fatalf("Error fetching deployments: %v", err)
			}

			cmpDeployments := func(d1, d2 db.Deployment) bool {
				return d1.Env < d2.Env
			}
			if diff := cmp.Diff(tc.ExpectedDeployments, *deployments, cmpopts.SortSlices(cmpDeployments), cmpopts.IgnoreFields(db.Deployment{}, "Created", "Metadata")); diff != "" {
				t.Errorf("result mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

// DBParseToEvents gets all events from Raw DB data
func DBParseToEvents(rows []db.EventRow) ([]event.Event, error) {
	var result []event.Event
	for _, row := range rows {
		evGo, err := event.UnMarshallEvent(row.EventType, row.EventJson)
		if err != nil {
			return result, fmt.Errorf("Error unmarshalling event: %v\n", err)
		}
		result = append(result, evGo.EventData)
	}
	return result, nil
}
