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
	"fmt"
	"regexp"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
)

func makeApps(apps ...*api.Environment_Application) map[string]*api.Environment_Application {
	var result map[string]*api.Environment_Application = map[string]*api.Environment_Application{}
	for i := 0; i < len(apps); i++ {
		app := apps[i]
		result[app.Name] = app
	}
	return result
}

func makeEnv(envName string, groupName string, upstream *api.EnvironmentConfig_Upstream, apps map[string]*api.Environment_Application) *api.Environment {
	return &api.Environment{
		Name: envName,
		Config: &api.EnvironmentConfig{
			Upstream:         upstream,
			EnvironmentGroup: &groupName,
		},
		Locks: map[string]*api.Lock{},

		Applications:       apps,
		DistanceToUpstream: 0,
		Priority:           api.Priority_UPSTREAM, // we are 1 away from prod, hence pre-prod
	}
}

func makeApp(appName string, version uint64) *api.Environment_Application {
	return &api.Environment_Application{
		Name:            appName,
		Version:         version,
		Locks:           nil,
		QueuedVersion:   0,
		UndeployVersion: false,
		ArgoCd:          nil,
	}
}
func makeEnvGroup(envGroupName string, environments []*api.Environment) *api.EnvironmentGroup {
	return &api.EnvironmentGroup{
		EnvironmentGroupName: envGroupName,
		Environments:         environments,
		DistanceToUpstream:   0,
	}
}

func makeUpstreamLatest() *api.EnvironmentConfig_Upstream {
	f := true
	return &api.EnvironmentConfig_Upstream{
		Latest: &f,
	}
}

func makeUpstreamEnv(upstream string) *api.EnvironmentConfig_Upstream {
	return &api.EnvironmentConfig_Upstream{
		Environment: &upstream,
	}
}

func TestCalculateWarnings(t *testing.T) {
	var dev = "dev"
	tcs := []struct {
		Name             string
		AppName          string
		Groups           []*api.EnvironmentGroup
		ExpectedWarnings []*api.Warning
	}{
		{
			Name:    "no envs - no warning",
			AppName: "foo",
			Groups: []*api.EnvironmentGroup{
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("dev-de", dev, makeUpstreamLatest(), nil),
				})},
			ExpectedWarnings: []*api.Warning{},
		},
		{
			Name:    "app deployed in higher version on upstream should warn",
			AppName: "foo",
			Groups: []*api.EnvironmentGroup{
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("prod", dev, makeUpstreamEnv("dev"),
						makeApps(makeApp("foo", 2))),
				}),
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("dev", dev, makeUpstreamLatest(),
						makeApps(makeApp("foo", 1))),
				}),
			},
			ExpectedWarnings: []*api.Warning{
				{
					WarningType: &api.Warning_UnusualDeploymentOrder{
						UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
							UpstreamVersion:     1,
							UpstreamEnvironment: "dev",
							ThisVersion:         2,
							ThisEnvironment:     "prod",
						},
					},
				},
			},
		},
		{
			Name:    "app deployed in same version on upstream should not warn",
			AppName: "foo",
			Groups: []*api.EnvironmentGroup{
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("prod", dev, makeUpstreamEnv("dev"),
						makeApps(makeApp("foo", 2))),
				}),
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("dev", dev, makeUpstreamLatest(),
						makeApps(makeApp("foo", 2))),
				}),
			},
			ExpectedWarnings: []*api.Warning{},
		},
		{
			Name:    "app deployed in no version on upstream should warn",
			AppName: "foo",
			Groups: []*api.EnvironmentGroup{
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("prod", dev, makeUpstreamEnv("dev"),
						makeApps(makeApp("foo", 1))),
				}),
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("dev", dev, makeUpstreamLatest(),
						makeApps()),
				}),
			},
			ExpectedWarnings: []*api.Warning{
				{
					WarningType: &api.Warning_UpstreamNotDeployed{
						UpstreamNotDeployed: &api.UpstreamNotDeployed{
							UpstreamEnvironment: "dev",
							ThisVersion:         1,
							ThisEnvironment:     "prod",
						},
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			actualWarnings := CalculateWarnings(testutil.MakeTestContext(), tc.AppName, tc.Groups)
			if len(actualWarnings) != len(tc.ExpectedWarnings) {
				t.Errorf("Different number of warnings. got: %s\nwant: %s", actualWarnings, tc.ExpectedWarnings)
			}
			for i := 0; i < len(actualWarnings); i++ {
				actualWarning := actualWarnings[i]
				expectedWarning := tc.ExpectedWarnings[i]
				if diff := cmp.Diff(actualWarning.String(), expectedWarning.String()); diff != "" {
					t.Errorf("Different warning at index [%d]:\ngot:  %s\nwant: %s", i, actualWarning, expectedWarning)
				}
			}
		})
	}

}

func groupFromEnvs(environments []*api.Environment) []*api.EnvironmentGroup {
	return []*api.EnvironmentGroup{
		{
			EnvironmentGroupName: "group1",
			Environments:         environments,
		},
	}
}

func TestDeriveUndeploySummary(t *testing.T) {
	var tcs = []struct {
		Name           string
		AppName        string
		groups         []*api.EnvironmentGroup
		ExpectedResult api.UndeploySummary
	}{
		{
			Name:           "No Environments",
			AppName:        "foo",
			groups:         []*api.EnvironmentGroup{},
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "one Environment but no Application",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"bar": { // different app
							UndeployVersion: true,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "One Env with undeploy",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: true,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "One Env with normal version",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: false,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_NORMAL,
		},
		{
			Name:    "Two Envs all undeploy",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: true,
							Version:         666,
						},
					},
				},
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: true,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "Two Envs all normal",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: false,
							Version:         666,
						},
					},
				},
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: false,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_NORMAL,
		},
		{
			Name:    "Two Envs all different",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: true,
							Version:         666,
						},
					},
				},
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: false,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_MIXED,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			actualResult := DeriveUndeploySummary(tc.AppName, tc.groups)
			if !cmp.Equal(tc.ExpectedResult, actualResult) {
				t.Fatal("Output mismatch (-want +got):\n", cmp.Diff(tc.ExpectedResult, actualResult))
			}
		})
	}
}

func TestUpdateOverviewCacheDB(t *testing.T) {
	tcs := []struct {
		Name           string
		CreateTestData func(context.Context, *sql.Tx, *db.DBHandler) error
		ExpectedBlob   string
	}{
		{
			Name: "No Environments",
			CreateTestData: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				if err := dbHandler.DBWriteAllApplications(ctx, transaction, 1, []string{"foo"}); err != nil {
					return err
				}
				if err := dbHandler.DBInsertApplication(ctx, transaction, "foo", 1, db.AppStateChangeCreate, db.DBAppMetaData{Team: "foo"}); err != nil {
					return err
				}
				if err := dbHandler.DBInsertAllReleases(ctx, transaction, "foo", []int64{1}, db.EslId(1)); err != nil {
					return err
				}
				if err := dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					App:           "foo",
					ReleaseNumber: 1,
					Manifests:     db.DBReleaseManifests{},
				}, 1); err != nil {
					return err
				}
				return nil
			},
			ExpectedBlob: "{\"applications\":{\"foo\":{\"name\":\"foo\",\"releases\":[{\"version\":1,\"created_at\":{\"seconds\":1,\"nanos\":1}}],\"team\":\"foo\",\"undeploy_summary\":1}}}",
		},
		{
			Name: "One Environment",
			CreateTestData: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				if err := dbHandler.DBWriteAllApplications(ctx, transaction, 1, []string{"foo"}); err != nil {
					return err
				}
				if err := dbHandler.DBInsertApplication(ctx, transaction, "foo", 1, db.AppStateChangeCreate, db.DBAppMetaData{Team: "foo"}); err != nil {
					return err
				}
				if err := dbHandler.DBInsertAllReleases(ctx, transaction, "foo", []int64{1}, db.EslId(1)); err != nil {
					return err
				}
				if err := dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					App:           "foo",
					ReleaseNumber: 1,
					Manifests:     db.DBReleaseManifests{},
				}, 1); err != nil {
					return err
				}
				if err := dbHandler.DBWriteAllEnvironments(ctx, transaction, []string{"testEnv"}); err != nil {
					return err
				}
				envGroup := "testGroup"
				if err := dbHandler.DBWriteEnvironment(ctx, transaction, "testEnv", config.EnvironmentConfig{
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					EnvironmentGroup: &envGroup,
				}); err != nil {
					return err
				}
				return nil
			},
			ExpectedBlob: "{\"applications\":{\"foo\":{\"name\":\"foo\",\"releases\":[{\"version\":1,\"created_at\":{\"seconds\":1,\"nanos\":1}}],\"team\":\"foo\",\"undeploy_summary\":1}},\"environment_groups\":[{\"environment_group_name\":\"testGroup\",\"environments\":[{\"name\":\"testEnv\",\"config\":{\"upstream\":{\"latest\":true},\"argocd\":{},\"environment_group\":\"testGroup\"},\"applications\":{\"foo\":{\"name\":\"foo\",\"deployment_meta_data\":{},\"team\":\"foo\"}},\"priority\":5}],\"priority\":5}]}",
		},
		{
			Name: "Without Envs",
			CreateTestData: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return nil
			},
			ExpectedBlob: "{}",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := SetupRepositoryTestWithDB(t)
			dbHandler := repo.State().DBHandler
			dbHandler.WithTransaction(nil, true, func(ctx context.Context, transaction *sql.Tx) error {

				tc.CreateTestData(ctx, transaction, dbHandler)
				err := UpdateOverviewDB(ctx, repo.State(), transaction)
				if err != nil {
					t.Fatal(err)
				}
				latestOverview, err := dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					t.Fatal(err)
				}
				re := regexp.MustCompile(`"seconds":\d+`)
				latestOverview.Blob = re.ReplaceAllString(latestOverview.Blob, fmt.Sprintf(`"seconds":%d`, 1))
				re = regexp.MustCompile(`"nanos":\d+`)
				latestOverview.Blob = re.ReplaceAllString(latestOverview.Blob, fmt.Sprintf(`"nanos":%d`, 1))
				if diff := cmp.Diff(tc.ExpectedBlob, latestOverview.Blob); diff != "" {
					t.Fatalf("Output mismatch (-want +got):\n%s", diff)
				}
				return nil
			})

		})
	}
}
