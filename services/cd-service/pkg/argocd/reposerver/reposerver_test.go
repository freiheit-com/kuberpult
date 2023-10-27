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

Copyright 2023 freiheit.com*/

package reposerver

import (
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
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
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

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

func TestGenerateManifest(t *testing.T) {
	tcs := []struct {
		Name              string
		Setup             []repository.Transformer
		Request           *argorepo.ManifestRequest
		ExpectedResponse  *argorepo.ManifestResponse
		ExpectedError     string
		ExpectedArgoError *regexp.Regexp
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
		{
			Name:  "generates a manifest for a fixed commit id",
			Setup: createOneAppInDevelopment,
			Request: &argorepo.ManifestRequest{
				Revision: "<last-commit-id>",
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
			Name:  "rejectes unknown refs",
			Setup: createOneAppInDevelopment,
			Request: &argorepo.ManifestRequest{
				Revision: "not-our-branch",
				Repo: &v1alpha1.Repository{
					Repo: "<the-repo-url>",
				},
				ApplicationSource: &v1alpha1.ApplicationSource{
					Path: "environments/development/applications/app/manifests",
				},
			},

			ExpectedArgoError: regexp.MustCompile("\\AUnable to resolve 'not-our-branch' to a commit SHA\\z"),
			ExpectedError:     "rpc error: code = NotFound desc = unknown revision \"not-our-branch\", I only know \"HEAD\", \"master\" and commit hashes",
		},
		{
			Name:  "rejectes unknown commit ids",
			Setup: createOneAppInDevelopment,
			Request: &argorepo.ManifestRequest{
				Revision: "b551320bc327abfabf9df32ee5a830f8ccb1e88d",
				Repo: &v1alpha1.Repository{
					Repo: "<the-repo-url>",
				},
				ApplicationSource: &v1alpha1.ApplicationSource{
					Path: "environments/development/applications/app/manifests",
				},
			},

			// The error message from argo cd contains the output log of git which differs slightly with the git version. Therefore, we don't match on that.
			ExpectedArgoError: regexp.MustCompile("\\Arpc error: code = Internal desc = Failed to checkout revision b551320bc327abfabf9df32ee5a830f8ccb1e88d:"),
			ExpectedError:     "rpc error: code = NotFound desc = unknown revision \"b551320bc327abfabf9df32ee5a830f8ccb1e88d\", I only know \"HEAD\", \"master\" and commit hashes",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, cfg := testRepository(t)
			err := repo.Apply(testutil.MakeTestContext(), tc.Setup...)
			if err != nil {
				t.Fatalf("failed setup: %s", err)
			}
			// These two values change every run:
			if tc.Request.Repo.Repo == "<the-repo-url>" {
				tc.Request.Repo.Repo = cfg.URL
			}
			if tc.Request.Revision == "<last-commit-id>" {

				tc.Request.Revision = repo.State().Commit.Id().String()
			}
			if tc.ExpectedResponse != nil {
				tc.ExpectedResponse.Revision = repo.State().Commit.Id().String()
			}

			srv := New(repo, cfg)
			resp, err := srv.GenerateManifest(context.Background(), tc.Request)
			if tc.ExpectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, but got %q", err)
				}

				d := cmp.Diff(resp, tc.ExpectedResponse, protocmp.Transform())
				if d != "" {
					t.Errorf("unexpected response: %s", d)
				}
			} else {
				if err.Error() != tc.ExpectedError {
					t.Errorf("got wrong error, expected %q but got %q", tc.ExpectedError, err.Error())
				}
			}
			asrv := testArgoServer(t)
			aresp, err := asrv.GenerateManifest(context.Background(), tc.Request)
			if tc.ExpectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, but got %q", err)
				}

				d := cmp.Diff(aresp, tc.ExpectedResponse, protocmp.Transform())
				if d != "" {
					t.Errorf("unexpected response: %s", d)
				}
			} else {
				if !tc.ExpectedArgoError.MatchString(err.Error()) {
					t.Fatalf("got wrong error, expected to match %q but got %q", tc.ExpectedArgoError, err)
				}
			}
		})
	}
}

func TestResolveRevision(t *testing.T) {
	tcs := []struct {
		Name              string
		Setup             []repository.Transformer
		Request           *argorepo.ResolveRevisionRequest
		ExpectedError     string
		ExpectedArgoError string
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

			ExpectedError: "rpc error: code = NotFound desc = unknown revision \"not-our-branch\", I only know \"HEAD\", \"master\" and commit hashes",

			ExpectedArgoError: "Unable to resolve 'not-our-branch' to a commit SHA",
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
			repo, cfg := testRepository(t)
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
			if tc.ExpectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, but got %q", err)
				}
			} else {
				if err == nil {

					t.Fatalf("expected error %q but got response %#v", tc.ExpectedArgoError, aresp)
				}
				if err.Error() != tc.ExpectedArgoError {
					t.Fatalf("got wrong error, expected %q but got %q", tc.ExpectedArgoError, err)
				}
			}

			srv := New(repo, cfg)
			resp, err := srv.ResolveRevision(context.Background(), tc.Request)
			if tc.ExpectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, but got %q", err)
				}
				// We only need to check here if both are the same
				d := cmp.Diff(resp, aresp, protocmp.Transform())
				if d != "" {
					t.Errorf("unexpected response: %s", d)
				}
			} else {
				if err.Error() != tc.ExpectedError {
					t.Errorf("got wrong error, expected %q but got %q", tc.ExpectedError, err.Error())
				}
			}

		})
	}
}

func testRepository(t *testing.T) (repository.Repository, repository.RepositoryConfig) {
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Run()
	cfg := repository.RepositoryConfig{
		URL:    "file://" + remoteDir,
		Path:   localDir,
		Branch: "master",
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
