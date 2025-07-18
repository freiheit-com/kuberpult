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

package reposerver

import (
	"context"
	"database/sql"
	"fmt"
	v1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argorepo "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"testing"
)

const appVersion = 1

var createOneAppInDevelopment []repository.Transformer = []repository.Transformer{
	&repository.CreateEnvironment{
		Environment: "development",
		Config: config.EnvironmentConfig{
			Upstream: &config.EnvironmentConfigUpstream{
				Latest: true,
			},
		},
	},
	&repository.CreateApplicationVersion{
		Application: "app",
		Version:     appVersion,
		Manifests: map[types.EnvName]string{
			"development": `
api: v1
kind: ConfigMap
metadata:
  name: something
  namespace: something
data:
  key: value
---
api: v1
kind: ConfigMap
metadata:
  name: somethingelse
  namespace: somethingelse
data:
  key: value
`,
		},
	},
}

func TestToRevision(t *testing.T) {
	tcs := []struct {
		Name           string
		ReleaseVersion uint64
		Expected       PseudoRevision
	}{
		{
			ReleaseVersion: 0,
			Expected:       "0000000000000000000000000000000000000000",
		},
		{
			ReleaseVersion: 666,
			Expected:       "0000000000000000000000000000000000000666",
		},
		{
			ReleaseVersion: 1234567890,
			Expected:       "0000000000000000000000000000001234567890",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(fmt.Sprintf("TestToRevision_%v", tc.ReleaseVersion), func(t *testing.T) {
			{
				// one way test:
				actual := ToRevision(tc.ReleaseVersion)
				if diff := cmp.Diff(tc.Expected, actual); diff != "" {
					t.Errorf("response mismatch (-want, +got):\n%s", diff)
				}
			}
			{
				// round-trip test:
				actual, err := FromRevision(tc.Expected)
				if err != nil {
					t.Fatalf("FromRevision failed: %v", err)
				}
				if diff := cmp.Diff(tc.ReleaseVersion, actual); diff != "" {
					t.Errorf("response mismatch (-want, +got):\n%s", diff)
				}
			}

		})
	}
}

func TestGenerateManifest(t *testing.T) {
	tcs := []struct {
		Name              string
		Setup             []repository.Transformer
		Request           *argorepo.ManifestRequest
		ExpectedResponse  *argorepo.ManifestResponse
		ExpectedError     error
		ExpectedArgoError *regexp.Regexp
		DBOnlyTest        bool
	}{
		{
			Name:  "generates a manifest for HEAD",
			Setup: createOneAppInDevelopment,
			Request: &argorepo.ManifestRequest{
				Revision: "HEAD",
				Repo: &v1alpha1.Repository{
					Repo: "<the-repo-url>",
				},
				ApplicationSource: &v1alpha1.ApplicationSource{
					Path: "environments/development/applications/app/manifests",
				},
			},

			ExpectedResponse: &argorepo.ManifestResponse{
				Manifests: []string{
					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"something","namespace":"something"}}`,
					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"somethingelse","namespace":"somethingelse"}}`,
				},
				SourceType: "Directory",
			},
		},
		{
			Name:  "generates a manifest for the branch itself",
			Setup: createOneAppInDevelopment,
			Request: &argorepo.ManifestRequest{
				Revision: "master",
				Repo: &v1alpha1.Repository{
					Repo: "<the-repo-url>",
				},
				ApplicationSource: &v1alpha1.ApplicationSource{
					Path: "environments/development/applications/app/manifests",
				},
			},

			ExpectedResponse: &argorepo.ManifestResponse{
				Manifests: []string{
					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"something","namespace":"something"}}`,
					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"somethingelse","namespace":"somethingelse"}}`,
				},
				SourceType: "Directory",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name+"_with_db", func(t *testing.T) {
			repo, cfg := SetupRepositoryTestWithDBOptions(t, false)
			ctx := testutil.MakeTestContext()
			_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Setup...)
				if err2 != nil {
					return err2
				}

				return nil
			})
			// These two values change every run:
			if tc.Request.Repo.Repo == "<the-repo-url>" {
				tc.Request.Repo.Repo = cfg.URL
			}
			if tc.ExpectedResponse != nil {
				tc.ExpectedResponse.Revision = ToRevision(appVersion)
				mn := make([]string, 0)
				for _, m := range tc.ExpectedResponse.Manifests {
					mn = append(mn, strings.ReplaceAll(m, "<the-repo-url>", cfg.URL))
				}
				tc.ExpectedResponse.Manifests = mn
			}

			srv := New(repo, *cfg)
			resp, err := srv.GenerateManifest(context.Background(), tc.Request)
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.ExpectedResponse, resp, protocmp.Transform()); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}

		})
	}
}

func TestGetRevisionMetadata(t *testing.T) {
	tcs := []struct {
		Name string
	}{
		{
			Name: "returns a dummy",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			srv := (*reposerver)(nil)
			req := argorepo.RepoServerRevisionMetadataRequest{}
			_, err := srv.GetRevisionMetadata(
				context.Background(),
				&req,
			)
			if err != nil {
				t.Errorf("expected no error, but got %q", err)
			}
		})
	}
}

func SetupRepositoryTestWithDBOptions(t *testing.T, writeEslOnly bool) (repository.Repository, *repository.RepositoryConfig) {
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(5)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, writeEslOnly, t.Name())
	if err != nil {
		t.Fatalf("SetupPostgres: %v", err)
	}

	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	err = cmd.Start()
	if err != nil {
		t.Fatalf("error starting %v", err)
		return nil, nil
	}
	err = cmd.Wait()
	if err != nil {
		t.Fatalf("error waiting %v", err)
		return nil, nil
	}
	t.Logf("test created dir: %s", localDir)

	repoCfg := repository.RepositoryConfig{
		URL:                 remoteDir,
		ArgoCdGenerateFiles: true,
	}

	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(fmt.Errorf("path %s, error: %w", migrationsPath, migErr))
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	repoCfg.DBHandler = dbHandler

	repo, err := repository.New(
		testutil.MakeTestContext(),
		repoCfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, &repoCfg
}
