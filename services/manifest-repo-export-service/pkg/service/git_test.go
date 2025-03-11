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
	"sort"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	git "github.com/libgit2/git2go/v34"

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

func setupDBFixtures(ctx context.Context, dbHandler *db.DBHandler, transaction *sql.Tx) error {
	err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
	if err != nil {
		return err
	}
	fixtureAppications := []string{"app", "app-1", "app-2", "app-3"}
	eslVersion := 0
	for _, app := range fixtureAppications {
		err = dbHandler.DBInsertOrUpdateApplication(ctx, transaction, app, db.AppStateChangeCreate, db.DBAppMetaData{Team: "team"})
		if err != nil {
			return err
		}
		for releaseNumber := 1; releaseNumber < 4; releaseNumber++ {
			err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
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

func TestGetSyncData(t *testing.T) {
	const appName = "test-app-name"
	const anotherAppName = "yet-another-app-name"
	const envName = "test-env-name"
	const anotherEnvName = "yet-another-env-name"
	type TestSyncData struct {
		AppName string
		EnvName string
		status  db.SyncStatus
	}
	type TestCase struct {
		name             string
		dbInput          []TestSyncData
		expectedResponse *api.GetGitSyncStatusResponse
	}

	tcs := []TestCase{
		{
			name:             "No data",
			dbInput:          []TestSyncData{},
			expectedResponse: &api.GetGitSyncStatusResponse{},
		},
		{
			name: "One Unsynced app",
			dbInput: []TestSyncData{
				{
					AppName: appName,
					EnvName: envName,
					status:  db.UNSYNCED,
				},
			},
			expectedResponse: &api.GetGitSyncStatusResponse{
				AppStatuses: map[string]*api.EnvSyncStatus{
					appName: {
						EnvStatus: map[string]api.GitSyncStatus{
							envName: api.GitSyncStatus_GIT_SYNC_STATUS_UNSYNCED,
						},
					},
				},
			},
		},
		{
			name: "One SYNC_FAILED app",
			dbInput: []TestSyncData{
				{
					AppName: appName,
					EnvName: envName,
					status:  db.SYNC_FAILED,
				},
			},
			expectedResponse: &api.GetGitSyncStatusResponse{
				AppStatuses: map[string]*api.EnvSyncStatus{
					appName: {
						EnvStatus: map[string]api.GitSyncStatus{
							envName: api.GitSyncStatus_GIT_SYNC_STATUS_ERROR,
						},
					},
				},
			},
		},
		{
			name: "Multiple UNSYNCED app",
			dbInput: []TestSyncData{
				{
					AppName: appName,
					EnvName: envName,
					status:  db.UNSYNCED,
				},
				{
					AppName: anotherAppName,
					EnvName: envName,
					status:  db.UNSYNCED,
				},
			},
			expectedResponse: &api.GetGitSyncStatusResponse{
				AppStatuses: map[string]*api.EnvSyncStatus{
					appName: {
						EnvStatus: map[string]api.GitSyncStatus{
							envName: api.GitSyncStatus_GIT_SYNC_STATUS_UNSYNCED,
						},
					},
					anotherAppName: {
						EnvStatus: map[string]api.GitSyncStatus{
							envName: api.GitSyncStatus_GIT_SYNC_STATUS_UNSYNCED,
						},
					},
				},
			},
		},
		{
			name: "Multiple SYNC and SYNC failed apps, with some SYNCED aswell",
			dbInput: []TestSyncData{
				{
					AppName: appName,
					EnvName: envName,
					status:  db.UNSYNCED,
				},
				{
					AppName: anotherAppName,
					EnvName: envName,
					status:  db.SYNCED,
				},
				{
					AppName: appName,
					EnvName: anotherEnvName,
					status:  db.SYNC_FAILED,
				},
				{
					AppName: anotherAppName,
					EnvName: anotherEnvName,
					status:  db.SYNC_FAILED,
				},
			},
			expectedResponse: &api.GetGitSyncStatusResponse{
				AppStatuses: map[string]*api.EnvSyncStatus{
					appName: {
						EnvStatus: map[string]api.GitSyncStatus{
							envName:        api.GitSyncStatus_GIT_SYNC_STATUS_UNSYNCED,
							anotherEnvName: api.GitSyncStatus_GIT_SYNC_STATUS_ERROR,
						},
					},
					anotherAppName: {
						EnvStatus: map[string]api.GitSyncStatus{
							anotherEnvName: api.GitSyncStatus_GIT_SYNC_STATUS_ERROR,
						},
					},
				},
			},
		},
		{
			name: "All SYNCED returns nothing",
			dbInput: []TestSyncData{
				{
					AppName: appName,
					EnvName: envName,
					status:  db.SYNCED,
				},
				{
					AppName: anotherAppName,
					EnvName: envName,
					status:  db.SYNCED,
				},
				{
					AppName: appName,
					EnvName: anotherEnvName,
					status:  db.SYNCED,
				},
				{
					AppName: anotherAppName,
					EnvName: anotherEnvName,
					status:  db.SYNCED,
				},
			},
			expectedResponse: &api.GetGitSyncStatusResponse{},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := setupRepositoryTestWithPath(t)
			pageSize := 100
			ctx := testutil.MakeTestContext()
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
			//DB setup
			err := repo.State().DBHandler.WithTransactionR(ctx, 0, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, in := range tc.dbInput {
					err := repo.State().DBHandler.DBWriteNewSyncEvent(ctx, transaction, &db.GitSyncData{
						EnvName:    in.EnvName,
						AppName:    in.AppName,
						SyncStatus: in.status,
					})
					if err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("DB error no expected: %v", err)
			}
			response, err := sv.GetGitSyncStatus(ctx, &api.GetGitSyncStatusRequest{})
			if err != nil {
				t.Error(err)
			}
			if diff := cmp.Diff(tc.expectedResponse, response, protocmp.Transform()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}

		})
	}
}

func TestRetryEvent(t *testing.T) {
	const appName = "test-app-name"
	const anotherAppName = "yet-another-app-name"
	const envName = "test-env-name"
	const anotherEnvName = "yet-another-env-name"
	const testEventType = "test-event-type"

	type TestCase struct {
		name                   string
		initialFailedEslEvents []*db.EslFailedEventRow
		initialEslEvents       []*db.EslEventRow
		initialSyncData        []*db.GitSyncData
		initialDeployments     []*db.Deployment

		expectedFailedEvents []*db.EslFailedEventRow
		expectedEslEvents    []*db.EslEventRow
		expectedSyncData     []*db.GitSyncData
		expectedDeployments  []db.Deployment
		expectedError        error

		eventIdToRetry db.TransformerID
	}

	tcs := []TestCase{
		{
			name:           "No failed events - error ",
			eventIdToRetry: 0,
			initialSyncData: []*db.GitSyncData{
				{
					AppName:       appName,
					EnvName:       envName,
					TransformerID: 1,
					SyncStatus:    db.SYNCED,
				},
			},
			expectedSyncData: []*db.GitSyncData{
				{
					AppName:       appName,
					EnvName:       envName,
					TransformerID: 1,
					SyncStatus:    db.SYNCED,
				},
			},
			initialEslEvents: []*db.EslEventRow{
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
			initialFailedEslEvents: []*db.EslFailedEventRow{},
			expectedFailedEvents:   []*db.EslFailedEventRow{},
			initialDeployments: []*db.Deployment{
				{
					Created: time.Now(),
					Env:     envName,
					App:     appName,
					Metadata: db.DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: 1,
				},
			},
			expectedDeployments: []db.Deployment{
				{
					Created: time.Now(),
					Env:     envName,
					App:     appName,
					Metadata: db.DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: 1,
				},
			},
			expectedEslEvents: []*db.EslEventRow{ //DESC
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
			expectedError: errMatcher{
				msg: "Couldn't find failed event with eslVersion: 0",
			},
		},
		{
			name:           "Simple retry",
			eventIdToRetry: 1,
			initialSyncData: []*db.GitSyncData{
				{
					AppName:       appName,
					EnvName:       envName,
					TransformerID: 1,
					SyncStatus:    db.SYNC_FAILED,
				},
			},
			expectedSyncData: []*db.GitSyncData{
				{
					AppName:       appName,
					EnvName:       envName,
					TransformerID: 2,
					SyncStatus:    db.UNSYNCED,
				},
			},
			initialEslEvents: []*db.EslEventRow{
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
			initialFailedEslEvents: []*db.EslFailedEventRow{
				{
					EslVersion:            0, //Esl Version is not used with separation between current and history
					EventType:             testEventType,
					EventJson:             "{}",
					Reason:                "some-reason",
					TransformerEslVersion: 1,
				},
			},
			expectedFailedEvents: []*db.EslFailedEventRow{},
			expectedEslEvents: []*db.EslEventRow{ //DESC
				{
					EslVersion: 2,
					EventType:  testEventType,
					EventJson:  "{}",
				},
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
			initialDeployments: []*db.Deployment{
				{
					Created: time.Now(),
					Env:     envName,
					App:     appName,
					Metadata: db.DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: 1,
				},
			},
			expectedDeployments: []db.Deployment{
				{
					Created: time.Now(),
					Env:     envName,
					App:     appName,
					Metadata: db.DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: 2,
				},
			},
		},
		{
			name:           "Retry with many events",
			eventIdToRetry: 2,
			initialSyncData: []*db.GitSyncData{
				{
					AppName:       appName,
					EnvName:       envName,
					TransformerID: 2,
					SyncStatus:    db.SYNC_FAILED,
				},
				{
					AppName:       appName,
					EnvName:       anotherEnvName,
					TransformerID: 1,
					SyncStatus:    db.UNSYNCED,
				},
			},
			expectedSyncData: []*db.GitSyncData{
				{
					AppName:       appName,
					EnvName:       envName,
					TransformerID: 3,
					SyncStatus:    db.UNSYNCED,
				},
				{
					AppName:       appName,
					EnvName:       anotherEnvName,
					TransformerID: 1,
					SyncStatus:    db.UNSYNCED,
				},
			},
			initialEslEvents: []*db.EslEventRow{
				{
					EslVersion: 2,
					EventType:  testEventType,
					EventJson:  "{}",
				},
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
			initialFailedEslEvents: []*db.EslFailedEventRow{
				{
					EslVersion:            1,
					EventType:             testEventType,
					EventJson:             "{}",
					Reason:                "some-reason",
					TransformerEslVersion: 1,
				},
				{
					EslVersion:            2,
					EventType:             testEventType,
					EventJson:             "{}",
					Reason:                "some-reason",
					TransformerEslVersion: 2,
				},
			},
			expectedFailedEvents: []*db.EslFailedEventRow{
				{
					EslVersion:            0, //Esl Version is not used with separation between current and history
					EventType:             testEventType,
					EventJson:             "{}",
					Reason:                "some-reason",
					TransformerEslVersion: 1,
				},
			},
			expectedEslEvents: []*db.EslEventRow{ //DESC
				{
					EslVersion: 3,
					EventType:  testEventType,
					EventJson:  "{}",
				},
				{
					EslVersion: 2,
					EventType:  testEventType,
					EventJson:  "{}",
				},
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
			initialDeployments: []*db.Deployment{
				{
					Created: time.Now(),
					Env:     envName,
					App:     appName,
					Metadata: db.DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: 2,
				},
			},
			expectedDeployments: []db.Deployment{
				{
					Created: time.Now(),
					Env:     envName,
					App:     appName,
					Metadata: db.DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: 3,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := setupRepositoryTestWithPath(t)
			pageSize := 100
			ctx := testutil.MakeTestContext()
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
			//DB setup
			err := repo.State().DBHandler.WithTransactionR(ctx, 0, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, in := range tc.initialEslEvents {
					err := repo.State().DBHandler.DBWriteEslEventWithJson(ctx, transaction, in.EventType, in.EventJson)
					if err != nil {
						return err
					}
				}
				for _, in := range tc.initialFailedEslEvents {
					err := repo.State().DBHandler.DBInsertNewFailedESLEvent(ctx, transaction, in)
					if err != nil {
						return err
					}
				}

				for _, in := range tc.initialSyncData {
					err := repo.State().DBHandler.DBWriteNewSyncEvent(ctx, transaction, in)
					if err != nil {
						return err
					}
				}
				for _, in := range tc.initialDeployments {
					err := repo.State().DBHandler.DBUpdateOrCreateDeployment(ctx, transaction, *in)
					if err != nil {
						return err
					}
				}

				return nil
			})
			if err != nil {
				t.Fatalf("DB error no expected: %v", err)
			}
			_, err = sv.RetryFailedEvent(ctx, &api.RetryFailedEventRequest{Eslversion: uint64(tc.eventIdToRetry)})
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}

			err = repo.State().DBHandler.WithTransactionR(ctx, 0, true, func(ctx context.Context, transaction *sql.Tx) error {
				actualFailedEvents, err := repo.State().DBHandler.DBReadLastFailedEslEvents(ctx, transaction, 25, 0)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(tc.expectedFailedEvents, actualFailedEvents, cmpopts.IgnoreFields(db.EslFailedEventRow{}, "Created")); diff != "" {
					t.Errorf("failed events mismatch (-want, +got):\n%s", diff)
				}

				actualEvents, err := repo.State().DBHandler.DBReadLastEslEvents(ctx, transaction, 10)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(tc.expectedEslEvents, actualEvents, cmpopts.IgnoreFields(db.EslEventRow{}, "Created")); diff != "" {
					t.Errorf("esl events mismatch (-want, +got):\n%s", diff)
				}

				for _, in := range tc.expectedSyncData {
					currSyncData, err := repo.State().DBHandler.DBRetrieveSyncStatus(ctx, transaction, in.AppName, in.EnvName)
					if err != nil {
						return err
					}
					if diff := cmp.Diff(in, currSyncData); diff != "" {
						t.Errorf("sync status mismatch (-want, +got):\n%s", diff)
					}
				}

				finalDeployments, err := repo.State().DBHandler.DBSelectDeploymentsByTransformerID(ctx, transaction, tc.eventIdToRetry+1)
				if diff := cmp.Diff(tc.expectedDeployments, finalDeployments, cmpopts.IgnoreFields(db.Deployment{}, "Created")); diff != "" {
					t.Errorf("deployments mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})

		})
	}
}

func TestSkipEvent(t *testing.T) {
	const appName = "test-app-name"
	const anotherAppName = "yet-another-app-name"
	const envName = "test-env-name"
	const anotherEnvName = "yet-another-env-name"
	const testEventType = "test-event-type"

	type TestCase struct {
		name                   string
		initialFailedEslEvents []*db.EslFailedEventRow
		initialEslEvents       []*db.EslEventRow
		syncData               []*db.GitSyncData // for skipping events, syncData should remain unchanged

		expectedFailedEvents []*db.EslFailedEventRow
		expectedEslEvents    []*db.EslEventRow
		expectedError        error

		eventIdToSkip db.TransformerID
	}

	tcs := []TestCase{
		{
			name:          "No failed events - error ",
			eventIdToSkip: 1,
			syncData: []*db.GitSyncData{
				{
					AppName:       appName,
					EnvName:       envName,
					TransformerID: 1,
					SyncStatus:    db.SYNCED,
				},
			},
			initialEslEvents: []*db.EslEventRow{
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
			initialFailedEslEvents: []*db.EslFailedEventRow{},
			expectedFailedEvents:   []*db.EslFailedEventRow{},
			expectedEslEvents: []*db.EslEventRow{ //DESC
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
			expectedError: errMatcher{
				msg: "Couldn't find failed event with eslVersion: 1",
			},
		},
		{
			name:          "Simple skip",
			eventIdToSkip: 1,
			syncData: []*db.GitSyncData{
				{
					AppName:       appName,
					EnvName:       envName,
					TransformerID: 1,
					SyncStatus:    db.SYNC_FAILED,
				},
			},
			initialEslEvents: []*db.EslEventRow{
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
			initialFailedEslEvents: []*db.EslFailedEventRow{
				{
					EslVersion:            0,
					EventType:             testEventType,
					EventJson:             "{}",
					Reason:                "some-reason",
					TransformerEslVersion: 1,
				},
			},
			expectedFailedEvents: []*db.EslFailedEventRow{},
			expectedEslEvents: []*db.EslEventRow{ //DESC
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
		},
		{
			name:          "Retry with many events",
			eventIdToSkip: 2,
			syncData: []*db.GitSyncData{
				{
					AppName:       appName,
					EnvName:       envName,
					TransformerID: 2,
					SyncStatus:    db.SYNC_FAILED,
				},
				{
					AppName:       appName,
					EnvName:       anotherEnvName,
					TransformerID: 1,
					SyncStatus:    db.UNSYNCED,
				},
			},
			initialEslEvents: []*db.EslEventRow{
				{
					EslVersion: 2,
					EventType:  testEventType,
					EventJson:  "{}",
				},
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
			initialFailedEslEvents: []*db.EslFailedEventRow{
				{
					EslVersion:            0,
					EventType:             testEventType,
					EventJson:             "{}",
					Reason:                "some-reason",
					TransformerEslVersion: 1,
				},
				{
					EslVersion:            0,
					EventType:             testEventType,
					EventJson:             "{}",
					Reason:                "some-reason",
					TransformerEslVersion: 2,
				},
			},
			expectedFailedEvents: []*db.EslFailedEventRow{
				{
					EslVersion:            0,
					EventType:             testEventType,
					EventJson:             "{}",
					Reason:                "some-reason",
					TransformerEslVersion: 1,
				},
			},
			expectedEslEvents: []*db.EslEventRow{ //DESC
				{
					EslVersion: 2,
					EventType:  testEventType,
					EventJson:  "{}",
				},
				{
					EslVersion: 1,
					EventType:  testEventType,
					EventJson:  "{}",
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := setupRepositoryTestWithPath(t)
			pageSize := 100
			ctx := testutil.MakeTestContext()
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
			//DB setup
			err := repo.State().DBHandler.WithTransactionR(ctx, 0, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, in := range tc.initialEslEvents {
					err := repo.State().DBHandler.DBWriteEslEventWithJson(ctx, transaction, in.EventType, in.EventJson)
					if err != nil {
						return err
					}
				}
				for _, in := range tc.initialFailedEslEvents {
					err := repo.State().DBHandler.DBInsertNewFailedESLEvent(ctx, transaction, in)
					if err != nil {
						return err
					}
				}

				for _, in := range tc.syncData {
					err := repo.State().DBHandler.DBWriteNewSyncEvent(ctx, transaction, in)
					if err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("DB error no expected: %v", err)
			}
			_, err = sv.SkipEslEvent(ctx, &api.SkipEslEventRequest{EventEslVersion: uint64(tc.eventIdToSkip)})
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}

			err = repo.State().DBHandler.WithTransactionR(ctx, 0, true, func(ctx context.Context, transaction *sql.Tx) error {
				actualFailedEvents, err := repo.State().DBHandler.DBReadLastFailedEslEvents(ctx, transaction, 25, 0)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(tc.expectedFailedEvents, actualFailedEvents, cmpopts.IgnoreFields(db.EslFailedEventRow{}, "Created")); diff != "" {
					t.Errorf("failed events mismatch (-want, +got):\n%s", diff)
				}

				actualEvents, err := repo.State().DBHandler.DBReadLastEslEvents(ctx, transaction, 10)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(tc.expectedEslEvents, actualEvents, cmpopts.IgnoreFields(db.EslEventRow{}, "Created")); diff != "" {
					t.Errorf("esl events mismatch (-want, +got):\n%s", diff)
				}

				for _, in := range tc.syncData {
					currSyncData, err := repo.State().DBHandler.DBRetrieveSyncStatus(ctx, transaction, in.AppName, in.EnvName)
					if err != nil {
						return err
					}
					if diff := cmp.Diff(in, currSyncData); diff != "" {
						t.Errorf("sync status mismatch (-want, +got):\n%s", diff)
					}
				}

				return nil
			})

		})
	}
}

func TestGetGitTags(t *testing.T) {
	// When making this test with multiple test cases, the tags from both test cases
	// were returning when the repository wasn't the same. In a production environment these
	// tags wouldn't be created by kuberpult, but manually by a person. Trying to insert these
	// tags properly wasn't possible, as such, a single test case is used to prevent the aforementioned
	// issue.
	//
	// This endpoint only returns the tags from repository.GetTags, which has its own unit test with more
	// than one test case. If that function is working properly then so will this one.
	tagsToAdd := []string{"v1.0.1", "v0.0.1"}
	expectedTags := []*api.TagData{&api.TagData{Tag: "refs/tags/v0.0.1", CommitId: ""}, &api.TagData{Tag: "refs/tags/v1.0.1", CommitId: ""}}

	repo, remoteDir := setupRepositoryTestWithPath(t)
	config := rp.RepositoryConfig{
		ArgoCdGenerateFiles:  true,
		DBHandler:            repo.State().DBHandler,
		MinimizeExportedData: false,
		URL:                  "file://" + remoteDir,
		Path:                 remoteDir,
		CommitterEmail:       "kuberpult@freiheit.com",
		CommitterName:        "kuberpult",
		Branch:               "master",
	}
	sv := &GitServer{
		Repository: repo,
		Config:     config,
		PageSize:   uint64(100),
	}

	gitRepo, err := git.OpenRepository(config.Path)
	if err != nil {
		t.Fatal(err)
	}

	idx, err := gitRepo.Index()
	if err != nil {
		t.Fatal(err)
	}
	treeId, err := idx.WriteTree()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := gitRepo.LookupTree(treeId)
	if err != nil {
		t.Fatal(err)
	}
	oid, err := gitRepo.CreateCommit("HEAD", &git.Signature{Name: "SRE", Email: "testing@gmail"}, &git.Signature{Name: "SRE", Email: "testing@gmail"}, "testing", tree)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := gitRepo.LookupCommit(oid)
	if err != nil {
		t.Fatal(err)
	}

	for addTag := range tagsToAdd {
		_, err := gitRepo.Tags.Create(tagsToAdd[addTag], commit, &git.Signature{Name: "SRE", Email: "testing@gmail"}, "testing")
		if err != nil {
			t.Fatal(err)
		}
	}

	res, err := sv.GetGitTags(testutil.MakeTestContext(), &api.GetGitTagsRequest{})
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(expectedTags, res.TagData, cmpopts.IgnoreUnexported(api.TagData{}), cmpopts.IgnoreFields(api.TagData{}, "CommitId")); diff != "" {
		t.Fatalf("tags mismatch (-want, +got):\n%s", diff)
	}
}
