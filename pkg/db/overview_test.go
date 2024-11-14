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
						Priority: api.Priority_YOLO,
					},
				},
				Priority: api.Priority_YOLO,
			},
		},
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
									Priority: api.Priority_YOLO,
								},
							},
							Priority: api.Priority_YOLO,
						},
					},
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
									Priority: api.Priority_YOLO,
								},
							},
							Priority: api.Priority_YOLO,
						},
					},
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
								AppLocks: map[string]*api.Locks{
									"test": {
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
			ExpectedError: errMatcher{"could not find application 'does-not-exists' in apps table: got no result"},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBInsertApplication(ctx, transaction, "test", AppStateChangeCreate, DBAppMetaData{})
				if err != nil {
					return err
				}
				err = dbHandler.WriteOverviewCache(ctx, transaction, startingOverview)
				if err != nil {
					return err
				}
				err = dbHandler.UpdateOverviewApplicationLock(ctx, transaction, tc.NewApplicationLock, tc.NewApplicationLock.Created)
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
				err = dbHandler.UpdateOverviewApplicationLock(ctx, transaction, tc.NewApplicationLock, tc.NewApplicationLock.Created)
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
