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
	"slices"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/testutilauth"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

// releaseStep is one history-producing action, applied in its own transaction
// so it gets a distinct `created`. Delete=true soft-deletes the release
// (produces a releases_history row with deleted=true) instead of creating it.
type releaseStep struct {
	Release DBReleaseWithMetaData
	Delete  bool
}

// applyReleaseSteps replays each step in its own transaction and returns the
// transaction timestamp captured immediately after each step, in the same
// order - so step i's result is timestamps[i].
func applyReleaseSteps(t *testing.T, ctx context.Context, dbHandler *DBHandler, steps []releaseStep) []time.Time {
	t.Helper()
	timestamps := make([]time.Time, 0, len(steps))
	for _, step := range steps {
		ts, err := WithTransactionT(dbHandler, ctx, 1, false, func(ctx context.Context, transaction *sql.Tx) (*time.Time, error) {
			var err error
			if step.Delete {
				err = dbHandler.DBDeleteFromReleases(ctx, transaction, step.Release.App, step.Release.ReleaseNumbers)
			} else {
				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, step.Release)
			}
			if err != nil {
				return nil, err
			}
			return dbHandler.DBReadTransactionTimestamp(ctx, transaction)
		})
		if err != nil {
			t.Fatalf("error applying release step for %v (delete=%v): %v", step.Release.ReleaseNumbers, step.Delete, err)
		}
		timestamps = append(timestamps, *ts)
	}
	return timestamps
}

// applyReleaseStepsInOneTransaction runs every step in a single transaction, so all
// resulting releases_history rows share the same `created` and can only be
// disambiguated by the `version` sequence column.
func applyReleaseStepsInOneTransaction(t *testing.T, ctx context.Context, dbHandler *DBHandler, steps []releaseStep) time.Time {
	t.Helper()
	ts, err := WithTransactionT(dbHandler, ctx, 1, false, func(ctx context.Context, transaction *sql.Tx) (*time.Time, error) {
		for _, step := range steps {
			var err error
			if step.Delete {
				err = dbHandler.DBDeleteFromReleases(ctx, transaction, step.Release.App, step.Release.ReleaseNumbers)
			} else {
				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, step.Release)
			}
			if err != nil {
				return nil, err
			}
		}
		return dbHandler.DBReadTransactionTimestamp(ctx, transaction)
	})
	if err != nil {
		t.Fatalf("error applying release steps in one transaction: %v", err)
	}
	return *ts
}

const (
	appFoo = types.AppName("foo")
	appPow = types.AppName("pow")
	appBar = types.AppName("bar")
)

func fooRevisionRelease(version uint64, revision uint64, envs map[types.EnvName]string) DBReleaseWithMetaData {
	return DBReleaseWithMetaData{
		ReleaseNumbers: types.MakeReleaseNumbers(version, revision),
		App:            appFoo,
		Manifests:      DBReleaseManifests{Manifests: envs},
	}
}

func TestDBSelectAppsWithReleaseAtTimestamp(t *testing.T) {
	const dev = types.EnvName("dev")
	const stg = types.EnvName("staging")

	// Note: DBReleaseWithMetaData.Environments is ignored on write -
	// upsertReleaseRow/insertReleaseHistoryRow overwrite it with the sorted
	// keys of Manifests.Manifests. The query filters on `environments @> env`,
	// so an env only matches if it is a key in the manifests map here.
	fooReleaseDevStg := func(version uint64, revision uint64) DBReleaseWithMetaData {
		return fooRevisionRelease(version, revision, map[types.EnvName]string{dev: "manifest1", stg: "manifest2"})
	}
	stgOnlyRelease := func(version uint64, revision uint64) DBReleaseWithMetaData {
		return fooRevisionRelease(version, revision, map[types.EnvName]string{stg: "manifest2"})
	}

	tcs := []struct {
		Name              string
		Steps             []releaseStep
		InputDeployedApps []DeployedApp
		QueryAfterStep    int
		ExpectedApps      []types.AppName
	}{
		{
			Name: "no deployed apps yields an empty result",
			Steps: []releaseStep{
				{Release: fooReleaseDevStg(1, 0)},
			},
			InputDeployedApps: nil,
			QueryAfterStep:    0,
			ExpectedApps:      nil,
		},
		{
			Name: "smallest case with 1 result row",
			Steps: []releaseStep{
				{Release: fooReleaseDevStg(1, 0)},
			},
			InputDeployedApps: []DeployedApp{
				{
					AppName:        appFoo,
					ReleaseVersion: types.Ptr(types.ReleaseVersion(1)),
					Revision:       0,
				},
			},
			QueryAfterStep: 0,
			ExpectedApps: []types.AppName{
				appFoo,
			},
		},
		{
			Name: "release still present as of the timestamp right after creation",
			Steps: []releaseStep{
				{Release: fooReleaseDevStg(1, 0)},
				{Release: fooReleaseDevStg(1, 0), Delete: true},
			},
			InputDeployedApps: []DeployedApp{
				{
					AppName:        appFoo,
					ReleaseVersion: types.Ptr(types.ReleaseVersion(1)),
					Revision:       0,
				},
			},
			QueryAfterStep: 0,
			ExpectedApps: []types.AppName{
				appFoo,
			},
		},
		{
			Name: "release excluded as of the timestamp after it was deleted",
			Steps: []releaseStep{
				{Release: fooReleaseDevStg(1, 0)},
				{Release: fooReleaseDevStg(1, 0), Delete: true},
			},
			InputDeployedApps: []DeployedApp{
				{
					AppName:        appFoo,
					ReleaseVersion: types.Ptr(types.ReleaseVersion(1)),
					Revision:       0,
				},
			},
			QueryAfterStep: 1,
			ExpectedApps:   nil,
		},
		{
			Name: "release deployed to a different env is excluded",
			Steps: []releaseStep{
				{Release: stgOnlyRelease(1, 0)}, // manifests: {stg: "manifest2"} only, no dev key
			},
			InputDeployedApps: []DeployedApp{
				{AppName: appFoo, ReleaseVersion: types.Ptr(types.ReleaseVersion(1)), Revision: 0},
			},
			QueryAfterStep: 0,
			ExpectedApps:   nil, // queried env is dev, release only has stg
		},
		{
			Name: "multiple deployed apps: only the matching one is returned",
			Steps: []releaseStep{
				{Release: fooReleaseDevStg(1, 0)}, // appPow gets no history row at all
			},
			InputDeployedApps: []DeployedApp{
				{AppName: appFoo, ReleaseVersion: types.Ptr(types.ReleaseVersion(1)), Revision: 0}, // matches
				{AppName: appPow, ReleaseVersion: types.Ptr(types.ReleaseVersion(1)), Revision: 0}, // no history row exists
			},
			QueryAfterStep: 0,
			ExpectedApps:   []types.AppName{appFoo},
		},
		{
			Name: "revision is part of the match: wrong revision's env doesn't leak through",
			Steps: []releaseStep{
				{Release: fooRevisionRelease(1, 0, map[types.EnvName]string{dev: "manifest1"})}, // revision 0 -> dev
				{Release: fooRevisionRelease(1, 1, map[types.EnvName]string{stg: "manifest2"})}, // revision 1 -> stg only
			},
			InputDeployedApps: []DeployedApp{
				{AppName: appFoo, ReleaseVersion: types.Ptr(types.ReleaseVersion(1)), Revision: 1}, // asking for revision 1
			},
			QueryAfterStep: 1,
			ExpectedApps:   nil, // revision 1 has no `dev` manifest — must not match on revision 0's dev instead
		},
		{
			Name: "nil ReleaseVersion never matches and the app is therefor dropped",
			Steps: []releaseStep{
				{Release: fooReleaseDevStg(1, 0)},
			},
			InputDeployedApps: []DeployedApp{
				{AppName: appFoo, ReleaseVersion: nil, Revision: 0},
			},
			QueryAfterStep: 0,
			ExpectedApps:   nil,
		},
		{
			Name: "two deployed apps that both match are both returned in order",
			Steps: []releaseStep{
				{Release: fooReleaseDevStg(1, 0)},
				{Release: DBReleaseWithMetaData{
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
					App:            appPow,
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{dev: "manifest1", stg: "manifest2"}},
				}},
			},
			InputDeployedApps: []DeployedApp{
				{AppName: appFoo, ReleaseVersion: types.Ptr(types.ReleaseVersion(1)), Revision: 0},
				{AppName: appPow, ReleaseVersion: types.Ptr(types.ReleaseVersion(1)), Revision: 0},
			},
			QueryAfterStep: 1,
			ExpectedApps:   []types.AppName{appFoo, appPow},
		},
		{
			Name: "releaseversion filters correctly even when a different, newer version exists",
			Steps: []releaseStep{
				{Release: fooReleaseDevStg(1, 0)}, // requested version, has dev
				{Release: stgOnlyRelease(2, 0)},   // newer write, different releaseversion, stg only
			},
			InputDeployedApps: []DeployedApp{
				{AppName: appFoo, ReleaseVersion: types.Ptr(types.ReleaseVersion(1)), Revision: 0},
			},
			QueryAfterStep: 1,
			ExpectedApps:   []types.AppName{appFoo}, // must match version 1's dev env, not version 2's stg-only
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutilauth.MakeTestContext()
			dbHandler := setupDB(t)

			timestamps := applyReleaseSteps(t, ctx, dbHandler, tc.Steps)
			ts := timestamps[tc.QueryAfterStep]

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				actualApps, err := dbHandler.DBSelectAppsWithReleaseAtTimestamp(ctx, transaction, tc.InputDeployedApps, dev, ts)
				if err != nil {
					t.Fatalf("error selecting apps with release at timestamp: %v", err)
				}
				if diff := testutil.CmpDiff(tc.ExpectedApps, actualApps); diff != "" {
					t.Fatalf("apps mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestDBSelectAppsWithReleaseAtTimestampVersionTieBreak(t *testing.T) {
	const dev = types.EnvName("dev")
	const stg = types.EnvName("staging")

	// All steps within a case share one transaction (identical `created`), so the
	// result depends entirely on the `version` sequence tie-break correctly
	// reflecting write order.
	tcs := []struct {
		Name         string
		Steps        []releaseStep
		ExpectedApps []types.AppName
	}{
		{
			Name: "create then delete in the same transaction: delete wins via version tie-break",
			Steps: []releaseStep{
				{Release: fooRevisionRelease(1, 0, map[types.EnvName]string{dev: "manifest1"})},
				{Release: fooRevisionRelease(1, 0, map[types.EnvName]string{dev: "manifest1"}), Delete: true},
			},
			ExpectedApps: nil,
		},
		{
			Name: "two updates in the same transaction: latest env re-adds dev via version tie-break",
			Steps: []releaseStep{
				{Release: fooRevisionRelease(1, 0, map[types.EnvName]string{stg: "manifest2"})},
				{Release: fooRevisionRelease(1, 0, map[types.EnvName]string{dev: "manifest1", stg: "manifest2"})},
			},
			ExpectedApps: []types.AppName{appFoo},
		},
		{
			Name: "two updates in the same transaction: latest env drops dev via version tie-break",
			Steps: []releaseStep{
				{Release: fooRevisionRelease(1, 0, map[types.EnvName]string{dev: "manifest1"})},
				{Release: fooRevisionRelease(1, 0, map[types.EnvName]string{stg: "manifest2"})},
			},
			ExpectedApps: nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutilauth.MakeTestContext()
			dbHandler := setupDB(t)

			ts := applyReleaseStepsInOneTransaction(t, ctx, dbHandler, tc.Steps)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				actualApps, err := dbHandler.DBSelectAppsWithReleaseAtTimestamp(ctx, transaction,
					[]DeployedApp{{AppName: appFoo, ReleaseVersion: types.Ptr(types.ReleaseVersion(1)), Revision: 0}}, dev, ts)
				if err != nil {
					t.Fatalf("error selecting apps with release at timestamp: %v", err)
				}
				if diff := testutil.CmpDiff(tc.ExpectedApps, actualApps); diff != "" {
					t.Fatalf("apps mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestDeployedAppsToArrays(t *testing.T) {
	tcs := []struct {
		Name                   string
		InputDeployedApps      []DeployedApp
		ExpectedAppNames       []types.AppName
		ExpectedReleaseVersion []*types.ReleaseVersion
		ExpectedRevisions      []types.Revision
	}{
		{
			Name:                   "empty input",
			InputDeployedApps:      []DeployedApp{},
			ExpectedAppNames:       []types.AppName{},
			ExpectedReleaseVersion: []*types.ReleaseVersion{},
			ExpectedRevisions:      []types.Revision{},
		},
		{
			Name: "reverse-sorted input is sorted by app name, arrays stay aligned",
			InputDeployedApps: []DeployedApp{
				{AppName: appPow, ReleaseVersion: types.Ptr(types.ReleaseVersion(3)), Revision: 30},
				{AppName: appFoo, ReleaseVersion: types.Ptr(types.ReleaseVersion(2)), Revision: 20},
				{AppName: appBar, ReleaseVersion: types.Ptr(types.ReleaseVersion(1)), Revision: 10},
			},
			ExpectedAppNames:       []types.AppName{appBar, appFoo, appPow},
			ExpectedReleaseVersion: []*types.ReleaseVersion{types.Ptr(types.ReleaseVersion(1)), types.Ptr(types.ReleaseVersion(2)), types.Ptr(types.ReleaseVersion(3))},
			ExpectedRevisions:      []types.Revision{10, 20, 30},
		},
		{
			Name: "nil ReleaseVersion stays with its own app after sorting",
			InputDeployedApps: []DeployedApp{
				{AppName: appPow, ReleaseVersion: nil, Revision: 30},
				{AppName: appFoo, ReleaseVersion: types.Ptr(types.ReleaseVersion(2)), Revision: 20},
			},
			ExpectedAppNames:       []types.AppName{appFoo, appPow},
			ExpectedReleaseVersion: []*types.ReleaseVersion{types.Ptr(types.ReleaseVersion(2)), nil},
			ExpectedRevisions:      []types.Revision{20, 30},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			inputBefore := slices.Clone(tc.InputDeployedApps)
			actualAppNames, actualReleaseVersions, actualRevisions := deployedAppsToArrays(tc.InputDeployedApps)

			if diff := testutil.CmpDiff(tc.ExpectedAppNames, actualAppNames); diff != "" {
				t.Errorf("app names mismatch (-want, +got):\n%s", diff)
			}
			if diff := testutil.CmpDiff(tc.ExpectedReleaseVersion, actualReleaseVersions); diff != "" {
				t.Errorf("release versions mismatch (-want, +got):\n%s", diff)
			}
			if diff := testutil.CmpDiff(tc.ExpectedRevisions, actualRevisions); diff != "" {
				t.Errorf("revisions mismatch (-want, +got):\n%s", diff)
			}
			if diff := testutil.CmpDiff(inputBefore, tc.InputDeployedApps); diff != "" {
				t.Errorf("input slice was mutated (-want, +got):\n%s", diff)
			}
		})
	}
}
