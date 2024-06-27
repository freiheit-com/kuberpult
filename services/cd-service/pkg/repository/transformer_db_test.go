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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"testing"
	gotime "time"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/time"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/google/go-cmp/cmp"
)

func TestTransformerWritesEslDataRoundTrip(t *testing.T) {
	setupTransformers := []Transformer{
		&CreateEnvironment{
			Authentication: Authentication{},
			Environment:    "dev",
			Config: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCd:           nil,
				EnvironmentGroup: conversion.FromString("mygroup"),
			},
		},
		&CreateEnvironment{
			Authentication: Authentication{},
			Environment:    "staging",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "dev",
					Latest:      false,
				},
				ArgoCd:           nil,
				EnvironmentGroup: conversion.FromString("staging-group"),
			},
		},
		&CreateApplicationVersion{
			Authentication: Authentication{},
			Version:        666,
			Application:    "myapp",
			Manifests: map[string]string{
				"dev": "dev manifest",
			},
			SourceCommitId:  "",
			SourceAuthor:    "",
			SourceMessage:   "",
			Team:            "myteam",
			DisplayVersion:  "",
			WriteCommitData: false,
			PreviousCommit:  "",
		},
		&CreateUndeployApplicationVersion{
			Authentication:  Authentication{},
			Application:     "myapp",
			WriteCommitData: false,
		},
		&CreateEnvironmentLock{
			Authentication: Authentication{},
			Environment:    "dev",
			LockId:         "setup-lock-1",
			Message:        "msg321",
		},
		&CreateEnvironmentTeamLock{
			Authentication: Authentication{},
			Environment:    "dev",
			LockId:         "setup-lock-2",
			Message:        "msg321",
			Team:           "myteam",
		},
		&CreateEnvironmentGroupLock{
			Authentication:   Authentication{},
			LockId:           "setup-lock-3",
			Message:          "msg321",
			EnvironmentGroup: "mygroup",
		},
		&CreateEnvironmentApplicationLock{
			Authentication: Authentication{},
			Environment:    "dev",
			Application:    "myapp",
			LockId:         "setup-lock-4",
			Message:        "msg321",
		},
	}
	tcs := []struct {
		Name              string
		Transformer       Transformer
		expectedEventJson string
		dataType          interface{}
	}{
		// each transformer should appear here once:
		{
			Name: "CreateApplicationVersion",
			Transformer: &CreateApplicationVersion{
				Authentication:  Authentication{},
				Version:         0,
				Application:     "dummy",
				Manifests:       nil,
				SourceCommitId:  "",
				SourceAuthor:    "",
				SourceMessage:   "",
				Team:            "myteam",
				DisplayVersion:  "",
				WriteCommitData: false,
				PreviousCommit:  "",
			},
			dataType: &CreateApplicationVersion{},
		},
		{
			Name: "DeployApplicationVersion",
			Transformer: &DeployApplicationVersion{
				Authentication:  Authentication{},
				Environment:     "dev",
				Application:     "myapp",
				Version:         666,
				LockBehaviour:   0,
				WriteCommitData: false,
				SourceTrain:     nil,
				Author:          "",
			},
			dataType: &DeployApplicationVersion{},
		},
		{
			Name: "CreateUndeployApplicationVersion",
			Transformer: &CreateUndeployApplicationVersion{
				Authentication:  Authentication{},
				Application:     "myapp",
				WriteCommitData: false,
			},
			dataType: &CreateUndeployApplicationVersion{},
		},
		{
			Name: "UndeployApplication",
			Transformer: &UndeployApplication{
				Authentication: Authentication{},
				Application:    "myapp",
			},
			dataType: &UndeployApplication{},
		},
		{
			Name: "DeleteEnvFromApp",
			Transformer: &DeleteEnvFromApp{
				Authentication: Authentication{},
				Application:    "myapp",
				Environment:    "dev",
			},
			dataType: &DeleteEnvFromApp{},
		},
		{
			Name: "CreateEnvironmentLock",
			Transformer: &CreateEnvironmentLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "lock123",
				Message:        "msg321",
			},
			dataType: &CreateEnvironmentLock{},
		},
		{
			Name: "DeleteEnvironmentLock",
			Transformer: &DeleteEnvironmentLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "setup-lock-1",
			},
			dataType: &DeleteEnvironmentLock{},
		},
		{
			Name: "CreateEnvironmentTeamLock",
			Transformer: &CreateEnvironmentTeamLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "dontcare",
				Message:        "msg321",
				Team:           "myteam",
			},
			dataType: &CreateEnvironmentTeamLock{},
		},
		{
			Name: "DeleteEnvironmentTeamLock",
			Transformer: &DeleteEnvironmentTeamLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "setup-lock-2",
				Team:           "myteam",
			},
			dataType: &DeleteEnvironmentTeamLock{},
		},
		{
			Name: "CreateEnvironmentGroupLock",
			Transformer: &CreateEnvironmentGroupLock{
				Authentication:   Authentication{},
				EnvironmentGroup: "mygroup",
				LockId:           "lock123",
				Message:          "msg321",
			},
			dataType: &CreateEnvironmentGroupLock{},
		},
		{
			Name: "DeleteEnvironmentGroupLock",
			Transformer: &DeleteEnvironmentGroupLock{
				Authentication:   Authentication{},
				LockId:           "setup-lock-3",
				EnvironmentGroup: "mygroup",
			},
			dataType: &DeleteEnvironmentGroupLock{},
		},
		{
			Name: "CreateEnvironment",
			Transformer: &CreateEnvironment{
				Authentication: Authentication{},
				Environment:    "temp-env",
				Config: config.EnvironmentConfig{
					Upstream:         nil,
					ArgoCd:           nil,
					EnvironmentGroup: nil,
				},
			},
			dataType: &CreateEnvironment{},
		},
		{
			Name: "CreateEnvironmentApplicationLock",
			Transformer: &CreateEnvironmentApplicationLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "lock123",
				Message:        "msg321",
				Application:    "myapp",
			},
			dataType: &CreateEnvironmentApplicationLock{},
		},
		{
			Name: "DeleteEnvironmentApplicationLock",
			Transformer: &DeleteEnvironmentApplicationLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "setup-lock-4",
				Application:    "myapp",
			},
			dataType: &DeleteEnvironmentApplicationLock{},
		},
		{
			Name: "ReleaseTrain",
			Transformer: &ReleaseTrain{
				Authentication:  Authentication{},
				Target:          "staging",
				Team:            "",
				CommitHash:      "",
				WriteCommitData: false,
				Repo:            nil,
			},
			dataType: &ReleaseTrain{},
		},
	}

	dir, err := testutil.CreateMigrationsPath(2)
	if err != nil {
		t.Fatalf("setup error could not detect dir \n%v", err)
		return
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("detected dir: %s - err=%v", dir, err)
			t.Parallel()
			ctx := testutil.MakeTestContext()
			repo := SetupRepositoryTestWithDBOptions(t, true)
			r := repo.(*repository)
			row := &db.EslEventRow{}
			err = repo.Apply(ctx, setupTransformers...)
			if err != nil {
				t.Errorf("setup error could not set up transformers \n%v", err)
			}

			err = r.DB.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformer)
				if err2 != nil {
					return err2
				}
				tmp, batchErr := r.DB.DBReadEslEventInternal(ctx, transaction, false)
				if batchErr != nil {
					return batchErr
				}
				if tmp == nil && batchErr == nil {
					return errors.New("expected at least one row, but got 0")
				}
				row = tmp
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
			var jsonInterface interface{} = tc.dataType
			err = json.Unmarshal(([]byte)(row.EventJson), &jsonInterface)
			if err != nil {
				t.Fatalf("marshal error: %v\njson: \n%s\n", err, row.EventJson)
			}

			if diff := cmp.Diff(tc.Transformer, jsonInterface, protocmp.Transform()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestEnvLockTransformersWithDB(t *testing.T) {
	const env = envProduction
	const lockID = "l123"
	const message = "my lock"
	tcs := []struct {
		Name                     string
		Transformers             []Transformer
		expectedError            *TransformerBatchApplyError
		expectedCommitMsg        string
		shouldSucceed            bool
		numberExpectedLocks      int
		ExpectedLockIds          []string
		ExpectedEnvironmentLocks []db.EnvironmentLock
	}{
		{
			Name: "Simple Create env lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: env,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironmentLock{
					Environment: env,
					LockId:      lockID,
					Message:     message,
				},
			},
			expectedCommitMsg:   "Created lock " + lockID + " on environment " + env,
			shouldSucceed:       true,
			numberExpectedLocks: 1,
			ExpectedLockIds: []string{
				lockID,
			},
		},
		{
			Name: "Simple Create and Deleted env lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: env,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironmentLock{
					Environment: env,
					LockId:      lockID,
					Message:     message,
				},
				&DeleteEnvironmentLock{
					Environment: env,
					LockId:      lockID,
				},
			},
			expectedCommitMsg:   "Created lock " + lockID + " on environment " + env,
			shouldSucceed:       true,
			numberExpectedLocks: 0,
			ExpectedLockIds:     []string{},
		},
		{
			Name: "Create three env locks and delete one ",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: env,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironmentLock{
					Environment: env,
					LockId:      "l1",
					Message:     message,
				},
				&CreateEnvironmentLock{
					Environment: env,
					LockId:      "l2",
					Message:     message,
				},
				&DeleteEnvironmentLock{
					Environment: env,
					LockId:      "l1",
				},
				&CreateEnvironmentLock{
					Environment: env,
					LockId:      "l3",
					Message:     message,
				},
			},
			expectedCommitMsg:   "Created lock " + lockID + " on environment " + env,
			shouldSucceed:       true,
			numberExpectedLocks: 2,
			ExpectedLockIds: []string{
				"l2", "l3",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			fakeGen := testutil.NewIncrementalUUIDGenerator()
			ctx := testutil.MakeTestContext()
			ctx = AddGeneratorToContext(ctx, fakeGen)
			var repo Repository
			var err error = nil
			repo = SetupRepositoryTestWithDB(t)
			r := repo.(*repository)
			err = r.DB.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				var batchError *TransformerBatchApplyError = nil
				_, _, _, batchError = r.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if batchError != nil {
					return batchError
				}
				return nil
			})
			if err != nil {
				if diff := cmp.Diff(tc.expectedError, err.(*TransformerBatchApplyError), cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				if !tc.shouldSucceed {
					return
				}
			}

			locks, err := db.WithTransactionT(repo.State().DBHandler, ctx, func(ctx context.Context, transaction *sql.Tx) (*db.AllEnvLocksGo, error) {
				return repo.State().DBHandler.DBSelectAllEnvironmentLocks(ctx, transaction, envProduction)
			})

			if locks == nil {
				t.Fatalf("Expected locks but got none")
			}

			if diff := cmp.Diff(tc.numberExpectedLocks, len(locks.EnvLocks)); diff != "" {
				t.Fatalf("error mismatch on number of expected locks (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.ExpectedLockIds, locks.EnvLocks); diff != "" {
				t.Fatalf("error mismatch on expected lock ids (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestTeamLockTransformersWithDB(t *testing.T) {
	const team = "test-team"
	const lockID = "l123"
	const message = "my lock"
	tcs := []struct {
		Name            string
		Transformers    []Transformer
		expectedError   *TransformerBatchApplyError
		shouldSucceed   bool
		ExpectedLockIds []string
	}{
		{
			Name: "Simple Create team lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						envAcceptance: envAcceptance,
					},
					Team:    team,
					Version: 1,
				},
				&CreateEnvironmentTeamLock{
					Environment: envAcceptance,
					LockId:      lockID,
					Message:     message,
					Team:        team,
				},
			},
			shouldSucceed: true,
			ExpectedLockIds: []string{
				lockID,
			},
		},
		{
			Name: "Simple Create and Deleted team lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						envAcceptance: envAcceptance,
					},
					Team:    team,
					Version: 1,
				},
				&CreateEnvironmentTeamLock{
					Environment: envAcceptance,
					LockId:      lockID,
					Message:     message,
					Team:        team,
				},
				&DeleteEnvironmentTeamLock{
					Environment: envAcceptance,
					LockId:      lockID,
					Team:        team,
				},
			},
			shouldSucceed:   true,
			ExpectedLockIds: []string{},
		},
		{
			Name: "Create three team locks and delete one ",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						envAcceptance: envAcceptance,
					},
					Team:    team,
					Version: 1,
				},
				&CreateEnvironmentTeamLock{
					Environment: envAcceptance,
					LockId:      "l1",
					Message:     message,
					Team:        team,
				},
				&CreateEnvironmentTeamLock{
					Environment: envAcceptance,
					LockId:      "l2",
					Message:     message,
					Team:        team,
				},
				&DeleteEnvironmentTeamLock{
					Environment: envAcceptance,
					LockId:      "l1",
					Team:        team,
				},
				&CreateEnvironmentTeamLock{
					Environment: envAcceptance,
					LockId:      "l3",
					Message:     message,
					Team:        team,
				},
			},
			shouldSucceed: true,
			ExpectedLockIds: []string{
				"l2", "l3",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			fakeGen := testutil.NewIncrementalUUIDGenerator()
			ctx := testutil.MakeTestContext()
			ctx = AddGeneratorToContext(ctx, fakeGen)
			var repo Repository
			var err error = nil
			repo = SetupRepositoryTestWithDB(t)
			r := repo.(*repository)
			err = r.DB.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				var batchError *TransformerBatchApplyError = nil
				_, _, _, batchError = r.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if batchError != nil {
					return batchError
				}
				return nil
			})
			if err != nil {
				if diff := cmp.Diff(tc.expectedError, err.(*TransformerBatchApplyError), cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				if !tc.shouldSucceed {
					return
				}
			}

			locks, err := db.WithTransactionT(repo.State().DBHandler, ctx, func(ctx context.Context, transaction *sql.Tx) (*db.AllTeamLocksGo, error) {
				return repo.State().DBHandler.DBSelectAllTeamLocks(ctx, transaction, envAcceptance, team)
			})

			if locks == nil {
				t.Fatalf("Expected locks but got none")
			}

			if diff := cmp.Diff(tc.ExpectedLockIds, locks.TeamLocks); diff != "" {
				t.Fatalf("error mismatch on expected lock ids (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestCreateApplicationVersionDB(t *testing.T) {
	const appName = "app1"
	tcs := []struct {
		Name               string
		Transformers       []Transformer
		expectedDbContent  *db.DBAppWithMetaData
		expectedDbReleases *db.DBAllReleasesWithMetaData
	}{
		{
			Name: "create one version",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     10000,
					Manifests: map[string]string{
						envAcceptance: "{}",
					},
				},
			},
			expectedDbContent: &db.DBAppWithMetaData{
				EslId:       2,
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "",
				},
			},
			expectedDbReleases: &db.DBAllReleasesWithMetaData{
				EslId:   1,
				Created: gotime.Time{},
				App:     appName,
				Metadata: db.DBAllReleaseMetaData{
					Releases: []int64{10000},
				},
			},
		},
		{
			Name: "create two versions, same team",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "{}",
					},
					Team: "noteam",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "{}",
					},
					Team: "noteam",
				},
			},
			expectedDbContent: &db.DBAppWithMetaData{
				EslId:       2, // even when CreateApplicationVersion is called twice, we still write the app only once
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "noteam",
				},
			},
			expectedDbReleases: &db.DBAllReleasesWithMetaData{
				EslId:   2,
				Created: gotime.Time{},
				App:     appName,
				Metadata: db.DBAllReleaseMetaData{
					Releases: []int64{10, 11},
				},
			},
		},
		{
			Name: "create two versions, different teams",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "{}",
					},
					Team: "old",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "{}",
					},
					Team: "new",
				},
			},
			expectedDbContent: &db.DBAppWithMetaData{
				EslId:       3, // CreateApplicationVersion was called twice with different teams, so there's 2 new entries, instead of onc
				App:         appName,
				StateChange: db.AppStateChangeUpdate,
				Metadata: db.DBAppMetaData{
					Team: "new",
				},
			},
			expectedDbReleases: &db.DBAllReleasesWithMetaData{
				EslId:   2,
				Created: gotime.Time{},
				App:     appName,
				Metadata: db.DBAllReleaseMetaData{
					Releases: []int64{10, 11},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				res, err2 := state.DBHandler.DBSelectApp(ctx, transaction, tc.expectedDbContent.App)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				if diff := cmp.Diff(tc.expectedDbContent, res); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				actualRelease, err3 := state.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, appName)
				if err3 != nil {
					return fmt.Errorf("error: %v", err3)
				}
				if diff := cmp.Diff(tc.expectedDbReleases, actualRelease, cmpopts.IgnoreFields(db.DBAllReleasesWithMetaData{}, "Created")); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
			}
		})
	}
}

func TestDeleteQueueApplicationVersion(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedDbContent []db.QueuedDeployment
	}{
		{
			Name: "create one version",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironmentLock{
					Authentication: Authentication{},
					Environment:    envProduction,
					Message:        "don't",
					LockId:         "manual",
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
			},
			expectedDbContent: []db.QueuedDeployment{
				{
					EslVersion: 2,
					Env:        "production",
					App:        "test",
					Version:    nil,
				},
				{
					EslVersion: 1,
					Env:        "production",
					App:        "test",
					Version:    version(1),
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				err2 := state.DeleteQueuedVersion(ctx, transaction, envProduction, "test")
				if err2 != nil {
					t.Fatalf("expected no error, got %v", err2)
				}
				result, err2 := state.DBHandler.DBSelectDeploymentAttemptHistory(ctx, transaction, envProduction, "test", 10)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				if diff := cmp.Diff(tc.expectedDbContent, result, cmpopts.IgnoreFields(db.QueuedDeployment{}, "Created")); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
			}
		})
	}
}
func TestQueueDeploymentTransformer(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedDbContent []db.QueuedDeployment
	}{
		{
			Name: "Test queue deployment through CreateApplicationVersion",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironmentLock{
					Authentication: Authentication{},
					Environment:    envProduction,
					Message:        "don't",
					LockId:         "manual",
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
			},
			expectedDbContent: []db.QueuedDeployment{
				{
					EslVersion: 1,
					Env:        envProduction,
					App:        "test",
					Version:    version(1),
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				result, err2 := state.DBHandler.DBSelectDeploymentAttemptHistory(ctx, transaction, envProduction, "test", 10)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				if diff := cmp.Diff(tc.expectedDbContent, result, cmpopts.IgnoreFields(db.QueuedDeployment{}, "Created")); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
			}
		})
	}
}

func TestCleanupOldVersionDB(t *testing.T) {
	const appName = "app1"
	tcs := []struct {
		Name                   string
		ReleaseVersionLimit    uint
		Transformers           []Transformer
		ExpectedActiveReleases []int64
	}{
		{
			Name:                "Three Versions, Keep 2",
			ReleaseVersionLimit: 2,
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     1,
					Manifests: map[string]string{
						envAcceptance: "{}",
					},
					Team: "myteam",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     2,
					Manifests: map[string]string{
						envAcceptance: "{}",
					},
					Team: "myteam",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     3,
					Manifests: map[string]string{
						envAcceptance: "{}",
					},
					Team: "myteam",
				},
				&DeployApplicationVersion{
					Application: appName,
					Environment: envAcceptance,
					Version:     3,
				},
			},
			ExpectedActiveReleases: []int64{2, 3},
		},
		{
			Name:                "No release is old, but number of releases > ReleaseVersionLimit",
			ReleaseVersionLimit: 2,
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envProduction, Latest: false}},
				},
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     1,
					Manifests: map[string]string{
						envAcceptance: "{}",
						envProduction: "{}",
					},
					Team: "myteam",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     2,
					Manifests: map[string]string{
						envAcceptance: "{}",
						envProduction: "{}",
					},
					Team: "myteam",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     3,
					Manifests: map[string]string{
						envAcceptance: "{}",
						envProduction: "{}",
					},
					Team: "myteam",
				},
				&DeployApplicationVersion{
					Application: appName,
					Environment: envAcceptance,
					Version:     1,
				},
				&DeployApplicationVersion{
					Application: appName,
					Environment: envProduction,
					Version:     3,
				},
			},
			ExpectedActiveReleases: []int64{1, 2, 3},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			repo.(*repository).config.ReleaseVersionsLimit = tc.ReleaseVersionLimit
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				res, err2 := state.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, appName)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				if diff := cmp.Diff(tc.ExpectedActiveReleases, res.Metadata.Releases); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
			}
		})
	}
}

func TestCreateEnvironmentTransformer(t *testing.T) {
	type TestCase struct {
		Name                      string
		Transformers              []Transformer
		expectedEnvironmentConfig map[string]config.EnvironmentConfig
	}

	testCases := []TestCase{
		{
			Name: "create a single environment",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "development",
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
			},
			expectedEnvironmentConfig: map[string]config.EnvironmentConfig{
				"development": testutil.MakeEnvConfigLatest(nil),
			},
		},
		{
			Name: "create a single environment twice",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigUpstream("development", nil),
				},
			},
			expectedEnvironmentConfig: map[string]config.EnvironmentConfig{
				"staging": testutil.MakeEnvConfigUpstream("development", nil),
			},
		},
		{
			Name: "create multiple environments",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "development",
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigUpstream("development", nil),
				},
			},
			expectedEnvironmentConfig: map[string]config.EnvironmentConfig{
				"development": testutil.MakeEnvConfigLatest(nil),
				"staging":     testutil.MakeEnvConfigUpstream("development", nil),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				result, err2 := state.GetAllEnvironmentConfigs(ctx, transaction)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				if diff := cmp.Diff(tc.expectedEnvironmentConfig, result, cmpopts.IgnoreFields(db.QueuedDeployment{}, "Created")); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
			}
		})
	}
}

func TestEventGenerationFromTransformers(t *testing.T) {
	type TestCase struct {
		Name                      string
		Transformers              []Transformer
		expectedEnvironmentConfig map[string]config.EnvironmentConfig
	}

	testCases := []TestCase{
		{
			Name: "create a single environment",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "development",
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
			},
			expectedEnvironmentConfig: map[string]config.EnvironmentConfig{
				"development": testutil.MakeEnvConfigLatest(nil),
			},
		},
		{
			Name: "create a single environment twice",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigUpstream("development", nil),
				},
			},
			expectedEnvironmentConfig: map[string]config.EnvironmentConfig{
				"staging": testutil.MakeEnvConfigUpstream("development", nil),
			},
		},
		{
			Name: "create multiple environments",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "development",
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigUpstream("development", nil),
				},
			},
			expectedEnvironmentConfig: map[string]config.EnvironmentConfig{
				"development": testutil.MakeEnvConfigLatest(nil),
				"staging":     testutil.MakeEnvConfigUpstream("development", nil),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				result, err2 := state.GetAllEnvironmentConfigs(ctx, transaction)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				if diff := cmp.Diff(tc.expectedEnvironmentConfig, result, cmpopts.IgnoreFields(db.QueuedDeployment{}, "Created")); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
			}
		})
	}
}

func TestEvents(t *testing.T) {
	type TestCase struct {
		Name             string
		Transformers     []Transformer
		expectedDBEvents []event.Event
	}

	tcs := []TestCase{
		{
			Name: "Create a single application version and deploy it with DB",
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
					Application:      "app",
					Environment:      "staging",
					WriteCommitData:  true,
					Version:          1,
					TransformerEslID: 1,
				},
			},
			expectedDBEvents: []event.Event{
				&event.Deployment{
					Application: "app",
					Environment: "staging",
				},
			},
		},
		{
			Name: "Create a single application version and get deployment locked with DB",
			// no need to bother with environments here
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "dev",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&CreateEnvironmentLock{
					Environment: "dev",
					LockId:      "my-lock",
					Message:     "my-message",
				},
				&CreateApplicationVersion{ //This will create a
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[string]string{
						"dev": "doesn't matter",
					},
					WriteCommitData:  true,
					Version:          1,
					TransformerEslID: 1,
				},
			},
			expectedDBEvents: []event.Event{
				&event.LockPreventedDeployment{
					Application: "app",
					Environment: "dev",
					LockType:    "environment",
					LockMessage: "my-message",
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
			var repo Repository
			var err error = nil
			repo = SetupRepositoryTestWithDB(t)
			r := repo.(*repository)
			err = r.DB.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				var batchError *TransformerBatchApplyError = nil
				_, _, _, batchError = r.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if batchError != nil {
					// Note that we cannot just `return err2` here,
					// because it's a "TransformerBatchApplyError", not an "error"
					return batchError
				}

				rows, err := repo.State().DBHandler.DBSelectAllEventsForCommit(ctx, transaction, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != len(tc.expectedDBEvents) {
					t.Fatalf("error event count mismatch expected '%d' events but got '%d'\n", len(tc.expectedDBEvents), len(rows))
				}
				dEvents, err := DBParseToEvents(rows)
				if err != nil {
					t.Fatalf("encountered error but no error is expected here: %v", err)
				}
				if len(dEvents) != len(tc.expectedDBEvents) {
					t.Fatalf("error event count mismatch expected '%d' events but got '%d'\n", len(tc.expectedDBEvents), len(rows))
				}

				return nil

			})
			if err != nil {
				t.Fatalf("encountered error but no error is expected here: '%v'", err)
			}
		})
	}
}

func version(v int) *int64 {
	var result = int64(v)
	return &result
}
