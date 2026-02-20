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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	git "github.com/libgit2/git2go/v34"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/testutilauth"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

var versionZero = uint64(0)
var versionOne = uint64(1)
var versionTwo = uint64(2)

func TestRetrySsh(t *testing.T) {
	tcs := []struct {
		Name              string
		NumOfFailures     int
		ExpectedNumOfCall int
		ExpectedResponse  error
		CustomResponse    error
	}{
		{
			Name:              "No retries success from 1st try",
			NumOfFailures:     0,
			ExpectedNumOfCall: 1,
			ExpectedResponse:  nil,
			CustomResponse:    nil,
		}, {
			Name:              "Success after the 4th attempt",
			NumOfFailures:     4,
			ExpectedNumOfCall: 5,
			ExpectedResponse:  nil,
			CustomResponse:    &git.GitError{Message: "mock error"},
		}, {
			Name:              "Fail after the 6th attempt",
			NumOfFailures:     6,
			ExpectedNumOfCall: 6,
			ExpectedResponse:  &git.GitError{Message: "max number of retries exceeded error"},
			CustomResponse:    &git.GitError{Message: "max number of retries exceeded error"},
		}, {
			Name:              "Do not retry after a permanent error",
			NumOfFailures:     1,
			ExpectedNumOfCall: 1,
			ExpectedResponse:  &git.GitError{Message: "permanent error"},
			CustomResponse:    &git.GitError{Message: "permanent error", Code: git.ErrorCodeNonFastForward},
		}, {
			Name:              "Fail after the 6th attempt = Max number of retries ",
			NumOfFailures:     12,
			ExpectedNumOfCall: 6,
			ExpectedResponse:  &git.GitError{Message: "max number of retries exceeded error"},
			CustomResponse:    nil,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := &repository{}
			counter := 0
			repo.backOffProvider = func() backoff.BackOff {
				return backoff.WithMaxRetries(&backoff.ZeroBackOff{}, 5)
			}
			resp := repo.Push(testutilauth.MakeTestContext(), func() error {
				counter++
				if counter > tc.NumOfFailures {
					return nil
				}
				if counter == tc.NumOfFailures { //  Custom response
					return tc.CustomResponse
				}
				if counter == 6 { // max number of retries
					return &git.GitError{Message: "max number of retries exceeded error"}
				}
				return &git.GitError{Message: fmt.Sprintf("mock error %d", counter)}
			})

			if resp == nil || tc.ExpectedResponse == nil {
				if resp != tc.ExpectedResponse {
					t.Fatalf("new: expected '%v',  got '%v'", tc.ExpectedResponse, resp)
				}
			} else if resp.Error() != tc.ExpectedResponse.Error() {
				t.Fatalf("new: expected '%v',  got '%v'", tc.ExpectedResponse.Error(), resp.Error())
			}
			if counter != tc.ExpectedNumOfCall {
				t.Fatalf("new: expected number of calls  '%d',  got '%d'", tc.ExpectedNumOfCall, counter)
			}

		})
	}
}

func TestPushUpdate(t *testing.T) {
	tcs := []struct {
		Name            string
		InputBranch     string
		InputRefName    string
		InputStatus     string
		ExpectedSuccess bool
	}{
		{
			Name:            "Should succeed",
			InputBranch:     "main",
			InputRefName:    "refs/heads/main",
			InputStatus:     "",
			ExpectedSuccess: true,
		},
		{
			Name:            "Should fail because wrong branch",
			InputBranch:     "main",
			InputRefName:    "refs/heads/master",
			InputStatus:     "",
			ExpectedSuccess: false,
		},
		{
			Name:            "Should fail because status not empty",
			InputBranch:     "master",
			InputRefName:    "refs/heads/master",
			InputStatus:     "i am the status, stopping this from working",
			ExpectedSuccess: false,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var success = false
			actualError := commitPushUpdate(tc.InputBranch, &success)(tc.InputRefName, tc.InputStatus)
			if success != tc.ExpectedSuccess {
				t.Fatalf("expected sucess=%t but got %t", tc.ExpectedSuccess, success)
			}
			if actualError != nil {
				t.Fatalf("expected no error but got %s but got none", actualError)
			}
		})
	}
}

func TestDeleteDirIfEmpty(t *testing.T) {
	tcs := []struct {
		Name           string
		CreateThisDir  string
		DeleteThisDir  string
		ExpectedError  error
		ExpectedReason SuccessReason
	}{
		{
			Name:           "Should succeed: dir exists and is empty",
			CreateThisDir:  "foo/bar",
			DeleteThisDir:  "foo/bar",
			ExpectedReason: NoReason,
		},
		{
			Name:           "Should succeed: dir does not exist",
			CreateThisDir:  "foo/bar",
			DeleteThisDir:  "foo/bar/pow",
			ExpectedReason: DirDoesNotExist,
		},
		{
			Name:           "Should succeed: dir does not exist",
			CreateThisDir:  "foo/bar/pow",
			DeleteThisDir:  "foo/bar",
			ExpectedReason: DirNotEmpty,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _, _ := SetupRepositoryTestWithDB(t)
			state := repo.State()
			err := state.Filesystem.MkdirAll(tc.CreateThisDir, 0777)
			if err != nil {
				t.Fatalf("error in mkdir: %v", err)
				return
			}

			successReason, err := state.DeleteDirIfEmpty(tc.DeleteThisDir)
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			if successReason != tc.ExpectedReason {
				t.Fatal("Output mismatch (-want +got):\n", cmp.Diff(tc.ExpectedReason, successReason))
			}
		})
	}
}

func SetupRepositoryTestWithDB(t *testing.T) (Repository, *db.DBHandler, *RepositoryConfig) {
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, false, t.Name())
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
		return nil, nil, nil
	}
	err = cmd.Wait()
	if err != nil {
		t.Fatalf("error waiting %v", err)
		return nil, nil, nil
	}
	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	config := RepositoryConfig{
		URL:                 "file://" + remoteDir,
		Path:                localDir,
		CommitterEmail:      "kuberpult@freiheit.com",
		CommitterName:       "kuberpult",
		ArgoCdGenerateFiles: true,
		DBHandler:           dbHandler,
		Branch:              "master",
	}
	repo, err := New(
		testutilauth.MakeTestContext(),
		config,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, dbHandler, &config
}

func TestGetTagsNoTags(t *testing.T) {
	name := "No tags to be returned at all"

	t.Run(name, func(t *testing.T) {
		t.Parallel()

		_, _, repoConfig := SetupRepositoryTestWithDB(t)
		localDir := repoConfig.Path
		// dbHandler can be nil here, because that part of the code is not reached, because we do not have tags:
		var dbHandler *db.DBHandler
		tags, err := GetTags(testutilauth.MakeTestContext(), dbHandler, *repoConfig, localDir)
		if err != nil {
			t.Fatalf("new: expected no error, got '%e'", err)
		}
		if len(tags) != 0 {
			t.Fatalf("expected %v tags but got %v", 0, len(tags))
		}
	})

}

func TestGetTags(t *testing.T) {
	tcs := []struct {
		Name         string
		expectedTags []api.TagData
		tagsToAdd    []string
	}{
		{
			Name:         "Single Tag is returned",
			tagsToAdd:    []string{"v1.0.0"},
			expectedTags: []api.TagData{{Tag: "refs/tags/v1.0.0", CommitId: ""}},
		},
		{
			Name:         "Multiple tags are returned sorted",
			tagsToAdd:    []string{"v1.0.1", "v0.0.1"},
			expectedTags: []api.TagData{{Tag: "refs/tags/v0.0.1", CommitId: ""}, {Tag: "refs/tags/v1.0.1", CommitId: ""}},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			_, dbHandler, repoConfig := SetupRepositoryTestWithDB(t)
			localDir := repoConfig.Path
			_, err := New(
				testutilauth.MakeTestContext(),
				*repoConfig,
			)
			if err != nil {
				t.Fatal(err)
			}
			repo, err := git.OpenRepository(localDir)
			if err != nil {
				t.Fatal(err)
			}
			idx, err := repo.Index()
			if err != nil {
				t.Fatal(err)
			}
			treeId, err := idx.WriteTree()
			if err != nil {
				t.Fatal(err)
			}

			tree, err := repo.LookupTree(treeId)
			if err != nil {
				t.Fatal(err)
			}
			oid, err := repo.CreateCommit("HEAD", &git.Signature{Name: "SRE", Email: "testing@gmail"}, &git.Signature{Name: "SRE", Email: "testing@gmail"}, "testing", tree)
			if err != nil {
				t.Fatal(err)
			}
			commit, err := repo.LookupCommit(oid)
			if err != nil {
				t.Fatal(err)
			}
			var expectedCommits []api.TagData
			for addTag := range tc.tagsToAdd {
				commit, err := repo.Tags.Create(tc.tagsToAdd[addTag], commit, &git.Signature{Name: "SRE", Email: "testing@gmail"}, "testing")
				expectedCommits = append(expectedCommits, api.TagData{Tag: tc.tagsToAdd[addTag], CommitId: commit.String()})
				if err != nil {
					t.Fatal(err)
				}
			}
			tags, err := GetTags(testutilauth.MakeTestContext(), dbHandler, *repoConfig, localDir)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}
			if len(tags) != len(tc.expectedTags) {
				t.Fatalf("expected %v tags but got %v", len(tc.expectedTags), len(tags))
			}

			iter := 0
			for _, tagData := range tags {
				for commit := range expectedCommits {
					if tagData.Tag != expectedCommits[commit].Tag {
						if tagData.CommitId == expectedCommits[commit].CommitId {
							t.Fatalf("expected [%v] for TagList commit but got [%v]", expectedCommits[commit].CommitId, tagData.CommitId)
						}
					}
				}
				if tagData.Tag != tc.expectedTags[iter].Tag {
					t.Fatalf("expected [%v] for TagList tag but got [%v] with tagList %v", tc.expectedTags[iter].Tag, tagData.Tag, tags)
				}
				iter += 1
			}
		})
	}
}

func TestArgoCDFileGeneration(t *testing.T) {
	transformers := []Transformer{
		&CreateEnvironment{
			Environment: "production",
			Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}, ArgoCd: &config.EnvironmentConfigArgoCd{
				Destination: config.ArgoCdDestination{
					Server: "development",
				},
			}},
			TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
		},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[types.EnvName]string{
				"production": "manifest",
			},
			Version:             1,
			TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
		},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[types.EnvName]string{
				"production": "manifest2",
			},
			Version:             2,
			TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
		},
	}
	commonName := "common"
	tcs := []struct {
		Name                string
		shouldGenerateFiles bool
		TransformerSetup    []Transformer
		ExpectedFiles       []string
	}{
		{
			Name:                "ArgoCD files should NOT be generated",
			shouldGenerateFiles: false,
			TransformerSetup:    transformers,
			ExpectedFiles:       make([]string, 0),
		},
		{
			Name:                "Argo CD files should be generated",
			shouldGenerateFiles: true,
			TransformerSetup:    transformers,
			ExpectedFiles:       make([]string, 0),
		},
		{
			Name:                "Create argo files for normal env",
			shouldGenerateFiles: true,
			TransformerSetup: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}, ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Server: "development",
						},
					}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production": "manifest",
					},
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			ExpectedFiles: []string{
				"argocd/v1alpha1/production.yaml",
			},
		},
		{
			Name:                "Create argo files for normal AA env",
			shouldGenerateFiles: true,
			TransformerSetup: []Transformer{
				&CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true},
						ArgoCdConfigs: &config.ArgoCDConfigs{
							CommonEnvPrefix: &commonName,
							ArgoCdConfigurations: []*config.EnvironmentConfigArgoCd{
								{
									Destination: config.ArgoCdDestination{
										Server: "development",
									},
									ConcreteEnvName: "test-1",
								},
								{
									Destination: config.ArgoCdDestination{
										Server: "development",
									},
									ConcreteEnvName: "test-2",
								},
							},
						},
					},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"development": "manifest",
					},
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			ExpectedFiles: []string{
				"argocd/v1alpha1/common-development-test-1.yaml",
				"argocd/v1alpha1/common-development-test-2.yaml",
			},
		},
		{
			Name:                "Create argo files for normal AA env",
			shouldGenerateFiles: true,
			TransformerSetup: []Transformer{
				&CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true},
						ArgoCdConfigs: &config.ArgoCDConfigs{
							CommonEnvPrefix: &commonName,
							ArgoCdConfigurations: []*config.EnvironmentConfigArgoCd{
								{
									Destination: config.ArgoCdDestination{
										Server: "development",
									},
									ConcreteEnvName: "test-1",
								},
								{
									Destination: config.ArgoCdDestination{
										Server: "development",
									},
									ConcreteEnvName: "test-2",
								},
							},
						},
					},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"development": "manifest",
					},
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			ExpectedFiles: []string{
				"argocd/v1alpha1/common-development-test-1.yaml",
				"argocd/v1alpha1/common-development-test-2.yaml",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			r, dbHandler, _ := SetupRepositoryTestWithDB(t)
			repo := r.(*repository)
			repo.config.ArgoCdGenerateFiles = tc.shouldGenerateFiles
			ctx := testutilauth.MakeTestContext()

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, tr := range tc.TransformerSetup {
					prepareDatabaseLikeCdService(ctx, transaction, tr, dbHandler, t, "author", "email")
				}
				return nil
			})

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for i, tr := range tc.TransformerSetup {
					_, applyErr := repo.ApplyTransformer(ctx, transaction, tr)
					if applyErr != nil {
						t.Fatalf("Unexpected error applying transformer[%d]: Error: %v", i, applyErr)
					}
				}
				return nil
			})

			state := repo.State() //update state

			if _, err := state.Filesystem.Stat("argocd"); errors.Is(err, os.ErrNotExist) {
				if tc.shouldGenerateFiles {
					t.Fatalf("Expected ArgoCD directory, but none was found. %v", err)
				}
			} else { //Argo CD dir exists
				if !tc.shouldGenerateFiles {
					t.Fatalf("ArgoCD files should not have been generated. Found ArgoCD directory.")
				}
			}

			for _, currFileName := range tc.ExpectedFiles {
				if _, err := state.Filesystem.Stat(currFileName); errors.Is(err, os.ErrNotExist) {
					if tc.shouldGenerateFiles {
						t.Fatalf("Expected %q file, but none was found. Filesystem content:\n%s", currFileName, strings.Join(listFiles(state.Filesystem), "\n"))
					}
				}
			}
		})
	}
}

func TestArgoCDFileGenerationAcrossTimestamps(t *testing.T) {
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	var aaEnvName = "aa"
	type Step struct {
		Transformers        []Transformer
		ExpectedFiles       []*FilenameAndData
		ExpectedMissing     []*FilenameAndData
		shouldGenerateFiles bool
	}
	tcs := []struct {
		Name  string
		Steps []Step
	}{
		{
			Name: "Create argo files for normal env",
			Steps: []Step{
				{
					ExpectedFiles: []*FilenameAndData{
						{
							path: "argocd/v1alpha1/production.yaml",
							fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: production
spec:
  description: production
  destinations:
  - server: development
  sourceRepos:
  - '*'
`),
						},
					},
					Transformers: []Transformer{
						&CreateEnvironment{
							Environment: "production",
							Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}, ArgoCd: &config.EnvironmentConfigArgoCd{
								Destination: config.ArgoCdDestination{
									Server: "development",
								},
							}},
							TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
						},
					},
				},
				{
					Transformers: []Transformer{
						&CreateApplicationVersion{
							Application: "test",
							Manifests: map[types.EnvName]string{
								"production": "manifest",
							},
							Version:             1,
							TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
						},
					},
					ExpectedFiles: []*FilenameAndData{
						{
							path: "argocd/v1alpha1/production.yaml",
							fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: production
spec:
  description: production
  destinations:
  - server: development
  sourceRepos:
  - '*'
`),
						},
					},
				},
				{
					Transformers: []Transformer{
						&DeployApplicationVersion{
							Application:         "test",
							Environment:         "production",
							Version:             1,
							Revision:            0,
							TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
						},
					},
					ExpectedFiles: []*FilenameAndData{
						{
							path: "argocd/v1alpha1/production.yaml",
							fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: production
spec:
  description: production
  destinations:
  - server: development
  sourceRepos:
  - '*'
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: /environments/production/applications/test/manifests
    com.freiheit.kuberpult/aa-parent-environment: production
    com.freiheit.kuberpult/application: test
    com.freiheit.kuberpult/environment: production
    com.freiheit.kuberpult/team: ""
  finalizers:
  - resources-finalizer.argocd.argoproj.io
  labels:
    com.freiheit.kuberpult/team: ""
  name: production-test
spec:
  destination:
    server: development
  project: production
  source:
    path: environments/production/applications/test/manifests
    repoURL: test
    targetRevision: master
  syncPolicy:
    automated:
      allowEmpty: true
      prune: true
      selfHeal: true
`),
						},
					},
				},
			},
		},
		{
			Name: "Create argo files for AA environment",
			Steps: []Step{
				{
					Transformers: []Transformer{
						&CreateEnvironment{
							Environment: "production",
							Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}, ArgoCd: &config.EnvironmentConfigArgoCd{
								Destination: config.ArgoCdDestination{
									Server: "development",
								},
							}},
							TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
						},
					},
					ExpectedMissing: []*FilenameAndData{
						{
							path: "argocd/v1alpha1/some-concrete-env-name-2.yaml",
						},
						{
							path: "argocd/v1alpha1/some-concrete-env-name-1.yaml",
						},
					},
					ExpectedFiles: []*FilenameAndData{
						{
							path: "argocd/v1alpha1/production.yaml",
							fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: production
spec:
  description: production
  destinations:
  - server: development
  sourceRepos:
  - '*'
`),
						},
					},
				},
				{
					Transformers: []Transformer{
						&CreateApplicationVersion{
							Application: "test",
							Manifests: map[types.EnvName]string{
								"production": "manifest",
							},
							Version:             1,
							TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
						},
					},
					ExpectedMissing: []*FilenameAndData{
						{
							path: "argocd/v1alpha1/some-concrete-env-name-2.yaml",
						},
						{
							path: "argocd/v1alpha1/some-concrete-env-name-1.yaml",
						},
					},
					ExpectedFiles: []*FilenameAndData{
						{
							path: "argocd/v1alpha1/production.yaml",
							fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: production
spec:
  description: production
  destinations:
  - server: development
  sourceRepos:
  - '*'
`),
						},
					},
				},
				{
					Transformers: []Transformer{
						&DeployApplicationVersion{
							Application:         "test",
							Environment:         "production",
							Version:             1,
							Revision:            0,
							TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
						},
					},
					ExpectedMissing: []*FilenameAndData{
						{
							path: "argocd/v1alpha1/aa-production-some-concrete-env-name-2.yaml",
						},
						{
							path: "argocd/v1alpha1/aa-production-some-concrete-env-name-1.yaml",
						},
					},
					ExpectedFiles: []*FilenameAndData{
						{
							path: "argocd/v1alpha1/production.yaml",
							fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: production
spec:
  description: production
  destinations:
  - server: development
  sourceRepos:
  - '*'
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: /environments/production/applications/test/manifests
    com.freiheit.kuberpult/aa-parent-environment: production
    com.freiheit.kuberpult/application: test
    com.freiheit.kuberpult/environment: production
    com.freiheit.kuberpult/team: ""
  finalizers:
  - resources-finalizer.argocd.argoproj.io
  labels:
    com.freiheit.kuberpult/team: ""
  name: production-test
spec:
  destination:
    server: development
  project: production
  source:
    path: environments/production/applications/test/manifests
    repoURL: test
    targetRevision: master
  syncPolicy:
    automated:
      allowEmpty: true
      prune: true
      selfHeal: true
`),
						},
					},
				},
				{
					Transformers: []Transformer{
						&CreateEnvironment{
							Environment: "aa-test",
							Config: config.EnvironmentConfig{
								ArgoCdConfigs: &config.ArgoCDConfigs{
									CommonEnvPrefix: &aaEnvName,

									ArgoCdConfigurations: []*config.EnvironmentConfigArgoCd{
										{
											Destination: config.ArgoCdDestination{
												Name:   "some-destination-1",
												Server: "some-server",
											},
											ConcreteEnvName: "some-concrete-env-name-1",
										},
										{
											Destination: config.ArgoCdDestination{
												Name:   "some-destination-2",
												Server: "some-server",
											},
											ConcreteEnvName: "some-concrete-env-name-2",
										},
									},
								},
							},
							TransformerEslVersion: 1,
							TransformerMetadata: TransformerMetadata{
								AuthorName:  authorName,
								AuthorEmail: authorEmail,
							},
						},
					},
					ExpectedFiles: []*FilenameAndData{
						{
							path: "argocd/v1alpha1/aa-aa-test-some-concrete-env-name-1.yaml",
							fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: aa-aa-test-some-concrete-env-name-1
spec:
  description: aa-aa-test-some-concrete-env-name-1
  destinations:
  - name: some-destination-1
    server: some-server
  sourceRepos:
  - '*'
`),
						},
						{
							path: "argocd/v1alpha1/aa-aa-test-some-concrete-env-name-2.yaml",
							fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: aa-aa-test-some-concrete-env-name-2
spec:
  description: aa-aa-test-some-concrete-env-name-2
  destinations:
  - name: some-destination-2
    server: some-server
  sourceRepos:
  - '*'
`),
						},
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			r, dbHandler, _ := SetupRepositoryTestWithDB(t)
			repo := r.(*repository)
			repo.config.ArgoCdGenerateFiles = true
			ctx := testutilauth.MakeTestContext()
			for _, step := range tc.Steps {
				_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					ts, err := dbHandler.DBReadTransactionTimestamp(ctx, transaction)
					if err != nil {
						return err
					}
					for _, tr := range step.Transformers {
						tr.SetCreationTimestamp(*ts)
						prepareDatabaseLikeCdService(ctx, transaction, tr, dbHandler, t, "authorEmail", "authorName")
					}
					return nil
				})
			}
			for si, currentStep := range tc.Steps {
				_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					for i, tr := range currentStep.Transformers {
						/*
							The Generated argo files pick up the URL from the config.
							On unit tests this depends on the temporary directory the test is being run in.
						*/
						oldUrl := r.(*repository).config.URL
						r.(*repository).config.URL = "test"
						_, applyErr := repo.ApplyTransformer(ctx, transaction, tr)
						if applyErr != nil {
							t.Fatalf("Unexpected error applying transformer[%d] on step[%d]: Error: %v", i, si, applyErr)
						}
						r.(*repository).config.URL = oldUrl //Re-instate URL, in case it is needed somewhere else

					}
					return nil
				})

				state := repo.State() //update state
				if err := verifyMissing(state.Filesystem, currentStep.ExpectedMissing); err != nil {
					t.Fatalf("Error while verifying missing content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(state.Filesystem), "\n"))
				}
				if err := verifyContent(state.Filesystem, currentStep.ExpectedFiles); err != nil {
					t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(state.Filesystem), "\n"))
				}
			}
		})
	}
}

func TestMinimizeCommitsGeneration(t *testing.T) {
	var group = "dev"
	tcs := []struct {
		Name               string
		setup              []Transformer
		targetTransformers []Transformer
		shouldCreateCommit bool
		databasePopulation func(ctx context.Context, transaction *sql.Tx, handler *db.DBHandler) error
	}{
		{
			Name: "No-operation should not create new commits (Control)",
			setup: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}, ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Server: "development",
						},
					}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return nil
			},
		},
		{
			Name: "Delete environment Locks do not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}, ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Server: "development",
						},
					}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvironmentLock{
					Environment:           "production",
					LockId:                "my-lock",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return dbHandler.DBWriteEnvironmentLock(ctx, transaction, "my-lock", "production", db.LockMetadata{})
			},
		},
		{
			Name: "Create Environment Application Locks does not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: false}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateEnvironmentApplicationLock{
					Environment:           "production",
					LockId:                "my-lock",
					Application:           "test",
					Message:               "my-lock-message",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				}, "")
				if err != nil {
					return err
				}
				return dbHandler.DBWriteApplicationLock(ctx, transaction, "my-lock", "production", "test", db.LockMetadata{})
			},
		},
		{
			Name: "Delete Environment Application Locks does not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: false}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironmentApplicationLock{
					Environment:           "production",
					LockId:                "my-lock",
					Application:           "test",
					Message:               "my-lock-message",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvironmentApplicationLock{
					Environment:           "production",
					LockId:                "my-lock",
					Application:           "test",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				}, "")
				if err != nil {
					return err
				}
				return dbHandler.DBWriteApplicationLock(ctx, transaction, "my-lock", "production", "test", db.LockMetadata{})
			},
		},
		{
			Name: "CreateEnvironmentTeamLock does not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: false}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateEnvironmentTeamLock{
					Environment:           "production",
					LockId:                "my-lock",
					Team:                  "team-123",
					Message:               "my-lock-message",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				}, "")
				if err != nil {
					return err
				}
				return dbHandler.DBWriteTeamLock(ctx, transaction, "my-lock", "production", "team-123", db.LockMetadata{})
			},
		},
		{
			Name: "DeleteEnvironmentTeamLock does not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: false}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironmentTeamLock{
					Environment:           "production",
					LockId:                "my-lock",
					Team:                  "team-123",
					Message:               "my-lock-message",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvironmentTeamLock{
					Environment:           "production",
					LockId:                "my-lock",
					Team:                  "team-123",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				}, "")
				if err != nil {
					return err
				}
				return dbHandler.DBWriteTeamLock(ctx, transaction, "my-lock", "production", "team-123", db.LockMetadata{})
			},
		},
		{
			Name: "Migration transformer creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&MigrationTransformer{
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return nil
			},
		},
		{
			Name: "Delete Env From App creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironment{
					Environment:         "development",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production":  "manifest",
						"development": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvFromApp{
					Application:         "test",
					Environment:         "development",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				}, "")
				if err != nil {
					return err
				}
				return dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					App:          "test",
					Environments: []types.EnvName{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[types.EnvName]string{
						"production":  "manifest",
						"development": "manifest",
					}},
				})
			},
		},
		{
			Name: "CreateUndeployApplicationVersion creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironment{
					Environment:         "development",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production":  "manifest",
						"development": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateUndeployApplicationVersion{
					Application:         "test",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				}, "")
				if err != nil {
					return err
				}
				return dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					App:          "test",
					Environments: []types.EnvName{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[types.EnvName]string{
						"production":  "manifest",
						"development": "manifest",
					}},
				})
			},
		},
		{
			Name: "CreateEnvironmentGroupLock does not create new commits",
			setup: []Transformer{

				&CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						EnvironmentGroup: &group,
					},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateEnvironmentGroupLock{
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return nil
			},
		},
		{
			Name: "DeleteEnvironmentGroupLock does not create new commits",
			setup: []Transformer{

				&CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						EnvironmentGroup: &group,
					},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvironmentGroupLock{
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return nil
			},
		},
		{
			Name: "UndeployApplication creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironment{
					Environment:         "development",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production":  "manifest",
						"development": "manifest",
					},
					Team:                "team-123",
					Version:             0,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateUndeployApplicationVersion{
					Application:         "test",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&UndeployApplication{
					Application:         "test",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				}, "")
				if err != nil {
					return err
				}
				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					App:          "test",
					Environments: []types.EnvName{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[types.EnvName]string{
						"production":  "manifest",
						"development": "manifest",
					}},
				})
				if err != nil {
					return err
				}

				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					App:          "test",
					Environments: []types.EnvName{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[types.EnvName]string{
						"production":  "manifest",
						"development": "manifest",
					}},
					Metadata: db.DBReleaseMetaData{
						UndeployVersion: true,
					},
				})
				return err
			},
		},
		{
			Name: "UndeployApplication creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironment{
					Environment:         "development",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production":  "manifest",
						"development": "manifest",
					},
					Team:                "team-123",
					Version:             0,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateUndeployApplicationVersion{
					Application:         "test",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&UndeployApplication{
					Application:         "test",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				}, "")
				if err != nil {
					return err
				}
				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					App:          "test",
					Environments: []types.EnvName{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[types.EnvName]string{
						"production":  "manifest",
						"development": "manifest",
					}},
				})
				if err != nil {
					return err
				}

				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					App:          "test",
					Environments: []types.EnvName{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[types.EnvName]string{
						"production":  "manifest",
						"development": "manifest",
					}},
					Metadata: db.DBReleaseMetaData{
						UndeployVersion: true,
					},
				})
				return err
			},
		},
		{
			Name: "Create Application Version with no deployments does not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Server: "development",
						},
					}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				}, "")
				if err != nil {
					return err
				}
				return dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					App:          "test",
					Environments: []types.EnvName{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[types.EnvName]string{
						"production": "manifest",
					}},
				})
			},
		},
		{
			Name: "Create Application Version with deployments should create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production": "manifest",
					},
					Team:                  "team-123",
					Version:               1,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
					TransformerEslVersion: 2,
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				}, "")
				if err != nil {
					return err
				}
				version := uint64(1)
				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &version,
					},
					App:          "test",
					Environments: []types.EnvName{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[types.EnvName]string{
						"production": "manifest",
					}},
				})
				if err != nil {
					return err
				}

				return dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, db.Deployment{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &version,
					},
					App:           "test",
					Env:           "production",
					TransformerID: 2,
				})
			},
		},
		{
			Name: "Deployments creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Server: "development",
						},
					}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&DeployApplicationVersion{
					Application:         "test",
					Environment:         "production",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				}, "")
				if err != nil {
					return err
				}
				return dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					App:          "test",
					Environments: []types.EnvName{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[types.EnvName]string{
						"production": "manifest",
					}},
				})
			},
		},
		{
			Name: "Create environment should create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateEnvironment{
					Environment:         "staging",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return nil
			},
		},
		{
			Name: "Deleting environemnts should create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvironment{
					Environment:         "production",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return dbHandler.DBWriteEnvironment(ctx, transaction, "production", config.EnvironmentConfig{})
			},
		},
		{
			Name: "Mixed should create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"production": "manifest",
					},
					Team:                  "team",
					Version:               1,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
					TransformerEslVersion: 2,
				},
				&CreateEnvironmentLock{
					Environment:           "production",
					LockId:                "my-lock",
					Message:               "my lock message",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team",
				}, "")
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteEnvironmentLock(ctx, transaction, "my-lock", "production", db.LockMetadata{})
				if err != nil {
					return err
				}
				version := uint64(1)
				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &version,
					},
					App:          "test",
					Environments: []types.EnvName{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[types.EnvName]string{
						"production": "manifest",
					}},
				})
				if err != nil {
					return err
				}

				return dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, db.Deployment{
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &version,
					},
					App:           "test",
					Env:           "production",
					TransformerID: 2,
				})
			},
		},
	}
	for _, tc := range tcs {
		//tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			r, dbHandler, _ := SetupRepositoryTestWithDB(t)
			repo := r.(*repository)

			ctx := testutilauth.MakeTestContext()

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := tc.databasePopulation(ctx, transaction, dbHandler)
				if err != nil {
					t.Fatalf("Unexpected error populating the database. Error: %v", err)
				}
				for _, transformer := range tc.setup {
					_, applyErr := repo.ApplyTransformer(ctx, transaction, transformer)
					if applyErr != nil && applyErr.TransformerError != nil {
						t.Fatalf("Unexpected error applying transformers: Error: %v", applyErr)
					}
				}
				return nil
			})
			repo.config.MinimizeExportedData = true

			initialCommitId, err := repo.GetHeadCommitId()
			if err != nil {
				t.Fatalf("Could not retrieve commit: Error: %v", err)
			}
			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, transformer := range tc.targetTransformers {
					_, applyErr := repo.ApplyTransformer(ctx, transaction, transformer)
					if applyErr != nil && applyErr.TransformerError != nil {
						t.Fatalf("Unexpected error applying transformers: Error: %v", applyErr)
					}
				}
				return nil
			})
			finalCommitId, err := repo.GetHeadCommitId()
			if err != nil {
				t.Fatalf("Could not retrieve commit: Error: %v", err)
			}
			if initialCommitId.String() == finalCommitId.String() == tc.shouldCreateCommit {
				t.Fatalf("commit check failed. commits same: %v want: %v", initialCommitId.String() == finalCommitId.String(), tc.shouldCreateCommit)
			}
		})
	}
}

func getTransformer(i int) (Transformer, error) {
	// transformerType := i % 5
	// switch transformerType {
	// case 0:
	// case 1:
	// case 2:
	return &CreateApplicationVersion{
		Application: "foo",
		Manifests: map[types.EnvName]string{
			"development": fmt.Sprintf("%d", i),
		},
		Version:               uint64(i),
		WriteCommitData:       true,
		TransformerEslVersion: db.TransformerID(i),
		TransformerMetadata: TransformerMetadata{
			AuthorName:  "test-author",
			AuthorEmail: "test@example.com",
		},
	}, nil
	// case 3:
	// 	return &ErrorTransformer{}, TransformerError
	// case 4:
	// 	return &InvalidJsonTransformer{}, InvalidJson
	// }
	// return &ErrorTransformer{}, TransformerError
}

type TestStruct struct {
	Version  uint64
	Revision uint64
}

func convertToSet(list []types.ReleaseNumbers) map[TestStruct]bool {
	set := make(map[TestStruct]bool)
	for _, i := range list {
		set[TestStruct{Version: *i.Version, Revision: i.Revision}] = true
	}
	return set
}

func setupRepositoryBenchmarkWithPath(t *testing.B) (Repository, string) {
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}

	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, false, fmt.Sprintf("%s_%d", t.Name(), t.N))
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}

	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	err = cmd.Start()
	if err != nil {
		t.Errorf("could not start git init")
		return nil, ""
	}
	err = cmd.Wait()
	if err != nil {
		t.Errorf("could not wait for git init to finish")
		return nil, ""
	}

	repoCfg := RepositoryConfig{
		URL:                  remoteDir,
		Path:                 localDir,
		CommitterEmail:       "kuberpult@freiheit.com",
		CommitterName:        "kuberpult",
		ArgoCdGenerateFiles:  true,
		ReleaseVersionLimit:  2,
		MinimizeExportedData: false,
	}

	if dbConfig != nil {

		migErr := db.RunDBMigrations(ctx, *dbConfig)
		if migErr != nil {
			t.Fatal(migErr)
		}

		dbHandler, err := db.Connect(ctx, *dbConfig)
		if err != nil {
			t.Fatal(err)
		}
		repoCfg.DBHandler = dbHandler
	}

	repo, err := New(
		testutilauth.MakeTestContext(),
		repoCfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, remoteDir
}

func BenchmarkApplyQueue(t *testing.B) {
	t.StopTimer()
	repo, _ := setupRepositoryBenchmarkWithPath(t)
	ctx := testutilauth.MakeTestContext()
	generator := testutil.NewIncrementalUUIDGenerator()
	dbHandler := repo.State().DBHandler

	repoInternal := repo.(*repository)
	// The worker go routine is now blocked. We can move some items into the queue now.
	results := make([]error, t.N)
	expectedResults := make([]error, t.N)
	expectedReleases := make(map[TestStruct]bool, t.N)
	tf, _ := getTransformer(0)

	err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
		if err != nil {
			return err
		}
		err = dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "foo", db.AppStateChangeCreate, db.DBAppMetaData{
			Team: "team-123",
		}, "")
		if err != nil {
			return err
		}
		err = dbHandler.DBWriteEslEventInternal(ctx, tf.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tf.GetMetadata().AuthorName, AuthorEmail: tf.GetMetadata().AuthorEmail})
		if err != nil {
			return err
		}
		err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
			ReleaseNumbers: types.ReleaseNumbers{
				Revision: 0,
				Version:  &versionZero,
			},
			Created:   time.Time{},
			App:       "foo",
			Manifests: db.DBReleaseManifests{},
			Metadata:  db.DBReleaseMetaData{},
		})
		if err != nil {
			return err
		}
		err = dbHandler.DBWriteNewReleaseEvent(ctx, transaction, 0, types.ReleaseNumbers{Version: &versionZero, Revision: 0}, generator.Generate(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.NewRelease{})
		if err != nil {
			return err
		}
		err = repoInternal.Apply(ctx, transaction, tf)
		if err != nil {
			return err
		}
		expectedReleases[TestStruct{Version: 0, Revision: 0}] = true

		t.StartTimer()
		for i := 1; i < t.N; i++ {
			tf, expectedResult := getTransformer(i)
			err = dbHandler.DBWriteEslEventInternal(ctx, tf.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tf.GetMetadata().AuthorName, AuthorEmail: tf.GetMetadata().AuthorEmail})
			if err != nil {
				return err
			}
			version := uint64(i)
			err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
				ReleaseNumbers: types.ReleaseNumbers{
					Revision: 0,
					Version:  &version,
				},
				Created:   time.Time{},
				App:       "foo",
				Manifests: db.DBReleaseManifests{},
				Metadata:  db.DBReleaseMetaData{},
			})
			if err != nil {
				return err
			}
			var v = uint64(i)
			err = dbHandler.DBWriteNewReleaseEvent(ctx, transaction, db.TransformerID(i), types.ReleaseNumbers{Version: &v, Revision: 0}, generator.Generate(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.NewRelease{})
			if err != nil {
				return err
			}
			results[i] = repoInternal.Apply(ctx, transaction, tf)
			expectedResults[i] = expectedResult
			if expectedResult == nil {
				expectedReleases[TestStruct{Version: uint64(i), Revision: 0}] = true
			}
		}

		return nil
	})
	if err != nil {
		t.Errorf("Error applying transformers: %v", err)
	}

	for i := 0; i < t.N; i++ {
		if diff := cmp.Diff(expectedResults[i], results[i], cmpopts.EquateErrors()); diff != "" {
			t.Errorf("result[%d] expected error \"%v\" but got \"%v\"", i, expectedResults[i], err)
		}
	}
	releases, _ := repo.State().GetAllApplicationReleasesFromManifest("foo")
	if !cmp.Equal(expectedReleases, convertToSet(releases)) {
		t.Fatal("Output mismatch (-want +got):\n", cmp.Diff(expectedReleases, convertToSet(releases)))
	}
}

type Gauge struct {
	Name  string
	Value float64
	Tags  []string
	Rate  float64
}

type MockClient struct {
	gauges []Gauge
	statsd.ClientInterface
}

func (c *MockClient) Gauge(name string, value float64, tags []string, rate float64) error {
	c.gauges = append(c.gauges, Gauge{
		Name:  name,
		Value: value,
		Tags:  tags,
		Rate:  rate,
	})
	return nil
}

// Verify that MockClient implements the ClientInterface.
// https://golang.org/doc/faq#guarantee_satisfies_interface
var _ statsd.ClientInterface = &MockClient{}

func TestMeasureGitSyncStatus(t *testing.T) {
	tcs := []struct {
		Name             string
		SyncedFailedApps []db.EnvApp
		UnsyncedApps     []db.EnvApp
		ExpectedGauges   []Gauge
	}{
		{
			Name:             "No unsynced or sync failed apps",
			SyncedFailedApps: []db.EnvApp{},
			UnsyncedApps:     []db.EnvApp{},
			ExpectedGauges: []Gauge{
				{Name: "git_sync_unsynced", Value: 0, Tags: []string{}, Rate: 1},
				{Name: "git_sync_failed", Value: 0, Tags: []string{}, Rate: 1},
			},
		},
		{
			Name: "Some sync failed apps",
			SyncedFailedApps: []db.EnvApp{
				{EnvName: "dev", AppName: "app"},
				{EnvName: "dev", AppName: "app2"},
			},
			UnsyncedApps: []db.EnvApp{
				{EnvName: "staging", AppName: "app"},
			},
			ExpectedGauges: []Gauge{
				{Name: "git_sync_unsynced", Value: 1, Tags: []string{}, Rate: 1},
				{Name: "git_sync_failed", Value: 2, Tags: []string{}, Rate: 1},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			var mockClient = &MockClient{}
			var client statsd.ClientInterface = mockClient

			ctx := testutilauth.MakeTestContext()
			repo, _, _ := SetupRepositoryTestWithDB(t)
			repo.(*repository).ddMetrics = client
			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteNewSyncEventBulk(ctx, transaction, 0, tc.SyncedFailedApps, db.SYNC_FAILED)
				if err != nil {
					return err
				}

				err = dbHandler.DBWriteNewSyncEventBulk(ctx, transaction, 0, tc.UnsyncedApps, db.UNSYNCED)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				t.Fatalf("failed to write sync events to db: %v", err)
			}

			err = MeasureGitSyncStatus(ctx, client, dbHandler)
			if err != nil {
				t.Fatalf("failed to send git sync status metrics: %v", err)
			}

			cmpGauge := func(i, j Gauge) bool {
				if len(i.Tags) == 0 && len(j.Tags) == 0 {
					return i.Name > j.Name
				} else if len(i.Tags) != len(j.Tags) {
					return len(i.Tags) > len(j.Tags)
				} else {
					for tagIndex := range i.Tags {
						if i.Tags[tagIndex] != j.Tags[tagIndex] {
							return i.Tags[tagIndex] > j.Tags[tagIndex]
						}
					}
					return true
				}
			}
			if diff := cmp.Diff(tc.ExpectedGauges, mockClient.gauges, cmpopts.SortSlices(cmpGauge)); diff != "" {
				t.Errorf("gauges mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestTransformerResultEnvironments(t *testing.T) {
	// basic idea here is to make sure that the "EnvironmentsToRender" field in the TransformerResult is correct.
	setup := []Transformer{
		&CreateEnvironment{
			Environment: "dev",
			Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: false}, ArgoCd: &config.EnvironmentConfigArgoCd{
				Destination: config.ArgoCdDestination{
					Server: "dev",
				},
			}},
			TransformerMetadata: metadata(),
		},
		&CreateEnvironment{
			Environment: "prod",
			Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "dev"}, ArgoCd: &config.EnvironmentConfigArgoCd{
				Destination: config.ArgoCdDestination{
					Server: "prod",
				},
			}},
			TransformerMetadata: metadata(),
		},
		&CreateApplicationVersion{
			Application: "app1",
			Version:     1,
			Revision:    1,
			Manifests: map[types.EnvName]string{
				"dev":  "dev-manifest",
				"prod": "prod-manifest",
			},
			TransformerMetadata: metadata(),
		},
	}

	tcs := []struct {
		Name                          string
		GivenTransformers             []Transformer
		ExpectedLastTransformerResult *TransformerResult
	}{
		{
			Name: "CreateEnvironment: New environment leads to result with env",
			GivenTransformers: []Transformer{
				&CreateEnvironment{
					Environment: "prod2",
					Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: "dev"}, ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Server: "prod2",
						},
					}},
					TransformerMetadata: metadata(),
				},
			},
			ExpectedLastTransformerResult: &TransformerResult{
				AppEnvsToRender: nil,
				EnvironmentsToRender: []EnvironmentToRender{
					{
						Env: "prod2",
					},
				},
			},
		},
		{
			Name: "Deploying an existing release of an app leads to result without environment change",
			GivenTransformers: []Transformer{
				&DeployApplicationVersion{
					Environment:         "prod2",
					Application:         "app1",
					Version:             1,
					Revision:            1,
					TransformerMetadata: metadata(),
				},
			},
			ExpectedLastTransformerResult: &TransformerResult{
				AppEnvsToRender: []AppEnvToRender{
					{
						App: "app1",
						Env: "prod2",
					},
				},
				EnvironmentsToRender: nil,
			},
		},
		{
			Name: "Creating a release of an app leads to no deployment, and no environment change",
			GivenTransformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app2",
					Version:     1,
					Revision:    1,
					Manifests: map[types.EnvName]string{
						"dev":  "dev-manifest",
						"prod": "prod-manifest",
					},
					TransformerMetadata: metadata(),
				},
			},
			ExpectedLastTransformerResult: &TransformerResult{
				AppEnvsToRender:      nil,
				EnvironmentsToRender: nil,
			},
		},
		{
			Name: "Creating a deployment leads to an app/env change, and no environment change",
			GivenTransformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app2",
					Version:     1,
					Revision:    1,
					Manifests: map[types.EnvName]string{
						"dev":  "dev-manifest",
						"prod": "prod-manifest",
					},
					TransformerMetadata: metadata(),
				},
				&DeployApplicationVersion{
					Application:         "app2",
					Environment:         "dev",
					Version:             1,
					Revision:            1,
					TransformerMetadata: metadata(),
				},
			},
			ExpectedLastTransformerResult: &TransformerResult{
				AppEnvsToRender: []AppEnvToRender{
					{
						App: "app2",
						Env: "dev",
					},
				},
				EnvironmentsToRender: nil,
			},
		},
		{
			Name: "Creating a lock leads to no change",
			GivenTransformers: []Transformer{
				&CreateEnvironmentLock{
					Environment:         "dev",
					TransformerMetadata: metadata(),
				},
			},
			ExpectedLastTransformerResult: &TransformerResult{
				AppEnvsToRender:      nil,
				EnvironmentsToRender: nil,
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			r, dbHandler, _ := SetupRepositoryTestWithDB(t)
			repo := r.(*repository)
			repo.config.MinimizeExportedData = true

			ctx := testutilauth.MakeTestContext()

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, transformer := range setup {
					prepareDatabaseLikeCdService(ctx, transaction, transformer, dbHandler, t, "author", "email")
					_, applyErr := repo.ApplyTransformer(ctx, transaction, transformer)
					if applyErr != nil && applyErr.TransformerError != nil {
						t.Fatalf("Unexpected error applying transformers: Error: %v", applyErr)
					}
				}
				return nil
			})

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for i, transformer := range tc.GivenTransformers {
					prepareDatabaseLikeCdService(ctx, transaction, transformer, dbHandler, t, "author", "email")
					_, _, actualResult, applyErr := repo.ApplyTransformersInternal(ctx, transaction, transformer)
					if applyErr != nil && applyErr.TransformerError != nil {
						t.Fatalf("Unexpected error applying transformers: Error: %v", applyErr)
					}
					isLast := i == len(tc.GivenTransformers)-1
					if isLast {
						// we only test the last transformer result:
						if diff := testutil.CmpDiff(tc.ExpectedLastTransformerResult, actualResult); diff != "" {
							t.Errorf("expected from transformer (%s) args:\n  %v\ngot:\n  %v\ndiff:\n  %s\n",
								transformer.GetDBEventType(), actualResult, tc.ExpectedLastTransformerResult, diff)
						}
					}
				}
				return nil
			})
		})
	}
}

func metadata() TransformerMetadata {
	return TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"}
}

func TestCombineTransformerResult(t *testing.T) {
	tcs := []struct {
		Name                          string
		GivenTransformerResultA       *TransformerResult
		GivenTransformerResultB       *TransformerResult
		ExpectedLastTransformerResult *TransformerResult
	}{
		{
			Name: "nil + nil = nil",
			GivenTransformerResultA: &TransformerResult{
				AppEnvsToRender:      nil,
				EnvironmentsToRender: nil,
				Commits:              nil,
			},
			GivenTransformerResultB: &TransformerResult{
				AppEnvsToRender:      nil,
				EnvironmentsToRender: nil,
				Commits:              nil,
			},
			ExpectedLastTransformerResult: &TransformerResult{
				AppEnvsToRender:      nil,
				EnvironmentsToRender: nil,
				Commits:              nil,
			},
		},
		{
			Name: "A + nil = A",
			GivenTransformerResultA: &TransformerResult{
				AppEnvsToRender: []AppEnvToRender{
					{
						App: "app1",
						Env: "dev",
					},
				},
				EnvironmentsToRender: []EnvironmentToRender{
					{
						Env: "prod",
					},
				},
				Commits: nil,
			},
			GivenTransformerResultB: &TransformerResult{
				AppEnvsToRender:      nil,
				EnvironmentsToRender: nil,
				Commits:              nil,
			},
			ExpectedLastTransformerResult: &TransformerResult{
				AppEnvsToRender: []AppEnvToRender{
					{
						App: "app1",
						Env: "dev",
					},
				},
				EnvironmentsToRender: []EnvironmentToRender{
					{
						Env: "prod",
					},
				},
				Commits: nil,
			},
		},
		{
			Name: "A + B = C",
			GivenTransformerResultA: &TransformerResult{
				AppEnvsToRender: []AppEnvToRender{
					{
						App: "app1",
						Env: "dev",
					},
				},
				EnvironmentsToRender: []EnvironmentToRender{
					{
						Env: "prod",
					},
				},
				Commits: nil,
			},
			GivenTransformerResultB: &TransformerResult{
				AppEnvsToRender: []AppEnvToRender{
					{
						App: "app2",
						Env: "dev",
					},
				},
				EnvironmentsToRender: []EnvironmentToRender{
					{
						Env: "testing",
					},
				},
				Commits: nil,
			},
			ExpectedLastTransformerResult: &TransformerResult{
				AppEnvsToRender: []AppEnvToRender{
					{
						App: "app1",
						Env: "dev",
					},
					{
						App: "app2",
						Env: "dev",
					},
				},
				EnvironmentsToRender: []EnvironmentToRender{
					{
						Env: "prod",
					},
					{
						Env: "testing",
					},
				},
				Commits: nil,
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			tmp := *tc.GivenTransformerResultA
			tmp.Combine(tc.GivenTransformerResultB)
			actualResult := &tmp
			if diff := testutil.CmpDiff(tc.ExpectedLastTransformerResult, actualResult); diff != "" {
				t.Errorf("got:\n  %v\ndiff:\n  %s\n", actualResult, diff)
			}
		})
	}
}
