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
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	v1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argorepo "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/cache"
	"github.com/argoproj/argo-cd/v2/reposerver/metrics"
	argosrv "github.com/argoproj/argo-cd/v2/reposerver/repository"
	"github.com/argoproj/argo-cd/v2/util/argo"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
)

// Used to compare two error message strings, needed because errors.Is(fmt.Errorf(text),fmt.Errorf(text)) == false
type errMatcher struct {
	msg string
}

func (e errMatcher) Error() string {
	return e.msg
}

func (e errMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}

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
		Manifests: map[string]string{
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

var createOneAppInDevelopmentAndTesting []repository.Transformer = []repository.Transformer{
	&repository.CreateEnvironment{
		Environment: "development",
		Config: config.EnvironmentConfig{
			Upstream: &config.EnvironmentConfigUpstream{
				Latest: true,
			},
			ArgoCd: &config.EnvironmentConfigArgoCd{
				Destination: config.ArgoCdDestination{
					Server: "development",
				},
			},
		},
	},
	&repository.CreateEnvironment{
		Environment: "testing",
		Config: config.EnvironmentConfig{
			Upstream: &config.EnvironmentConfigUpstream{
				Latest: true,
			},
			ArgoCd: &config.EnvironmentConfigArgoCd{
				Destination: config.ArgoCdDestination{
					Server: "testing",
				},
			},
		},
	},
	&repository.CreateApplicationVersion{
		Application: "app",
		Manifests: map[string]string{
			"development": `
api: v1
kind: ConfigMap
metadata:
  name: something
  namespace: something
data:
  key: value`,
			"testing": `
api: v1
kind: ConfigMap
metadata:
  name: something
  namespace: something
data:
  key: value`,
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

//func TestGenerateManifest(t *testing.T) {
//	tcs := []struct {
//		Name              string
//		Setup             []repository.Transformer
//		Request           *argorepo.ManifestRequest
//		ExpectedResponse  *argorepo.ManifestResponse
//		ExpectedError     error
//		ExpectedArgoError *regexp.Regexp
//		RepoOnlyTest      bool
//		DBOnlyTest        bool
//	}{
//		{
//			Name:  "generates a manifest for HEAD",
//			Setup: createOneAppInDevelopment,
//			Request: &argorepo.ManifestRequest{
//				Revision: "HEAD",
//				Repo: &v1alpha1.Repository{
//					Repo: "<the-repo-url>",
//				},
//				ApplicationSource: &v1alpha1.ApplicationSource{
//					Path: "environments/development/applications/app/manifests",
//				},
//			},
//
//			ExpectedResponse: &argorepo.ManifestResponse{
//				Manifests: []string{
//					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"something","namespace":"something"}}`,
//					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"somethingelse","namespace":"somethingelse"}}`,
//				},
//				SourceType: "Directory",
//			},
//		},
//		{
//			Name:  "generates a manifest for the branch itself",
//			Setup: createOneAppInDevelopment,
//			Request: &argorepo.ManifestRequest{
//				Revision: "master",
//				Repo: &v1alpha1.Repository{
//					Repo: "<the-repo-url>",
//				},
//				ApplicationSource: &v1alpha1.ApplicationSource{
//					Path: "environments/development/applications/app/manifests",
//				},
//			},
//
//			ExpectedResponse: &argorepo.ManifestResponse{
//				Manifests: []string{
//					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"something","namespace":"something"}}`,
//					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"somethingelse","namespace":"somethingelse"}}`,
//				},
//				SourceType: "Directory",
//			},
//		},
//		{
//			Name:  "supports the include filter",
//			Setup: createOneAppInDevelopmentAndTesting,
//			Request: &argorepo.ManifestRequest{
//				Revision: "master",
//				Repo: &v1alpha1.Repository{
//					Repo: "<the-repo-url>",
//				},
//				ApplicationSource: &v1alpha1.ApplicationSource{
//					Path: "argocd/v1alpha1",
//					Directory: &v1alpha1.ApplicationSourceDirectory{
//						Include: "development.yaml",
//					},
//				},
//			},
//			RepoOnlyTest: true,
//			ExpectedResponse: &argorepo.ManifestResponse{
//				Manifests: []string{
//					`{"apiVersion":"argoproj.io/v1alpha1","kind":"AppProject","metadata":{"name":"development"},"spec":{"description":"development","destinations":[{"server":"development"}],"sourceRepos":["*"]}}`,
//					`{"apiVersion":"argoproj.io/v1alpha1","kind":"Application","metadata":{"annotations":{"argocd.argoproj.io/manifest-generate-paths":"/environments/development/applications/app/manifests","com.freiheit.kuberpult/application":"app","com.freiheit.kuberpult/environment":"development","com.freiheit.kuberpult/team":""},"finalizers":["resources-finalizer.argocd.argoproj.io"],"labels":{"com.freiheit.kuberpult/team":""},"name":"development-app"},"spec":{"destination":{"server":"development"},"project":"development","source":{"path":"environments/development/applications/app/manifests","repoURL":"<the-repo-url>","targetRevision":"master"},"syncPolicy":{"automated":{"allowEmpty":true,"prune":true,"selfHeal":true}}}}`,
//				},
//				SourceType: "Directory",
//			},
//		},
//		{
//			Name:  "generates a manifest for a fixed commit id",
//			Setup: createOneAppInDevelopment,
//			Request: &argorepo.ManifestRequest{
//				Revision: "<last-commit-id>",
//				Repo: &v1alpha1.Repository{
//					Repo: "<the-repo-url>",
//				},
//				ApplicationSource: &v1alpha1.ApplicationSource{
//					Path: "environments/development/applications/app/manifests",
//				},
//			},
//
//			ExpectedResponse: &argorepo.ManifestResponse{
//				Manifests: []string{
//					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"something","namespace":"something"}}`,
//					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"somethingelse","namespace":"somethingelse"}}`,
//				},
//				SourceType: "Directory",
//			},
//		},
//		{
//			Name:  "rejects unknown refs",
//			Setup: createOneAppInDevelopment,
//			Request: &argorepo.ManifestRequest{
//				Revision: "not-our-branch",
//				Repo: &v1alpha1.Repository{
//					Repo: "<the-repo-url>",
//				},
//				ApplicationSource: &v1alpha1.ApplicationSource{
//					Path: "environments/development/applications/app/manifests",
//				},
//			},
//			RepoOnlyTest:      true,
//			ExpectedArgoError: regexp.MustCompile("\\AUnable to resolve 'not-our-branch' to a commit SHA\\z"),
//			ExpectedError:     errMatcher{"rpc error: code = NotFound desc = unknown revision \"not-our-branch\", I only know \"HEAD\", \"master\" and commit hashes"},
//		},
//		{
//			Name:  "rejects unknown commit ids",
//			Setup: createOneAppInDevelopment,
//			Request: &argorepo.ManifestRequest{
//				Revision: "b551320bc327abfabf9df32ee5a830f8ccb1e88d",
//				Repo: &v1alpha1.Repository{
//					Repo: "<the-repo-url>",
//				},
//				ApplicationSource: &v1alpha1.ApplicationSource{
//					Path: "environments/development/applications/app/manifests",
//				},
//			},
//			RepoOnlyTest: true,
//			// The error message from argo cd contains the output log of git which differs slightly with the git version. Therefore, we don't match on that.
//			ExpectedArgoError: regexp.MustCompile("\\A.*rpc error: code = Internal desc = Failed to checkout revision b551320bc327abfabf9df32ee5a830f8ccb1e88d:"),
//			ExpectedError:     errMatcher{"rpc error: code = NotFound desc = unknown revision \"b551320bc327abfabf9df32ee5a830f8ccb1e88d\", I only know \"HEAD\", \"master\" and commit hashes"},
//		},
//		{
//			Name:  "rejects unexpected path",
//			Setup: createOneAppInDevelopment,
//			Request: &argorepo.ManifestRequest{
//				Revision: "<last-commit-id>",
//				Repo: &v1alpha1.Repository{
//					Repo: "<the-repo-url>",
//				},
//				ApplicationSource: &v1alpha1.ApplicationSource{
//					Path: "environments/development/app/manifests",
//				},
//			},
//			DBOnlyTest: true,
//			// The error message from argo cd contains the output log of git which differs slightly with the git version. Therefore, we don't match on that.
//			ExpectedArgoError: regexp.MustCompile("\\A.*rpc error: code = Internal desc = Failed to checkout revision b551320bc327abfabf9df32ee5a830f8ccb1e88d:"),
//			ExpectedError:     errMatcher{"unexpected path: 'environments/development/app/manifests'"},
//		},
//	}
//	for _, tc := range tcs {
//		tc := tc
//		if !tc.DBOnlyTest {
//			t.Run(tc.Name+"_no_db", func(t *testing.T) {
//				repo, cfg := setupRepository(t)
//				err := repo.Apply(testutil.MakeTestContext(), tc.Setup...)
//				if err != nil {
//					t.Fatalf("failed setup: %s", err)
//				}
//				// These two values change every run:
//				if tc.Request.Repo.Repo == "<the-repo-url>" {
//					tc.Request.Repo.Repo = cfg.URL
//				}
//				if tc.Request.Revision == "<last-commit-id>" {
//
//					tc.Request.Revision = repo.State().Commit.Id().String()
//				}
//				if tc.ExpectedResponse != nil {
//					tc.ExpectedResponse.Revision = repo.State().Commit.Id().String()
//					mn := make([]string, 0)
//					for _, m := range tc.ExpectedResponse.Manifests {
//						mn = append(mn, strings.ReplaceAll(m, "<the-repo-url>", cfg.URL))
//					}
//					tc.ExpectedResponse.Manifests = mn
//				}
//
//				srv := New(repo, cfg)
//				resp, err := srv.GenerateManifest(context.Background(), tc.Request)
//				if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
//					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
//				}
//				if diff := cmp.Diff(tc.ExpectedResponse, resp, protocmp.Transform()); diff != "" {
//					t.Errorf("response mismatch (-want, +got):\n%s", diff)
//				}
//
//				asrv := testArgoServer(t)
//				aresp, err := asrv.GenerateManifest(context.Background(), tc.Request)
//				if tc.ExpectedError != nil {
//					if !tc.ExpectedArgoError.MatchString(err.Error()) {
//						t.Fatalf("got wrong error, expected to match %q but got %q", tc.ExpectedArgoError, err)
//					}
//				} else if err != nil {
//					t.Fatalf("unexpected error: %s", err.Error())
//				}
//				if diff := cmp.Diff(tc.ExpectedResponse, aresp, protocmp.Transform()); diff != "" {
//					t.Errorf("response mismatch (-want, +got):\n%s", diff)
//				}
//			})
//		}
//		if !tc.RepoOnlyTest {
//			t.Run(tc.Name+"_with_db", func(t *testing.T) {
//				repo, cfg := SetupRepositoryTestWithDBOptions(t, false)
//
//				err := repo.Apply(testutil.MakeTestContext(), tc.Setup...)
//				if err != nil {
//					t.Fatalf("failed setup: %s", err)
//				}
//				// These two values change every run:
//				if tc.Request.Repo.Repo == "<the-repo-url>" {
//					tc.Request.Repo.Repo = cfg.URL
//				}
//				if tc.Request.Revision == "<last-commit-id>" {
//
//					tc.Request.Revision = repo.State().Commit.Id().String()
//				}
//				if tc.ExpectedResponse != nil {
//					tc.ExpectedResponse.Revision = ToRevision(appVersion)
//					mn := make([]string, 0)
//					for _, m := range tc.ExpectedResponse.Manifests {
//						mn = append(mn, strings.ReplaceAll(m, "<the-repo-url>", cfg.URL))
//					}
//					tc.ExpectedResponse.Manifests = mn
//				}
//
//				srv := New(repo, *cfg)
//				resp, err := srv.GenerateManifest(context.Background(), tc.Request)
//				if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
//					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
//				}
//				if diff := cmp.Diff(tc.ExpectedResponse, resp, protocmp.Transform()); diff != "" {
//					t.Errorf("response mismatch (-want, +got):\n%s", diff)
//				}
//
//			})
//		}
//	}
//}

func TestResolveRevision(t *testing.T) {
	tcs := []struct {
		Name              string
		Setup             []repository.Transformer
		Request           *argorepo.ResolveRevisionRequest
		ExpectedError     error
		ExpectedArgoError error
	}{
		{
			Name:  "resolves HEAD",
			Setup: createOneAppInDevelopment,
			Request: &argorepo.ResolveRevisionRequest{
				App: &v1alpha1.Application{
					Spec: v1alpha1.ApplicationSpec{},
				},
				Repo: &v1alpha1.Repository{
					Repo: "<the-repo-url>",
				},
				AmbiguousRevision: "HEAD",
			},
		},
		{
			Name:  "resolves master",
			Setup: createOneAppInDevelopment,
			Request: &argorepo.ResolveRevisionRequest{
				App: &v1alpha1.Application{
					Spec: v1alpha1.ApplicationSpec{},
				},
				Repo: &v1alpha1.Repository{
					Repo: "<the-repo-url>",
				},
				AmbiguousRevision: "master",
			},
		},
		{
			Name:  "resolves a commit id",
			Setup: createOneAppInDevelopment,
			Request: &argorepo.ResolveRevisionRequest{
				App: &v1alpha1.Application{
					Spec: v1alpha1.ApplicationSpec{},
				},
				Repo: &v1alpha1.Repository{
					Repo: "<the-repo-url>",
				},
				AmbiguousRevision: "<last-commit-id>",
			},
		},
		{
			Name:  "rejects an unknown branch",
			Setup: createOneAppInDevelopment,
			Request: &argorepo.ResolveRevisionRequest{
				App: &v1alpha1.Application{
					Spec: v1alpha1.ApplicationSpec{},
				},
				Repo: &v1alpha1.Repository{
					Repo: "<the-repo-url>",
				},
				AmbiguousRevision: "not-our-branch",
			},

			ExpectedError: errMatcher{"rpc error: code = NotFound desc = unknown revision \"not-our-branch\", I only know \"HEAD\", \"master\" and commit hashes"},

			ExpectedArgoError: errMatcher{"Unable to resolve 'not-our-branch' to a commit SHA"},
		},
		{
			Name:  "accepts unknown commit ids",
			Setup: createOneAppInDevelopment,
			Request: &argorepo.ResolveRevisionRequest{
				App: &v1alpha1.Application{
					Spec: v1alpha1.ApplicationSpec{},
				},
				Repo: &v1alpha1.Repository{
					Repo: "<the-repo-url>",
				},
				AmbiguousRevision: "b551320bc327abfabf9df32ee5a830f8ccb1e88d",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, cfg := setupRepository(t)
			err := repo.Apply(testutil.MakeTestContext(), tc.Setup...)
			if err != nil {
				t.Fatalf("failed setup: %s", err)
			}
			// These two values change every run:
			if tc.Request.Repo.Repo == "<the-repo-url>" {
				tc.Request.Repo.Repo = cfg.URL
			}
			if tc.Request.AmbiguousRevision == "<last-commit-id>" {
				tc.Request.AmbiguousRevision = repo.State().Commit.Id().String()
			}
			asrv := testArgoServer(t)
			aresp, err := asrv.ResolveRevision(context.Background(), tc.Request)
			if diff := cmp.Diff(tc.ExpectedArgoError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}

			srv := New(repo, cfg)
			resp, err := srv.ResolveRevision(context.Background(), tc.Request)
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}

			if tc.ExpectedError == nil {
				// We only need to check here if both are the same
				if diff := cmp.Diff(aresp, resp, protocmp.Transform()); diff != "" {
					t.Errorf("responses mismatch (-want, +got):\n%s", diff)
				}
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

func setupRepository(t *testing.T) (repository.Repository, repository.RepositoryConfig) {
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Run()
	cfg := repository.RepositoryConfig{
		URL:                 "file://" + remoteDir,
		Path:                localDir,
		Branch:              "master",
		ArgoCdGenerateFiles: true,
	}
	repo, err := repository.New(
		testutil.MakeTestContext(),
		cfg,
	)
	if err != nil {
		t.Fatalf("expected no error, got '%e'", err)
	}
	return repo, cfg
}

func SetupRepositoryTestWithDBOptions(t *testing.T, writeEslOnly bool) (repository.Repository, *repository.RepositoryConfig) {
	ctx := context.Background()
	migrationsPath, err := testutil.CreateMigrationsPath(5)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dbConfig := &db.DBConfig{
		DriverName:     "sqlite3",
		MigrationsPath: migrationsPath,
		WriteEslOnly:   writeEslOnly,
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
		Path:                localDir,
		CommitterEmail:      "kuberpult@freiheit.com",
		CommitterName:       "kuberpult",
		ArgoCdGenerateFiles: true,
	}
	dbConfig.DbHost = dir

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

func testArgoServer(t *testing.T) argorepo.RepoServerServiceServer {
	argoRoot := t.TempDir()
	t.Cleanup(
		func() {
			// argocd chmods all its directories in such a way that they can't be listed.
			// this makes a lot of sense until you actually want to remove them cleanly.
			os.Chmod(argoRoot, 0700)
			dirs, _ := os.ReadDir(argoRoot)
			for _, dir := range dirs {
				os.Chmod(filepath.Join(argoRoot, dir.Name()), 0700)
			}
		})
	asrv := argosrv.NewService(
		metrics.NewMetricsServer(),
		cache.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Hour)), time.Hour, time.Hour),
		argosrv.RepoServerInitConstants{},
		argo.NewResourceTracking(),
		&git.NoopCredsStore{},
		argoRoot,
	)
	err := asrv.Init()
	if err != nil {
		t.Fatal(err)
	}
	return asrv

}
