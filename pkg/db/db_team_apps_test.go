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
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/freiheit-com/kuberpult/pkg/testutilauth"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

func TestUpsertAppTeam(t *testing.T) {
	tcs := []struct {
		Name     string
		Initial  TeamToAppsMap
		TeamName string
		AppName  types.AppName
		Expected TeamToAppsMap
	}{
		{
			Name:     "add to nil map",
			Initial:  nil,
			TeamName: "team1",
			AppName:  "app1",
			Expected: TeamToAppsMap{"team1": {"app1"}},
		},
		{
			Name:     "app already in the right team is a no-op",
			Initial:  TeamToAppsMap{"team1": {"app1", "app2"}},
			TeamName: "team1",
			AppName:  "app1",
			Expected: TeamToAppsMap{"team1": {"app1", "app2"}},
		},
		{
			Name:     "moving an app keeps the other apps of the old team",
			Initial:  TeamToAppsMap{"team1": {"app1", "app2"}},
			TeamName: "team2",
			AppName:  "app1",
			Expected: TeamToAppsMap{"team1": {"app2"}, "team2": {"app1"}},
		},
		{
			Name:     "emptied team is removed",
			Initial:  TeamToAppsMap{"team1": {"app1"}},
			TeamName: "team2",
			AppName:  "app1",
			Expected: TeamToAppsMap{"team2": {"app1"}},
		},
		{
			Name:     "app listed under several teams ends up in exactly one",
			Initial:  TeamToAppsMap{"team1": {"app1"}, "team2": {"app1"}, "team3": {"app1", "app2"}},
			TeamName: "team4",
			AppName:  "app1",
			Expected: TeamToAppsMap{"team3": {"app2"}, "team4": {"app1"}},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			actual := upsertAppTeam(tc.Initial, tc.TeamName, tc.AppName)
			if diff := cmp.Diff(tc.Expected, actual); diff != "" {
				t.Errorf("app team map mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestRemoveAppFromTeams(t *testing.T) {
	tcs := []struct {
		Name     string
		Initial  TeamToAppsMap
		AppName  types.AppName
		Expected TeamToAppsMap
	}{
		{
			Name:     "removing keeps the other apps of the team",
			Initial:  TeamToAppsMap{"team1": {"app1", "app2"}},
			AppName:  "app1",
			Expected: TeamToAppsMap{"team1": {"app2"}},
		},
		{
			Name:     "emptied team is removed",
			Initial:  TeamToAppsMap{"team1": {"app1"}, "team2": {"app2"}},
			AppName:  "app1",
			Expected: TeamToAppsMap{"team2": {"app2"}},
		},
		{
			Name:     "unknown app is a no-op",
			Initial:  TeamToAppsMap{"team1": {"app1"}},
			AppName:  "app-does-not-exist",
			Expected: TeamToAppsMap{"team1": {"app1"}},
		},
		{
			Name:     "app listed under several teams is removed everywhere",
			Initial:  TeamToAppsMap{"team1": {"app1"}, "team2": {"app1", "app2"}},
			AppName:  "app1",
			Expected: TeamToAppsMap{"team2": {"app2"}},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			actual := removeAppFromTeams(tc.Initial, tc.AppName)
			if diff := cmp.Diff(tc.Expected, actual); diff != "" {
				t.Errorf("app team map mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

// TestAppsTeamsHistoryConcurrentCreatesKeepAllApps writes apps in parallel transactions and
// asserts that none of them is dropped from the apps_teams_history table.
// DBInsertAppsTeamsHistory is a read-modify-write of the whole app->team blob, so without
// serialisation all transactions read the same row and only the one that inserts last
// survives.
// Today this test fails because of that, but note that it relies on the transactions really
// overlapping, which is not guaranteed - see
// TestAppsTeamsHistoryConcurrentTransactionsLoseUpdates for the deterministic version.
func TestAppsTeamsHistoryConcurrentCreatesKeepAllApps(t *testing.T) {
	tcs := []struct {
		Name string
		// must stay below 10, so that the app names sort the same way in go and in postgres:
		NumApps int
	}{
		{
			Name:    "apps created in parallel transactions must all be kept",
			NumApps: 8,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutilauth.MakeTestContext()
			dbHandler := setupDB(t)

			// start releases all goroutines at once, so that their transactions overlap:
			var start sync.WaitGroup
			start.Add(1)
			var done sync.WaitGroup
			done.Add(tc.NumApps)
			errs := make(chan error, tc.NumApps)

			for i := range tc.NumApps {
				go func() {
					defer done.Done()
					appName := types.AppName(fmt.Sprintf("app-%d", i))
					teamName := fmt.Sprintf("team-%d", i)
					transaction, err := dbHandler.BeginTransaction(ctx, false)
					if err != nil {
						errs <- fmt.Errorf("error while beginning transaction for app %s: %w", appName, err)
						return
					}
					defer func() { _ = transaction.Rollback() }()

					// the barrier must be here, and not after the write: a lock-based fix would
					// deadlock if one goroutine held the lock while waiting for the others.
					start.Wait()

					err = dbHandler.DBInsertOrUpdateApplication(ctx, transaction, appName, AppStateChangeCreate, DBAppMetaData{
						Team: teamName,
					}, "")
					if err != nil {
						errs <- fmt.Errorf("error while writing app %s: %w", appName, err)
						return
					}
					if err := transaction.Commit(); err != nil {
						errs <- fmt.Errorf("error while committing transaction for app %s: %w", appName, err)
					}
				}()
			}
			start.Done()
			done.Wait()
			close(errs)
			for err := range errs {
				t.Fatalf("error in parallel transaction: %v", err)
			}

			var expectedApps []types.AppName
			expectedMap := TeamToAppsMap{}
			for i := range tc.NumApps {
				appName := types.AppName(fmt.Sprintf("app-%d", i))
				expectedApps = append(expectedApps, appName)
				expectedMap[fmt.Sprintf("team-%d", i)] = []types.AppName{appName}
			}

			err := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				actualApps, err := dbHandler.DBSelectAllApplications(ctx, transaction)
				if err != nil {
					return fmt.Errorf("error while selecting all applications: %w", err)
				}
				if diff := cmp.Diff(expectedApps, actualApps); diff != "" {
					t.Errorf("applications mismatch (-want, +got):\n%s", diff)
				}

				_, teamAppMap, err := dbHandler.DBSelectLatestAppsTeamsHistory(ctx, transaction)
				if err != nil {
					return fmt.Errorf("error while selecting apps teams history: %w", err)
				}
				if diff := cmp.Diff(expectedMap, teamAppMap); diff != "" {
					t.Errorf("apps teams history mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error: %v", err)
			}
		})
	}
}
