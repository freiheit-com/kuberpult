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
package db

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
)

func getOverviewIgnoredTypes() cmp.Option {
	return cmpopts.IgnoreUnexported(api.GetOverviewResponse{},
		api.EnvironmentGroup{},
		api.Environment{},
		api.Application{},
		api.Release{},
		timestamppb.Timestamp{},
		api.EnvironmentConfig{},
		api.EnvironmentConfig_Upstream{},
		api.Environment_Application{},
		api.Environment_Application_DeploymentMetaData{},
		api.EnvironmentConfig_ArgoCD{},
		api.Lock{},
		api.Actor{})
}

func makeTestStartingOverview() *api.GetOverviewResponse {
	var dev = "dev"
	var upstreamLatest = true
	startingOverview := &api.GetOverviewResponse{
		EnvironmentGroups: []*api.EnvironmentGroup{
			{
				EnvironmentGroupName: "dev",
				Environments: []*api.Environment{
					{
						Name: "development",
						Config: &api.EnvironmentConfig{
							Upstream: &api.EnvironmentConfig_Upstream{
								Latest: &upstreamLatest,
							},
							Argocd:           &api.EnvironmentConfig_ArgoCD{},
							EnvironmentGroup: &dev,
						},
						Applications: map[string]*api.Environment_Application{
							"test": {
								Name:    "test",
								Version: 1,
								DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
									DeployAuthor: "testmail@example.com",
									DeployTime:   "1",
								},
								Team: "team-123",
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				Priority: api.Priority_YOLO,
			},
		},
		Applications: map[string]*api.Application{
			"test": {
				Name: "test",
				Releases: []*api.Release{
					{
						Version:        1,
						SourceCommitId: "deadbeef",
						SourceAuthor:   "example <example@example.com>",
						SourceMessage:  "changed something (#678)",
						PrNumber:       "678",
						CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
					},
				},
				Team: "team-123",
			},
		},
		GitRevision: "0",
	}
	return startingOverview
}

func TestUpdateOverviewTeamLock(t *testing.T) {
	var dev = "dev"
	var upstreamLatest = true
	startingOverview := makeTestStartingOverview()
	tcs := []struct {
		Name              string
		NewTeamLock       TeamLock
		ExcpectedOverview *api.GetOverviewResponse
		ExpectedError     error
	}{
		{
			Name: "Update overview",
			NewTeamLock: TeamLock{
				Env:        "development",
				Team:       "team-123",
				LockID:     "dev-lock",
				EslVersion: 2,
				Deleted:    false,
				Metadata: LockMetadata{
					Message:        "My lock on dev for my-team",
					CreatedByName:  "myself",
					CreatedByEmail: "myself@example.com",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExcpectedOverview: &api.GetOverviewResponse{
				EnvironmentGroups: []*api.EnvironmentGroup{
					{
						EnvironmentGroupName: "dev",
						Environments: []*api.Environment{
							{
								Name: "development",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: &upstreamLatest,
									},
									Argocd:           &api.EnvironmentConfig_ArgoCD{},
									EnvironmentGroup: &dev,
								},
								Applications: map[string]*api.Environment_Application{
									"test": {
										Name:    "test",
										Version: 1,
										DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
											DeployAuthor: "testmail@example.com",
											DeployTime:   "1",
										},
										Team: "team-123",
										Locks: map[string]*api.Lock{
											"dev-lock": {
												Message:   "My lock on dev for my-team",
												LockId:    "dev-lock",
												CreatedAt: timestamppb.New(time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC)),
												CreatedBy: &api.Actor{
													Name:  "myself",
													Email: "myself@example.com",
												},
											},
										},
									},
								},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				Applications: map[string]*api.Application{
					"test": {
						Name: "test",
						Releases: []*api.Release{
							{
								Version:        1,
								SourceCommitId: "deadbeef",
								SourceAuthor:   "example <example@example.com>",
								SourceMessage:  "changed something (#678)",
								PrNumber:       "678",
								CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							},
						},
						Team: "team-123",
					},
				},
				GitRevision: "0",
			},
		},
		{
			Name: "env does not exists",
			NewTeamLock: TeamLock{
				Env:        "does-not-exists",
				Team:       "team-123",
				LockID:     "dev-lock",
				EslVersion: 2,
				Deleted:    false,
				Metadata: LockMetadata{
					Message:        "My lock on dev for my-team",
					CreatedByName:  "myself",
					CreatedByEmail: "myself@example.com",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExpectedError: errMatcher{"could not find environment does-not-exists in overview"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.WriteOverviewCache(ctx, transaction, startingOverview)
				if err != nil {
					return err
				}
				err = dbHandler.UpdateOverviewTeamLock(ctx, transaction, tc.NewTeamLock)
				if err != nil {
					if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
						return fmt.Errorf("mismatch between errors (-want +got):\n%s", diff)
					}
					return nil
				}
				latestOverview, err := dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					return err
				}
				opts := getOverviewIgnoredTypes()
				if diff := cmp.Diff(tc.ExcpectedOverview, latestOverview, opts); diff != "" {
					return fmt.Errorf("mismatch (-want +got):\n%s", diff)
				}
				tc.NewTeamLock.Deleted = true
				err = dbHandler.UpdateOverviewTeamLock(ctx, transaction, tc.NewTeamLock)
				if err != nil {
					return err
				}
				latestOverview, err = dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(startingOverview, latestOverview, opts); diff != "" {
					return fmt.Errorf("mismatch (-want +got):\n%s", diff)
				}
				return nil
			})

			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUpdateOverviewEnvironmentLock(t *testing.T) {
	var dev = "dev"
	var upstreamLatest = true
	startingOverview := makeTestStartingOverview()
	tcs := []struct {
		Name               string
		NewEnvironmentLock EnvironmentLock
		ExcpectedOverview  *api.GetOverviewResponse
		ExpectedError      error
	}{
		{
			Name: "Update overview",
			NewEnvironmentLock: EnvironmentLock{
				Env:        "development",
				LockID:     "dev-lock",
				EslVersion: 2,
				Deleted:    false,
				Metadata: LockMetadata{
					Message:        "My lock on dev",
					CreatedByName:  "myself",
					CreatedByEmail: "myself@example.com",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExcpectedOverview: &api.GetOverviewResponse{
				EnvironmentGroups: []*api.EnvironmentGroup{
					{
						EnvironmentGroupName: "dev",
						Environments: []*api.Environment{
							{
								Name: "development",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: &upstreamLatest,
									},
									Argocd:           &api.EnvironmentConfig_ArgoCD{},
									EnvironmentGroup: &dev,
								},
								Applications: map[string]*api.Environment_Application{
									"test": {
										Name:    "test",
										Version: 1,
										DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
											DeployAuthor: "testmail@example.com",
											DeployTime:   "1",
										},
										Team: "team-123",
									},
								},
								Locks: map[string]*api.Lock{
									"dev-lock": {
										LockId:    "dev-lock",
										Message:   "My lock on dev",
										CreatedAt: timestamppb.New(time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC)),
										CreatedBy: &api.Actor{
											Name:  "myself",
											Email: "myself@example.com",
										},
									},
								},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				Applications: map[string]*api.Application{
					"test": {
						Name: "test",
						Releases: []*api.Release{
							{
								Version:        1,
								SourceCommitId: "deadbeef",
								SourceAuthor:   "example <example@example.com>",
								SourceMessage:  "changed something (#678)",
								PrNumber:       "678",
								CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							},
						},
						Team: "team-123",
					},
				},
				GitRevision: "0",
			},
		},
		{
			Name: "env does not exists",
			NewEnvironmentLock: EnvironmentLock{
				Env:        "does-not-exists",
				LockID:     "dev-lock",
				EslVersion: 2,
				Deleted:    false,
				Metadata: LockMetadata{
					Message:        "My lock on dev for my-team",
					CreatedByName:  "myself",
					CreatedByEmail: "myself@example.com",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExpectedError: errMatcher{"could not find environment does-not-exists in overview"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.WriteOverviewCache(ctx, transaction, startingOverview)
				if err != nil {
					return err
				}
				err = dbHandler.UpdateOverviewEnvironmentLock(ctx, transaction, tc.NewEnvironmentLock)
				if err != nil {
					if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
						return fmt.Errorf("mismatch between errors (-want +got):\n%s", diff)
					}
					return nil
				}
				latestOverview, err := dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					return err
				}
				opts := getOverviewIgnoredTypes()
				if diff := cmp.Diff(tc.ExcpectedOverview, latestOverview, opts); diff != "" {
					return fmt.Errorf("mismatch (-want +got):\n%s", diff)
				}
				tc.NewEnvironmentLock.Deleted = true
				err = dbHandler.UpdateOverviewEnvironmentLock(ctx, transaction, tc.NewEnvironmentLock)
				if err != nil {
					return err
				}
				latestOverview, err = dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(startingOverview, latestOverview, opts); diff != "" {
					return fmt.Errorf("mismatch (-want +got):\n%s", diff)
				}
				return nil
			})

			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUpdateOverviewDeployment(t *testing.T) {
	var dev = "dev"
	var upstreamLatest = true
	var version int64 = 12
	startingOverview := makeTestStartingOverview()
	tcs := []struct {
		Name              string
		NewDeployment     Deployment
		ExcpectedOverview *api.GetOverviewResponse
		ExpectedError     error
	}{
		{
			Name: "Update overview",
			NewDeployment: Deployment{
				Env:     "development",
				App:     "test",
				Version: &version,
				Metadata: DeploymentMetadata{
					DeployedByEmail: "testmail2@example.com",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExcpectedOverview: &api.GetOverviewResponse{
				EnvironmentGroups: []*api.EnvironmentGroup{
					{
						EnvironmentGroupName: "dev",
						Environments: []*api.Environment{
							{
								Name: "development",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: &upstreamLatest,
									},
									Argocd:           &api.EnvironmentConfig_ArgoCD{},
									EnvironmentGroup: &dev,
								},
								Applications: map[string]*api.Environment_Application{
									"test": {
										Name:    "test",
										Version: 12,
										DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
											DeployAuthor: "testmail2@example.com",
											DeployTime:   fmt.Sprintf("%d", time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC).Unix()),
										},
										Team: "team-123",
									},
								},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				Applications: map[string]*api.Application{
					"test": {
						Name: "test",
						Releases: []*api.Release{
							{
								Version:        1,
								SourceCommitId: "deadbeef",
								SourceAuthor:   "example <example@example.com>",
								SourceMessage:  "changed something (#678)",
								PrNumber:       "678",
								CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							},
						},
						Team: "team-123",
					},
				},
				GitRevision: "0",
			},
		},
		{
			Name: "env does not exists",
			NewDeployment: Deployment{
				Env:     "does-not-exists",
				App:     "test",
				Version: &version,
				Metadata: DeploymentMetadata{
					DeployedByEmail: "testmail2@example.com",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExpectedError: errMatcher{"could not find environment does-not-exists in overview"},
		},
		{
			Name: "app does not exists",
			NewDeployment: Deployment{
				Env:     "development",
				App:     "does-not-exists",
				Version: &version,
				Metadata: DeploymentMetadata{
					DeployedByEmail: "testmail2@example.com",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExpectedError: errMatcher{"could not find application does-not-exists in environment development in overview"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.WriteOverviewCache(ctx, transaction, startingOverview)
				if err != nil {
					return err
				}
				err = dbHandler.UpdateOverviewDeployment(ctx, transaction, tc.NewDeployment)
				if err != nil {
					if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
						return fmt.Errorf("mismatch between errors (-want +got):\n%s", diff)
					}
					return nil
				}
				latestOverview, err := dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					return err
				}
				opts := getOverviewIgnoredTypes()
				if diff := cmp.Diff(tc.ExcpectedOverview, latestOverview, opts); diff != "" {
					return fmt.Errorf("mismatch (-want +got):\n%s", diff)
				}
				return nil
			})

			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUpdateOverviewApplicationLock(t *testing.T) {
	var dev = "dev"
	var upstreamLatest = true
	startingOverview := makeTestStartingOverview()
	tcs := []struct {
		Name               string
		NewApplicationLock ApplicationLock
		ExcpectedOverview  *api.GetOverviewResponse
		ExpectedError      error
	}{
		{
			Name: "Update overview",
			NewApplicationLock: ApplicationLock{
				Env:        "development",
				App:        "test",
				LockID:     "dev-lock",
				EslVersion: 2,
				Deleted:    false,
				Metadata: LockMetadata{
					Message:        "My lock on dev for my-team",
					CreatedByName:  "myself",
					CreatedByEmail: "myself@example.com",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExcpectedOverview: &api.GetOverviewResponse{
				EnvironmentGroups: []*api.EnvironmentGroup{
					{
						EnvironmentGroupName: "dev",
						Environments: []*api.Environment{
							{
								Name: "development",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: &upstreamLatest,
									},
									Argocd:           &api.EnvironmentConfig_ArgoCD{},
									EnvironmentGroup: &dev,
								},
								Applications: map[string]*api.Environment_Application{
									"test": {
										Name:    "test",
										Version: 1,
										DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
											DeployAuthor: "testmail@example.com",
											DeployTime:   "1",
										},
										Team: "team-123",
										Locks: map[string]*api.Lock{
											"dev-lock": {
												Message:   "My lock on dev for my-team",
												LockId:    "dev-lock",
												CreatedAt: timestamppb.New(time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC)),
												CreatedBy: &api.Actor{
													Name:  "myself",
													Email: "myself@example.com",
												},
											},
										},
									},
								},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				Applications: map[string]*api.Application{
					"test": {
						Name: "test",
						Releases: []*api.Release{
							{
								Version:        1,
								SourceCommitId: "deadbeef",
								SourceAuthor:   "example <example@example.com>",
								SourceMessage:  "changed something (#678)",
								PrNumber:       "678",
								CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							},
						},
						Team: "team-123",
					},
				},
				GitRevision: "0",
			},
		},
		{
			Name: "env does not exists",
			NewApplicationLock: ApplicationLock{
				Env:        "does-not-exists",
				App:        "test",
				LockID:     "dev-lock",
				EslVersion: 2,
				Deleted:    false,
				Metadata: LockMetadata{
					Message:        "My lock on dev for my-team",
					CreatedByName:  "myself",
					CreatedByEmail: "myself@example.com",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExpectedError: errMatcher{"could not find environment does-not-exists in overview"},
		},
		{
			Name: "app does not exists",
			NewApplicationLock: ApplicationLock{
				Env:        "development",
				App:        "does-not-exists",
				LockID:     "dev-lock",
				EslVersion: 2,
				Deleted:    false,
				Metadata: LockMetadata{
					Message:        "My lock on dev for my-team",
					CreatedByName:  "myself",
					CreatedByEmail: "myself@example.com",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExpectedError: errMatcher{"could not find application does-not-exists in environment development in overview"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.WriteOverviewCache(ctx, transaction, startingOverview)
				if err != nil {
					return err
				}
				err = dbHandler.UpdateOverviewApplicationLock(ctx, transaction, tc.NewApplicationLock)
				if err != nil {
					if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
						return fmt.Errorf("mismatch between errors (-want +got):\n%s", diff)
					}
					return nil
				}
				latestOverview, err := dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					return err
				}
				opts := getOverviewIgnoredTypes()
				if diff := cmp.Diff(tc.ExcpectedOverview, latestOverview, opts); diff != "" {
					return fmt.Errorf("mismatch (-want +got):\n%s", diff)
				}
				tc.NewApplicationLock.Deleted = true
				err = dbHandler.UpdateOverviewApplicationLock(ctx, transaction, tc.NewApplicationLock)
				if err != nil {
					return err
				}
				latestOverview, err = dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(startingOverview, latestOverview, opts); diff != "" {
					return fmt.Errorf("mismatch (-want +got):\n%s", diff)
				}
				return nil
			})

			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUpdateOverviewRelease(t *testing.T) {
	var dev = "dev"
	var upstreamLatest = true
	startingOverview := makeTestStartingOverview()
	tcs := []struct {
		Name              string
		NewRelease        DBReleaseWithMetaData
		ExcpectedOverview *api.GetOverviewResponse
		ExpectedError     error
	}{
		{
			Name: "Update overview add release",
			NewRelease: DBReleaseWithMetaData{
				App:           "test",
				ReleaseNumber: 12,
				Metadata: DBReleaseMetaData{
					SourceAuthor:   "testmail@example.com",
					SourceCommitId: "testcommit",
					SourceMessage:  "changed something (#677)",
					DisplayVersion: "12",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExcpectedOverview: &api.GetOverviewResponse{
				EnvironmentGroups: []*api.EnvironmentGroup{
					{
						EnvironmentGroupName: "dev",
						Environments: []*api.Environment{
							{
								Name: "development",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: &upstreamLatest,
									},
									Argocd:           &api.EnvironmentConfig_ArgoCD{},
									EnvironmentGroup: &dev,
								},
								Applications: map[string]*api.Environment_Application{
									"test": {
										Name:    "test",
										Version: 1,
										DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
											DeployAuthor: "testmail@example.com",
											DeployTime:   "1",
										},
										Team: "team-123",
									},
								},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				Applications: map[string]*api.Application{
					"test": {
						Name: "test",
						Releases: []*api.Release{
							{
								Version:        1,
								SourceCommitId: "deadbeef",
								SourceAuthor:   "example <example@example.com>",
								SourceMessage:  "changed something (#678)",
								PrNumber:       "678",
								CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							},
							{
								Version:        12,
								SourceCommitId: "testcommit",
								SourceAuthor:   "testmail@example.com",
								SourceMessage:  "changed something (#678)",
								PrNumber:       "677",
								CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							},
						},
						Team: "team-123",
					},
				},
				GitRevision: "0",
			},
		},
		{
			Name: "Update overview delete release",
			NewRelease: DBReleaseWithMetaData{
				App:           "test",
				ReleaseNumber: 1,
				Metadata: DBReleaseMetaData{
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeef",
					SourceMessage:  "changed something (#678)",
					DisplayVersion: "12",
				},
				Deleted: true,
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExcpectedOverview: &api.GetOverviewResponse{
				EnvironmentGroups: []*api.EnvironmentGroup{
					{
						EnvironmentGroupName: "dev",
						Environments: []*api.Environment{
							{
								Name: "development",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: &upstreamLatest,
									},
									Argocd:           &api.EnvironmentConfig_ArgoCD{},
									EnvironmentGroup: &dev,
								},
								Applications: map[string]*api.Environment_Application{
									"test": {
										Name:    "test",
										Version: 1,
										DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
											DeployAuthor: "testmail@example.com",
											DeployTime:   "1",
										},
										Team: "team-123",
									},
								},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				Applications: map[string]*api.Application{
					"test": {
						Name:     "test",
						Releases: []*api.Release{},
						Team:     "team-123",
					},
				},
				GitRevision: "0",
			},
		},
		{
			Name: "Update overview update release",
			NewRelease: DBReleaseWithMetaData{
				App:           "test",
				ReleaseNumber: 1,
				Metadata: DBReleaseMetaData{
					SourceAuthor:   "changedAuthor",
					SourceCommitId: "changedcommitId",
					SourceMessage:  "changed changed something (#679)",
					DisplayVersion: "12",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExcpectedOverview: &api.GetOverviewResponse{
				EnvironmentGroups: []*api.EnvironmentGroup{
					{
						EnvironmentGroupName: "dev",
						Environments: []*api.Environment{
							{
								Name: "development",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: &upstreamLatest,
									},
									Argocd:           &api.EnvironmentConfig_ArgoCD{},
									EnvironmentGroup: &dev,
								},
								Applications: map[string]*api.Environment_Application{
									"test": {
										Name:    "test",
										Version: 1,
										DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
											DeployAuthor: "testmail@example.com",
											DeployTime:   "1",
										},
										Team: "team-123",
									},
								},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				Applications: map[string]*api.Application{
					"test": {
						Name: "test",
						Releases: []*api.Release{
							{
								Version:        1,
								SourceCommitId: "changedcommitId",
								SourceAuthor:   "changedAuthor",
								SourceMessage:  "changed changed something (#679)",
								PrNumber:       "679",
								CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							},
						},
						Team: "team-123",
					},
				},
				GitRevision: "0",
			},
		},

		{
			Name: "app does not exists",
			NewRelease: DBReleaseWithMetaData{
				App:           "does-not-exists",
				ReleaseNumber: 12,
				Metadata: DBReleaseMetaData{
					SourceAuthor:   "testmail@example.com",
					SourceCommitId: "testcommit",
					SourceMessage:  "changed something (#677)",
					DisplayVersion: "12",
				},
				Created: time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC),
			},
			ExpectedError: errMatcher{"could not find application does-not-exists in overview"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.WriteOverviewCache(ctx, transaction, startingOverview)
				if err != nil {
					return err
				}
				err = dbHandler.UpdateOverviewRelease(ctx, transaction, tc.NewRelease)
				if err != nil {
					if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
						return fmt.Errorf("mismatch between errors (-want +got):\n%s", diff)
					}
					return nil
				}
				latestOverview, err := dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					return err
				}
				opts := getOverviewIgnoredTypes()
				if diff := cmp.Diff(tc.ExcpectedOverview, latestOverview, opts); diff != "" {
					return fmt.Errorf("mismatch (-want +got):\n%s", diff)
				}
				return nil
			})

			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestForceOverviewRecalculation(t *testing.T) {
	tcs := []struct {
		Name string
	}{
		{
			Name: "Check if ForceOverviewRecalculation creates an empty overview",
		},
	}

	for _, tc := range tcs {
		t.Run("ForceOverviewRecalculation", func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.ForceOverviewRecalculation(ctx, transaction)
				if err != nil {
					return err
				}
				latestOverview, err := dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					return err
				}
				if !dbHandler.IsOverviewEmpty(latestOverview) {
					t.Fatalf("%s overview should be empty", tc.Name)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
