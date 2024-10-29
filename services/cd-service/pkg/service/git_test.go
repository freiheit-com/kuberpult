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
	"context"
	"database/sql"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/uuid"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	rp "github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type StageError struct {
	err error
}

func TestGetProductDB(t *testing.T) {
	tcs := []struct {
		Name                   string
		givenEnv               *string
		givenEnvGroup          *string
		expectedProductSummary []*api.GetProductSummaryResponse
		expectedErrors         []StageError
		SetupStages            [][]rp.Transformer
	}{
		{
			Name:     "get Product Overview as expected with env",
			givenEnv: conversion.FromString("development"),
			SetupStages: [][]rp.Transformer{
				{
					&rp.CreateEnvironment{
						Environment: "development",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Latest: true,
							},
							ArgoCd:           nil,
							EnvironmentGroup: conversion.FromString("dev"),
						},
					},
					&rp.CreateApplicationVersion{
						Application: "test",
						Manifests: map[string]string{
							"development": "dev",
						},
						SourceAuthor:    "example <example@example.com>",
						SourceCommitId:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
						SourceMessage:   "changed something (#678)",
						DisplayVersion:  "v1.0.2",
						Team:            "sre-team",
						WriteCommitData: true,
						Version:         1,
					},
					&rp.DeployApplicationVersion{
						Application: "test",
						Environment: "development",
						Version:     1,
					},
				},
			},
			expectedProductSummary: []*api.GetProductSummaryResponse{
				{
					ProductSummary: []*api.ProductSummary{
						{
							App: "test", Version: "1", DisplayVersion: "v1.0.2", CommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", Team: "sre-team", Environment: "development",
						},
					},
				},
			},
		},
		{
			Name:     "get Product Overview as expected with env but without team - Multiple stages",
			givenEnv: conversion.FromString("development"),
			SetupStages: [][]rp.Transformer{
				{
					&rp.CreateEnvironment{
						Environment: "development",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Latest: true,
							},
							ArgoCd:           nil,
							EnvironmentGroup: conversion.FromString("dev"),
						},
					},
					&rp.CreateApplicationVersion{
						Application: "test",
						Manifests: map[string]string{
							"development": "dev",
						},
						SourceAuthor:    "example <example@example.com>",
						SourceCommitId:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
						SourceMessage:   "changed something (#678)",
						DisplayVersion:  "v1.0.2",
						WriteCommitData: true,
						Version:         1,
					},

					&rp.CreateApplicationVersion{
						Application: "test",
						Manifests: map[string]string{
							"development": "dev",
						},
						SourceAuthor:    "example <example@example.com>",
						SourceCommitId:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
						SourceMessage:   "changed something (#678)",
						DisplayVersion:  "v1.0.2",
						WriteCommitData: true,
						Version:         2,
					},
					&rp.DeployApplicationVersion{
						Application: "test",
						Environment: "development",
						Version:     1,
					},
				},
				{
					&rp.DeployApplicationVersion{
						Application: "test",
						Environment: "development",
						Version:     2,
					},
				},
			},
			expectedProductSummary: []*api.GetProductSummaryResponse{
				{
					ProductSummary: []*api.ProductSummary{
						{
							App: "test", Version: "1", DisplayVersion: "v1.0.2", CommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", Team: "", Environment: "development",
						},
					},
				},
				{
					ProductSummary: []*api.ProductSummary{
						{
							App: "test", Version: "2", DisplayVersion: "v1.0.2", CommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", Team: "", Environment: "development",
						},
					},
				},
			},
		},
		{
			Name:          "get Product Overview as expected with envGroup",
			givenEnvGroup: conversion.FromString("dev"),
			SetupStages: [][]rp.Transformer{
				{
					&rp.CreateEnvironment{
						Environment: "development",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Latest: true,
							},
							ArgoCd:           nil,
							EnvironmentGroup: conversion.FromString("dev"),
						},
					},
					&rp.CreateApplicationVersion{
						Application: "test",
						Manifests: map[string]string{
							"development": "dev",
						},
						SourceAuthor:    "example <example@example.com>",
						SourceCommitId:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
						SourceMessage:   "changed something (#678)",
						DisplayVersion:  "v1.0.2",
						Team:            "sre-team",
						WriteCommitData: true,
						Version:         1,
					},
					&rp.DeployApplicationVersion{
						Application: "test",
						Environment: "development",
						Version:     1,
					},
				},
			},
			expectedProductSummary: []*api.GetProductSummaryResponse{
				{
					ProductSummary: []*api.ProductSummary{
						{
							App: "test", Version: "1", DisplayVersion: "v1.0.2", CommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", Team: "sre-team", Environment: "development",
						},
					},
				},
			},
		},
		{
			Name:     "environment did not yet exist",
			givenEnv: conversion.FromString("staging"),
			SetupStages: [][]rp.Transformer{
				{
					&rp.CreateEnvironment{
						Environment: "development",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Latest: true,
							},
							ArgoCd:           nil,
							EnvironmentGroup: conversion.FromString("dev"),
						},
					},
					&rp.CreateApplicationVersion{
						Application: "test",
						Manifests: map[string]string{
							"development": "dev",
						},
						SourceAuthor:    "example <example@example.com>",
						SourceCommitId:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
						SourceMessage:   "changed something (#678)",
						DisplayVersion:  "v1.0.2",
						WriteCommitData: true,
						Version:         1,
					},
					&rp.DeployApplicationVersion{
						Application: "test",
						Environment: "development",
						Version:     1,
					},
				},
				{
					&rp.CreateEnvironment{
						Environment: "staging",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Latest: true,
							},
							ArgoCd:           nil,
							EnvironmentGroup: conversion.FromString("staging"),
						},
					},
					&rp.CreateApplicationVersion{
						Application: "test2",
						Manifests: map[string]string{
							"development": "dev",
							"staging":     "staging",
						},
						SourceAuthor:    "example <example@example.com>",
						SourceCommitId:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
						SourceMessage:   "changed something (#678)",
						DisplayVersion:  "v1.0.2",
						WriteCommitData: true,
						Version:         2,
						Team:            "sre-team",
					},
					&rp.DeployApplicationVersion{
						Application: "test2",
						Environment: "staging",
						Version:     2,
					},
				},
			},
			expectedErrors: []StageError{
				{
					err: errMatcher{"unable to get applications for environment 'staging': environment staging not found"},
				},
				{
					err: nil,
				},
			},
			expectedProductSummary: []*api.GetProductSummaryResponse{
				{
					ProductSummary: nil,
				},
				{
					ProductSummary: []*api.ProductSummary{
						{
							App: "test2", Version: "2", DisplayVersion: "v1.0.2", CommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", Team: "sre-team", Environment: "staging",
						},
					},
				},
			},
		},
		{
			Name:     "no data no error",
			givenEnv: conversion.FromString("staging"),
			SetupStages: [][]rp.Transformer{
				{
					&rp.CreateEnvironment{
						Environment: "development",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Latest: true,
							},
							ArgoCd:           nil,
							EnvironmentGroup: conversion.FromString("dev"),
						},
					},
					&rp.CreateEnvironment{
						Environment: "staging",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Latest: true,
							},
							ArgoCd:           nil,
							EnvironmentGroup: conversion.FromString("staging"),
						},
					},
					&rp.CreateApplicationVersion{
						Application: "test",
						Manifests: map[string]string{
							"development": "dev",
						},
						SourceAuthor:    "example <example@example.com>",
						SourceCommitId:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
						SourceMessage:   "changed something (#678)",
						DisplayVersion:  "v1.0.2",
						WriteCommitData: true,
						Version:         1,
					},
					&rp.DeployApplicationVersion{
						Application: "test",
						Environment: "development",
						Version:     1,
					},
				},
			},
			expectedProductSummary: []*api.GetProductSummaryResponse{
				{
					ProductSummary: nil,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			migrationsPath, err := testutil.CreateMigrationsPath(4)
			if err != nil {
				t.Fatal(err)
			}
			dbConfig := &db.DBConfig{
				DriverName:     "sqlite3",
				MigrationsPath: migrationsPath,
				WriteEslOnly:   false,
			}
			repo, err := setupRepositoryTestWithDB(t, dbConfig)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			config := rp.RepositoryConfig{
				ArgoCdGenerateFiles: true,
				DBHandler:           repo.State().DBHandler,
			}
			sv := &GitServer{OverviewService: &OverviewServiceServer{Repository: repo, Shutdown: shutdown}, Config: config}
			ctx := testutil.MakeTestContext()

			var commitHashes []string
			for idx, steps := range tc.SetupStages {
				err = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					//Apply the setup transformers
					_, _, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, steps...)
					if err != nil {
						return err
					}

					ts, err2 := repo.State().DBHandler.DBReadTransactionTimestamp(ctx, transaction)
					if err2 != nil {
						return err2
					}

					currentCommitHash := strings.Repeat(strconv.Itoa(idx), 40)

					commitHashes = append(commitHashes, currentCommitHash)
					//Register these timestamps. Essentially we are 'tagging' every step so that we can access them further down
					err2 = repo.State().DBHandler.DBWriteCommitTransactionTimestamp(ctx, transaction, currentCommitHash, ts.UTC())
					return err2
				})
				if err != nil {
					t.Fatalf("Error applying transformers step %d: %v", idx, err)
				}
				time.Sleep(1000 * time.Millisecond) //This is here so that timestamps on sqlite do not collide when multiple stages are involved.
			}

			if len(tc.expectedProductSummary) > 0 {
				for iter, currentExpectedProductSummary := range tc.expectedProductSummary {
					var productSummary *api.GetProductSummaryResponse
					if tc.givenEnvGroup != nil {
						productSummary, err = sv.GetProductSummary(testutil.MakeTestContext(), &api.GetProductSummaryRequest{ManifestRepoCommitHash: commitHashes[iter], Environment: nil, EnvironmentGroup: tc.givenEnvGroup})
					} else {
						productSummary, err = sv.GetProductSummary(testutil.MakeTestContext(), &api.GetProductSummaryRequest{ManifestRepoCommitHash: commitHashes[iter], Environment: tc.givenEnv, EnvironmentGroup: tc.givenEnvGroup})
					}
					if err != nil && tc.expectedErrors == nil {
						t.Errorf("error mismatch. did not expect any errors but got: %v", err)
					}
					if tc.expectedErrors != nil {
						if diff := cmp.Diff(tc.expectedErrors[iter].err, err, cmpopts.EquateErrors()); diff != "" {
							t.Errorf("error mismatch (-want, +got):\n%s", diff)
						}
						if tc.expectedErrors[iter].err == nil {
							if diff := cmp.Diff(currentExpectedProductSummary, productSummary, cmpopts.IgnoreUnexported(api.ProductSummary{}), cmpopts.IgnoreUnexported(api.GetProductSummaryResponse{})); diff != "" {
								t.Fatalf("output mismatch (-want, +got):\n%s", diff)
							}
						}
					} else {
						if diff := cmp.Diff(currentExpectedProductSummary, productSummary, cmpopts.IgnoreUnexported(api.ProductSummary{}), cmpopts.IgnoreUnexported(api.GetProductSummaryResponse{})); diff != "" {
							t.Fatalf("output mismatch (-want, +got):\n%s", diff)
						}
					}

				}
			}
		})
	}
}

func TestGetProductDBFailureCases(t *testing.T) {
	ts := time.Now()
	tcs := []struct {
		Name                   string
		givenEnv               *string
		givenEnvGroup          *string
		expectedProductSummary []*api.ProductSummary
		expectedErr            error
		timestamp              *time.Time
		Setup                  []rp.Transformer
	}{
		{
			Name:        "get Product Overview with no env or envGroup",
			timestamp:   &ts,
			expectedErr: errMatcher{"Must have an environment or environmentGroup to get the product summary for"},
		},
		{
			Name:          "get Product Overview with both env and envGroup",
			givenEnv:      conversion.FromString("testing"),
			givenEnvGroup: conversion.FromString("testingGroup"),
			timestamp:     &ts,
			expectedErr:   errMatcher{"Can not have both an environment and environmentGroup to get the product summary for"},
		},
		{
			Name:      "invalid environment used",
			givenEnv:  conversion.FromString("staging"),
			timestamp: &ts,
			Setup: []rp.Transformer{

				&rp.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: conversion.FromString("dev"),
					},
				},
				&rp.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:    "example <example@example.com>",
					SourceCommitId:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:   "changed something (#678)",
					DisplayVersion:  "v1.0.2",
					WriteCommitData: true,
					Version:         1,
				},
				&rp.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
			},
			expectedErr:            errMatcher{"unable to get applications for environment 'staging': environment staging not found"},
			expectedProductSummary: []*api.ProductSummary{},
		},
		{
			Name:          "invalid envGroup used",
			timestamp:     &ts,
			givenEnvGroup: conversion.FromString("notDev"),
			Setup: []rp.Transformer{

				&rp.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: conversion.FromString("dev"),
					},
				},
				&rp.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:    "example <example@example.com>",
					SourceCommitId:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:   "changed something (#678)",
					DisplayVersion:  "v1.0.2",
					WriteCommitData: true,
					Version:         1,
				},
				&rp.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
			},
			expectedProductSummary: []*api.ProductSummary{},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			migrationsPath, err := testutil.CreateMigrationsPath(4)
			if err != nil {
				t.Fatal(err)
			}
			dbConfig := &db.DBConfig{
				DriverName:     "sqlite3",
				MigrationsPath: migrationsPath,
				WriteEslOnly:   false,
			}
			repo, err := setupRepositoryTestWithDB(t, dbConfig)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			config := rp.RepositoryConfig{
				ArgoCdGenerateFiles: true,
				DBHandler:           repo.State().DBHandler,
			}
			sv := &GitServer{OverviewService: &OverviewServiceServer{Repository: repo, Shutdown: shutdown}, Config: config}
			if !sv.Config.DBHandler.ShouldUseOtherTables() {
				t.Fatal("database is not setup correctly")
			}
			for _, transformer := range tc.Setup {
				err := repo.Apply(testutil.MakeTestContext(), transformer)
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx := testutil.MakeTestContext()
			err = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err2 := repo.State().DBHandler.DBWriteCommitTransactionTimestamp(ctx, transaction, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ts.UTC())
				return err2
			})
			if err != nil {
				t.Error(err)
			}
			if tc.timestamp != nil {
				_, err = sv.GetProductSummary(testutil.MakeTestContext(), &api.GetProductSummaryRequest{ManifestRepoCommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Environment: tc.givenEnv, EnvironmentGroup: tc.givenEnvGroup})

			} else {
				_, err = sv.GetProductSummary(testutil.MakeTestContext(), &api.GetProductSummaryRequest{ManifestRepoCommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Environment: tc.givenEnv, EnvironmentGroup: tc.givenEnvGroup})
			}
			if diff := cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
func TestGetCommitInfo(t *testing.T) {
	environmentSetup := []rp.Transformer{
		&rp.CreateEnvironment{
			Environment: "development-1",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Latest: true,
				},
				EnvironmentGroup: conversion.FromString("development"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "development-2",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Latest: true,
				},
				EnvironmentGroup: conversion.FromString("development"),
			},
		},
		&rp.CreateEnvironment{
			Environment: "development-3",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Latest: true,
				},
				EnvironmentGroup: conversion.FromString("development"),
			},
		},

		&rp.CreateEnvironment{
			Environment: "staging-1",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "development-1",
				},
				EnvironmentGroup: conversion.FromString("staging"),
			},
		},
	}

	type TestCase struct {
		name                   string
		transformers           []rp.Transformer
		request                *api.GetCommitInfoRequest
		allowReadingCommitData bool
		expectedResponse       *api.GetCommitInfoResponse
		expectedError          error
		testPageSize           bool
	}

	tcs := []TestCase{
		{
			name:         "check if the number of events is equal to pageNumber plus pageSize",
			testPageSize: true,
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application: "app",
					Manifests: map[string]string{
						"development-1": "manifest 1",
						"staging-1":     "manifest 2",
					},
					SourceCommitId:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					WriteCommitData: true,
				},
				&rp.CreateApplicationVersion{
					Application: "app",
					Manifests: map[string]string{
						"development-1": "manifest 1",
						"staging-1":     "manifest 2",
					},
					SourceCommitId:  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					WriteCommitData: true,
				},
				&rp.ReleaseTrain{
					Target:          "staging",
					WriteCommitData: true,
				},
			},
			allowReadingCommitData: true,
			request: &api.GetCommitInfoRequest{
				CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				PageNumber: 0,
			},
			expectedResponse: &api.GetCommitInfoResponse{
				CommitHash:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				LoadMore:      true,
				CommitMessage: "",
				TouchedApps: []string{
					"app",
				},
				Events: []*api.Event{
					{
						Uuid:      "df93c826-4f41-11ef-b685-00e04c684024",
						CreatedAt: uuid.TimeFromUUID("df93c826-4f41-11ef-b685-00e04c684024"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{
									"development-1",
									"staging-1",
								},
							},
						},
					},
					{
						Uuid:      "e15d9a99-4f41-11ef-9ae5-00e04c684024",
						CreatedAt: uuid.TimeFromUUID("e15d9a99-4f41-11ef-9ae5-00e04c684024"),
						EventType: &api.Event_DeploymentEvent{
							DeploymentEvent: &api.DeploymentEvent{
								Application:        "app",
								TargetEnvironment:  "development-1",
								ReleaseTrainSource: nil,
							},
						},
					},
				},
			},
		},
		{
			name: "create one commit with one app and get its info",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-1": "dev-manifest",
					},
					WriteCommitData: true,
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				PageNumber: 0,
			},
			allowReadingCommitData: true,
			expectedError:          nil,
			expectedResponse: &api.GetCommitInfoResponse{
				LoadMore:      false,
				CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				CommitMessage: "some message",
				TouchedApps: []string{
					"app",
				},
				Events: []*api.Event{
					{
						Uuid:      "00000000-0000-0000-0000-000000000000",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000000"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{"development-1"},
							},
						},
					},
					{
						Uuid:      "00000000-0000-0000-0000-000000000001",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000001"),
						EventType: &api.Event_DeploymentEvent{
							DeploymentEvent: &api.DeploymentEvent{
								Application:        "app",
								TargetEnvironment:  "development-1",
								ReleaseTrainSource: nil,
							},
						},
					},
				},
			},
		},
		{
			name: "create one commit with several apps and get its info",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application:    "app-1",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-1": "dev-manifest1",
					},
					WriteCommitData: true,
				},
				&rp.CreateApplicationVersion{
					Application:    "app-2",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-2": "dev-manifest2",
					},
					WriteCommitData: true,
				},
				&rp.CreateApplicationVersion{
					Application:    "app-3",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-3": "dev-manifest3",
					},
					WriteCommitData: true,
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				PageNumber: 0,
			},
			allowReadingCommitData: true,
			expectedError:          nil,
			expectedResponse: &api.GetCommitInfoResponse{
				LoadMore:      false,
				CommitHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				CommitMessage: "some message",
				TouchedApps: []string{
					"app-1",
					"app-2",
					"app-3",
				},
				Events: []*api.Event{
					{
						Uuid:      "00000000-0000-0000-0000-000000000000",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000000"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{"development-1"},
							},
						},
					},
					{
						Uuid:      "00000000-0000-0000-0000-000000000001",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000001"),
						EventType: &api.Event_DeploymentEvent{
							DeploymentEvent: &api.DeploymentEvent{
								Application:        "app-1",
								TargetEnvironment:  "development-1",
								ReleaseTrainSource: nil,
							},
						},
					},
					{
						Uuid:      "00000000-0000-0000-0000-000000000002",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000002"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{"development-2"},
							},
						},
					},
					{
						Uuid:      "00000000-0000-0000-0000-000000000003",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000003"),
						EventType: &api.Event_DeploymentEvent{
							DeploymentEvent: &api.DeploymentEvent{
								Application:        "app-2",
								TargetEnvironment:  "development-2",
								ReleaseTrainSource: nil,
							},
						},
					},
					{
						Uuid:      "00000000-0000-0000-0000-000000000004",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000004"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{"development-3"},
							},
						},
					},
					{
						Uuid:      "00000000-0000-0000-0000-000000000005",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000005"),
						EventType: &api.Event_DeploymentEvent{
							DeploymentEvent: &api.DeploymentEvent{
								Application:        "app-3",
								TargetEnvironment:  "development-3",
								ReleaseTrainSource: nil,
							},
						},
					},
				},
			},
		},
		{
			name: "create one commit with one app but get the info of a nonexistent commit",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application:     "app",
					SourceCommitId:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:   "some message",
					WriteCommitData: true,
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
			allowReadingCommitData: true,
			expectedError:          status.Error(codes.NotFound, "error: commit bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb was not found in the manifest repo"),
			expectedResponse:       nil,
		},
		{
			name: "find a commit by prefix",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application:     "app",
					SourceCommitId:  "32a5b7b27fe0e7c328e8ec4615cb34750bc328bd",
					SourceMessage:   "some message",
					WriteCommitData: true,
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "32a5b7b27",
				PageNumber: 0,
			},
			allowReadingCommitData: true,
			expectedResponse: &api.GetCommitInfoResponse{
				CommitHash:    "32a5b7b27fe0e7c328e8ec4615cb34750bc328bd",
				LoadMore:      false,
				CommitMessage: "some message",
				TouchedApps:   []string{"app"},
				Events: []*api.Event{
					{
						Uuid:      "00000000-0000-0000-0000-000000000000",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000000"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{},
						},
					},
				},
			},
		},
		{
			name: "no commit info returned if feature toggle not set",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-1": "dev-manifest",
					},
					WriteCommitData: true, // we still write the info …
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			allowReadingCommitData: false, // … but do not return it
			expectedError:          status.Error(codes.FailedPrecondition, "no written commit info available; set KUBERPULT_GIT_WRITE_COMMIT_DATA=true to enable"),
			expectedResponse:       nil,
		},
		{
			name: "no commit info written if toggle not set",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-1": "dev-manifest",
					},
					WriteCommitData: false, // do not write commit data …
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			allowReadingCommitData: true, // … but attempt to read anyway
			expectedError:          status.Error(codes.NotFound, "error: commit aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa was not found in the manifest repo"),
			expectedResponse:       nil,
		},
		{
			name: "events for release trains on environments are correctly retrieved by GetCommitInfo",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application: "app",
					Manifests: map[string]string{
						"development-1": "manifest 1",
						"staging-1":     "manifest 2",
					},
					SourceCommitId:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					WriteCommitData: true,
				},
				&rp.CreateApplicationVersion{
					Application: "app",
					Manifests: map[string]string{
						"development-1": "manifest 1",
						"staging-1":     "manifest 2",
					},
					SourceCommitId:  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					WriteCommitData: true,
				},
				&rp.ReleaseTrain{
					Target:          "staging-1",
					WriteCommitData: true,
				},
			},
			allowReadingCommitData: true,
			request: &api.GetCommitInfoRequest{
				CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				PageNumber: 0,
			},
			expectedResponse: &api.GetCommitInfoResponse{
				LoadMore:      false,
				CommitHash:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				CommitMessage: "",
				TouchedApps: []string{
					"app",
				},
				Events: []*api.Event{
					{
						Uuid:      "00000000-0000-0000-0000-000000000002",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000002"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{
									"development-1",
									"staging-1",
								},
							},
						},
					},
					{
						Uuid:      "00000000-0000-0000-0000-000000000003",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000003"),
						EventType: &api.Event_DeploymentEvent{
							DeploymentEvent: &api.DeploymentEvent{
								Application:        "app",
								TargetEnvironment:  "development-1",
								ReleaseTrainSource: nil,
							},
						},
					},

					{
						Uuid:      "00000000-0000-0000-0000-000000000005",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000005"),
						EventType: &api.Event_DeploymentEvent{
							DeploymentEvent: &api.DeploymentEvent{
								Application:       "app",
								TargetEnvironment: "staging-1",
								ReleaseTrainSource: &api.DeploymentEvent_ReleaseTrainSource{
									UpstreamEnvironment:    "development-1",
									TargetEnvironmentGroup: nil,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "release trains on environment groups are correctly retrieved by GetCommitInfo",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application: "app",
					Manifests: map[string]string{
						"development-1": "manifest 1",
						"staging-1":     "manifest 2",
					},
					SourceCommitId:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					WriteCommitData: true,
				},
				&rp.CreateApplicationVersion{
					Application: "app",
					Manifests: map[string]string{
						"development-1": "manifest 1",
						"staging-1":     "manifest 2",
					},
					SourceCommitId:  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					WriteCommitData: true,
				},
				&rp.ReleaseTrain{
					Target:          "staging",
					WriteCommitData: true,
				},
			},
			allowReadingCommitData: true,
			request: &api.GetCommitInfoRequest{
				CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				PageNumber: 0,
			},
			expectedResponse: &api.GetCommitInfoResponse{
				CommitHash:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				LoadMore:      false,
				CommitMessage: "",
				TouchedApps: []string{
					"app",
				},
				Events: []*api.Event{
					{
						Uuid:      "00000000-0000-0000-0000-000000000002",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000002"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{
									"development-1",
									"staging-1",
								},
							},
						},
					},
					{
						Uuid:      "00000000-0000-0000-0000-000000000003",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000003"),
						EventType: &api.Event_DeploymentEvent{
							DeploymentEvent: &api.DeploymentEvent{
								Application:        "app",
								TargetEnvironment:  "development-1",
								ReleaseTrainSource: nil,
							},
						},
					},

					{
						Uuid:      "00000000-0000-0000-0000-000000000005",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000005"),
						EventType: &api.Event_DeploymentEvent{
							DeploymentEvent: &api.DeploymentEvent{
								Application:       "app",
								TargetEnvironment: "staging-1",
								ReleaseTrainSource: &api.DeploymentEvent_ReleaseTrainSource{
									UpstreamEnvironment:    "development-1",
									TargetEnvironmentGroup: conversion.FromString("staging"),
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			repo, err := setupRepositoryTestWithoutDB(t)

			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			uuidGenerate := testutil.NewIncrementalUUIDGenerator()
			pageSize := 100
			if tc.testPageSize {
				uuidGenerate = testutil.NewIncrementalUUIDGeneratorForPageSizeTest()
				pageSize = 2
			}
			ctx := rp.AddGeneratorToContext(testutil.MakeTestContext(), uuidGenerate)

			for _, transformer := range environmentSetup {
				err := repo.Apply(ctx, transformer)
				if err != nil {
					t.Fatalf("expected no error in transformer but got:\n%v\n", err)
				}
			}
			for _, transformer := range tc.transformers {
				err := repo.Apply(ctx, transformer)
				if err != nil {
					t.Fatalf("expected no error in transformer but got:\n%v\n", err)
				}
			}

			config := rp.RepositoryConfig{
				WriteCommitData:     tc.allowReadingCommitData,
				ArgoCdGenerateFiles: true,
				DBHandler:           repo.State().DBHandler,
			}
			sv := &GitServer{
				OverviewService: &OverviewServiceServer{Repository: repo, Shutdown: shutdown, Context: context.Background()},
				Config:          config,
				PageSize:        uint64(pageSize),
			}

			commitInfo, err := sv.GetCommitInfo(ctx, tc.request)

			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}

			if commitInfo != nil {
				sort.Slice(commitInfo.Events, func(i, j int) bool {
					return commitInfo.Events[i].Uuid < commitInfo.Events[j].Uuid
				})
				for _, event := range commitInfo.Events {
					if createReleaseEvent, ok := event.EventType.(*api.Event_CreateReleaseEvent); ok {
						sort.Strings(createReleaseEvent.CreateReleaseEvent.EnvironmentNames)
					}
				}
			}

			if diff := cmp.Diff(tc.expectedResponse, commitInfo, protocmp.Transform()); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
