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
	"strconv"
	"strings"
	"testing"
	"time"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"


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
