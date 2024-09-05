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
	"regexp"
	"testing"
	gotime "time"

	"github.com/freiheit-com/kuberpult/pkg/event"

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
			var jsonInterface interface{} = tc.dataType
			err = json.Unmarshal(([]byte)(row.EventJson), &jsonInterface)
			if err != nil {
				t.Fatalf("marshal error: %v\njson: \n%s\n", err, row.EventJson)
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
			err = r.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
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

			locks, err := db.WithTransactionT(repo.State().DBHandler, ctx, db.DefaultNumRetries, false, func(ctx context.Context, transaction *sql.Tx) (*db.AllEnvLocksGo, error) {
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
			err = r.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
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

			locks, err := db.WithTransactionT(repo.State().DBHandler, ctx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*db.AllTeamLocksGo, error) {
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
				EslVersion:  2,
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "",
				},
			},
			expectedDbReleases: &db.DBAllReleasesWithMetaData{
				EslVersion: 1,
				Created:    gotime.Time{},
				App:        appName,
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
				EslVersion:  2, // even when CreateApplicationVersion is called twice, we still write the app only once
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "noteam",
				},
			},
			expectedDbReleases: &db.DBAllReleasesWithMetaData{
				EslVersion: 2,
				Created:    gotime.Time{},
				App:        appName,
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
				EslVersion:  3, // CreateApplicationVersion was called twice with different teams, so there's 2 new entries, instead of onc
				App:         appName,
				StateChange: db.AppStateChangeUpdate,
				Metadata: db.DBAppMetaData{
					Team: "new",
				},
			},
			expectedDbReleases: &db.DBAllReleasesWithMetaData{
				EslVersion: 2,
				Created:    gotime.Time{},
				App:        appName,
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

func TestMinorFlag(t *testing.T) {
	appName := "app"
	tcs := []struct {
		Name           string
		Transformers   []Transformer
		ExpectedMinors []uint64
		ExpectedMajors []uint64
		MinorRegexes   []*regexp.Regexp
	}{
		{
			Name: "No previous or next releases",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []uint64{},
			ExpectedMajors: []uint64{10},
		},
		{
			Name: "No next Release, Previous Releases manifest equals current releases",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []uint64{11},
			ExpectedMajors: []uint64{10},
		},
		{
			Name: "No next Release, Previous Releases Manifest does not equal current's",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest2",
					},
				},
			},
			ExpectedMinors: []uint64{},
			ExpectedMajors: []uint64{10, 11},
		},
		{
			Name: "No prev Release, next Releases Manifest equals current's",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []uint64{11},
			ExpectedMajors: []uint64{10},
		},
		{
			Name: "No prev Release, next Releases Manifest does not equal current's",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest2",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []uint64{},
			ExpectedMajors: []uint64{10, 11},
		},
		{
			Name: "prev, next, and current are not equal",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[string]string{
						envAcceptance: "manifest3",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest2",
					},
				},
			},
			ExpectedMinors: []uint64{},
			ExpectedMajors: []uint64{10, 11, 12},
		},
		{
			Name: "prev and current are equal but not next",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
						"new env":     "new manifest",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []uint64{11},
			ExpectedMajors: []uint64{10, 12},
		},
		{
			Name: "prev and next are equal but not current",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest2",
					},
				},
			},
			ExpectedMinors: []uint64{},
			ExpectedMajors: []uint64{10, 11, 12},
		},
		{
			Name: "current and next are equal but not prev",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest2",
						"new env":     "new manifest",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[string]string{
						envAcceptance: "manifest2",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest2",
					},
				},
			},
			ExpectedMinors: []uint64{12},
			ExpectedMajors: []uint64{10, 11},
		},
		{
			Name: "all equal",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
			},
			ExpectedMinors: []uint64{11, 12},
			ExpectedMajors: []uint64{10},
		},
		{
			Name: "With Regex, all manifests are equal",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[string]string{
						envAcceptance: "manifest3",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest2",
					},
				},
			},
			MinorRegexes:   []*regexp.Regexp{regexp.MustCompile(".*manifest.*")},
			ExpectedMinors: []uint64{11, 12},
			ExpectedMajors: []uint64{10},
		},
		{
			Name: "Multiple Regexes",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1\nfirstLine1\nsecondLine1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[string]string{
						envAcceptance: "manifest2\nfirstLine3\nsecondLine3",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest2\nfirstLine2\nsecondLine2",
					},
				},
			},
			MinorRegexes:   []*regexp.Regexp{regexp.MustCompile(".*firstLine.*"), regexp.MustCompile(".*secondLine.*")},
			ExpectedMinors: []uint64{12},
			ExpectedMajors: []uint64{10, 11},
		},
		{
			Name: "Multiple Regexes and one of them do not match",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Version:     10,
					Manifests: map[string]string{
						envAcceptance: "manifest1\nfirstLine1\nsecondLine1",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     12,
					Manifests: map[string]string{
						envAcceptance: "manifest2\nfirstLine3\nsecondLine3",
					},
				},
				&CreateApplicationVersion{
					Application: appName,
					Version:     11,
					Manifests: map[string]string{
						envAcceptance: "manifest2\nfirstLine2\nsecondLine2",
					},
				},
			},
			MinorRegexes:   []*regexp.Regexp{regexp.MustCompile(".*firstLine.*"), regexp.MustCompile(".*ItDoesNotMatch.*")},
			ExpectedMinors: []uint64{},
			ExpectedMajors: []uint64{10, 11, 12},
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
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, &CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				})
				if err != nil {
					return err
				}
				_, state, _, err = repo.ApplyTransformersInternal(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				for _, minorVersion := range tc.ExpectedMinors {
					release, err := state.DBHandler.DBSelectReleaseByVersion(ctxWithTime, transaction, appName, minorVersion, true)
					if err != nil {
						return err
					}
					if !release.Metadata.IsMinor {
						t.Errorf("Expected release %d to be minor but its major", minorVersion)
					}
				}
				for _, majorVersion := range tc.ExpectedMajors {
					release, err := state.DBHandler.DBSelectReleaseByVersion(ctxWithTime, transaction, appName, majorVersion, true)
					if err != nil {
						return err
					}
					if release.Metadata.IsMinor {
						t.Errorf("Expected release %d to be major but its minor", majorVersion)
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
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
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
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
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
			err3 := repo.State().DBHandler.WithTransaction(ctxWithTime, false, func(ctx context.Context, transaction *sql.Tx) error {
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
					Manifests: map[string]string{
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
					Manifests: map[string]string{
						"dev": "doesn't matter",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					Manifests: map[string]string{
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
			var err error = nil
			repo = SetupRepositoryTestWithDB(t)
			r := repo.(*repository)
			err = r.DB.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				var batchError *TransformerBatchApplyError = nil
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
					t.Fatalf("error event count mismatch expected '%d' events but got '%d'\n", len(tc.expectedDBEvents), len(rows))
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
	firstRelease := db.DBReleaseWithMetaData{
		EslVersion:    1,
		ReleaseNumber: 10,
		App:           "app",
		Manifests: db.DBReleaseManifests{
			Manifests: map[string]string{
				"env":  "testenvmanifest",
				"env2": "testenvmanifest2",
			},
		},
		Metadata: db.DBReleaseMetaData{
			SourceAuthor:   "test",
			SourceMessage:  "test",
			SourceCommitId: "test",
			DisplayVersion: "test",
		},
	}
	secondRelease := db.DBReleaseWithMetaData{
		EslVersion:    1,
		ReleaseNumber: 11,
		App:           "app",
		Manifests: db.DBReleaseManifests{
			Manifests: map[string]string{
				"env1": "testenvmanifest",
				"env2": "testenvmanifest2",
			},
		},
		Metadata: db.DBReleaseMetaData{
			SourceAuthor:   "test",
			SourceMessage:  "test",
			SourceCommitId: "test",
			DisplayVersion: "test",
		},
	}
	tcs := []struct {
		Name              string
		PrevReleases      []db.DBReleaseWithMetaData
		Transforms        []Transformer
		ExpectedManifests map[string]string
	}{
		{
			Name:         "Simple Delete Env From App",
			PrevReleases: []db.DBReleaseWithMetaData{firstRelease},
			Transforms: []Transformer{
				&DeleteEnvFromApp{
					Application: firstRelease.App,
					Environment: "env",
				},
			},
			ExpectedManifests: map[string]string{
				"env2": "testenvmanifest2",
			},
		},
		{
			Name:         "Delete Env that doesn't exist",
			PrevReleases: []db.DBReleaseWithMetaData{firstRelease},
			Transforms: []Transformer{
				&DeleteEnvFromApp{
					Application: firstRelease.App,
					Environment: "env3",
				},
			},
			ExpectedManifests: firstRelease.Manifests.Manifests,
		},
		{
			Name:         "Multiple Manifests",
			PrevReleases: []db.DBReleaseWithMetaData{firstRelease, secondRelease},
			Transforms: []Transformer{
				&DeleteEnvFromApp{
					Application: firstRelease.App,
					Environment: "env",
				},
			},
			ExpectedManifests: map[string]string{
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
				for _, release := range tc.PrevReleases {
					repo.State().DBHandler.DBInsertRelease(ctx, transaction, release, 0)
				}
				_, state, _, err := repo.ApplyTransformersInternal(ctx, transaction, tc.Transforms...)
				if err != nil {
					return fmt.Errorf("error: %v", err)
				}
				releases, err2 := state.DBHandler.DBSelectReleasesByApp(ctx, transaction, firstRelease.App, false, true)
				if err2 != nil {
					return fmt.Errorf("error retrieving release: %v", err2)
				}
				for _, release := range releases {
					if diff := cmp.Diff(firstRelease.EslVersion+1, release.EslVersion); diff != "" {
						return fmt.Errorf("error mismatch ReleaseNumber - want, +got:\n%s", diff)
					}
					for env, manifest := range tc.ExpectedManifests {
						if diff := cmp.Diff(manifest, release.Manifests.Manifests[env]); diff != "" {
							return fmt.Errorf("error mismatch Manifests - want, +got:\n%s", diff)
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
		ExpectedVersion      uint
		TargetEnv            string
		TargetApp            string
	}{
		{
			Name:            "Release train",
			ExpectedVersion: 2,
			TargetEnv:       envProduction,
			TargetApp:       "test",
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
					Application: "test",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData: true,
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
				},
				&ReleaseTrain{
					Target: envProduction,
				},
			},
		},
		{
			Name:            "Release train from Latest",
			ExpectedVersion: 2,
			TargetEnv:       envAcceptance,
			TargetApp:       "test",
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
					Application: "test",
					Manifests: map[string]string{
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 0,
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData:       true,
					Version:               2,
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
			ExpectedVersion: 2,
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
					Manifests: map[string]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					Team:                  "test",
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
					Manifests: map[string]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					WriteCommitData:       true,
					Version:               2,
					TransformerEslVersion: 0,
					Team:                  "test",
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
					Team:                  "test",
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
			//check deployments
			ctx := testutil.MakeTestContext()
			r := repo.(*repository)
			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, state, _, err2 := r.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if err2 != nil {
					return err2
				}

				deployment, dplError := state.DBHandler.DBSelectLatestDeployment(ctx, transaction, tc.TargetApp, tc.TargetEnv)
				if dplError != nil {
					return dplError
				}

				if deployment == nil {
					t.Fatalf("Expected deployment but none was found.")
				}
				if deployment.Version == nil {
					t.Fatalf("Expected deployment version, but got nil.")

				}
				if diff := cmp.Diff(uint(*deployment.Version), tc.ExpectedVersion); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Err: %v\n", err)
			}
		})
	}
}

func TestUndeployApplicationDB(t *testing.T) {
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
				Index:            3,
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
			r := repo.(*repository)
			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				commitMsg, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)

				if err != nil {
					return err
				}

				actualMsg := ""
				if len(commitMsg) > 0 {
					actualMsg = commitMsg[len(commitMsg)-1]
				}
				if diff := cmp.Diff(tc.expectedCommitMsg, actualMsg); diff != "" {
					t.Errorf("commit message mismatch (-want, +got):\n%s", diff)
					return nil
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
			err := r.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				commitMsg, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers...)

				if err != nil {
					return nil
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
		expectedCommitMsg      string
		expectedReleaseNumbers []int64
	}{
		{
			Name: "Success",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: appName,
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData: true,
					Version:         1,
				},
				&CreateUndeployApplicationVersion{
					Application: appName,
				},
			},
			expectedReleaseNumbers: []int64{1, 2},
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
				if allReleases == nil || len(allReleases.Metadata.Releases) == 0 {
					t.Fatal("Expected some releases, but got none")
				}
				if diff := cmp.Diff(tc.expectedReleaseNumbers, allReleases.Metadata.Releases); diff != "" {
					t.Fatalf("error mismatch on expected lock ids (-want, +got):\n%s", diff)
				}
				release, err2 := s.DBHandler.DBSelectReleaseByVersion(ctx, transaction, appName, uint64(allReleases.Metadata.Releases[len(allReleases.Metadata.Releases)-1]), true)
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
		expectedCommitMsg   string
		expectedAllReleases []int64
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
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
					CiLink:                "",
					AllowedDomains:        []string{"google.com", "freiheit.com"},
				},
			},
			expectedAllReleases: []int64{1},
			expectedDeployments: []db.Deployment{
				{
					EslVersion: 1,
					App:        appName,
					Env:        envProduction,
					Version:    version(1),
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
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
					CiLink:                "",
					AllowedDomains:        []string{""},
				},
			},
			expectedAllReleases: []int64{1},
			expectedDeployments: []db.Deployment{
				{
					EslVersion: 1,
					App:        appName,
					Env:        envProduction,
					Version:    version(1),
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
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
					CiLink:                "https://google.com/search?q=freiheit.com",
					AllowedDomains:        []string{"google.com", "freiheit.com"},
				},
			},
			expectedAllReleases: []int64{1},
			expectedDeployments: []db.Deployment{
				{
					EslVersion: 1,
					App:        appName,
					Env:        envProduction,
					Version:    version(1),
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
					Manifests: map[string]string{
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
				TransformerError: errMatcher{"general_failure:{message:\"Provided CI Link: https://github.com/freiheit-com/kuberpult is not valid or does not match any of the allowed domain\"}"},
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
					Manifests: map[string]string{
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
				TransformerError: errMatcher{"general_failure:{message:\"Provided CI Link: https://google.com/search?q=freiheit.com is not valid or does not match any of the allowed domain\"}"},
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

				if diff := cmp.Diff(tc.expectedAllReleases, allReleases.Metadata.Releases); diff != "" {
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
		expectedCommitMsg   string
		expectedAllReleases []int64
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
					Manifests: map[string]string{
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
			expectedAllReleases: []int64{},
			expectedDeployments: []db.Deployment{
				{
					EslVersion: 3,
					App:        appName,
					Env:        envProduction,
					Version:    nil,
					Metadata: db.DeploymentMetadata{
						DeployedByEmail: "testmail@example.com",
						DeployedByName:  "test tester",
					},
					TransformerID: 3,
				},
				{
					EslVersion: 2,
					App:        appName,
					Env:        envProduction,
					Version:    version(2),
					Metadata: db.DeploymentMetadata{
						DeployedByEmail: "testmail@example.com",
						DeployedByName:  "test tester",
					},
					TransformerID: 3,
				},
				{
					EslVersion: 1,
					App:        appName,
					Env:        envProduction,
					Version:    version(1),
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

				if diff := cmp.Diff(tc.expectedAllReleases, allReleases.Metadata.Releases); diff != "" {
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

				if len(allDeployments.Deployments) != 0 {
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

func version(v int) *int64 {
	var result = int64(v)
	return &result
}

func TestTransaction(t *testing.T) {
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
				EslVersion:  2,
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "",
				},
			},
			expectedDbReleases: &db.DBAllReleasesWithMetaData{
				EslVersion: 1,
				Created:    gotime.Time{},
				App:        appName,
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
				EslVersion:  2, // even when CreateApplicationVersion is called twice, we still write the app only once
				App:         appName,
				StateChange: db.AppStateChangeCreate,
				Metadata: db.DBAppMetaData{
					Team: "noteam",
				},
			},
			expectedDbReleases: &db.DBAllReleasesWithMetaData{
				EslVersion: 2,
				Created:    gotime.Time{},
				App:        appName,
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
				EslVersion:  3, // CreateApplicationVersion was called twice with different teams, so there's 2 new entries, instead of onc
				App:         appName,
				StateChange: db.AppStateChangeUpdate,
				Metadata: db.DBAppMetaData{
					Team: "new",
				},
			},
			expectedDbReleases: &db.DBAllReleasesWithMetaData{
				EslVersion: 2,
				Created:    gotime.Time{},
				App:        appName,
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
