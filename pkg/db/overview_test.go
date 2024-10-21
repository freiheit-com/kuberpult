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
		api.Warning{},
		api.Warning_UnusualDeploymentOrder{},
		api.UnusualDeploymentOrder{},
		api.UpstreamNotDeployed{},
		api.Warning_UpstreamNotDeployed{},
		api.Release{},
		timestamppb.Timestamp{},
		api.EnvironmentConfig{},
		api.EnvironmentConfig_Upstream{},
		api.EnvironmentConfig_ArgoCD{},
		api.Lock{},
		api.Actor{},
		api.OverviewApplication{},
		api.Deployment{},
		api.Locks{})
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
						//Applications: map[string]*api.Environment_Application{
						//	"test": {
						//		Name:    "test",
						//		Version: 1,
						//		DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
						//			DeployAuthor: "testmail@example.com",
						//			DeployTime:   "1",
						//		},
						//		Team: "team-123",
						//	},
						//},
						Priority: api.Priority_YOLO,
					},
				},
				Priority: api.Priority_YOLO,
			},
		},
		//Applications: map[string]*api.Application{
		//	"test": {
		//		Name: "test",
		//		Releases: []*api.Release{
		//			{
		//				Version:        1,
		//				SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		//				SourceAuthor:   "example <example@example.com>",
		//				SourceMessage:  "changed something (#678)",
		//				PrNumber:       "678",
		//				CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
		//			},
		//		},
		//		Team: "team-123",
		//		Warnings: []*api.Warning{
		//			{
		//				WarningType: &api.Warning_UnusualDeploymentOrder{
		//					UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
		//						UpstreamEnvironment: "staging",
		//						ThisVersion:         12,
		//						ThisEnvironment:     "development",
		//					},
		//				},
		//			},
		//		},
		//	},
		//},
		LightweightApps: []*api.OverviewApplication{
			{
				Name: "test",
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
								TeamLocks: map[string]*api.Locks{
									"team-123": {
										Locks: []*api.Lock{
											{
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
				//Applications: map[string]*api.Application{
				//	"test": {
				//		Name: "test",
				//		Releases: []*api.Release{
				//			{
				//				Version:        1,
				//				SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				//				SourceAuthor:   "example <example@example.com>",
				//				SourceMessage:  "changed something (#678)",
				//				PrNumber:       "678",
				//				CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
				//			},
				//		},
				//		Team: "team-123",
				//		Warnings: []*api.Warning{
				//			{
				//				WarningType: &api.Warning_UnusualDeploymentOrder{
				//					UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
				//						UpstreamEnvironment: "staging",
				//						ThisVersion:         12,
				//						ThisEnvironment:     "development",
				//					},
				//				},
				//			},
				//		},
				//	},
				//},
				LightweightApps: []*api.OverviewApplication{
					{
						Name: "test",
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
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.WriteOverviewCache(ctx, transaction, startingOverview)
				if err != nil {
					return err
				}
				opts := getOverviewIgnoredTypes()
				latestOverview, err := dbHandler.ReadLatestOverviewCache(ctx, transaction) //sanity check
				if err != nil {
					return err
				}
				if diff := cmp.Diff(startingOverview, latestOverview, opts); diff != "" {
					return fmt.Errorf("starting overviews mismatch (-want +got):\n%s", diff)
				}
				err = dbHandler.UpdateOverviewTeamLock(ctx, transaction, tc.NewTeamLock)
				if err != nil {
					if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
						return fmt.Errorf("mismatch between errors (-want +got):\n%s", diff)
					}
					return nil
				}
				latestOverview, err = dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.ExcpectedOverview, latestOverview, opts); diff != "" {
					return fmt.Errorf("expected overview and last overview mismatch (-want +got):\n%s", diff)
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
					return fmt.Errorf("starting overview and last overview mismatch (-want +got):\n%s", diff)
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
								//Applications: map[string]*api.Environment_Application{
								//	"test": {
								//		Name:    "test",
								//		Version: 1,
								//		DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
								//			DeployAuthor: "testmail@example.com",
								//			DeployTime:   "1",
								//		},
								//		Team: "team-123",
								//	},
								//},
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
				//Applications: map[string]*api.Application{
				//	"test": {
				//		Name: "test",
				//		Releases: []*api.Release{
				//			{
				//				Version:        1,
				//				SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				//				SourceAuthor:   "example <example@example.com>",
				//				SourceMessage:  "changed something (#678)",
				//				PrNumber:       "678",
				//				CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
				//			},
				//		},
				//		Team: "team-123",
				//		Warnings: []*api.Warning{
				//			{
				//				WarningType: &api.Warning_UnusualDeploymentOrder{
				//					UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
				//						UpstreamEnvironment: "staging",
				//						ThisVersion:         12,
				//						ThisEnvironment:     "development",
				//					},
				//				},
				//			},
				//		},
				//	},
				//},
				LightweightApps: []*api.OverviewApplication{
					{
						Name: "test",
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
		tc := tc
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
								//Applications: map[string]*api.Environment_Application{
								//	"test": {
								//		Name:    "test",
								//		Version: 12,
								//		DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
								//			DeployAuthor: "testmail2@example.com",
								//			DeployTime:   fmt.Sprintf("%d", time.Date(2024, time.July, 12, 15, 30, 0, 0, time.UTC).Unix()),
								//		},
								//		Team: "team-123",
								//	},
								//},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				//Applications: map[string]*api.Application{
				//	"test": {
				//		Name: "test",
				//		Releases: []*api.Release{
				//			{
				//				Version:        1,
				//				SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				//				SourceAuthor:   "example <example@example.com>",
				//				SourceMessage:  "changed something (#678)",
				//				PrNumber:       "678",
				//				CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
				//			},
				//		},
				//		Team: "team-123",
				//	},
				//},
				LightweightApps: []*api.OverviewApplication{
					{
						Name: "test",
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
			ExpectedError: errMatcher{"could not find application 'does-not-exists' in apps table: got no result"},
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
				err = dbHandler.UpdateOverviewDeployment(ctx, transaction, tc.NewDeployment, tc.NewDeployment.Created)
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
								//Applications: map[string]*api.Environment_Application{
								//	"test": {
								//		Name:    "test",
								//		Version: 1,
								//		DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
								//			DeployAuthor: "testmail@example.com",
								//			DeployTime:   "1",
								//		},
								//		Team: "team-123",
								//	},
								//},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				//Applications: map[string]*api.Application{
				//	"test": {
				//		Name: "test",
				//		Releases: []*api.Release{
				//			{
				//				Version:        1,
				//				SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				//				SourceAuthor:   "example <example@example.com>",
				//				SourceMessage:  "changed something (#678)",
				//				PrNumber:       "678",
				//				CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
				//			},
				//			{
				//				Version:        12,
				//				SourceCommitId: "testcommit",
				//				SourceAuthor:   "testmail@example.com",
				//				SourceMessage:  "changed something (#678)",
				//				PrNumber:       "677",
				//				CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
				//			},
				//		},
				//		Team: "team-123",
				//	},
				//},
				LightweightApps: []*api.OverviewApplication{
					{
						Name: "test",
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
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
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
								//Applications: map[string]*api.Environment_Application{
								//	"test": {
								//		Name:    "test",
								//		Version: 1,
								//		DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
								//			DeployAuthor: "testmail@example.com",
								//			DeployTime:   "1",
								//		},
								//		Team: "team-123",
								//	},
								//},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				//Applications: map[string]*api.Application{
				//	"test": {
				//		Name:     "test",
				//		Releases: []*api.Release{},
				//		Team:     "team-123",
				//	},
				//},
				LightweightApps: []*api.OverviewApplication{
					{
						Name: "test",
						Team: "team-123",
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
								//Applications: map[string]*api.Environment_Application{
								//	"test": {
								//		Name:    "test",
								//		Version: 1,
								//		DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
								//			DeployAuthor: "testmail@example.com",
								//			DeployTime:   "1",
								//		},
								//		Team: "team-123",
								//	},
								//},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				//Applications: map[string]*api.Application{
				//	"test": {
				//		Name: "test",
				//		Releases: []*api.Release{
				//			{
				//				Version:        1,
				//				SourceCommitId: "changedcommitId",
				//				SourceAuthor:   "changedAuthor",
				//				SourceMessage:  "changed changed something (#679)",
				//				PrNumber:       "679",
				//				CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
				//			},
				//		},
				//		Team: "team-123",
				//	},
				//},
				LightweightApps: []*api.OverviewApplication{
					{
						Name: "test",
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
		{
			Name: "Update overview with prepublish release",
			NewRelease: DBReleaseWithMetaData{
				App:           "test",
				ReleaseNumber: 12,
				Metadata: DBReleaseMetaData{
					SourceAuthor:   "testmail@example.com",
					SourceCommitId: "testcommit",
					SourceMessage:  "changed something (#677)",
					DisplayVersion: "12",
					IsPrepublish:   true,
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
								//Applications: map[string]*api.Environment_Application{
								//	"test": {
								//		Name:    "test",
								//		Version: 1,
								//		DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
								//			DeployAuthor: "testmail@example.com",
								//			DeployTime:   "1",
								//		},
								//		Team: "team-123",
								//	},
								//},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				//Applications: map[string]*api.Application{
				//	"test": {
				//		Name: "test",
				//		Warnings: []*api.Warning{
				//			{
				//				WarningType: &api.Warning_UnusualDeploymentOrder{
				//					UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
				//						UpstreamEnvironment: "staging",
				//						ThisVersion:         12,
				//						ThisEnvironment:     "development",
				//					},
				//				},
				//			},
				//		},
				//		Releases: []*api.Release{
				//			{
				//				Version:        1,
				//				SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				//				SourceAuthor:   "example <example@example.com>",
				//				SourceMessage:  "changed something (#678)",
				//				PrNumber:       "678",
				//				CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
				//			},
				//			{
				//				Version:        12,
				//				SourceCommitId: "testcommit",
				//				SourceAuthor:   "testmail@example.com",
				//				SourceMessage:  "changed something (#677)",
				//				DisplayVersion: "12",
				//				PrNumber:       "677",
				//				IsPrepublish:   true,
				//				CreatedAt:      &timestamppb.Timestamp{Seconds: 1720798200, Nanos: 0},
				//			},
				//		},
				//		Team: "team-123",
				//	},
				//},
				LightweightApps: []*api.OverviewApplication{
					{
						Name: "test",
						Team: "team-123",
					},
				},
				GitRevision: "0",
			},
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

//func TestCalculateWarnings(t *testing.T) {
//	var dev = "dev"
//	tcs := []struct {
//		Name             string
//		AppName          string
//		Groups           []*api.EnvironmentGroup
//		ExpectedWarnings []*api.Warning
//	}{
//		{
//			Name:    "no envs - no warning",
//			AppName: "foo",
//			Groups: []*api.EnvironmentGroup{
//				makeEnvGroup(dev, []*api.Environment{
//					makeEnv("dev-de", dev, makeUpstreamLatest(), nil),
//				})},
//			ExpectedWarnings: []*api.Warning{},
//		},
//		{
//			Name:    "app deployed in higher version on upstream should warn",
//			AppName: "foo",
//			Groups: []*api.EnvironmentGroup{
//				makeEnvGroup(dev, []*api.Environment{
//					makeEnv("prod", dev, makeUpstreamEnv("dev"),
//						makeApps(makeApp("foo", 2))),
//				}),
//				makeEnvGroup(dev, []*api.Environment{
//					makeEnv("dev", dev, makeUpstreamLatest(),
//						makeApps(makeApp("foo", 1))),
//				}),
//			},
//			ExpectedWarnings: []*api.Warning{
//				{
//					WarningType: &api.Warning_UnusualDeploymentOrder{
//						UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
//							UpstreamVersion:     1,
//							UpstreamEnvironment: "dev",
//							ThisVersion:         2,
//							ThisEnvironment:     "prod",
//						},
//					},
//				},
//			},
//		},
//		{
//			Name:    "app deployed in same version on upstream should not warn",
//			AppName: "foo",
//			Groups: []*api.EnvironmentGroup{
//				makeEnvGroup(dev, []*api.Environment{
//					makeEnv("prod", dev, makeUpstreamEnv("dev"),
//						makeApps(makeApp("foo", 2))),
//				}),
//				makeEnvGroup(dev, []*api.Environment{
//					makeEnv("dev", dev, makeUpstreamLatest(),
//						makeApps(makeApp("foo", 2))),
//				}),
//			},
//			ExpectedWarnings: []*api.Warning{},
//		},
//		{
//			Name:    "app deployed in no version on upstream should warn",
//			AppName: "foo",
//			Groups: []*api.EnvironmentGroup{
//				makeEnvGroup(dev, []*api.Environment{
//					makeEnv("prod", dev, makeUpstreamEnv("dev"),
//						makeApps(makeApp("foo", 1))),
//				}),
//				makeEnvGroup(dev, []*api.Environment{
//					makeEnv("dev", dev, makeUpstreamLatest(),
//						makeApps()),
//				}),
//			},
//			ExpectedWarnings: []*api.Warning{
//				{
//					WarningType: &api.Warning_UpstreamNotDeployed{
//						UpstreamNotDeployed: &api.UpstreamNotDeployed{
//							UpstreamEnvironment: "dev",
//							ThisVersion:         1,
//							ThisEnvironment:     "prod",
//						},
//					},
//				},
//			},
//		},
//	}
//	for _, tc := range tcs {
//		tc := tc
//		t.Run(tc.Name, func(t *testing.T) {
//			actualWarnings := CalculateWarnings(testutil.MakeTestContext(), tc.AppName, tc.Groups)
//			if len(actualWarnings) != len(tc.ExpectedWarnings) {
//				t.Errorf("Different number of warnings. got: %s\nwant: %s", actualWarnings, tc.ExpectedWarnings)
//			}
//			for i := 0; i < len(actualWarnings); i++ {
//				actualWarning := actualWarnings[i]
//				expectedWarning := tc.ExpectedWarnings[i]
//				if diff := cmp.Diff(actualWarning.String(), expectedWarning.String()); diff != "" {
//					t.Errorf("Different warning at index [%d]:\ngot:  %s\nwant: %s", i, actualWarning, expectedWarning)
//				}
//			}
//		})
//	}
//}
//
//func groupFromEnvs(environments []*api.Environment) []*api.EnvironmentGroup {
//	return []*api.EnvironmentGroup{
//		{
//			EnvironmentGroupName: "group1",
//			Environments:         environments,
//		},
//	}
//}

func TestDBDeleteOldOverview(t *testing.T) {
	upstreamLatest := true
	dev := "dev"
	var tcs = []struct {
		Name                               string
		inputOverviews                     []*api.GetOverviewResponse
		timeThresholdDiff                  time.Duration
		numberOfOverviewsToKeep            uint64
		expectedNumberOfRemainingOverviews uint64
	}{
		{
			Name: "4 overviews, should keep two",
			inputOverviews: []*api.GetOverviewResponse{
				&api.GetOverviewResponse{},
				&api.GetOverviewResponse{},
				&api.GetOverviewResponse{},
				&api.GetOverviewResponse{
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
									//Applications: map[string]*api.Environment_Application{
									//	"test": {
									//		Name:    "test",
									//		Version: 1,
									//		DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
									//			DeployAuthor: "testmail@example.com",
									//			DeployTime:   "1",
									//		},
									//		Team: "team-123",
									//	},
									//},
									Priority: api.Priority_YOLO,
								},
							},
							Priority: api.Priority_YOLO,
						},
					},
					//Applications: map[string]*api.Application{
					//	"test": {
					//		Name: "test",
					//		Releases: []*api.Release{
					//			{
					//				Version:        1,
					//				SourceCommitId: "changedcommitId",
					//				SourceAuthor:   "changedAuthor",
					//				SourceMessage:  "changed changed something (#679)",
					//				PrNumber:       "679",
					//				CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
					//			},
					//		},
					//		Team: "team-123",
					//	},
					//},
					LightweightApps: []*api.OverviewApplication{
						{
							Name: "test",
							Team: "team-123",
						},
					},
					GitRevision: "0",
				},
			},
			timeThresholdDiff:                  150 * time.Second,
			numberOfOverviewsToKeep:            2,
			expectedNumberOfRemainingOverviews: 2,
		},
		{
			Name: "4 overviews, early time threshhold, all should remain",
			inputOverviews: []*api.GetOverviewResponse{
				&api.GetOverviewResponse{},
				&api.GetOverviewResponse{},
				&api.GetOverviewResponse{},
				&api.GetOverviewResponse{
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
									//Applications: map[string]*api.Environment_Application{
									//	"test": {
									//		Name:    "test",
									//		Version: 1,
									//		DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
									//			DeployAuthor: "testmail@example.com",
									//			DeployTime:   "1",
									//		},
									//		Team: "team-123",
									//	},
									//},
									Priority: api.Priority_YOLO,
								},
							},
							Priority: api.Priority_YOLO,
						},
					},
					//Applications: map[string]*api.Application{
					//	"test": {
					//		Name: "test",
					//		Releases: []*api.Release{
					//			{
					//				Version:        1,
					//				SourceCommitId: "changedcommitId",
					//				SourceAuthor:   "changedAuthor",
					//				SourceMessage:  "changed changed something (#679)",
					//				PrNumber:       "679",
					//				CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
					//			},
					//		},
					//		Team: "team-123",
					//	},
					//},
					LightweightApps: []*api.OverviewApplication{
						{
							Name: "test",
							Team: "team-123",
						},
					},
					GitRevision: "0",
				},
			},
			timeThresholdDiff:                  -300 * time.Second,
			numberOfOverviewsToKeep:            0,
			expectedNumberOfRemainingOverviews: 4,
		},
		{
			Name: "4 overviews, late time threshold, zero to remain",
			inputOverviews: []*api.GetOverviewResponse{
				&api.GetOverviewResponse{},
				&api.GetOverviewResponse{},
				&api.GetOverviewResponse{},
				&api.GetOverviewResponse{},
			},
			timeThresholdDiff:                  300 * time.Second,
			numberOfOverviewsToKeep:            0,
			expectedNumberOfRemainingOverviews: 0,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, overview := range tc.inputOverviews {
					err := dbHandler.WriteOverviewCache(ctx, transaction, overview)
					if err != nil {
						return err
					}
				}
				err := dbHandler.DBDeleteOldOverviews(ctx, transaction, tc.numberOfOverviewsToKeep, time.Now().Add(tc.timeThresholdDiff))
				if err != nil {
					return err
				}
				remainingOverviewsCount, err := calculateNumberOfOverviews(dbHandler, ctx, transaction)
				if err != nil {
					return err
				}
				if remainingOverviewsCount != tc.expectedNumberOfRemainingOverviews {
					return fmt.Errorf("Expected number of remaining overviews: %d, got: %d", tc.expectedNumberOfRemainingOverviews, remainingOverviewsCount)
				}
				if tc.expectedNumberOfRemainingOverviews > 0 {
					latestOverview, err := dbHandler.ReadLatestOverviewCache(ctx, transaction)
					if err != nil {
						return err
					}
					opts := getOverviewIgnoredTypes()
					if diff := cmp.Diff(tc.inputOverviews[len(tc.inputOverviews)-1], latestOverview, opts); diff != "" {
						return fmt.Errorf("mismatch latest overview (-want +got):\n%s", diff)
					}
				}
				return nil
			})

			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func calculateNumberOfOverviews(h *DBHandler, ctx context.Context, tx *sql.Tx) (uint64, error) {

	selectQuery := h.AdaptQuery(`SELECT COUNT(*) FROM overview_cache`)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	var result int64
	if err != nil {
		return 0, fmt.Errorf("error calculating number of overviews: %w", err)
	}
	if rows.Next() {
		err := rows.Scan(&result)
		if err != nil {
			return 0, fmt.Errorf("Error scanning overview_cache ,Error: %w\n", err)
		}
	} else {
		result = 0
	}
	return uint64(result), nil
}
