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
	"os/exec"
	"path"
	"sort"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/testutil"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/uuid"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	rp "github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
)

func setupRepositoryTestWithoutDB(t *testing.T) (rp.Repository, error) {
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Start()
	cmd.Wait()
	t.Logf("test created dir: %s", localDir)

	repoCfg := rp.RepositoryConfig{
		URL:                  remoteDir,
		Path:                 localDir,
		CommitterEmail:       "kuberpult@freiheit.com",
		CommitterName:        "kuberpult",
		ArgoCdGenerateFiles:  true,
		MinimizeExportedData: false,
	}
	repoCfg.DBHandler = nil

	repo, err := rp.New(
		testutil.MakeTestContext(),
		repoCfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, nil
}

func setupDBFixtures(ctx context.Context, dbHandler *db.DBHandler, transaction *sql.Tx) error {
	err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
	if err != nil {
		return err
	}
	fixtureAppications := []string{"app", "app-1", "app-2", "app-3"}
	err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslVersion), fixtureAppications)
	if err != nil {
		return err
	}
	eslVersion := 0
	for _, app := range fixtureAppications {
		err = dbHandler.DBInsertApplication(ctx, transaction, app, db.AppStateChangeCreate, db.DBAppMetaData{Team: "team"})
		if err != nil {
			return err
		}
		for releaseNumber := 1; releaseNumber < 4; releaseNumber++ {
			err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
				ReleaseNumber: uint64(releaseNumber),
				Created:       time.Time{},
				App:           app,
				Manifests:     db.DBReleaseManifests{},
				Metadata:      db.DBReleaseMetaData{},
			})
			if err != nil {
				return err
			}
		}
		err = dbHandler.DBInsertAllReleases(ctx, transaction, app, []int64{1, 2, 3}, db.EslVersion(eslVersion))
		if err != nil {
			return err
		}
		eslVersion++
	}
	fixtureEnvironments := []string{"development-1", "development-2", "development-3"}
	for _, env := range fixtureEnvironments {
		err = dbHandler.DBWriteEnvironment(ctx, transaction, env, config.EnvironmentConfig{
			Upstream: &config.EnvironmentConfigUpstream{
				Latest: true,
			},
		}, fixtureAppications)
		if err != nil {
			return err
		}
	}
	return nil
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
			TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
		},
		&rp.CreateEnvironment{
			Environment: "development-2",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Latest: true,
				},
				EnvironmentGroup: conversion.FromString("development"),
			},
			TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
		},
		&rp.CreateEnvironment{
			Environment: "development-3",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Latest: true,
				},
				EnvironmentGroup: conversion.FromString("development"),
			},
			TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
		},

		&rp.CreateEnvironment{
			Environment: "staging-1",
			Config: config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{
					Environment: "development-1",
				},
				EnvironmentGroup: conversion.FromString("staging"),
			},
			TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
		},
	}
	type initialEvent struct {
		api.Event
		CommitHash string
	}
	type TestCase struct {
		name                   string
		transformers           []rp.Transformer
		InitialEvents          []*initialEvent
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
					Team:        "team",
					Manifests: map[string]string{
						"development-1": "manifest 1",
						"staging-1":     "manifest 2",
					},
					SourceCommitId:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					WriteCommitData:     true,
					Version:             1,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&rp.CreateApplicationVersion{
					Application: "app",
					Team:        "team",
					Manifests: map[string]string{
						"development-1": "manifest 1",
						"staging-1":     "manifest 2",
					},
					SourceCommitId:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					WriteCommitData:     true,
					Version:             2,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&rp.ReleaseTrain{
					Target:              "staging",
					WriteCommitData:     true,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
			},
			InitialEvents: []*initialEvent{
				{
					Event: api.Event{
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
					CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
				{
					Event: api.Event{
						Uuid:      "e15d9a99-4f41-11ef-9ae5-00e04c684023",
						CreatedAt: uuid.TimeFromUUID("e15d9a99-4f41-11ef-9ae5-00e04c684023"),
						EventType: &api.Event_DeploymentEvent{
							DeploymentEvent: &api.DeploymentEvent{
								Application:        "app",
								TargetEnvironment:  "development-1",
								ReleaseTrainSource: nil,
							},
						},
					},
					CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
				{
					Event: api.Event{
						Uuid:      "e4f13c8b-4f41-11ef-9735-00e04c684025",
						CreatedAt: uuid.TimeFromUUID("e4f13c8b-4f41-11ef-9735-00e04c684025"),
						EventType: &api.Event_DeploymentEvent{
							DeploymentEvent: &api.DeploymentEvent{
								Application:        "app",
								TargetEnvironment:  "development-1",
								ReleaseTrainSource: nil,
							},
						},
					},
					CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
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
						Uuid:      "e15d9a99-4f41-11ef-9ae5-00e04c684023",
						CreatedAt: uuid.TimeFromUUID("e15d9a99-4f41-11ef-9ae5-00e04c684023"),
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
					Team:           "team",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-1": "dev-manifest",
					},
					Version:             1,
					WriteCommitData:     true,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
			},
			InitialEvents: []*initialEvent{
				{
					Event: api.Event{
						Uuid:      "00000000-0000-0000-0000-000000000000",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000000"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{"development-1"},
							},
						},
					},
					CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
				{
					Event: api.Event{
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
					CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
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
					Team:           "team",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-1": "dev-manifest1",
					},
					Version:             1,
					WriteCommitData:     true,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&rp.CreateApplicationVersion{
					Application:    "app-2",
					Team:           "team",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-2": "dev-manifest2",
					},
					Version:             1,
					WriteCommitData:     true,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&rp.CreateApplicationVersion{
					Application:    "app-3",
					Team:           "team",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-3": "dev-manifest3",
					},
					Version:             1,
					WriteCommitData:     true,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				PageNumber: 0,
			},
			allowReadingCommitData: true,
			InitialEvents: []*initialEvent{
				{
					Event: api.Event{
						Uuid:      "00000000-0000-0000-0000-000000000000",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000000"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{"development-1"},
							},
						},
					},
					CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
				{
					Event: api.Event{
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
					CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
				{
					Event: api.Event{
						Uuid:      "00000000-0000-0000-0000-000000000002",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000002"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{"development-2"},
							},
						},
					},
					CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
				{
					Event: api.Event{
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
					CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
				{
					Event: api.Event{
						Uuid:      "00000000-0000-0000-0000-000000000004",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000004"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{"development-3"},
							},
						},
					},
					CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
				{
					Event: api.Event{
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
					CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
			},
			expectedError: nil,
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
					Application:         "app",
					Team:                "team",
					SourceCommitId:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:       "some message",
					Version:             1,
					WriteCommitData:     true,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
			},
			InitialEvents: []*initialEvent{
				{
					Event: api.Event{
						Uuid:      "00000000-0000-0000-0000-000000000000",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000000"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{"development-1"},
							},
						},
					},
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
					Team:                "team",
					Application:         "app",
					SourceCommitId:      "32a5b7b27fe0e7c328e8ec4615cb34750bc328bd",
					SourceMessage:       "some message",
					WriteCommitData:     true,
					Version:             1,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "32a5b7b27",
				PageNumber: 0,
			},
			InitialEvents: []*initialEvent{
				{
					Event: api.Event{
						Uuid:      "00000000-0000-0000-0000-000000000000",
						CreatedAt: uuid.TimeFromUUID("00000000-0000-0000-0000-000000000000"),
						EventType: &api.Event_CreateReleaseEvent{
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{"staging"},
							},
						},
					},
					CommitHash: "32a5b7b27fe0e7c328e8ec4615cb34750bc328bd",
				},
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
							CreateReleaseEvent: &api.CreateReleaseEvent{
								EnvironmentNames: []string{"staging"},
							},
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
					Team:           "team",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-1": "dev-manifest",
					},
					Version:             1,
					WriteCommitData:     false, // we still write the info …
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			allowReadingCommitData: false, // … but do not return it
			expectedError:          status.Error(codes.NotFound, "error: commit aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa was not found in the manifest repo"),
			expectedResponse:       nil,
		},
		{
			name: "no commit info written if toggle not set",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application:    "app",
					Team:           "team",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
					Manifests: map[string]string{
						"development-1": "dev-manifest",
					},
					Version:             1,
					WriteCommitData:     false, // do not write commit data …
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
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
					Team:        "team",
					Manifests: map[string]string{
						"development-1": "manifest 1",
						"staging-1":     "manifest 2",
					},
					Version:             1,
					SourceCommitId:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					WriteCommitData:     true,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&rp.CreateApplicationVersion{
					Application: "app",
					Team:        "team",
					Manifests: map[string]string{
						"development-1": "manifest 1",
						"staging-1":     "manifest 2",
					},
					Version:             2,
					SourceCommitId:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					WriteCommitData:     true,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&rp.ReleaseTrain{
					Target:              "staging-1",
					WriteCommitData:     true,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
			},
			InitialEvents: []*initialEvent{
				{
					Event: api.Event{
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
					CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
				{
					Event: api.Event{
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
					CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},

				{
					Event: api.Event{
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
					CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
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
					Version:             1,
					SourceCommitId:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					WriteCommitData:     true,
					Team:                "team",
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&rp.CreateApplicationVersion{
					Application: "app",
					Manifests: map[string]string{
						"development-1": "manifest 1",
						"staging-1":     "manifest 2",
					},
					Version:             2,
					SourceCommitId:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					WriteCommitData:     true,
					Team:                "team",
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&rp.ReleaseTrain{
					Target:              "staging",
					WriteCommitData:     true,
					TransformerMetadata: rp.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
			},
			allowReadingCommitData: true,
			request: &api.GetCommitInfoRequest{
				CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				PageNumber: 0,
			},
			InitialEvents: []*initialEvent{
				{
					Event: api.Event{Uuid: "00000000-0000-0000-0000-000000000002",
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
					CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
				{
					Event: api.Event{
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
					CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
				{
					Event: api.Event{
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
					CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
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
								Application:        "app",
								TargetEnvironment:  "staging-1",
								ReleaseTrainSource: nil,
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
			repo, _ := setupRepositoryTestWithPath(t)

			uuidGenerate := testutil.NewIncrementalUUIDGenerator()
			pageSize := 100
			if tc.testPageSize {
				uuidGenerate = testutil.NewIncrementalUUIDGeneratorForPageSizeTest()
				pageSize = 2
			}
			ctx := rp.AddGeneratorToContext(testutil.MakeTestContext(), uuidGenerate)
			dbHandler := repo.State().DBHandler

			err := dbHandler.WithTransactionR(ctx, 0, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := setupDBFixtures(ctx, dbHandler, transaction)
				if err != nil {
					return err
				}
				for _, initialEvent := range tc.InitialEvents {
					switch initialEvent.EventType.(type) {
					case *api.Event_CreateReleaseEvent:
						eventType := initialEvent.EventType.(*api.Event_CreateReleaseEvent)
						environments := make(map[string]struct{}, 0)
						for _, environment := range eventType.CreateReleaseEvent.EnvironmentNames {
							environments[environment] = struct{}{}
						}
						err := dbHandler.DBWriteNewReleaseEvent(ctx, transaction, db.TransformerID(0), 1, initialEvent.Uuid, initialEvent.CommitHash, &event.NewRelease{Environments: environments})
						if err != nil {
							return err
						}
					case *api.Event_DeploymentEvent:
						eventType := initialEvent.EventType.(*api.Event_DeploymentEvent)
						err := dbHandler.DBWriteDeploymentEvent(ctx, transaction, db.TransformerID(0), initialEvent.Uuid, initialEvent.CommitHash, &event.Deployment{Application: eventType.DeploymentEvent.Application, Environment: eventType.DeploymentEvent.TargetEnvironment})
						if err != nil {
							return err
						}
					}
				}
				for _, transformer := range environmentSetup {
					err := repo.Apply(ctx, transaction, transformer)
					if err != nil {
						return err
					}
				}
				for _, transformer := range tc.transformers {
					err := repo.Apply(ctx, transaction, transformer)
					if err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Apply error: %v", err)
			}

			config := rp.RepositoryConfig{
				ArgoCdGenerateFiles:  true,
				DBHandler:            repo.State().DBHandler,
				MinimizeExportedData: false,
			}
			sv := &GitServer{
				Repository: repo,
				Config:     config,
				PageSize:   uint64(pageSize),
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
