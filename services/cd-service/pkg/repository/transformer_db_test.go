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
	"github.com/freiheit-com/kuberpult/pkg/types"
	"regexp"
	"sort"
	"testing"
	gotime "time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/lib/pq"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/time"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	testAppName     = "test"
	nextTestAppName = "test2"
)

func TestTransformerWritesEslDataRoundTrip(t *testing.T) {
	var lifeTime2d = "2d"
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
			Manifests: map[types.EnvName]string{
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
			Authentication:    Authentication{},
			Environment:       "dev",
			LockId:            "setup-lock-1",
			Message:           "msg321",
			SuggestedLifeTime: &lifeTime2d,
		},
		&CreateEnvironmentTeamLock{
			Authentication: Authentication{},
			Environment:    "dev",
			LockId:         "setup-lock-2",
			Message:        "msg321",
			Team:           "myteam",
		},
		&CreateEnvironmentGroupLock{
			Authentication:    Authentication{},
			LockId:            "setup-lock-3",
			Message:           "msg321",
			EnvironmentGroup:  "mygroup",
			SuggestedLifeTime: &lifeTime2d,
		},
		&CreateEnvironmentApplicationLock{
			Authentication:    Authentication{},
			Environment:       "dev",
			Application:       "myapp",
			LockId:            "setup-lock-4",
			Message:           "msg321",
			SuggestedLifeTime: &lifeTime2d,
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
				Version:         1,
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

	dir, err := db.CreateMigrationsPath(2)
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
			repo, _ := SetupRepositoryTestWithDBOptions(t, false)
			r := repo.(*repository)
			row := &db.EslEventRow{}
			err = repo.Apply(ctx, setupTransformers...)
			if err != nil {
				t.Errorf("setup error could not set up transformers \n%v", err)
			}

			err = r.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
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
			var jsonInterface = tc.dataType
			err = json.Unmarshal(([]byte)(row.EventJson), &jsonInterface)
			if err != nil {
				t.Fatalf("marshal error: %v\njson: \n%s", err, row.EventJson)
			}
			tc.Transformer.SetEslVersion(0) // the eslVersion is not part of the json blob anymore
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
	var lifeTime2d = "2d"
	var lifeTime4h = "4h"
	tcs := []struct {
		Name                     string
		Transformers             []Transformer
		expectedError            *TransformerBatchApplyError
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
					Environment:       env,
					LockId:            lockID,
					Message:           message,
					SuggestedLifeTime: &lifeTime2d,
				},
				&DeleteEnvironmentLock{
					Environment: env,
					LockId:      lockID,
				},
			},
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
					Environment:       env,
					LockId:            "l2",
					Message:           message,
					SuggestedLifeTime: &lifeTime4h,
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
			var err error
			repo = SetupRepositoryTestWithDB(t)
			r := repo.(*repository)
			err = r.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				var batchError *TransformerBatchApplyError
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

			locks, err := db.WithTransactionT(repo.State().DBHandler, ctx, db.DefaultNumRetries, false, func(ctx context.Context, transaction *sql.Tx) (*db.AllEnvLocksGo, error) {
				return repo.State().DBHandler.DBSelectAllEnvironmentLocks(ctx, transaction, envProduction)
			})
			if err != nil {
				t.Fatalf("unexpected error selecting env locks: %v", err)
			}

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
	var lifeTime2d = "2d"
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
					Manifests: map[types.EnvName]string{
						envAcceptance: envAcceptance,
					},
					Team:    team,
					Version: 1,
				},
				&CreateEnvironmentTeamLock{
					Environment:       envAcceptance,
					LockId:            lockID,
					Message:           message,
					Team:              team,
					SuggestedLifeTime: &lifeTime2d,
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
					Manifests: map[types.EnvName]string{
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
					Manifests: map[types.EnvName]string{
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
			var err error
			repo = SetupRepositoryTestWithDB(t)
			r := repo.(*repository)
			err = r.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				var batchError *TransformerBatchApplyError
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

			locks, err := db.WithTransactionT(repo.State().DBHandler, ctx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*[]string, error) {
				locks, err := repo.State().DBHandler.DBSelectAllTeamLocks(ctx, transaction, envAcceptance, team)
				return &locks, err
			})
			if err != nil {
				t.Fatalf("unexpected error selecting team locks: %v", err)
			}

			if locks == nil {
				t.Fatalf("Expected locks but got none")
			}

			if diff := cmp.Diff(tc.ExpectedLockIds, *locks); diff != "" {
				t.Fatalf("error mismatch on expected lock ids (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestCreateApplicationVersionDBRevisions(t *testing.T) {
	const appName = "app1"
	tcs := []struct {
		Name               string
		Transformers       []Transformer
		expectedDbContent  *db.DBAppWithMetaData
		expectedDbReleases []types.ReleaseNumbers
	}{
		{
			Name: "create two identical versions with different revision",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Revision:    1,
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "t1",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Revision:    2,
					Manifests: map[types.EnvName]string{
						envAcceptance: "{test}",
					},
					Team: "t1",
				},
			},
			expectedDbContent: &db.DBAppWithMetaData{
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "t1",
				},
			},
			expectedDbReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumbers(10, 2),
				types.MakeReleaseNumbers(10, 1),
			},
		},
		{
			Name: "create two identical versions with different revision - invert order of creation",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Revision:    2,
					Manifests: map[types.EnvName]string{
						envAcceptance: "{test}",
					},
					Team: "t1",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Revision:    1,
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "t1",
				},
			},
			expectedDbContent: &db.DBAppWithMetaData{
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "t1",
				},
			},
			expectedDbReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumbers(10, 2),
				types.MakeReleaseNumbers(10, 1),
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
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
				actualRelease, err3 := state.DBHandler.DBSelectAllReleaseNumbersOfApp(ctx, transaction, appName)
				if err3 != nil {
					return fmt.Errorf("error: %v", err3)
				}
				if diff := cmp.Diff(tc.expectedDbReleases, actualRelease); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				environment, err4 := state.DBHandler.DBSelectEnvironment(ctx, transaction, "acceptance")
				if err4 != nil {
					return fmt.Errorf("error retrieving environment: %w", err)
				}
				if diff := cmp.Diff([]string{appName}, environment.Applications); diff != "" {
					t.Errorf("environment applications list mismatch: (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
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
		expectedDbReleases []types.ReleaseNumbers
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
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
				},
			},
			expectedDbContent: &db.DBAppWithMetaData{
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "",
				},
			},
			expectedDbReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumbers(10000, 0),
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
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "noteam",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "noteam",
				},
			},
			expectedDbContent: &db.DBAppWithMetaData{
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "noteam",
				},
			},
			expectedDbReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(11),
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
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "old",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "new",
				},
			},
			expectedDbContent: &db.DBAppWithMetaData{
				App:         appName,
				StateChange: db.AppStateChangeUpdate,
				Metadata: db.DBAppMetaData{
					Team: "new",
				},
			},
			expectedDbReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(11),
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
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
				if diff := cmp.Diff(tc.expectedDbReleases, actualRelease); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				environment, err4 := state.DBHandler.DBSelectEnvironment(ctx, transaction, "acceptance")
				if err4 != nil {
					return fmt.Errorf("error retrieving environment: %w", err)
				}
				if diff := cmp.Diff([]string{appName}, environment.Applications); diff != "" {
					t.Errorf("environment applications list mismatch: (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
			}
		})
	}
}

func TestMinorFlag(t *testing.T) {
	appName := "app"
	tcs := []struct {
		Name           string
		Transformers   []Transformer
		ExpectedMinors []types.ReleaseNumbers
		ExpectedMajors []types.ReleaseNumbers
		MinorRegexes   []*regexp.Regexp
	}{
		{
			Name: "No previous or next releases",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []types.ReleaseNumbers{},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
			},
		},
		{
			Name: "No next Release, Previous Releases manifest equals current releases",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(11),
			},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
			},
		},
		{
			Name: "No next Release, Previous Releases Manifest does not equal current's",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2",
					},
				},
			},
			ExpectedMinors: []types.ReleaseNumbers{},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(11),
			},
		},
		{
			Name: "No prev Release, next Releases Manifest equals current's",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(11),
			},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
			},
		},
		{
			Name: "No prev Release, next Releases Manifest does not equal current's",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []types.ReleaseNumbers{},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(11),
			},
		},
		{
			Name: "prev, next, and current are not equal",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest3",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2",
					},
				},
			},
			ExpectedMinors: []types.ReleaseNumbers{},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(11),
				types.MakeReleaseNumberVersion(12),
			},
		},
		{
			Name: "prev and current are equal but not next",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
						"new env":     "new manifest",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(11),
			},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(12),
			},
		},
		{
			Name: "prev and next are equal but not current",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2",
					},
				},
			},
			ExpectedMinors: []types.ReleaseNumbers{},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(11),
				types.MakeReleaseNumberVersion(12),
			},
		},
		{
			Name: "current and next are equal but not prev",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2",
						"new env":     "new manifest",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2",
					},
				},
			},
			ExpectedMinors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(12),
			},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(11),
			},
		},
		{
			Name: "all equal",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(12),
				types.MakeReleaseNumberVersion(11),
			},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
			},
		},
		{
			Name: "With Regex, all manifests are equal",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest3",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2",
					},
				},
			},
			MinorRegexes: []*regexp.Regexp{regexp.MustCompile(".*manifest.*")},
			ExpectedMinors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(11),
				types.MakeReleaseNumberVersion(12),
			},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
			},
		},
		{
			Name: "Multiple Regexes",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1\nfirstLine1\nsecondLine1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2\nfirstLine3\nsecondLine3",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2\nfirstLine2\nsecondLine2",
					},
				},
			},
			MinorRegexes: []*regexp.Regexp{regexp.MustCompile(".*firstLine.*"), regexp.MustCompile(".*secondLine.*")},
			ExpectedMinors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(12),
			},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(11),
			},
		},
		{
			Name: "Multiple Regexes and one of them do not match",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest1\nfirstLine1\nsecondLine1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2\nfirstLine3\nsecondLine3",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "manifest2\nfirstLine2\nsecondLine2",
					},
				},
			},
			MinorRegexes:   []*regexp.Regexp{regexp.MustCompile(".*firstLine.*"), regexp.MustCompile(".*ItDoesNotMatch.*")},
			ExpectedMinors: []types.ReleaseNumbers{},
			ExpectedMajors: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(11),
				types.MakeReleaseNumberVersion(12),
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t).(*repository)
			repo.config.MinorRegexes = tc.MinorRegexes
			err3 := repo.State().DBHandler.WithTransactionR(ctxWithTime, 0, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err := repo.ApplyTransformersInternal(ctx, transaction, &CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				})
				if err != nil {
					return err
				}
				_, _, _, err = repo.ApplyTransformersInternal(ctx, transaction, &CreateEnvironment{
					Environment: "new env",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				})
				if err != nil {
					return err
				}
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				for _, minorVersion := range tc.ExpectedMinors {
					release, err := state.DBHandler.DBSelectReleaseByVersion(ctxWithTime, transaction, appName, minorVersion, true)
					if err != nil {
						return err
					}
					if !release.Metadata.IsMinor {
						t.Errorf("Expected release %v to be minor but its major", minorVersion)
					}
				}
				for _, majorVersion := range tc.ExpectedMajors {
					release, err := state.DBHandler.DBSelectReleaseByVersion(ctxWithTime, transaction, appName, majorVersion, true)
					if err != nil {
						return err
					}
					if release.Metadata.IsMinor {
						t.Errorf("Expected release %v to be major but its minor", majorVersion)
					}
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
			}
		})
	}
}

func TestFilterManifestLines(t *testing.T) {
	tcs := []struct {
		Name           string
		StartingString string
		Regexes        []*regexp.Regexp
		ExpectedResult []string
	}{
		{
			Name:           "Simple Use Case",
			StartingString: "line1\nline2\nline3\nline4\nline5\nline5\nline6",
			Regexes: []*regexp.Regexp{
				regexp.MustCompile(".*1$"),
				regexp.MustCompile(".*5.*"),
				regexp.MustCompile("line3"),
				regexp.MustCompile(".*8.*"),
				regexp.MustCompile(".*DoesNOTMatch.*"),
			},
			ExpectedResult: []string{
				"line2",
				"line4",
				"line6",
			},
		},
		{
			Name:           "Empty string",
			StartingString: "",
			Regexes: []*regexp.Regexp{
				regexp.MustCompile("^.*testRegex$"),
			},
			ExpectedResult: []string{""},
		},
		{
			Name:           "Empty list of regexes",
			StartingString: "line1\nline2\nline3",
			Regexes:        []*regexp.Regexp{},
			ExpectedResult: []string{
				"line1",
				"line2",
				"line3",
			},
		},
		{
			Name:           "All lines match",
			StartingString: "line1\nline2\nline3",
			Regexes: []*regexp.Regexp{
				regexp.MustCompile("^line1$"),
				regexp.MustCompile(".*2.*"),
				regexp.MustCompile("^line3.*"),
			},
			ExpectedResult: []string{},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			filteredLines := filterManifestLines(ctx, tc.StartingString, tc.Regexes)
			if diff := cmp.Diff(tc.ExpectedResult, filteredLines); diff != "" {
				t.Errorf("error mismatch in filtered lines (-want, +got):\n%s", diff)
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
					Application: testAppName,
					Manifests: map[types.EnvName]string{
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
					App:        testAppName,
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  nil,
						Revision: 0,
					},
				},
				{
					EslVersion:     1,
					Env:            "production",
					App:            testAppName,
					ReleaseNumbers: types.MakeReleaseNumberVersion(1),
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
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				err2 := state.DeleteQueuedVersion(ctx, transaction, envProduction, testAppName)
				if err2 != nil {
					t.Fatalf("expected no error, got %v", err2)
				}
				result, err2 := state.DBHandler.DBSelectDeploymentAttemptHistory(ctx, transaction, envProduction, testAppName, 10)
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
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
			},
			expectedDbContent: []db.QueuedDeployment{
				{
					EslVersion:     1,
					Env:            envProduction,
					App:            testAppName,
					ReleaseNumbers: types.MakeReleaseNumberVersion(1),
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
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				result, err2 := state.DBHandler.DBSelectDeploymentAttemptHistory(ctx, transaction, envProduction, testAppName, 10)
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
		ExpectedActiveReleases []types.ReleaseNumbers
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
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "myteam",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     2,
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "myteam",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     3,
					Manifests: map[types.EnvName]string{
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
			ExpectedActiveReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(2),
				types.MakeReleaseNumberVersion(3),
			},
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
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
						envProduction: "{}",
					},
					Team: "myteam",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     2,
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
						envProduction: "{}",
					},
					Team: "myteam",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     3,
					Manifests: map[types.EnvName]string{
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
			ExpectedActiveReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(1),
				types.MakeReleaseNumberVersion(2),
				types.MakeReleaseNumberVersion(3),
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			repo.(*repository).config.ReleaseVersionsLimit = tc.ReleaseVersionLimit
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				res, err2 := state.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, appName)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				if diff := cmp.Diff(tc.ExpectedActiveReleases, res); diff != "" {
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
		expectedEnvironmentConfig map[types.EnvName]config.EnvironmentConfig
		expectedStagingEnvApps    []string
	}

	testCases := []TestCase{
		{
			Name: "create a single environment",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
			},
			expectedEnvironmentConfig: map[types.EnvName]config.EnvironmentConfig{
				"staging": testutil.MakeEnvConfigLatest(nil),
			},
			expectedStagingEnvApps: []string{},
		},
		{
			Name: "create a single environment twice",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigLatest(nil),
				},
				&CreateApplicationVersion{
					Application: "testapp",
					Version:     1,
					Manifests: map[types.EnvName]string{
						"staging": "staging-manifest",
					},
				},
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigUpstream("development", nil),
				},
			},
			expectedEnvironmentConfig: map[types.EnvName]config.EnvironmentConfig{
				"staging": testutil.MakeEnvConfigUpstream("development", nil),
			},
			expectedStagingEnvApps: []string{"testapp"},
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
			expectedEnvironmentConfig: map[types.EnvName]config.EnvironmentConfig{
				"development": testutil.MakeEnvConfigLatest(nil),
				"staging":     testutil.MakeEnvConfigUpstream("development", nil),
			},
			expectedStagingEnvApps: []string{},
		},
		{
			Name: "create environment with argo cd configs",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						ArgoCdConfigs: testutil.MakeArgoCDConfigs("CN", "DE", 2),
					},
				},
				&CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						ArgoCdConfigs: testutil.MakeArgoCDConfigs("CN-STG", "PT", 100),
					},
				},
			},
			expectedEnvironmentConfig: map[types.EnvName]config.EnvironmentConfig{
				"development": {
					ArgoCdConfigs: testutil.MakeArgoCDConfigs("CN", "DE", 2),
				},
				"staging": {
					ArgoCdConfigs: testutil.MakeArgoCDConfigs("CN-STG", "PT", 100),
				},
			},
			expectedStagingEnvApps: []string{},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
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
				env, err2 := state.DBHandler.DBSelectEnvironment(ctx, transaction, "staging")
				if err2 != nil {
					return err2
				}
				if diff := cmp.Diff(tc.expectedStagingEnvApps, env.Applications); diff != "" {
					t.Errorf("error mismatch staging env apps (-want, +got):\n%s", diff)
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
		expectedEnvironmentConfig map[types.EnvName]config.EnvironmentConfig
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
			expectedEnvironmentConfig: map[types.EnvName]config.EnvironmentConfig{
				"development": testutil.MakeEnvConfigLatest(nil),
			},
		},
		{
			Name: "create a single environment twice: second one should overwrite first one",
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
			expectedEnvironmentConfig: map[types.EnvName]config.EnvironmentConfig{
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
			expectedEnvironmentConfig: map[types.EnvName]config.EnvironmentConfig{
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
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
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
		testPageSize     bool
	}

	tcs := []TestCase{
		{
			Name: "check if the number of events is equal to pageNumber plus pageSize",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"staging": "doesn't matter",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Application:           "app",
					Environment:           "staging",
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
				},
			},
			testPageSize: true,
			expectedDBEvents: []event.Event{
				&event.NewRelease{Environments: map[string]struct{}{"staging": {}}},
			},
		},
		{
			Name: "Create a single application version and deploy it with DB",
			Transformers: []Transformer{
				&CreateEnvironment{
					Authentication: Authentication{},
					Environment:    "staging",
					Config: config.EnvironmentConfig{
						Upstream:         nil,
						ArgoCd:           nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"staging": "doesn't matter",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Application:           "app",
					Environment:           "staging",
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
				},
			},
			expectedDBEvents: []event.Event{
				&event.NewRelease{Environments: map[string]struct{}{"staging": {}}},
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
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"dev": "doesn't matter",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
				},
			},
			expectedDBEvents: []event.Event{
				&event.NewRelease{Environments: map[string]struct{}{"dev": {}}},
				&event.LockPreventedDeployment{
					Application: "app",
					Environment: "dev",
					LockType:    "environment",
					LockMessage: "my-message",
				},
			},
		},
		{
			Name: "Replaced By test",
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
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"dev": "doesn't matter",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					Manifests: map[types.EnvName]string{
						"dev": "doesn't matter",
					},
					WriteCommitData:       true,
					Version:               2,
					TransformerEslVersion: 1,
					PreviousCommit:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
			},
			expectedDBEvents: []event.Event{
				&event.NewRelease{Environments: map[string]struct{}{"dev": {}}},
				&event.Deployment{
					Application: "app",
					Environment: "dev",
				},
				&event.ReplacedBy{Application: "app", Environment: "dev", CommitIDtoReplace: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
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
			var err error
			repo = SetupRepositoryTestWithDB(t)
			r := repo.(*repository)
			err = r.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				var batchError *TransformerBatchApplyError
				_, _, _, batchError = r.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if batchError != nil {
					return batchError
				}
				pageSize := 100
				if tc.testPageSize {
					pageSize = 0
					// we use 0 instead of 1 because the db queries for pagesize plus 1
				}
				rows, err := repo.State().DBHandler.DBSelectAllEventsForCommit(ctx, transaction, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 0, uint64(pageSize))
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != len(tc.expectedDBEvents) {
					t.Fatalf("error event count mismatch expected '%d' events but got '%d' rows:\n%v", len(tc.expectedDBEvents), len(rows), rows)
				}
				dEvents, err := DBParseToEvents(rows)
				if err != nil {
					t.Fatalf("encountered error but no error is expected here: %v", err)
				}
				for _, ev := range dEvents { //Events are not sortable. We need to check each one
					for idx, expected := range tc.expectedDBEvents {
						diff := cmp.Diff(expected, ev)
						if diff == "" {
							break
						}
						if idx == len(tc.expectedDBEvents)-1 {
							t.Errorf("error mismatch (-want, +got):\n%s", cmp.Diff(dEvents, tc.expectedDBEvents))
						}
					}
				}

				return nil

			})
			if err != nil {
				t.Fatalf("encountered error but no error is expected here: '%v'", err)
			}
		})
	}
}

func TestDeleteEnvFromAppWithDB(t *testing.T) {
	appName := "app"
	setupTransformers := []Transformer{
		&CreateEnvironment{
			Environment: "env",
			Config: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCd:           nil,
				EnvironmentGroup: conversion.FromString("mygroup"),
			},
		},
		&CreateEnvironment{
			Environment: "env1",
			Config: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCd:           nil,
				EnvironmentGroup: conversion.FromString("mygroup"),
			},
		},
		&CreateEnvironment{
			Environment: "env2",
			Config: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCd:           nil,
				EnvironmentGroup: conversion.FromString("mygroup"),
			},
		},
		&CreateEnvironment{
			Environment: "env3",
			Config: config.EnvironmentConfig{
				Upstream:         nil,
				ArgoCd:           nil,
				EnvironmentGroup: conversion.FromString("mygroup"),
			},
		},
	}
	tcs := []struct {
		Name              string
		Transforms        []Transformer
		ExpectedManifests map[types.EnvName]string
	}{
		{
			Name: "Simple Delete Env From App",
			Transforms: []Transformer{
				&CreateApplicationVersion{
					Version:     10,
					Application: appName,
					Manifests: map[types.EnvName]string{
						"env":  "testenvmanifest",
						"env2": "testenvmanifest2",
					},
					SourceCommitId: "0000000000000000000000000000000000000000",
					SourceAuthor:   "testmail@example.com",
					SourceMessage:  "test",
					DisplayVersion: "10",
				},
				&DeleteEnvFromApp{
					Application: appName,
					Environment: "env",
				},
			},
			ExpectedManifests: map[types.EnvName]string{
				"env2": "testenvmanifest2",
			},
		},
		{
			Name: "Delete Env that doesn't exist",
			Transforms: []Transformer{
				&CreateApplicationVersion{
					Version:     10,
					Application: appName,
					Manifests: map[types.EnvName]string{
						"env":  "testenvmanifest",
						"env2": "testenvmanifest2",
					},
					SourceCommitId: "0000000000000000000000000000000000000000",
					SourceAuthor:   "testmail@example.com",
					SourceMessage:  "test",
					DisplayVersion: "10",
				},
				&DeleteEnvFromApp{
					Application: appName,
					Environment: "env3",
				},
			},
			ExpectedManifests: map[types.EnvName]string{
				"env":  "testenvmanifest",
				"env2": "testenvmanifest2",
			},
		},
		{
			Name: "Multiple Manifests",
			Transforms: []Transformer{
				&CreateApplicationVersion{
					Version:     10,
					Application: appName,
					Manifests: map[types.EnvName]string{
						"env":  "testenvmanifest",
						"env2": "testenvmanifest2",
					},
					SourceCommitId: "0000000000000000000000000000000000000000",
					SourceAuthor:   "testmail@example.com",
					SourceMessage:  "test",
					DisplayVersion: "10",
				},
				&CreateApplicationVersion{
					Version:     11,
					Application: appName,
					Manifests: map[types.EnvName]string{
						"env":  "testenvmanifest",
						"env2": "testenvmanifest2",
					},
					SourceCommitId: "0000000000000000000000000000000000000000",
					SourceAuthor:   "testmail@example.com",
					SourceMessage:  "test",
					DisplayVersion: "10",
				},
				&DeleteEnvFromApp{
					Application: appName,
					Environment: "env",
				},
			},
			ExpectedManifests: map[types.EnvName]string{
				"env2": "testenvmanifest2",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			err := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err := repo.ApplyTransformersInternal(ctx, transaction, setupTransformers...)
				if err != nil {
					return err
				}
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transforms...)
				if err != nil {
					return fmt.Errorf("error: %v", err)
				}
				releases, err2 := state.DBHandler.DBSelectReleasesByAppLatestEslVersion(ctx, transaction, appName, true)
				if err2 != nil {
					return fmt.Errorf("error retrieving release: %v", err2)
				}
				for _, release := range releases {
					for env, manifest := range tc.ExpectedManifests {
						if diff := cmp.Diff(manifest, release.Manifests.Manifests[env]); diff != "" {
							return fmt.Errorf("error mismatch Manifests - want, +got:\n%s", diff)
						}
					}
				}
				environment, err2 := state.DBHandler.DBSelectEnvironment(ctx, transaction, types.EnvName(tc.Transforms[len(tc.Transforms)-1].(*DeleteEnvFromApp).Environment))
				if err2 != nil {
					return err2
				}
				if environment != nil {
					for _, envApp := range environment.Applications {
						if envApp == appName {
							return fmt.Errorf("Expected app %s to be deleted from environment %s", appName, environment.Name)
						}
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestReleaseTrain(t *testing.T) {
	tcs := []struct {
		Name                 string
		ReleaseVersionsLimit uint
		Transformers         []Transformer
		ExpectedVersion      types.ReleaseNumbers
		TargetEnv            types.EnvName
		TargetApp            string
	}{
		{
			Name:            "Release train",
			ExpectedVersion: types.MakeReleaseNumberVersion(2),
			TargetEnv:       envProduction,
			TargetApp:       testAppName,
			Transformers: []Transformer{
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
							Environment: envAcceptance,
							Latest:      true,
						},
					},
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment: envProduction,
					Application: testAppName,
					Version:     1,
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData: true,
					Version:         2,
				},
				&DeployApplicationVersion{
					Environment: envAcceptance,
					Application: testAppName,
					Version:     1,
				},
				&DeployApplicationVersion{
					Environment: envAcceptance,
					Application: testAppName,
					Version:     2,
				},
				&ReleaseTrain{
					Target: envProduction,
				},
			},
		},
		{
			Name:            "Release train from Latest",
			ExpectedVersion: types.MakeReleaseNumberVersion(2),
			TargetEnv:       envAcceptance,
			TargetApp:       testAppName,
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData:       true,
					Version:               2,
					TransformerEslVersion: 0,
				},
				&DeployApplicationVersion{
					Application:           testAppName,
					Environment:           envAcceptance,
					Version:               1,
					WriteCommitData:       true,
					TransformerEslVersion: 0,
				},
				&ReleaseTrain{
					Target:                envAcceptance,
					TransformerEslVersion: 0,
				},
			},
		},
		{
			Name:            "Release train for a Team",
			ExpectedVersion: types.MakeReleaseNumberVersion(2),
			TargetApp:       "test-my-app",
			TargetEnv:       envProduction,
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance, // train drives from acceptance to production
						},
					},
					TransformerEslVersion: 0,
				},
				&CreateEnvironment{
					Environment: envAcceptance,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance,
							Latest:      true,
						},
					},
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: "test-my-app",
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					Team:                  testAppName,
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 0,
				},
				&DeployApplicationVersion{
					Environment:           envProduction,
					Application:           "test-my-app",
					Version:               1,
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: "test-my-app",
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData:       true,
					Version:               2,
					TransformerEslVersion: 0,
					Team:                  testAppName,
				},
				&DeployApplicationVersion{
					Environment:           envAcceptance,
					Application:           "test-my-app",
					Version:               1,
					TransformerEslVersion: 0,
				},
				&DeployApplicationVersion{
					Environment:           envAcceptance,
					Application:           "test-my-app",
					Version:               2,
					TransformerEslVersion: 0,
				},
				&ReleaseTrain{
					Target:                envProduction,
					Team:                  testAppName,
					TransformerEslVersion: 0,
				},
			},
		},
		{
			Name:            "Release train - Test revision",
			ExpectedVersion: types.MakeReleaseNumbers(1, 1),
			TargetApp:       "test-my-app",
			TargetEnv:       envProduction,
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance, // train drives from acceptance to production
						},
					},
					TransformerEslVersion: 0,
				},
				&CreateEnvironment{
					Environment: envAcceptance,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance,
							Latest:      true,
						},
					},
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: "test-my-app",
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					Team:                  testAppName,
					WriteCommitData:       true,
					Version:               1,
					Revision:              0,
					TransformerEslVersion: 0,
				},
				&DeployApplicationVersion{
					Environment:           envProduction,
					Application:           "test-my-app",
					Version:               1,
					Revision:              0,
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: "test-my-app",
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					Revision:              1,
					TransformerEslVersion: 0,
					Team:                  testAppName,
				},
				&DeployApplicationVersion{
					Environment:           envAcceptance,
					Application:           "test-my-app",
					Version:               1,
					Revision:              1,
					TransformerEslVersion: 0,
				},
				&DeployApplicationVersion{
					Environment:           envAcceptance,
					Application:           "test-my-app",
					Version:               1,
					Revision:              1,
					TransformerEslVersion: 0,
				},
				&ReleaseTrain{
					Target:                envProduction,
					Team:                  testAppName,
					TransformerEslVersion: 0,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			for _, transformer := range tc.Transformers {
				err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					_, _, _, err2 := r.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, transformer)
					if err2 != nil {
						return err2
					}
					return nil
				})
				if err != nil {
					t.Fatalf("Err: %v", err)
				}
			}

			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				deployment, dplError := r.State().DBHandler.DBSelectLatestDeployment(ctx, transaction, tc.TargetApp, tc.TargetEnv)
				if dplError != nil {
					return dplError
				}

				if deployment == nil {
					t.Fatalf("Expected deployment but none was found.")
				}
				if deployment.ReleaseNumbers.Version == nil {
					t.Fatalf("Expected deployment version, but got nil.")

				}
				if diff := cmp.Diff(tc.ExpectedVersion, deployment.ReleaseNumbers); diff != "" {
					t.Fatalf("deployed version mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Err: %v", err)
			}
		})
	}
}

func TestDeleteEnvironmentDBState(t *testing.T) {

	// Env removed from all Envs
	// Env removed from all releases of that app
	type TestCase struct {
		Name                  string
		Transformers          []Transformer
		expectedLatestRelease map[string]db.DBReleaseWithMetaData
		expectedAllEnvs       []types.EnvName
	}

	tcs := []TestCase{
		{
			Name: "remove env with deployed app",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"staging": "doesn't matter",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeleteEnvironment{
					Environment: "staging",
				},
			},
			expectedLatestRelease: map[string]db.DBReleaseWithMetaData{
				"app": {
					App:            "app",
					ReleaseNumbers: types.MakeReleaseNumberVersion(1),
					Manifests: db.DBReleaseManifests{
						Manifests: map[types.EnvName]string{},
					},
					Environments: []types.EnvName{},
				},
			},
			expectedAllEnvs: []types.EnvName{},
		},
		{
			Name: "multiple envs",
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
							Latest:      false,
							Environment: "dev",
						},
					},
				},
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"staging": "doesn't matter",
						"dev":     "doesn't matter",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeleteEnvironment{
					Environment: "staging",
				},
			},
			expectedLatestRelease: map[string]db.DBReleaseWithMetaData{
				"app": {
					App: "app",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
					Manifests: db.DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"dev": "doesn't matter",
						},
					},
					Environments: []types.EnvName{"dev"},
				},
			},
			expectedAllEnvs: []types.EnvName{"dev"},
		},
		{
			Name: "multiple envs, multiple apps",
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
							Latest:      false,
							Environment: "dev",
						},
					},
				},
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"staging": "doesn't matter",
						"dev":     "doesn't matter",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateApplicationVersion{
					Application:    "app2",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"staging": "doesn't matter",
						"dev":     "doesn't matter",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeleteEnvironment{
					Environment: "staging",
				},
			},
			expectedLatestRelease: map[string]db.DBReleaseWithMetaData{
				"app": {
					App: "app",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
					Manifests: db.DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"dev": "doesn't matter",
						},
					},
					Environments: []types.EnvName{"dev"},
				},
				"app2": {
					App: "app2",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
					Manifests: db.DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"dev": "doesn't matter",
						},
					},
					Environments: []types.EnvName{"dev"},
				},
			},
			expectedAllEnvs: []types.EnvName{"dev"},
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
			var err error
			repo = SetupRepositoryTestWithDB(t)
			r := repo.(*repository)
			err = r.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				var batchError *TransformerBatchApplyError
				_, _, _, batchError = r.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if batchError != nil {
					return batchError
				}

				allEnvs, err := r.DB.DBSelectAllEnvironments(ctx, transaction)

				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.expectedAllEnvs, allEnvs); diff != "" {
					t.Errorf("all envs  mismatch (-want, +got):\n%s", diff)
					return nil
				}

				for appName, appConfig := range tc.expectedLatestRelease {
					app, err := r.DB.DBSelectReleaseByVersion(ctx, transaction, appName, types.ReleaseNumbers{Version: uversion(1), Revision: 0}, false)
					if err != nil {
						return err
					}
					if diff := cmp.Diff(appConfig, *app, cmpopts.IgnoreFields(db.DBReleaseWithMetaData{}, "Created"), cmpopts.IgnoreFields(db.DBReleaseWithMetaData{}, "Metadata")); diff != "" {
						t.Errorf("all envs  mismatch (-want, +got):\n%s", diff)
						return nil
					}
				}

				return nil

			})
			if err != nil {
				t.Fatalf("encountered error but no error is expected here: '%v'", err)
			}
		})
	}
}

func TestUndeployApplicationDB(t *testing.T) {
	tcs := []struct {
		Name          string
		Transformers  []Transformer
		expectedError *TransformerBatchApplyError
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
		},
		{
			Name: "Success",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[types.EnvName]string{
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
		},
		{
			Name: "Create un-deploy Version for un-deployed application should not work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[types.EnvName]string{
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
					Manifests: map[types.EnvName]string{
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
					Message:     testAppName,
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
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
					Manifests: map[types.EnvName]string{
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
					Message:     testAppName,
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
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
					Manifests: map[types.EnvName]string{
						envAcceptance: "acceptance",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateEnvironmentLock{
					Environment: "acceptance",
					LockId:      "22133",
					Message:     testAppName,
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
					Manifests: map[types.EnvName]string{
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
					Manifests: map[types.EnvName]string{
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
					Message:     testAppName,
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
		},
		{
			Name: "Undeploy application where the last release is not Undeploy shouldn't work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[types.EnvName]string{
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
				Index:            4,
				TransformerError: errMatcher{"UndeployApplication: error last release is not un-deployed application version of 'app1'"},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)

				if err != nil {
					return err
				}
				return nil
			})
			if tc.expectedError == nil && err != nil {
				t.Fatalf("Did no expect error but got):\n%+v", err)
			}
			if err != nil {
				applyErr := UnwrapUntilTransformerBatchApplyError(err)
				if diff := cmp.Diff(tc.expectedError, applyErr, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func TestDeleteEnvironment(t *testing.T) {
	const testAppName = "test-app"
	tcs := []struct {
		Name          string
		Transformers  []Transformer
		expectedError *TransformerBatchApplyError
	}{
		{
			Name: "Delete non-existent environment",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&DeleteEnvironment{
					Environment: "this-env-does-not-exist",
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            1,
				TransformerError: errMatcher{"error at index 1 of transformer batch: could not delete environment with name 'this-env-does-not-exist' from DB"},
			},
		},
		{
			Name: "Delete Env - Simple case",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&DeleteEnvironment{
					Environment: envProduction,
				},
			},
		},
		{
			Name: "Delete Env - App in env",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeleteEnvironment{
					Environment: envProduction,
				},
			},
		},
		{
			Name: "Delete Env - App in env & has a deployment",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment: envProduction,
					Application: testAppName,
					Version:     1,
				},
				&DeleteEnvironment{
					Environment: envProduction,
				},
			},
		},
		{
			Name: "Delete Env - Attempt to delete with env lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment: envProduction,
					Application: testAppName,
					Version:     1,
				},
				&CreateEnvironmentLock{
					Environment: envProduction,
					LockId:      "my-lock-for-prod",
					Message:     "This lock is for prod",
					CiLink:      "",
				},
				&DeleteEnvironment{
					Environment: envProduction,
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            4,
				TransformerError: errMatcher{"error at index 4 of transformer batch: rpc error: code = FailedPrecondition desc = error: could not delete environment 'production'. Environment locks for this environment exist"},
			},
		},
		{
			Name: "Delete Env - Attempt to delete with app lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment: envProduction,
					Application: testAppName,
					Version:     1,
				},
				&CreateEnvironmentApplicationLock{
					Environment: envProduction,
					Application: testAppName,
					LockId:      "my-lock-for-prod",
					Message:     "This lock is for prod",
					CiLink:      "",
				},
				&DeleteEnvironment{
					Environment: envProduction,
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            4,
				TransformerError: errMatcher{"error at index 4 of transformer batch: rpc error: code = FailedPrecondition desc = error: could not delete environment 'production'. Application locks for this environment exist"},
			},
		},
		{
			Name: "Delete Env - Attempt to delete with team lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Team:            "test-team",
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment: envProduction,
					Application: testAppName,
					Version:     1,
				},
				&CreateEnvironmentTeamLock{
					Environment: envProduction,
					Team:        "test-team",
					LockId:      "my-lock-for-prod",
					Message:     "This lock is for prod",
					CiLink:      "",
				},
				&DeleteEnvironment{
					Environment: envProduction,
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            4,
				TransformerError: errMatcher{"error at index 4 of transformer batch: rpc error: code = FailedPrecondition desc = error: could not delete environment 'production'. Team locks for this environment exist"},
			},
		},
		{
			Name: "Env to delete is upstream",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("production-group"),
					},
				},
				&CreateEnvironment{
					Environment: envAcceptance,
					Config: config.EnvironmentConfig{
						ArgoCd: nil,
						Upstream: &config.EnvironmentConfigUpstream{
							Latest:      false,
							Environment: envProduction,
						},
						EnvironmentGroup: conversion.FromString("acceptance-group"),
					},
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Team:            "test-team",
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment: envProduction,
					Application: testAppName,
					Version:     1,
				},
				&DeleteEnvironment{
					Environment: envProduction,
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            4,
				TransformerError: errMatcher{"error at index 4 of transformer batch: rpc error: code = FailedPrecondition desc = error: could not delete environment 'production'. Environment 'production' is upstream from 'acceptance'"},
			},
		},
		{
			Name: "Env to delete is upstream group",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("production-group"),
					},
				},
				&CreateEnvironment{
					Environment: "production-2",
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("production-group"),
					},
				},
				&CreateEnvironment{
					Environment: envAcceptance,
					Config: config.EnvironmentConfig{
						ArgoCd: nil,
						Upstream: &config.EnvironmentConfigUpstream{
							Latest:      false,
							Environment: "production-group",
						},
						EnvironmentGroup: conversion.FromString("acceptance-group"),
					},
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Team:            "test-team",
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment: envProduction,
					Application: testAppName,
					Version:     1,
				},
				&DeleteEnvironment{
					Environment: envProduction,
				},
				&DeleteEnvironment{
					Environment: "production-2",
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            6,
				TransformerError: errMatcher{"error at index 6 of transformer batch: rpc error: code = FailedPrecondition desc = error: could not delete environment 'production-2'. 'production-2' is part of environment group 'production-group', which is upstream from 'acceptance' and deleting 'production-2' would result in environment group deletion"},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				return nil
			})
			if tc.expectedError == nil && err != nil {
				t.Fatalf("Did no expect error but got):\n%+v", err)
			}
			if err != nil {
				applyErr := UnwrapUntilTransformerBatchApplyError(err)
				if diff := cmp.Diff(tc.expectedError, applyErr, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func TestUndeployTransformerDB(t *testing.T) {
	tcs := []struct {
		Name          string
		Transformers  []Transformer
		expectedError *TransformerBatchApplyError
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
		},
		{
			Name: "Success",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
			},
		},
		{
			Name: "Deploy after Undeploy should work",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[types.EnvName]string{
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
		},
		{
			Name: "Undeploy twice should succeed",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[types.EnvName]string{
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
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, &CreateEnvironment{Environment: "production"})
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Did no expect error but got):\n%+v", err)
			}
			err = r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				return nil
			})
			if tc.expectedError == nil && err != nil {
				t.Fatalf("Did no expect error but got):\n%+v", err)
			}
			if err != nil {
				if diff := cmp.Diff(tc.expectedError, err.(*TransformerBatchApplyError), cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func TestCreateUndeployDBState(t *testing.T) {
	const appName = "my-app"
	tcs := []struct {
		Name                   string
		TargetApp              string
		Transformers           []Transformer
		expectedError          *TransformerBatchApplyError
		expectedReleaseNumbers []types.ReleaseNumbers
	}{
		{
			Name: "Success",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: appName,
				},
			},
			expectedReleaseNumbers: []types.ReleaseNumbers{
				{
					Version:  uversion(1),
					Revision: 0,
				},
				{
					Version:  uversion(2),
					Revision: 0,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, s, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)

				if err != nil {
					return err
				}

				allReleases, err2 := s.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, appName)
				if err2 != nil {
					t.Fatal(err)
				}
				if len(allReleases) == 0 {
					t.Fatal("Expected some releases, but got none")
				}
				if diff := cmp.Diff(tc.expectedReleaseNumbers, allReleases); diff != "" {
					t.Fatalf("error mismatch on expected lock ids (-want, +got):\n%s", diff)
				}
				release, err2 := s.DBHandler.DBSelectReleaseByVersion(ctx, transaction, appName, allReleases[len(allReleases)-1], true)
				if err2 != nil {
					t.Fatal(err)
				}

				if !release.Metadata.UndeployVersion {
					t.Fatal("Expected last version to be un-deployed")
				}
				return nil
			})
			if tc.expectedError == nil && err != nil {
				t.Fatalf("Did no expect error but got):\n%+v", err)
			}
			if err != nil {
				if diff := cmp.Diff(tc.expectedError, err.(*TransformerBatchApplyError), cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAllowedCILinksState(t *testing.T) {
	const appName = "my-app"

	tcs := []struct {
		Name                string
		TargetApp           string
		Transformers        []Transformer
		expectedError       *TransformerBatchApplyError
		expectedAllReleases []types.ReleaseNumbers
		expectedDeployments []db.Deployment
	}{
		{
			Name: "No Link provided should succeed",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:           envProduction,
					Config:                config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envProduction, Latest: true}},
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: appName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
					CiLink:                "",
					AllowedDomains:        []string{"google.com", "freiheit.com"},
				},
			},
			expectedAllReleases: []types.ReleaseNumbers{
				{
					Version:  uversion(1),
					Revision: 0,
				},
			},
			expectedDeployments: []db.Deployment{
				{
					App: appName,
					Env: envProduction,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
					Metadata: db.DeploymentMetadata{
						DeployedByEmail: "testmail@example.com",
						DeployedByName:  "test tester",
						CiLink:          "",
					},
					TransformerID: 2,
				},
			},
		},
		{
			Name: "No accepted domains but no link provided should succeed",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:           envProduction,
					Config:                config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envProduction, Latest: true}},
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: appName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
					CiLink:                "",
					AllowedDomains:        []string{""},
				},
			},
			expectedAllReleases: []types.ReleaseNumbers{
				{
					Version:  uversion(1),
					Revision: 0,
				},
			},
			expectedDeployments: []db.Deployment{
				{
					App: appName,
					Env: envProduction,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
					Metadata: db.DeploymentMetadata{
						DeployedByEmail: "testmail@example.com",
						DeployedByName:  "test tester",
						CiLink:          "",
					},
					TransformerID: 2,
				},
			},
		},
		{
			Name: "Link in domain",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:           envProduction,
					Config:                config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envProduction, Latest: true}},
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: appName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
					CiLink:                "https://google.com/search?q=freiheit.com",
					AllowedDomains:        []string{"google.com", "freiheit.com"},
				},
			},
			expectedAllReleases: []types.ReleaseNumbers{
				{
					Version:  uversion(1),
					Revision: 0,
				},
			},
			expectedDeployments: []db.Deployment{
				{
					App: appName,
					Env: envProduction,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
					Metadata: db.DeploymentMetadata{
						DeployedByEmail: "testmail@example.com",
						DeployedByName:  "test tester",
						CiLink:          "https://google.com/search?q=freiheit.com",
					},
					TransformerID: 2,
				},
			},
		},
		{
			Name: "Link not in accepted domains",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:           envProduction,
					Config:                config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envProduction, Latest: true}},
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: appName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
					CiLink:                "https://github.com/freiheit-com/kuberpult",
					AllowedDomains:        []string{"google.com", "freiheit.com"},
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            1,
				TransformerError: errMatcher{"general_failure:{message:\"provided CI Link: https://github.com/freiheit-com/kuberpult is not valid or does not match any of the allowed domain\"}"},
			},
		},
		{
			Name: "No accepted domains should always fail",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:           envProduction,
					Config:                config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envProduction, Latest: true}},
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: appName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
					CiLink:                "https://google.com/search?q=freiheit.com",
					AllowedDomains:        []string{""},
				},
			},
			expectedError: &TransformerBatchApplyError{
				Index:            1,
				TransformerError: errMatcher{"general_failure:{message:\"provided CI Link: https://google.com/search?q=freiheit.com is not valid or does not match any of the allowed domain\"}"},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, s, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)

				if err != nil {
					return err
				}

				allReleases, err2 := s.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, appName)
				if err2 != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(tc.expectedAllReleases, allReleases); diff != "" {
					t.Fatalf("error mismatch on expected lock ids (-want, +got):\n%s", diff)
				}

				target, err2 := s.DBHandler.DBSelectDeploymentHistory(ctx, transaction, appName, envProduction, 10)

				if err2 != nil {
					t.Fatal(err2)
				}
				if diff := cmp.Diff(tc.expectedDeployments, target, cmpopts.IgnoreFields(db.Deployment{}, "Created")); diff != "" {
					t.Fatalf("error mismatch on expected lock ids (-want, +got):\n%s", diff)
				}
				return nil
			})
			if tc.expectedError == nil && err != nil {
				t.Fatalf("Did no expect error but got):\n%+v", err)
			}
			if err != nil {
				if diff := cmp.Diff(tc.expectedError, err.(*TransformerBatchApplyError), cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func TestUndeployDBState(t *testing.T) {
	const appName = "my-app"

	tcs := []struct {
		Name                string
		TargetApp           string
		Transformers        []Transformer
		expectedError       *TransformerBatchApplyError
		expectedAllReleases []types.ReleaseNumbers
		expectedDeployments []db.Deployment
	}{
		{
			Name: "Success",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment:           envProduction,
					Config:                config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envProduction, Latest: true}},
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: appName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
				},
				&CreateUndeployApplicationVersion{
					Application:           appName,
					TransformerEslVersion: 2,
				},
				&UndeployApplication{
					Application:           appName,
					TransformerEslVersion: 3,
				},
			},
			expectedAllReleases: []types.ReleaseNumbers{},
			expectedDeployments: []db.Deployment{
				{
					App: appName,
					Env: envProduction,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  nil,
					},
					Metadata: db.DeploymentMetadata{
						DeployedByEmail: "testmail@example.com",
						DeployedByName:  "test tester",
					},
					TransformerID: 3,
				},
				{
					App: appName,
					Env: envProduction,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(2),
					},
					Metadata: db.DeploymentMetadata{
						DeployedByEmail: "testmail@example.com",
						DeployedByName:  "test tester",
					},
					TransformerID: 3,
				},
				{
					App: appName,
					Env: envProduction,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
					Metadata: db.DeploymentMetadata{
						DeployedByEmail: "testmail@example.com",
						DeployedByName:  "test tester",
					},
					TransformerID: 2,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, s, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)

				if err != nil {
					return err
				}

				allReleases, err2 := s.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, appName)
				if err2 != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(tc.expectedAllReleases, allReleases); diff != "" {
					t.Fatalf("error mismatch on expected lock ids (-want, +got):\n%s", diff)
				}

				target, err2 := s.DBHandler.DBSelectDeploymentHistory(ctx, transaction, appName, envProduction, 10)

				if err2 != nil {
					t.Fatal(err2)
				}
				if diff := cmp.Diff(tc.expectedDeployments, target, cmpopts.IgnoreFields(db.Deployment{}, "Created")); diff != "" {
					t.Fatalf("error mismatch on expected lock ids (-want, +got):\n%s", diff)
				}
				allDeployments, err2 := s.DBHandler.DBSelectAllDeploymentsForApp(ctx, transaction, appName)
				if err2 != nil {
					t.Fatal(err)
				}

				if len(allDeployments) != 0 {
					t.Fatal("No deployments expected, but found some.")
				}
				return nil
			})
			if tc.expectedError == nil && err != nil {
				t.Fatalf("Did no expect error but got):\n%+v", err)
			}
			if err != nil {
				if diff := cmp.Diff(tc.expectedError, err.(*TransformerBatchApplyError), cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func uversion(v int) *uint64 {
	var result = uint64(v)
	return &result
}

func TestTransaction(t *testing.T) {
	const appName = "app1"
	tcs := []struct {
		Name               string
		Transformers       []Transformer
		expectedDbContent  *db.DBAppWithMetaData
		expectedDbReleases []types.ReleaseNumbers
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
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
				},
			},
			expectedDbContent: &db.DBAppWithMetaData{
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "",
				},
			},
			expectedDbReleases: []types.ReleaseNumbers{
				{
					Version:  uversion(10000),
					Revision: 0,
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
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "noteam",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "noteam",
				},
			},
			expectedDbContent: &db.DBAppWithMetaData{
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "noteam",
				},
			},
			expectedDbReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(11),
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
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "old",
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[types.EnvName]string{
						envAcceptance: "{}",
					},
					Team: "new",
				},
			},
			expectedDbContent: &db.DBAppWithMetaData{
				App:         appName,
				StateChange: db.AppStateChangeUpdate,
				Metadata: db.DBAppMetaData{
					Team: "new",
				},
			},
			expectedDbReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(10),
				types.MakeReleaseNumberVersion(11),
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctxWithTime := time.WithTimeNow(testutil.MakeTestContext(), timeNowOld)
			repo := SetupRepositoryTestWithDB(t)
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
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
				if diff := cmp.Diff(tc.expectedDbReleases, actualRelease); diff != "" {
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

func TestTimestampConsistency(t *testing.T) {
	tcs := []struct {
		Name                 string
		ReleaseVersionsLimit uint
		Transformers         []Transformer
		ExpectedVersion      uint
		TargetEnv            string
		TargetApp            string
	}{
		{
			Name:            "Release train",
			ExpectedVersion: 2,
			TargetEnv:       envProduction,
			TargetApp:       testAppName,
			Transformers: []Transformer{
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
							Environment: envAcceptance,
							Latest:      true,
						},
					},
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Environment: envProduction,
					Application: testAppName,
					Version:     1,
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData: true,
					Version:         2,
				},
				&DeployApplicationVersion{
					Environment: envAcceptance,
					Application: testAppName,
					Version:     1,
				},
				&DeployApplicationVersion{
					Environment: envAcceptance,
					Application: testAppName,
					Version:     2,
				},
				&ReleaseTrain{
					Target: envProduction,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			//check deployments
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, err2 := r.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if err2 != nil {
					return err2
				}
				//Get timestamp from transaction and check that every new entry has the same ts
				ts, err := state.DBHandler.DBReadTransactionTimestamp(ctx, transaction)
				if err != nil {
					return err
				}

				//Event sourcing light
				eslVersion, err := state.DBHandler.DBReadEslEventInternal(ctx, transaction, false)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(ts, &eslVersion.Created); diff != "" {
					t.Fatalf("error mismatch on eslVersion(-want, +got):\n%s", diff)
				}
				//Environment
				env, err := state.DBHandler.DBSelectEnvironment(ctx, transaction, envProduction)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(ts, &env.Created); diff != "" {
					t.Fatalf("error mismatch on envProduction(-want, +got):\n%s", diff)
				}
				//Environment
				env, err = state.DBHandler.DBSelectEnvironment(ctx, transaction, envAcceptance)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(ts, &env.Created); diff != "" {
					t.Fatalf("error mismatch on envAcceptance(-want, +got):\n%s", diff)
				}
				//Release
				releases, err := state.DBHandler.DBSelectReleasesByAppLatestEslVersion(ctx, transaction, testAppName, true)
				if err != nil {
					return err
				}
				for _, r := range releases {
					if diff := cmp.Diff(ts, &r.Created); diff != "" {
						t.Fatalf("error mismatch on releases(-want, +got):\n%s", diff)
					}
				}
				//Release
				deployments, err := state.DBHandler.DBSelectDeploymentHistory(ctx, transaction, testAppName, envAcceptance, 10)
				if err != nil {
					return err
				}
				for _, d := range deployments {
					if diff := cmp.Diff(ts, &d.Created); diff != "" {
						t.Fatalf("error mismatch on deployments(-want, +got):\n%s", diff)
					}
				}
				//Release
				deployments, err = state.DBHandler.DBSelectDeploymentHistory(ctx, transaction, testAppName, envProduction, 10)
				if err != nil {
					return err
				}
				for _, d := range deployments {
					if diff := cmp.Diff(ts, &d.Created); diff != "" {
						t.Fatalf("error mismatch on deployments(-want, +got):\n%s", diff)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Err: %v", err)
			}
		})
	}
}

func TestIsCustomErrorRetryable(t *testing.T) {
	tcs := []struct {
		InputCode         error
		ExpectedRetryable bool
	}{
		{
			InputCode:         GetCreateReleaseGeneralFailure(fmt.Errorf("other2: %w", fmt.Errorf("foobar: %w", &pq.Error{Code: pq.ErrorCode("23505")}))),
			ExpectedRetryable: true,
		},
		{
			InputCode:         GetCreateReleaseGeneralFailure(fmt.Errorf("could not write all apps: %w", &pq.Error{Code: pq.ErrorCode("23505")})),
			ExpectedRetryable: true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(fmt.Sprintf("endless_loop_check_%s", tc.InputCode.Error()), func(t *testing.T) {
			t.Parallel()

			actualRetryable := db.IsRetryablePostgresError(tc.InputCode)

			if diff := cmp.Diff(tc.ExpectedRetryable, actualRetryable); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateDatadogEventsInternal(t *testing.T) {
	tcs := []struct {
		Name           string
		changes        *TransformerResult
		transformers   []Transformer
		expectedEvents []statsd.Event
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
					Manifests: map[types.EnvName]string{
						"envA": "envA-manifest-1",
					},
					WriteCommitData: false,
					Version:         1,
				},
				&CreateApplicationVersion{
					Application: "app2",
					Manifests: map[types.EnvName]string{
						"envA": "envA-manifest-2",
					},
					WriteCommitData: false,
					Version:         1,
				},
			},
			changes: &TransformerResult{
				ChangedApps: []AppEnv{
					{
						App:  "app1",
						Env:  "envB",
						Team: "teamT",
					},
				},
				DeletedRootApps: nil,
			},
			expectedEvents: []statsd.Event{
				{
					Title:          "Kuberpult app deployed",
					Text:           "Kuberpult has deployed app1 to envB for team teamT",
					Timestamp:      gotime.Time{},
					Hostname:       "",
					AggregationKey: "",
					Priority:       "",
					SourceTypeName: "",
					AlertType:      "",
					Tags: []string{
						"kuberpult.application:app1",
						"kuberpult.environment:envB",
						"kuberpult.team:teamT",
					},
				},
			},
		},
		{
			Name: "Changes are sent as two events",
			transformers: []Transformer{
				&CreateEnvironment{
					Environment: "envA",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironment{
					Environment: "envB",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[types.EnvName]string{
						"envA": "envA-manifest-1",
						"envB": "envB-manifest-1",
					},
					WriteCommitData: false,
					Version:         1,
				},
				&CreateApplicationVersion{
					Application: "app2",
					Manifests: map[types.EnvName]string{
						"envA": "envA-manifest-2",
						"envB": "envB-manifest-2",
					},
					WriteCommitData: false,
					Version:         1,
				},
			},
			changes: &TransformerResult{
				ChangedApps: []AppEnv{
					{
						App:  "app1",
						Env:  "envB",
						Team: "teamT",
					},
					{
						App:  "app2",
						Env:  "envA",
						Team: "teamX",
					},
				},
				DeletedRootApps: nil,
			},
			expectedEvents: []statsd.Event{
				{
					Title:          "Kuberpult app deployed",
					Text:           "Kuberpult has deployed app1 to envB for team teamT",
					Timestamp:      gotime.Time{},
					Hostname:       "",
					AggregationKey: "",
					Priority:       "",
					SourceTypeName: "",
					AlertType:      "",
					Tags: []string{
						"kuberpult.application:app1",
						"kuberpult.environment:envB",
						"kuberpult.team:teamT",
					},
				},
				{
					Title:          "Kuberpult app deployed",
					Text:           "Kuberpult has deployed app2 to envA for team teamX",
					Timestamp:      gotime.Time{},
					Hostname:       "",
					AggregationKey: "",
					Priority:       "",
					SourceTypeName: "",
					AlertType:      "",
					Tags: []string{
						"kuberpult.application:app2",
						"kuberpult.environment:envA",
						"kuberpult.team:teamX",
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			//t.Parallel() // do not run in parallel because of the global var `ddMetrics`!
			ctx := time.WithTimeNow(testutil.MakeTestContext(), gotime.Unix(0, 0))
			var mockClient = &MockClient{}
			var client statsd.ClientInterface = mockClient
			ddMetrics = client
			repo := SetupRepositoryTestWithDB(t).(*repository)

			err := repo.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, applyErr := repo.ApplyTransformersInternal(ctx, transaction, tc.transformers...)
				if applyErr != nil {
					t.Fatalf("Expected no error: %v", applyErr)
					return nil
				}

				now, err := repo.DB.DBReadTransactionTimestamp(ctx, transaction)
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
					return nil
				}
				err = UpdateDatadogMetrics(ctx, transaction, state, repo, tc.changes, *now, true)
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
					return nil
				}

				return nil
			})
			if err != nil {
				t.Fatalf("Failed during transaction: %v", err)
			}

			if len(tc.expectedEvents) != len(mockClient.events) {
				t.Fatalf("expected %d events, but got %d", len(tc.expectedEvents), len(mockClient.events))
			}
			for i := range tc.expectedEvents {
				var expectedEvent = tc.expectedEvents[i]
				var actualEvent = *mockClient.events[i]

				if diff := cmp.Diff(expectedEvent, actualEvent, cmpopts.IgnoreFields(statsd.Event{}, "Timestamp")); diff != "" {
					t.Errorf("got %v, want %v, diff (-want +got) %s", actualEvent, expectedEvent, diff)
				}
			}
		})
	}
}

func TestUpdateDatadogMetricsInternal(t *testing.T) {
	makeGauge := func(name string, val float64, tags []string, rate float64) Gauge {
		return Gauge{
			Name:  name,
			Value: val,
			Tags:  tags,
			Rate:  rate,
		}
	}
	tcs := []struct {
		Name           string
		changes        *TransformerResult
		transformers   []Transformer
		expectedGauges []Gauge
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
					Manifests: map[types.EnvName]string{
						"envA": "envA-manifest-1",
					},
					WriteCommitData: false,
					Version:         1,
				},
				&CreateApplicationVersion{
					Application: "app2",
					Manifests: map[types.EnvName]string{
						"envA": "envA-manifest-2",
					},
					WriteCommitData: false,
					Version:         1,
				},
			},
			expectedGauges: []Gauge{
				makeGauge("request_queue_size", 0, []string{}, 1),
				makeGauge("environment_lock_count", 0, []string{"kuberpult_environment:envA"}, 1),
				makeGauge("application_lock_count", 0, []string{"kuberpult_environment:envA", "kuberpult_application:app1"}, 1),
				makeGauge("lastDeployed", 0, []string{"kuberpult_application:app1", "kuberpult_environment:envA"}, 1),
				makeGauge("git_sync_unsynced", 2, []string{}, 1),
				makeGauge("git_sync_failed", 0, []string{}, 1),
				makeGauge("application_lock_count", 0, []string{"kuberpult_environment:envA", "kuberpult_application:app2"}, 1),
				makeGauge("lastDeployed", 0, []string{"kuberpult_application:app2", "kuberpult_environment:envA"}, 1),
			},
		},
		{
			Name: "Changes are sent as two events",
			transformers: []Transformer{
				&CreateEnvironment{
					Environment: "envA",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironment{
					Environment: "envB",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[types.EnvName]string{
						"envA": "envA-manifest-1",
						"envB": "envB-manifest-1",
					},
					WriteCommitData: false,
					Version:         1,
				},
				&CreateApplicationVersion{
					Application: "app2",
					Manifests: map[types.EnvName]string{
						"envA": "envA-manifest-2",
						"envB": "envB-manifest-2",
					},
					WriteCommitData: false,
					Version:         1,
				},
			},
			expectedGauges: []Gauge{
				makeGauge("request_queue_size", 0, []string{}, 1),
				makeGauge("environment_lock_count", 0, []string{"kuberpult_environment:envA"}, 1),
				makeGauge("application_lock_count", 0, []string{"kuberpult_environment:envA", "kuberpult_application:app1"}, 1),
				makeGauge("lastDeployed", 0, []string{"kuberpult_application:app1", "kuberpult_environment:envA"}, 1),
				makeGauge("environment_lock_count", 0, []string{"kuberpult_environment:envB"}, 1),
				makeGauge("application_lock_count", 0, []string{"kuberpult_environment:envB", "kuberpult_application:app1"}, 1),

				// 10:
				makeGauge("lastDeployed", 0, []string{"kuberpult_application:app1", "kuberpult_environment:envB"}, 1),

				// 12:
				makeGauge("application_lock_count", 0, []string{"kuberpult_environment:envA", "kuberpult_application:app2"}, 1),
				makeGauge("lastDeployed", 0, []string{"kuberpult_application:app2", "kuberpult_environment:envA"}, 1),

				// 14:
				makeGauge("application_lock_count", 0, []string{"kuberpult_environment:envB", "kuberpult_application:app2"}, 1),
				makeGauge("git_sync_unsynced", 4, []string{}, 1),
				makeGauge("git_sync_failed", 0, []string{}, 1),
				makeGauge("lastDeployed", 0, []string{"kuberpult_application:app2", "kuberpult_environment:envB"}, 1),
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			//t.Parallel() // do not run in parallel because of the global var `ddMetrics`!
			ctx := time.WithTimeNow(testutil.MakeTestContext(), gotime.Unix(0, 0))
			var mockClient = &MockClient{}
			var client statsd.ClientInterface = mockClient
			ddMetrics = client
			repo := SetupRepositoryTestWithDB(t).(*repository)

			err := repo.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, applyErr := repo.ApplyTransformersInternal(ctx, transaction, tc.transformers...)
				if applyErr != nil {
					t.Fatalf("expected no error: %v", applyErr)
					return nil
				}
				now, err := repo.DB.DBReadTransactionTimestamp(ctx, transaction)
				if applyErr != nil {
					t.Fatalf("expected no error: %v", err)
					return nil
				}
				err = UpdateDatadogMetrics(ctx, transaction, state, repo, nil, *now, true)
				if err != nil {
					t.Fatalf("expected no error: %v", err)
					return nil
				}
				err = UpdateDatadogMetrics(ctx, transaction, state, repo, nil, *now, false)
				if err != nil {
					t.Fatalf("expected no error: %v", err)
					return nil
				}
				return nil
			})

			if err != nil {
				t.Fatalf("Failed during transaction: %v", err)
			}

			if len(tc.expectedGauges) != len(mockClient.gauges) {
				gaugesString := ""
				for i := range mockClient.gauges {
					gauge := mockClient.gauges[i]
					gaugesString += fmt.Sprintf("%v\n", gauge)
				}
				msg := fmt.Sprintf("expected %d gauges but got %d\nActual:\n%v\n",
					len(tc.expectedGauges), len(mockClient.gauges), gaugesString)
				t.Fatal(msg)
			}
			sortGauges := func(gaugesList []Gauge) {
				sort.Slice(gaugesList, func(i, j int) bool {
					if len(gaugesList[i].Tags) == 0 && len(gaugesList[j].Tags) == 0 {
						return gaugesList[i].Name > gaugesList[j].Name
					} else if len(gaugesList[i].Tags) != len(gaugesList[j].Tags) {
						return len(gaugesList[i].Tags) > len(gaugesList[j].Tags)
					} else {
						for tagIndex := range gaugesList[i].Tags {
							if gaugesList[i].Tags[tagIndex] != gaugesList[j].Tags[tagIndex] {
								return gaugesList[i].Tags[tagIndex] > gaugesList[j].Tags[tagIndex]
							}
						}
						return true
					}
				})
			}
			sortGauges(tc.expectedGauges)
			sortGauges(mockClient.gauges)
			for i := range tc.expectedGauges {
				var expectedGauge = tc.expectedGauges[i]
				sort.Strings(expectedGauge.Tags)
				var actualGauge = mockClient.gauges[i]
				sort.Strings(actualGauge.Tags)
				t.Logf("actualGauges:[%v] %v:%v", i, actualGauge.Name, actualGauge.Tags)
				t.Logf("expectedGauges:[%v] %v:%v", i, expectedGauge.Name, expectedGauge.Tags)
				if actualGauge.Name == "lastDeployed" {
					if actualGauge.Value < 1 {
						actualGauge.Value = 0
					}
				}

				if diff := cmp.Diff(expectedGauge, actualGauge, cmpopts.IgnoreFields(statsd.Event{}, "Timestamp")); diff != "" {
					t.Errorf("[%d] want %v, got %v, diff (-want +got) %s", i, expectedGauge, actualGauge, diff)
				}
			}
		})
	}
}

func TestChangedApps(t *testing.T) {
	tcs := []struct {
		Name                string
		Setup               []Transformer
		Transformers        []Transformer
		expectedChangedApps []AppEnv
	}{
		{
			Name: "Undeploy application sets changed apps",
			Setup: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						Upstream:         nil,
						EnvironmentGroup: conversion.FromString("mygroup"),
					},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Team:        "team1",
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
			},
			Transformers: []Transformer{
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedChangedApps: []AppEnv{
				{
					App:  "app1",
					Env:  "production",
					Team: "team1",
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)

			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Setup...)
				if err != nil {
					return err
				}
				return nil
			})

			if err != nil {
				t.Fatalf("Did not expect error during setup but got:\n%+v", err)
			}

			err = r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, changes, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if err != nil {
					return err
				}

				finalChanges := &TransformerResult{}
				for _, change := range changes {
					finalChanges.Combine(change)
				}

				if diff := cmp.Diff(tc.expectedChangedApps, finalChanges.ChangedApps); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}

				return nil
			})
			if err != nil {
				t.Fatalf("Did no expect error but got:\n%+v", err)
			}
		})
	}
}

func TestChangedAppsSyncStatus(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		changes           []db.EnvApp
		targetTransformer db.TransformerID
	}{
		{
			Name:              "Create Release",
			targetTransformer: db.TransformerID(3),
			changes: []db.EnvApp{
				{
					AppName: testAppName,
					EnvName: envAcceptance,
				},
				{
					AppName: testAppName,
					EnvName: envProduction,
				},
			},
			Transformers: []Transformer{
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
							Environment: envAcceptance,
							Latest:      false,
						},
					},
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
			},
		},
		{
			Name:              "Deploy Release",
			targetTransformer: db.TransformerID(4),
			changes: []db.EnvApp{
				{
					AppName: testAppName,
					EnvName: envAcceptance,
				},
			},
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance,
						},
					},
				},
				&CreateEnvironment{
					Environment: envAcceptance,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance,
							Latest:      false,
						},
					},
				},
				&CreateApplicationVersion{
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&DeployApplicationVersion{
					Application:     testAppName,
					Environment:     envAcceptance,
					WriteCommitData: true,
					Version:         1,
				},
			},
		},
		{
			Name:              "Release train",
			targetTransformer: db.TransformerID(5),
			changes: []db.EnvApp{
				{
					AppName: testAppName,
					EnvName: envProduction,
				},
				{
					AppName: nextTestAppName,
					EnvName: envProduction,
				},
			},
			Transformers: []Transformer{
				&CreateEnvironment{ //ID: 1
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance, // train drives from acceptance to production
						},
					},
				},
				&CreateEnvironment{ //ID: 2
					Environment: envAcceptance,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance,
							Latest:      true,
						},
					},
				},
				&CreateApplicationVersion{ //ID: 3
					Application: testAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateApplicationVersion{ //ID: 4
					Application: nextTestAppName,
					Manifests: map[types.EnvName]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&ReleaseTrain{ //ID: 5
					Target: envProduction,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			for _, transformer := range tc.Transformers {
				err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					_, _, _, err2 := r.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, transformer)
					if err2 != nil {
						return err2
					}
					return nil
				})
				if err != nil {
					t.Fatalf("Err: %v", err)
				}
			}

			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				changes, err := r.State().DBHandler.DBReadUnsyncedAppsForTransfomerID(ctx, transaction, tc.targetTransformer)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(tc.changes, changes); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Err: %v", err)
			}
		})
	}
}
