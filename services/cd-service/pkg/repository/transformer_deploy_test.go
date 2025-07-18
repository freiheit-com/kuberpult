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
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/google/go-cmp/cmp/cmpopts"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

func TestDeployApplicationVersionPartOne(t *testing.T) {
	var devConfig = config.EnvironmentConfig{
		Upstream:         nil,
		ArgoCd:           nil,
		EnvironmentGroup: conversion.FromString("mygroup"),
	}
	var stageConfig = config.EnvironmentConfig{
		Upstream: &config.EnvironmentConfigUpstream{
			Environment: "dev",
			Latest:      false,
		},
		ArgoCd:           nil,
		EnvironmentGroup: conversion.FromString("staging-group"),
	}
	const (
		manifestDev     = "hello-dev"
		manifestStaging = "bye-staging"
		firstCommitHash = "cafe456789012345789001234578900123457890"
	)
	setupTransformers := []Transformer{
		&CreateEnvironment{
			Authentication: Authentication{},
			Environment:    "dev",
			Config:         devConfig,
		},
		&CreateEnvironment{
			Authentication: Authentication{},
			Environment:    "staging",
			Config:         stageConfig,
		},
		&CreateApplicationVersion{
			Authentication: Authentication{},
			Version:        1,
			Application:    "app1",
			Manifests: map[types.EnvName]string{
				"dev":     manifestDev,
				"staging": manifestStaging,
			},
			SourceCommitId:        firstCommitHash,
			SourceAuthor:          "",
			SourceMessage:         "",
			Team:                  "team-sre",
			DisplayVersion:        "",
			WriteCommitData:       true,
			PreviousCommit:        "",
			CiLink:                "",
			AllowedDomains:        nil,
			TransformerEslVersion: 0,
			IsPrepublish:          false,
		},
	}
	tcs := []struct {
		Name              string
		Transformer       *DeployApplicationVersion
		expectedPrognosis *DeployPrognosis
	}{
		{
			Name: "Prognosis",
			Transformer: &DeployApplicationVersion{
				Authentication:        Authentication{},
				Environment:           "dev",
				Application:           "app1",
				Version:               1,
				LockBehaviour:         0,
				WriteCommitData:       true,
				SourceTrain:           nil,
				Author:                "",
				CiLink:                "",
				TransformerEslVersion: 0,
				SkipCleanup:           false,
			},
			expectedPrognosis: &DeployPrognosis{
				TeamName:           "team-sre",
				EnvironmentConfig:  &devConfig,
				ManifestContent:    []byte(manifestDev),
				EnvLocks:           map[string]Lock{},
				AppLocks:           map[string]Lock{},
				TeamLocks:          map[string]Lock{},
				NewReleaseCommitId: firstCommitHash,
				ExistingDeployment: nil,
				OldReleaseCommitId: "",
			},
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
			err = repo.Apply(ctx, setupTransformers...)
			if err != nil {
				t.Fatalf("setup error could not set up transformers \n%v", err)
			}

			const readonly = true // this is important to ensure that we do not write any data in the prognosis
			actualPrognosis, err := db.WithTransactionT(r.DB, ctx, 0, readonly, func(ctx context.Context, transaction *sql.Tx) (*DeployPrognosis, error) {
				prognosis, err2 := tc.Transformer.Prognosis(ctx, repo.State(), transaction)
				if err2 != nil {
					return nil, err2
				}
				return prognosis, nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}

			if diff := cmp.Diff(tc.expectedPrognosis, actualPrognosis); diff != "" {
				t.Fatalf("error mismatch on prognosis (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestDeployApplicationVersionPartTwo(t *testing.T) {
	var devConfig = config.EnvironmentConfig{
		Upstream:         nil,
		ArgoCd:           nil,
		EnvironmentGroup: conversion.FromString("mygroup"),
	}
	var stageConfig = config.EnvironmentConfig{
		Upstream: &config.EnvironmentConfigUpstream{
			Environment: "dev",
			Latest:      false,
		},
		ArgoCd:           nil,
		EnvironmentGroup: conversion.FromString("staging-group"),
	}
	const (
		manifestDev     = "hello-dev"
		manifestStaging = "bye-staging"
		firstCommitHash = "cafe456789012345789001234578900123457890"
		ciLink          = "https://example.com/42"
	)
	var version uint64 = 1
	setupTransformers := []Transformer{
		&CreateEnvironment{
			Authentication: Authentication{},
			Environment:    "dev",
			Config:         devConfig,
		},
		&CreateEnvironment{
			Authentication: Authentication{},
			Environment:    "staging",
			Config:         stageConfig,
		},
		&CreateApplicationVersion{
			Authentication: Authentication{},
			Version:        1,
			Application:    "app1",
			Manifests: map[types.EnvName]string{
				"dev":     manifestDev,
				"staging": manifestStaging,
			},
			SourceCommitId:        firstCommitHash,
			SourceAuthor:          "",
			SourceMessage:         "",
			Team:                  "team-sre",
			DisplayVersion:        "",
			WriteCommitData:       true,
			PreviousCommit:        "",
			CiLink:                "",
			AllowedDomains:        nil,
			TransformerEslVersion: 0,
			IsPrepublish:          false,
		},
	}
	tcs := []struct {
		Name               string
		Transformer        *DeployApplicationVersion
		inputPrognosis     *DeployPrognosis
		expectedDeployment *db.Deployment
	}{
		{
			Name: "DeployPart2s",
			Transformer: &DeployApplicationVersion{
				Authentication:        Authentication{},
				Environment:           "dev",
				Application:           "app1",
				Version:               1,
				LockBehaviour:         0,
				WriteCommitData:       true,
				SourceTrain:           nil,
				Author:                "",
				CiLink:                ciLink,
				TransformerEslVersion: 0,
				SkipCleanup:           false,
			},
			inputPrognosis: &DeployPrognosis{
				TeamName:           "team-sre",
				EnvironmentConfig:  &devConfig,
				ManifestContent:    []byte(manifestDev),
				EnvLocks:           map[string]Lock{},
				AppLocks:           map[string]Lock{},
				TeamLocks:          map[string]Lock{},
				NewReleaseCommitId: firstCommitHash,
				ExistingDeployment: nil,
				OldReleaseCommitId: "",
			},
			expectedDeployment: &db.Deployment{
				Created: time.Time{}, // ignored
				App:     "app1",
				Env:     "dev",
				ReleaseNumbers: types.ReleaseNumbers{
					Revision: 0,
					Version:  &version,
				},
				Metadata: db.DeploymentMetadata{
					DeployedByName:  "test tester",
					DeployedByEmail: "testmail@example.com",
					CiLink:          ciLink,
				},
				TransformerID: 0,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			repo, _ := SetupRepositoryTestWithDBOptions(t, false)
			r := repo.(*repository)
			err := repo.Apply(ctx, setupTransformers...)
			if err != nil {
				t.Fatalf("setup error could not set up transformers \n%v", err)
			}
			s := repo.State()
			const readonly = false // applying a prognosis needs write access
			_, err = db.WithTransactionT[string](r.DB, ctx, 0, readonly, func(ctx context.Context, transaction *sql.Tx) (*string, error) {
				var transformerContext TransformerContext = &transformerRunner{
					ChangedApps:     nil,
					DeletedRootApps: nil,
					State:           s,
				}
				_, err2 := tc.Transformer.ApplyPrognosis(ctx, s, transformerContext, transaction, tc.inputPrognosis)
				if err2 != nil {
					return nil, err2
				}
				return nil, nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
			actualDeployment, err := db.WithTransactionT(s.DBHandler, ctx, 0, false, func(ctx context.Context, transaction *sql.Tx) (*db.Deployment, error) {
				deployment, err := s.DBHandler.DBSelectLatestDeployment(ctx, transaction, "app1", "dev")
				if err != nil {
					return nil, err
				}
				return deployment, nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}

			if diff := cmp.Diff(tc.expectedDeployment, actualDeployment, cmpopts.IgnoreFields(db.Deployment{}, "Created")); diff != "" {
				t.Fatalf("error mismatch on prognosis (-want, +got):\n%s", diff)
			}

		})
	}
}

func TestDeployApplicationVersionWithRevision(t *testing.T) {
	var devConfig = config.EnvironmentConfig{
		Upstream:         nil,
		ArgoCd:           nil,
		EnvironmentGroup: conversion.FromString("mygroup"),
	}
	var stageConfig = config.EnvironmentConfig{
		Upstream: &config.EnvironmentConfigUpstream{
			Environment: "dev",
			Latest:      false,
		},
		ArgoCd:           nil,
		EnvironmentGroup: conversion.FromString("staging-group"),
	}
	const (
		manifestDev     = "hello-dev"
		manifestStaging = "bye-staging"
		firstCommitHash = "cafe456789012345789001234578900123457890"
		ciLink          = "https://example.com/42"
	)
	var version uint64 = 1
	var revision uint64 = 2
	setupTransformers := []Transformer{
		&CreateEnvironment{
			Authentication: Authentication{},
			Environment:    "dev",
			Config:         devConfig,
		},
		&CreateEnvironment{
			Authentication: Authentication{},
			Environment:    "staging",
			Config:         stageConfig,
		},
		&CreateApplicationVersion{
			Authentication: Authentication{},
			Version:        1,
			Revision:       revision,
			Application:    "app1",
			Manifests: map[types.EnvName]string{
				"dev":     manifestDev,
				"staging": manifestStaging,
			},
			SourceCommitId:        firstCommitHash,
			SourceAuthor:          "",
			SourceMessage:         "",
			Team:                  "team-sre",
			DisplayVersion:        "",
			WriteCommitData:       true,
			PreviousCommit:        "",
			CiLink:                "",
			AllowedDomains:        nil,
			TransformerEslVersion: 0,
			IsPrepublish:          false,
		},
	}
	tcs := []struct {
		Name               string
		Transformer        *DeployApplicationVersion
		inputPrognosis     *DeployPrognosis
		expectedDeployment *db.Deployment
	}{
		{
			Name: "DeployPart2s",
			Transformer: &DeployApplicationVersion{
				Authentication:        Authentication{},
				Environment:           "dev",
				Application:           "app1",
				Version:               1,
				Revision:              revision,
				LockBehaviour:         0,
				WriteCommitData:       true,
				SourceTrain:           nil,
				Author:                "",
				CiLink:                ciLink,
				TransformerEslVersion: 0,
				SkipCleanup:           false,
			},
			inputPrognosis: &DeployPrognosis{
				TeamName:           "team-sre",
				EnvironmentConfig:  &devConfig,
				ManifestContent:    []byte(manifestDev),
				EnvLocks:           map[string]Lock{},
				AppLocks:           map[string]Lock{},
				TeamLocks:          map[string]Lock{},
				NewReleaseCommitId: firstCommitHash,
				ExistingDeployment: nil,
				OldReleaseCommitId: "",
			},
			expectedDeployment: &db.Deployment{
				Created: time.Time{}, // ignored
				App:     "app1",
				Env:     "dev",
				ReleaseNumbers: types.ReleaseNumbers{
					Revision: revision,
					Version:  &version,
				},
				Metadata: db.DeploymentMetadata{
					DeployedByName:  "test tester",
					DeployedByEmail: "testmail@example.com",
					CiLink:          ciLink,
				},
				TransformerID: 0,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			repo, _ := SetupRepositoryTestWithDBOptions(t, false)
			r := repo.(*repository)
			err := repo.Apply(ctx, setupTransformers...)
			if err != nil {
				t.Fatalf("setup error could not set up transformers \n%v", err)
			}
			s := repo.State()
			const readonly = false // applying a prognosis needs write access
			_, err = db.WithTransactionT[string](r.DB, ctx, 0, readonly, func(ctx context.Context, transaction *sql.Tx) (*string, error) {
				var transformerContext TransformerContext = &transformerRunner{
					ChangedApps:     nil,
					DeletedRootApps: nil,
					State:           s,
				}
				_, err2 := tc.Transformer.ApplyPrognosis(ctx, s, transformerContext, transaction, tc.inputPrognosis)
				if err2 != nil {
					return nil, err2
				}
				return nil, nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
			actualDeployment, err := db.WithTransactionT(s.DBHandler, ctx, 0, false, func(ctx context.Context, transaction *sql.Tx) (*db.Deployment, error) {
				deployment, err := s.DBHandler.DBSelectLatestDeployment(ctx, transaction, "app1", "dev")
				if err != nil {
					return nil, err
				}
				return deployment, nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}

			if diff := cmp.Diff(tc.expectedDeployment, actualDeployment, cmpopts.IgnoreFields(db.Deployment{}, "Created")); diff != "" {
				t.Fatalf("error mismatch on prognosis (-want, +got):\n%s", diff)
			}

		})
	}
}
