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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/testutilauth"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/argocd"
)

const (
	envDev         = "dev"
	envAcceptance  = "acceptance"
	envAcceptance2 = "acceptance2"
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

func setupRepositoryTestWithPath(t *testing.T, argoRenderOpts ...func(*argocd.RenderOptions)) (Repository, string) {
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, t.Name())
	if err != nil {
		t.Fatalf("SetupPostgres: %v", err)
	}

	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	t.Logf("localdir: %s", localDir)
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
		URL:                 remoteDir,
		Path:                localDir,
		CommitterEmail:      "kuberpult@freiheit.com",
		CommitterName:       "kuberpult",
		ArgoCdGenerateFiles: true,
		ReleaseVersionLimit: 2,
		ArgoRenderOptions:   testRenderOptions(),
	}

	for _, mod := range argoRenderOpts {
		mod(repoCfg.ArgoRenderOptions)
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

type FilenameAndData struct {
	path     string
	fileData []byte
}

func verifyContent(fs billy.Filesystem, required []*FilenameAndData) error {
	for _, contentRequirement := range required {
		if data, err := util.ReadFile(fs, contentRequirement.path); err != nil {
			return fmt.Errorf("error while opening file %s, error: %w", contentRequirement.path, err)
		} else if diff := cmp.Diff(string(data), string(contentRequirement.fileData)); diff != "" {
			return fmt.Errorf("actual file content of file '%s' is not equal to required content.\nDiff: %s\nActual: %s\n", contentRequirement.path, diff, string(data))
		}
	}
	return nil
}

func verifyMissing(fs billy.Filesystem, required []*FilenameAndData) error {
	for _, contentRequirement := range required {
		if _, err := fs.Stat(contentRequirement.path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("error on Stat for file %s: %v", contentRequirement.path, err)
		}
		return fmt.Errorf("file exists '%s'", contentRequirement.path) //nolint:staticcheck
	}
	return nil
}

func listFiles(fs billy.Filesystem) []string {
	paths := listFilesHelper(fs, ".")
	sort.Slice(paths, func(i, j int) bool { return paths[i] < paths[j] })
	return paths
}

func listFilesHelper(fs billy.Filesystem, path string) []string {
	ret := make([]string, 0)

	files, err := fs.ReadDir(path)
	if err == nil {
		for _, file := range files {
			ret = append(ret, listFilesHelper(fs, fs.Join(path, file.Name()))...)
		}
	} else {
		ret = append(ret, path)
	}

	return ret
}

func TestUndeployLogic(t *testing.T) {
	const appName = "app1"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	environmentConfigAcceptance := config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}}
	environmentConfigAcceptance2 := config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance2, Latest: true}}
	tcs := []struct {
		Name            string
		Transformers    []Transformer
		expectedData    []*FilenameAndData
		expectedMissing []*FilenameAndData
		expectedMessage string
		expectedError   error
	}{
		{
			Name: "Undeploy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      environmentConfigAcceptance,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateEnvironment{
					Environment: envAcceptance2,
					Config:      environmentConfigAcceptance2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[types.EnvName]string{
						envAcceptance:  "mani-1-acc",
						envAcceptance2: "e2",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				//We need to deploy manually due to discrepancy between DB and manifest on testing env, causing locks to not work as the previous version isn't the lastest
				&DeployApplicationVersion{
					Application:     appName,
					Environment:     envAcceptance,
					Version:         1,
					LockBehaviour:   2,
					WriteCommitData: false,
					SourceTrain:     nil,
					Author:          "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 4,
				},
				&DeployApplicationVersion{
					Application:     appName,
					Environment:     envAcceptance2,
					Version:         1,
					LockBehaviour:   api.LockBehavior_RECORD,
					WriteCommitData: false,
					SourceTrain:     nil,
					Author:          "",

					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 5,
				},
				&CreateUndeployApplicationVersion{
					Authentication:  Authentication{},
					Application:     appName,
					WriteCommitData: false,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},

					TransformerEslVersion: 6,
				},
				&UndeployApplication{
					Application:           appName,
					TransformerEslVersion: 7,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedMissing: []*FilenameAndData{
				{
					path:     "environments/acceptance2/applications/app1/version/undeploy",
					fileData: []byte(""),
				},
				{
					path:     "/applications/app1/releases/2.0/undeploy",
					fileData: []byte(""),
				},
				{
					path:     "environments/acceptance/applications/app1/version/undeploy",
					fileData: []byte(""),
				},
				{
					path:     "environments/acceptance2/applications/app1/queued_version/undeploy",
					fileData: []byte(""),
				},
			},
		},
		{
			Name: "Try to undeploy application, with no undeploy versions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      environmentConfigAcceptance,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateEnvironment{
					Environment: envAcceptance2,
					Config:      environmentConfigAcceptance2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[types.EnvName]string{
						envAcceptance:  "mani-1-acc",
						envAcceptance2: "e2",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				&CreateEnvironmentLock{
					Environment:           envAcceptance2,
					LockId:                "my-lock",
					Message:               "Acceptance 2 is locked",
					TransformerEslVersion: 4,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&UndeployApplication{
					Application:           appName,
					TransformerEslVersion: 6,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedError: nil,
		},
		{
			Name: "Try to undeploy application, but no all envs with undeploy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      environmentConfigAcceptance,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateEnvironment{
					Environment: envAcceptance2,
					Config:      environmentConfigAcceptance2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[types.EnvName]string{
						envAcceptance:  "mani-1-acc",
						envAcceptance2: "e2",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				&CreateEnvironmentLock{
					Environment:           envAcceptance2,
					LockId:                "my-lock",
					Message:               "Acceptance 2 is locked",
					TransformerEslVersion: 4,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateUndeployApplicationVersion{
					Authentication:  Authentication{},
					Application:     appName,
					WriteCommitData: false,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},

					TransformerEslVersion: 5,
				},
				&UndeployApplication{
					Application:           appName,
					TransformerEslVersion: 6,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedError: nil,
			expectedData:  []*FilenameAndData{},
			expectedMissing: []*FilenameAndData{
				{ //The first env has the undeploy version deployed
					path:     "environments/acceptance/applications/app1/version/undeploy",
					fileData: []byte(""),
				},
				{ //The second env does NOT have the undeploy version
					path:     "environments/acceptance2/applications/app1/version/undeploy",
					fileData: []byte(""),
				},
				{ //There is no undeploy version, because all releases have been deleted
					path:     "/applications/app1/releases/2.0/undeploy",
					fileData: []byte(""),
				},
				{ //The second env has the undeploy version *queued*
					path:     "environments/acceptance2/applications/app1/queued_version",
					fileData: []byte(""),
				},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := AddGeneratorToContext(testutilauth.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransactionR(ctx, 0, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err2 := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err2 != nil {
					t.Fatal(err2)
				}
				err2 = dbHandler.DBWriteEnvironment(ctx, transaction, envAcceptance, environmentConfigAcceptance)
				if err2 != nil {
					return err2
				}
				err2 = dbHandler.DBWriteEnvironment(ctx, transaction, envAcceptance2, environmentConfigAcceptance2)
				if err2 != nil {
					return err2
				}
				//populate the database
				for _, tr := range tc.Transformers {
					err2 := dbHandler.DBWriteEslEventInternal(ctx, tr.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tr.GetMetadata().AuthorName, AuthorEmail: tr.GetMetadata().AuthorEmail})
					if err2 != nil {
						t.Fatal(err2)
					}
					if tr.GetDBEventType() == db.EvtCreateApplicationVersion {
						concreteTransformer := tr.(*CreateApplicationVersion)
						err2 = dbHandler.DBInsertOrUpdateApplication(ctx, transaction, types.AppName(concreteTransformer.Application), db.AppStateChangeCreate, db.DBAppMetaData{Team: concreteTransformer.Team}, types.ArgoBracketName(concreteTransformer.Application))
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
							ReleaseNumbers: types.ReleaseNumbers{
								Version:  &concreteTransformer.Version,
								Revision: 0,
							},
							App: types.AppName(concreteTransformer.Application),
							Manifests: db.DBReleaseManifests{
								Manifests: concreteTransformer.Manifests,
							},
						})
						if err2 != nil {
							t.Fatal(err2)
						}
					}

					if tr.GetDBEventType() == db.EvtCreateEnvironmentLock {
						concreteTransformer := tr.(*CreateEnvironmentLock)
						err2 = dbHandler.DBWriteEnvironmentLock(ctx, transaction, concreteTransformer.LockId, types.EnvName(concreteTransformer.Environment), db.LockMetadata{
							CreatedByName:  concreteTransformer.AuthorName,
							CreatedByEmail: concreteTransformer.AuthorEmail,
							Message:        concreteTransformer.Message,
							CiLink:         "", //not transported to repo
						})
						if err2 != nil {
							t.Fatal(err2)
						}
					}
					var version uint64 = 2
					if tr.GetDBEventType() == db.EvtCreateUndeployApplicationVersion {
						concreteTransformer := tr.(*CreateUndeployApplicationVersion)
						err2 = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
							ReleaseNumbers: types.ReleaseNumbers{
								Version:  &version,
								Revision: 0,
							},
							App: types.AppName(concreteTransformer.Application),
							Manifests: db.DBReleaseManifests{
								Manifests: map[types.EnvName]string{ //empty manifest
									"": "",
								},
							},
							Metadata: db.DBReleaseMetaData{
								SourceAuthor:    "",
								SourceCommitId:  "",
								SourceMessage:   "",
								DisplayVersion:  "",
								UndeployVersion: true,
							},
							Created: time.Now(),
						})
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, db.Deployment{
							ReleaseNumbers: types.ReleaseNumbers{
								Version:  &version,
								Revision: 0,
							},
							App:           appName,
							Env:           envAcceptance,
							Metadata:      db.DeploymentMetadata{},
							Created:       time.Now(),
							TransformerID: tr.GetEslVersion(),
						})
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, db.Deployment{
							ReleaseNumbers: types.ReleaseNumbers{
								Version:  &version,
								Revision: 0,
							},
							App:           appName,
							Env:           envAcceptance,
							Metadata:      db.DeploymentMetadata{},
							Created:       time.Now(),
							TransformerID: tr.GetEslVersion(),
						})
						if err2 != nil {
							t.Fatal(err2)
						}
					}
				}
				var commitMsg []string
				for _, t := range tc.Transformers {
					err := repo.Apply(ctx, transaction, t)
					if err != nil {
						return err
					}
					// just for testing, we push each transformer change separately.
					// if you need to debug this test, you can git clone the repo
					// and we will only see anything if we push.
					err = repo.PushRepo(ctx)
					if err != nil {
						return err
					}
				}
				actualMsg := ""
				// note that we only check the LAST error here:
				if len(commitMsg) > 0 {
					actualMsg = commitMsg[len(commitMsg)-1]
				}
				if diff := cmp.Diff(tc.expectedMessage, actualMsg); diff != "" {
					t.Errorf("commit message mismatch (-want, +got):\n%s", diff)
				}

				return nil
			})

			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			updatedState := repo.State()

			if err := verifyContent(updatedState.Filesystem, tc.expectedData); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			if err := verifyMissing(updatedState.Filesystem, tc.expectedMissing); err != nil {
				t.Fatalf("Error while verifying missing content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
		})
	}
}

func TestDeleteEnvironment(t *testing.T) {
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	envAcceptanceConfig := config.EnvironmentConfig{
		Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true},
		ArgoCd:   &config.EnvironmentConfigArgoCd{},
	}
	envAcceptance2Config := config.EnvironmentConfig{
		Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance2, Latest: true},
		ArgoCd:   &config.EnvironmentConfigArgoCd{},
	}

	tcs := []struct {
		Name            string
		Transformers    []Transformer
		expectedData    []*FilenameAndData
		expectedMissing []*FilenameAndData
		expectedMessage string
		expectedError   error
	}{
		{
			Name: "create an environment and delete it",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      envAcceptanceConfig,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&DeleteEnvironment{
					Environment: envAcceptance,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
			},
			expectedMissing: []*FilenameAndData{
				{
					path:     "/environments/acceptance",
					fileData: []byte(authorEmail),
				},
				{
					path:     "/argocd/v1alpha1/acceptance.yaml",
					fileData: []byte(authorEmail),
				},
			},
			expectedMessage: "delete environment \"acceptance\"",
		},
		{
			Name: "create two environments and delete one of them",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      envAcceptanceConfig,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateEnvironment{
					Environment: envAcceptance2,
					Config:      envAcceptance2Config,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&DeleteEnvironment{
					Environment: envAcceptance,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
			},
			expectedMissing: []*FilenameAndData{
				{
					path:     "/environments/acceptance",
					fileData: []byte(authorEmail),
				},
				{
					path:     "/argocd/v1alpha1/acceptance.yaml",
					fileData: []byte(authorEmail),
				},
			},
			expectedData: []*FilenameAndData{
				{
					path: "/environments/acceptance2/config.json",
					fileData: []byte(`{
  "upstream": {
    "environment": "acceptance2",
    "latest": true
  },
  "argocd": {
    "destination": {
      "name": "",
      "server": ""
    }
  }
}
`),
				},
				{
					path: "/argocd/v1alpha1/acceptance2.yaml",
					fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: acceptance2
spec:
  description: acceptance2
  destinations:
  - {}
  sourceRepos:
  - '*'
`),
				},
			},
			expectedMessage: "delete environment \"acceptance\"",
		},
		{
			Name: "delete an environment that does not exist",
			Transformers: []Transformer{
				&DeleteEnvironment{
					Environment: envAcceptance,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
			},
			expectedMessage: "delete environment \"acceptance\"",
		},
	}

	for _, tc := range tcs {

		t.Run(tc.Name, func(t *testing.T) {
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := AddGeneratorToContext(testutilauth.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler

			err := dbHandler.WithTransactionR(ctx, 0, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, tr := range tc.Transformers {
					prepareDatabaseLikeCdService(ctx, transaction, tr, dbHandler, t, authorName, authorEmail)
				}
				for _, t := range tc.Transformers {
					err := repo.Apply(ctx, transaction, t)
					if err != nil {
						return err
					}
					// just for testing, we push each transformer change separately.
					// if you need to debug this test, you can git clone the repo
					// and we will only see anything if we push.
					err = repo.PushRepo(ctx)
					if err != nil {
						return err
					}
				}

				actualMsg := repo.State().Commit.Message()
				if diff := cmp.Diff(tc.expectedMessage, actualMsg); diff != "" {
					t.Errorf("commit message mismatch (-want, +got):\n%s", diff)
				}

				return nil
			})

			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			updatedState := repo.State()

			if err := verifyContent(updatedState.Filesystem, tc.expectedData); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			if err := verifyMissing(updatedState.Filesystem, tc.expectedMissing); err != nil {
				t.Fatalf("Error while verifying missing content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
		})
	}
}

func TestRerenderEnvironment(t *testing.T) {
	const appName = "myapp"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	const brokenManifest = "this file is broken now"
	tcs := []struct {
		Name                 string
		SetupTransformers    []Transformer
		RenderEnvTransformer Transformer
		ExpectedFiles        []*FilenameAndData
		UnexpectedFiles      []*FilenameAndData
		ExpectedError        error
		ArgoRenderOptions    func(*argocd.RenderOptions)
	}{
		{
			Name: "should re-render the ArgoCD root app",
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = true
				opts.RenderBrackets = false
				opts.PointToBrackets = false
			},
			SetupTransformers: []Transformer{
				&CreateEnvironment{
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
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			RenderEnvTransformer: &RenderEnvironment{
				Environment:           "development",
				TransformerEslVersion: 4,
				TransformerMetadata: TransformerMetadata{
					AuthorName:  authorName,
					AuthorEmail: authorEmail,
				},
			},
			ExpectedFiles: []*FilenameAndData{
				{
					path: "argocd/v1alpha1/development.yaml",
					fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: development
spec:
  description: development
  destinations:
  - server: development
  sourceRepos:
  - '*'
`),
				},
			},
			UnexpectedFiles: []*FilenameAndData{},
		},
		{
			Name: "should re-render the application manifests only",
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = true
				opts.RenderBrackets = false
				opts.PointToBrackets = false
			},
			SetupTransformers: []Transformer{
				&CreateEnvironment{
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
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"development": "normal manifest",
					},
					WriteCommitData:       false,
					Version:               1,
					Revision:              0,
					TransformerEslVersion: 2,
					Team:                  "myteam",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeployApplicationVersion{
					Authentication:        Authentication{},
					Environment:           "development",
					Application:           appName,
					Version:               1,
					Revision:              0,
					LockBehaviour:         1,
					WriteCommitData:       false,
					SourceTrain:           nil,
					Author:                "",
					TransformerEslVersion: 3,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			RenderEnvTransformer: &RenderEnvironment{
				Environment:           "development",
				TransformerEslVersion: 4,
				TransformerMetadata: TransformerMetadata{
					AuthorName:  authorName,
					AuthorEmail: authorEmail,
				},
			},
			ExpectedFiles: []*FilenameAndData{
				{
					path:     "environments/development/applications/myapp/manifests/manifests.yaml",
					fileData: []byte("normal manifest"),
				},
			},
			UnexpectedFiles: []*FilenameAndData{
				{
					path:     "environments/development/brackets/myapp/myapp.yaml",
					fileData: []byte(""),
				},
			},
		},
		{
			Name: "should re-render bracket manifests only",
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = false
				opts.RenderBrackets = true
				opts.PointToBrackets = true
			},
			SetupTransformers: []Transformer{
				&CreateEnvironment{
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
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"development": "normal manifest",
					},
					WriteCommitData:       false,
					Version:               1,
					Revision:              0,
					TransformerEslVersion: 2,
					Team:                  "myteam",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeployApplicationVersion{
					Authentication:        Authentication{},
					Environment:           "development",
					Application:           appName,
					Version:               1,
					Revision:              0,
					LockBehaviour:         1,
					WriteCommitData:       false,
					SourceTrain:           nil,
					Author:                "",
					TransformerEslVersion: 3,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			RenderEnvTransformer: &RenderEnvironment{
				Environment:           "development",
				TransformerEslVersion: 4,
				TransformerMetadata: TransformerMetadata{
					AuthorName:  authorName,
					AuthorEmail: authorEmail,
				},
			},
			ExpectedFiles: []*FilenameAndData{
				{
					path:     "environments/development/brackets/myapp/myapp.yaml",
					fileData: []byte("normal manifest"),
				},
			},
			UnexpectedFiles: []*FilenameAndData{
				{
					path:     "environments/development/applications/myapp/manifests/manifests.yaml",
					fileData: []byte(""),
				},
			},
		},
		{
			Name: "should re-render both application and bracket manifests",
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = true
				opts.RenderBrackets = true
				opts.PointToBrackets = true
			},
			SetupTransformers: []Transformer{
				&CreateEnvironment{
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
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"development": "normal manifest",
					},
					WriteCommitData:       false,
					Version:               1,
					Revision:              0,
					TransformerEslVersion: 2,
					Team:                  "myteam",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeployApplicationVersion{
					Authentication:        Authentication{},
					Environment:           "development",
					Application:           appName,
					Version:               1,
					Revision:              0,
					LockBehaviour:         1,
					WriteCommitData:       false,
					SourceTrain:           nil,
					Author:                "",
					TransformerEslVersion: 3,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			RenderEnvTransformer: &RenderEnvironment{
				Environment:           "development",
				TransformerEslVersion: 4,
				TransformerMetadata: TransformerMetadata{
					AuthorName:  authorName,
					AuthorEmail: authorEmail,
				},
			},
			ExpectedFiles: []*FilenameAndData{
				{
					path:     "environments/development/brackets/myapp/myapp.yaml",
					fileData: []byte("normal manifest"),
				},
				{
					path:     "environments/development/applications/myapp/manifests/manifests.yaml",
					fileData: []byte("normal manifest"),
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t, tc.ArgoRenderOptions)

			ctx := AddGeneratorToContext(testutilauth.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())
			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return fmt.Errorf("migration error: %w", err)
				}
				for index, tr := range tc.SetupTransformers {
					err := dbHandler.DBWriteEslEventInternal(ctx, tr.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tr.GetMetadata().AuthorName, AuthorEmail: tr.GetMetadata().AuthorEmail})
					if err != nil {
						return fmt.Errorf("setup transformer[%d] failed: %w", index, err)
					}
					prepareDatabaseLikeCdService(ctx, transaction, tr, dbHandler, t, authorEmail, authorName)
				}

				for index, t := range tc.SetupTransformers {
					err := repo.Apply(ctx, transaction, t)
					if err != nil {
						return fmt.Errorf("apply[%d] failed: %w", index, err)
					}
					// just for testing, we push each transformer change separately.
					// if you need to debug this test, you can git clone the repo
					// and we will only see anything if we push.
					err = repo.PushRepo(ctx)
					if err != nil {
						return fmt.Errorf("push[%d] failed: %w", index, err)
					}
				}
				return nil
			})
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			updatedState := repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFiles); err != nil {
				t.Fatalf("error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}

			// Manually modify the manifests.yml in Git
			if tc.ExpectedFiles != nil {
				for i := range tc.ExpectedFiles {
					expectedFile := tc.ExpectedFiles[i]
					updatedState := repo.State()
					fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), expectedFile.path)

					if err := util.WriteFile(updatedState.Filesystem, fullPath, []byte(brokenManifest), 0666); err != nil {
						t.Fatalf("failed to write file: %v path=%s", err, fullPath)
					}

					_, _, applyError := repo.createCommit(ctx, updatedState, tc.SetupTransformers[i], []string{"broken content"})
					if applyError != nil {
						t.Fatalf("failed to create commit: %v", applyError)
					}

					actualFileData, err := util.ReadFile(updatedState.Filesystem, fullPath)
					if err != nil {
						t.Fatalf("failed to read file: %v path=%s", err, fullPath)
					}

					if !cmp.Equal(string(actualFileData), brokenManifest) {
						t.Fatalf("expected '%v', got '%v'", brokenManifest, string(actualFileData))
					}
				}
			}

			// RenderEnvironment transformer should re-render the state of manifests.yml in Git (data fetched from database)
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteEslEventInternal(ctx, tc.RenderEnvTransformer.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tc.RenderEnvTransformer.GetMetadata().AuthorName, AuthorEmail: tc.RenderEnvTransformer.GetMetadata().AuthorEmail})
				if err != nil {
					return err
				}
				prepareDatabaseLikeCdService(ctx, transaction, tc.RenderEnvTransformer, dbHandler, t, authorEmail, authorName)

				err = repo.Apply(ctx, transaction, tc.RenderEnvTransformer)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				t.Fatalf("failed to execute transaction: %v", err)
			}

			updatedState = repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFiles); err != nil {
				t.Fatalf("error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			if err := verifyMissing(updatedState.Filesystem, tc.UnexpectedFiles); err != nil {
				t.Fatalf("error while verifying missing content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
		})
	}
}

// TestRerenderEnvironmentFailures is very similar to TestRerenderEnvironment, but it handles the case when
// writing the file was interrupted (by a manifest lock).
// And it tests the before and after states of the files, which in this test are different.
func TestRerenderEnvironmentFailures(t *testing.T) {
	const appName = "myapp"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	const brokenManifest = "this file is broken now"
	tcs := []struct {
		Name                 string
		SetupTransformers    []Transformer
		RenderEnvTransformer Transformer
		ExpectedFilesBefore  []*FilenameAndData
		ExpectedFilesAfter   []*FilenameAndData
		ArgoRenderOptions    func(*argocd.RenderOptions)
	}{
		{
			Name: "should not re-render application and bracket manifests due to manifest-lock",
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = true
				opts.RenderBrackets = true
				opts.PointToBrackets = true
			},
			SetupTransformers: []Transformer{
				&CreateEnvironment{
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
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"development": "normal manifest",
					},
					WriteCommitData:       false,
					Version:               1,
					Revision:              0,
					TransformerEslVersion: 2,
					Team:                  "myteam",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeployApplicationVersion{
					Authentication:        Authentication{},
					Environment:           "development",
					Application:           appName,
					Version:               1,
					Revision:              0,
					LockBehaviour:         1,
					WriteCommitData:       false,
					SourceTrain:           nil,
					Author:                "",
					TransformerEslVersion: 3,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			RenderEnvTransformer: &RenderEnvironment{
				Environment:           "development",
				TransformerEslVersion: 4,
				TransformerMetadata: TransformerMetadata{
					AuthorName:  authorName,
					AuthorEmail: authorEmail,
				},
			},
			ExpectedFilesBefore: []*FilenameAndData{
				{
					path:     "environments/development/brackets/myapp/myapp.yaml",
					fileData: []byte("normal manifest"),
				},
				{
					path:     "environments/development/applications/myapp/manifests/manifests.yaml",
					fileData: []byte("normal manifest"),
				},
			},
			ExpectedFilesAfter: []*FilenameAndData{
				{
					path:     "environments/development/brackets/myapp/myapp.yaml",
					fileData: []byte("this file is broken now"),
				},
				{
					path:     "environments/development/applications/myapp/manifests/manifests.yaml",
					fileData: []byte("this file is broken now"),
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t, tc.ArgoRenderOptions)

			ctx := AddGeneratorToContext(testutilauth.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())
			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return fmt.Errorf("migration error: %w", err)
				}
				for index, tr := range tc.SetupTransformers {
					err := dbHandler.DBWriteEslEventInternal(ctx, tr.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tr.GetMetadata().AuthorName, AuthorEmail: tr.GetMetadata().AuthorEmail})
					if err != nil {
						return fmt.Errorf("setup transformer[%d] failed: %w", index, err)
					}
					prepareDatabaseLikeCdService(ctx, transaction, tr, dbHandler, t, authorEmail, authorName)
				}

				for index, t := range tc.SetupTransformers {
					err := repo.Apply(ctx, transaction, t)
					if err != nil {
						return fmt.Errorf("apply[%d] failed: %w", index, err)
					}
					// just for testing, we push each transformer change separately.
					// if you need to debug this test, you can git clone the repo
					// and we will only see anything if we push.
					err = repo.PushRepo(ctx)
					if err != nil {
						return fmt.Errorf("push[%d] failed: %w", index, err)
					}
				}
				return nil
			})
			if err != nil {
				t.Errorf("error: %v", err)
			}
			updatedState := repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFilesBefore); err != nil {
				t.Fatalf("error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}

			// Manually modify the manifests.yml in Git
			if tc.ExpectedFilesBefore != nil {
				for i := range tc.ExpectedFilesBefore {
					expectedFile := tc.ExpectedFilesBefore[i]
					updatedState := repo.State()
					fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), expectedFile.path)

					if err := util.WriteFile(updatedState.Filesystem, fullPath, []byte(brokenManifest), 0666); err != nil {
						t.Fatalf("failed to write file: %v path=%s", err, fullPath)
					}

					_, _, applyError := repo.createCommit(ctx, updatedState, tc.SetupTransformers[i], []string{"broken content"})
					if applyError != nil {
						t.Fatalf("failed to create commit: %v", applyError)
					}

					actualFileData, err := util.ReadFile(updatedState.Filesystem, fullPath)
					if err != nil {
						t.Fatalf("failed to read file: %v path=%s", err, fullPath)
					}

					if !cmp.Equal(string(actualFileData), brokenManifest) {
						t.Fatalf("expected '%v', got '%v'", brokenManifest, string(actualFileData))
					}
				}
			}

			// RenderEnvironment transformer should re-render the state of manifests.yml in Git (data fetched from database)
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err = dbHandler.DBWriteManifestLock(ctx, transaction, appName, "development", db.LockMetadata{})
				if err != nil {
					return fmt.Errorf("failed to write manifest lock: %w", err)
				}
				err := dbHandler.DBWriteEslEventInternal(ctx, tc.RenderEnvTransformer.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tc.RenderEnvTransformer.GetMetadata().AuthorName, AuthorEmail: tc.RenderEnvTransformer.GetMetadata().AuthorEmail})
				if err != nil {
					return err
				}
				prepareDatabaseLikeCdService(ctx, transaction, tc.RenderEnvTransformer, dbHandler, t, authorEmail, authorName)

				err = repo.Apply(ctx, transaction, tc.RenderEnvTransformer)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				t.Fatalf("failed to execute transaction: %v", err)
			}

			updatedState = repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFilesAfter); err != nil {
				t.Fatalf("error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
		})
	}
}

func TestReleasesAndDeployments(t *testing.T) {
	const appName = "myapp"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name          string
		Transformers  []Transformer
		ExpectedError error
		ExpectedFile  []*FilenameAndData
	}{
		{
			Name: "Check deployments",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "staging",
						},
					},
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "staging",
							Latest:      true,
						},
					},
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"production": "some production manifest 1.0",
						"staging":    "some staging manifest 1.0",
					},
					WriteCommitData:       false,
					Version:               1,
					Revision:              0,
					TransformerEslVersion: 3,
					Team:                  "team-123",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						"production": "some production manifest 1.1",
						"staging":    "some staging manifest 1.1",
					},
					WriteCommitData:       false,
					Version:               1,
					Revision:              1,
					TransformerEslVersion: 4,
					Team:                  "team-123",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeployApplicationVersion{
					Authentication:        Authentication{},
					Environment:           "staging",
					Application:           appName,
					Version:               1,
					Revision:              1,
					LockBehaviour:         1,
					WriteCommitData:       false,
					SourceTrain:           nil,
					Author:                "",
					TransformerEslVersion: 5,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeployApplicationVersion{
					Authentication:        Authentication{},
					Environment:           "production",
					Application:           appName,
					Version:               1,
					Revision:              0,
					LockBehaviour:         1,
					WriteCommitData:       false,
					SourceTrain:           nil,
					Author:                "",
					TransformerEslVersion: 5,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			ExpectedFile: []*FilenameAndData{
				{
					path:     "environments/production/applications/" + appName + "/manifests/manifests.yaml",
					fileData: []byte("some production manifest 1.0"),
				},
				{
					path:     "environments/staging/applications/" + appName + "/manifests/manifests.yaml",
					fileData: []byte("some staging manifest 1.1"),
				},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _, _ := SetupRepositoryTestWithDB(t)
			ctx := AddGeneratorToContext(testutilauth.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}
				for _, tr := range tc.Transformers {
					err := dbHandler.DBWriteEslEventInternal(ctx, tr.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tr.GetMetadata().AuthorName, AuthorEmail: tr.GetMetadata().AuthorEmail})
					if err != nil {
						return err
					}
					prepareDatabaseLikeCdService(ctx, transaction, tr, dbHandler, t, authorEmail, authorName)
				}

				// actual transformer to be tested:
				err = repo.Apply(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				return nil
			})
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			updatedState := repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFile); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			if tc.ExpectedFile != nil {
				for i := range tc.ExpectedFile {
					expectedFile := tc.ExpectedFile[i]
					updatedState := repo.State()
					fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), expectedFile.path)
					actualFileData, err := util.ReadFile(updatedState.Filesystem, fullPath)
					if err != nil {
						t.Fatalf("Expected no error: %v path=%s", err, fullPath)
					}

					if !cmp.Equal(actualFileData, expectedFile.fileData) {
						t.Fatalf("Expected '%v', got '%v'", string(expectedFile.fileData), string(actualFileData))
					}
				}
			}
		})
	}
}

func TestDeleteAAEnvironmentConfigTransformer(t *testing.T) {
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	var aaEnvName = "aa"
	tcs := []struct {
		Name          string
		Transformers  []Transformer
		ExpectedError error
		ExpectedFile  []*FilenameAndData
	}{
		{
			Name: "Create an AA environment with some Argo config and delete it",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production",
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
							},
						},
					},
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeleteAAEnvironmentConfig{
					Environment:             "production",
					ConcreteEnvironmentName: "some-concrete-env-name-1",
					TransformerEslVersion:   1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			ExpectedFile: []*FilenameAndData{
				{
					path: "environments/production/config.json",
					fileData: []byte(
						`{
  "argocdConfigs": {
    "ArgoCdConfigurations": [],
    "CommonEnvPrefix": "aa"
  }
}
`),
				},
			},
		},
		{
			Name: "Create an AA environment with some Argo config and delete it twice",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production",
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
							},
						},
					},
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeleteAAEnvironmentConfig{
					Environment:             "production",
					ConcreteEnvironmentName: "some-concrete-env-name-1",
					TransformerEslVersion:   1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeleteAAEnvironmentConfig{
					Environment:             "production",
					ConcreteEnvironmentName: "some-concrete-env-name-1",
					TransformerEslVersion:   1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			ExpectedFile: []*FilenameAndData{
				{
					path: "environments/production/config.json",
					fileData: []byte(
						`{
  "argocdConfigs": {
    "ArgoCdConfigurations": [],
    "CommonEnvPrefix": "aa"
  }
}
`),
				},
			},
		},
		{
			Name: "Create an environment with more than one config and delete one of them",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production",
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
				&DeleteAAEnvironmentConfig{
					Environment:             "production",
					ConcreteEnvironmentName: "some-concrete-env-name-1",
					TransformerEslVersion:   1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			ExpectedFile: []*FilenameAndData{
				{
					path: "/argocd/v1alpha1/aa-production-some-concrete-env-name-2.yaml",
					fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: aa-production-some-concrete-env-name-2
spec:
  description: aa-production-some-concrete-env-name-2
  destinations:
  - name: some-destination-2
    server: some-server
  sourceRepos:
  - '*'
`),
				},
				{
					path: "environments/production/config.json",
					fileData: []byte(
						`{
  "argocdConfigs": {
    "ArgoCdConfigurations": [
      {
        "destination": {
          "name": "some-destination-2",
          "server": "some-server"
        },
        "name": "some-concrete-env-name-2"
      }
    ],
    "CommonEnvPrefix": "aa"
  }
}
`),
				},
			},
		},
		{
			Name: "Deleting a concrete environment that does not exist has no effect",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production",
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
				&DeleteAAEnvironmentConfig{
					Environment:             "production",
					ConcreteEnvironmentName: "some-concrete-env-name-3",
					TransformerEslVersion:   1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			ExpectedFile: []*FilenameAndData{
				{
					path: "/argocd/v1alpha1/aa-production-some-concrete-env-name-1.yaml",
					fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: aa-production-some-concrete-env-name-1
spec:
  description: aa-production-some-concrete-env-name-1
  destinations:
  - name: some-destination-1
    server: some-server
  sourceRepos:
  - '*'
`),
				},
				{
					path: "/argocd/v1alpha1/aa-production-some-concrete-env-name-2.yaml",
					fileData: []byte(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: aa-production-some-concrete-env-name-2
spec:
  description: aa-production-some-concrete-env-name-2
  destinations:
  - name: some-destination-2
    server: some-server
  sourceRepos:
  - '*'
`),
				},
				{
					path: "environments/production/config.json",
					fileData: []byte(
						`{
  "argocdConfigs": {
    "ArgoCdConfigurations": [
      {
        "destination": {
          "name": "some-destination-1",
          "server": "some-server"
        },
        "name": "some-concrete-env-name-1"
      },
      {
        "destination": {
          "name": "some-destination-2",
          "server": "some-server"
        },
        "name": "some-concrete-env-name-2"
      }
    ],
    "CommonEnvPrefix": "aa"
  }
}
`),
				},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _, _ := SetupRepositoryTestWithDB(t)
			ctx := AddGeneratorToContext(testutilauth.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}
				for _, tr := range tc.Transformers {
					err := dbHandler.DBWriteEslEventInternal(ctx, tr.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tr.GetMetadata().AuthorName, AuthorEmail: tr.GetMetadata().AuthorEmail})
					if err != nil {
						return err
					}
					prepareDatabaseLikeCdService(ctx, transaction, tr, dbHandler, t, authorEmail, authorName)
				}

				// actual transformer to be tested:
				err = repo.Apply(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				return nil
			})
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			updatedState := repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFile); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			for i := range tc.ExpectedFile {
				expectedFile := tc.ExpectedFile[i]
				updatedState := repo.State()
				fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), expectedFile.path)
				actualFileData, err := util.ReadFile(updatedState.Filesystem, fullPath)
				if err != nil {
					t.Fatalf("Expected no error: %v path=%s", err, fullPath)
				}

				if !cmp.Equal(expectedFile.fileData, actualFileData) {
					t.Fatalf("Expected '%v', got '%v'", string(expectedFile.fileData), string(actualFileData))
				}
			}
		})
	}
}

func prepareDatabaseLikeCdService(ctx context.Context, transaction *sql.Tx, tr Transformer, dbHandler *db.DBHandler, t *testing.T, authorEmail string, authorName string) {
	now, err := dbHandler.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		t.Fatal(err)
	}
	tr.SetCreationTimestamp(*now)
	if tr.GetDBEventType() == db.EvtCreateEnvironmentLock {
		concreteTransformer := tr.(*CreateEnvironmentLock)
		err2 := dbHandler.DBWriteEnvironmentLock(ctx, transaction, concreteTransformer.LockId, types.EnvName(concreteTransformer.Environment), db.LockMetadata{
			CreatedByName:  concreteTransformer.AuthorName,
			CreatedByEmail: concreteTransformer.AuthorEmail,
			Message:        concreteTransformer.Message,
			CiLink:         "", //not transported to repo
		})
		if err2 != nil {
			t.Fatal(err2)
		}
	}
	if tr.GetDBEventType() == db.EvtCreateEnvironment {
		concreteTransformer := tr.(*CreateEnvironment)
		err2 := dbHandler.DBWriteEnvironment(ctx, transaction, concreteTransformer.Environment, concreteTransformer.Config)
		if err2 != nil {
			t.Fatal(err2)
		}
	}
	if tr.GetDBEventType() == db.EvtDeleteEnvironmentLock {
		concreteTransformer := tr.(*DeleteEnvironmentLock)
		err2 := dbHandler.DBDeleteEnvironmentLock(ctx, transaction, types.EnvName(concreteTransformer.Environment), concreteTransformer.LockId, db.LockDeletionMetadata{DeletedByUser: authorName, DeletedByEmail: authorEmail})
		if err2 != nil {
			t.Fatal(err2)
		}
	}
	if tr.GetDBEventType() == db.EvtDeployApplicationVersion {
		concreteTransformer := tr.(*DeployApplicationVersion)
		err2 := dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, db.Deployment{
			App:            types.AppName(concreteTransformer.Application),
			Env:            concreteTransformer.Environment,
			ReleaseNumbers: types.MakeReleaseNumbers(concreteTransformer.Version, concreteTransformer.Revision),
			Metadata: db.DeploymentMetadata{
				DeployedByEmail: authorEmail,
				DeployedByName:  authorName,
			},
			TransformerID: concreteTransformer.TransformerEslVersion,
		})
		if err2 != nil {
			t.Fatal(err2)
		}
	}
	if tr.GetDBEventType() == db.EvtCreateApplicationVersion {
		concreteTransformer := tr.(*CreateApplicationVersion)
		actualBracketName := db.ResolveBracketName(types.AppName(concreteTransformer.Application), concreteTransformer.ArgoBracket)
		if concreteTransformer.GetEslVersion() > 0 {
			bracketError := db.HandleBracketsHistoryUpdate(ctx, dbHandler, transaction, types.AppName(concreteTransformer.Application), actualBracketName, *now, concreteTransformer.GetEslVersion())
			if bracketError != nil {
				t.Fatal(bracketError)
			}
		}
		err2 := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, types.AppName(concreteTransformer.Application), db.AppStateChangeCreate, db.DBAppMetaData{Team: concreteTransformer.Team}, actualBracketName)
		if err2 != nil {
			t.Fatal(err2)
		}
		err2 = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
			ReleaseNumbers: types.ReleaseNumbers{
				Version:  &concreteTransformer.Version,
				Revision: concreteTransformer.Revision,
			},
			App: types.AppName(concreteTransformer.Application),
			Manifests: db.DBReleaseManifests{
				Manifests: concreteTransformer.Manifests,
			},
		})
		if err2 != nil {
			t.Fatal(err2)
		}
	}
	if tr.GetDBEventType() == db.EvtCreateEnvironmentApplicationLock {
		concreteTransformer := tr.(*CreateEnvironmentApplicationLock)
		err2 := dbHandler.DBWriteApplicationLock(ctx, transaction, concreteTransformer.LockId, concreteTransformer.Environment, types.AppName(concreteTransformer.Application), db.LockMetadata{
			CreatedByName:  concreteTransformer.AuthorName,
			CreatedByEmail: concreteTransformer.AuthorEmail,
			Message:        concreteTransformer.Message,
			CiLink:         "", //not transported to repo
		})
		if err2 != nil {
			t.Fatal(err2)
		}
	}
	if tr.GetDBEventType() == db.EvtDeleteEnvironmentApplicationLock {
		concreteTransformer := tr.(*DeleteEnvironmentApplicationLock)
		err2 := dbHandler.DBDeleteApplicationLock(ctx, transaction, concreteTransformer.Environment, types.AppName(concreteTransformer.Application), concreteTransformer.LockId, db.LockDeletionMetadata{
			DeletedByEmail: authorEmail,
			DeletedByUser:  authorName,
		})
		if err2 != nil {
			t.Fatal(err2)
		}
	}
	if tr.GetDBEventType() == db.EvtCreateEnvironmentTeamLock {
		concreteTransformer := tr.(*CreateEnvironmentTeamLock)

		err2 := dbHandler.DBWriteTeamLock(ctx, transaction, concreteTransformer.LockId, types.EnvName(concreteTransformer.Environment), concreteTransformer.Team, db.LockMetadata{
			CreatedByName:  concreteTransformer.AuthorName,
			CreatedByEmail: concreteTransformer.AuthorEmail,
			Message:        concreteTransformer.Message,
			CiLink:         "", //not transported to repo
		})
		if err2 != nil {
			t.Fatal(err2)
		}
	}
	if tr.GetDBEventType() == db.EvtDeleteEnvironmentTeamLock {
		concreteTransformer := tr.(*DeleteEnvironmentTeamLock)
		err2 := dbHandler.DBDeleteTeamLock(ctx,
			transaction,
			types.EnvName(concreteTransformer.Environment),
			concreteTransformer.Team,
			concreteTransformer.LockId,
			db.LockDeletionMetadata{
				DeletedByUser:  concreteTransformer.AuthorEmail,
				DeletedByEmail: concreteTransformer.AuthorEmail,
			})
		if err2 != nil {
			t.Fatal(err2)
		}
	}

}

func TestManifestLockRedeployAfterDelete(t *testing.T) {
	const appName = "myapp"
	const envName types.EnvName = "development"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name              string
		ArgoRenderOptions func(*argocd.RenderOptions)
		ExpectedFiles     []*FilenameAndData
	}{
		{
			Name: "manifests.yaml written after lock deleted and same-version redeploy",
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = true
				opts.RenderBrackets = false
			},
			ExpectedFiles: []*FilenameAndData{
				{path: "environments/development/applications/myapp/manifests/manifests.yaml", fileData: []byte("normal manifest")},
			},
		},
		{
			Name: "bracket file written after lock deleted and same-version redeploy",
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = false
				opts.RenderBrackets = true
				opts.PointToBrackets = true
			},
			ExpectedFiles: []*FilenameAndData{
				{path: "environments/development/brackets/myapp/myapp.yaml", fileData: []byte("normal manifest")},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t, tc.ArgoRenderOptions)
			ctx := AddGeneratorToContext(testutilauth.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())
			dbHandler := repo.State().DBHandler

			setupTransformers := []Transformer{
				&CreateEnvironment{
					Environment: envName,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{Latest: true},
						ArgoCd: &config.EnvironmentConfigArgoCd{
							Destination: config.ArgoCdDestination{Server: string(envName)},
						},
					},
					TransformerEslVersion: 1,
					TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
				},
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						envName: "normal manifest",
					},
					WriteCommitData:       false,
					Version:               1,
					Revision:              0,
					TransformerEslVersion: 2,
					Team:                  "myteam",
					TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
				},
			}
			deployTransformer := &DeployApplicationVersion{
				Authentication:        Authentication{},
				Environment:           envName,
				Application:           appName,
				Version:               1,
				Revision:              0,
				LockBehaviour:         1,
				WriteCommitData:       false,
				TransformerEslVersion: 3,
				TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
			}
			redeployTransformer := &DeployApplicationVersion{
				Authentication:        Authentication{},
				Environment:           envName,
				Application:           appName,
				Version:               1,
				Revision:              0,
				LockBehaviour:         1,
				WriteCommitData:       false,
				TransformerEslVersion: 4,
				TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
			}

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				if err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction); err != nil {
					return err
				}
				for _, tr := range setupTransformers {
					if err := dbHandler.DBWriteEslEventInternal(ctx, tr.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tr.GetMetadata().AuthorName, AuthorEmail: tr.GetMetadata().AuthorEmail}); err != nil {
						return err
					}
					prepareDatabaseLikeCdService(ctx, transaction, tr, dbHandler, t, authorEmail, authorName)
				}
				// First deploy while lock is active: files should NOT be written
				if err := dbHandler.DBWriteEslEventInternal(ctx, deployTransformer.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: authorName, AuthorEmail: authorEmail}); err != nil {
					return err
				}
				prepareDatabaseLikeCdService(ctx, transaction, deployTransformer, dbHandler, t, authorEmail, authorName)
				if err := dbHandler.DBWriteManifestLock(ctx, transaction, types.AppName(appName), envName, db.LockMetadata{
					CreatedByName:  authorName,
					CreatedByEmail: authorEmail,
					Message:        "test lock",
				}); err != nil {
					return err
				}

				for _, tr := range setupTransformers {
					if err := repo.Apply(ctx, transaction, tr); err != nil {
						return err
					}
				}
				if err := repo.Apply(ctx, transaction, deployTransformer); err != nil {
					return err
				}

				// Now delete the lock and do a same-version redeploy: files SHOULD be written
				if err := dbHandler.DBDeleteManifestLock(ctx, transaction, types.AppName(appName), envName); err != nil {
					return err
				}
				if err := dbHandler.DBWriteEslEventInternal(ctx, redeployTransformer.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: authorName, AuthorEmail: authorEmail}); err != nil {
					return err
				}
				prepareDatabaseLikeCdService(ctx, transaction, redeployTransformer, dbHandler, t, authorEmail, authorName)
				return repo.Apply(ctx, transaction, redeployTransformer)
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			updatedState := repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFiles); err != nil {
				t.Fatalf("error verifying content: %v\nFiles:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
		})
	}
}

func TestManifestLockBlocksDeploy(t *testing.T) {
	const appName = "myapp"
	const envName types.EnvName = "development"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name              string
		HasManifestLock   bool
		ArgoRenderOptions func(*argocd.RenderOptions)
		ExpectedFiles     []*FilenameAndData
		UnexpectedFiles   []*FilenameAndData
	}{
		{
			Name:            "manifests.yaml not written when manifest lock exists",
			HasManifestLock: true,
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = true
				opts.RenderBrackets = false
			},
			UnexpectedFiles: []*FilenameAndData{
				{path: "environments/development/applications/myapp/manifests/manifests.yaml"},
			},
		},
		{
			Name:            "bracket file not written when manifest lock exists",
			HasManifestLock: true,
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = false
				opts.RenderBrackets = true
				opts.PointToBrackets = true
			},
			UnexpectedFiles: []*FilenameAndData{
				{path: "environments/development/brackets/myapp/myapp.yaml"},
			},
		},
		{
			Name:            "manifests.yaml is written when no manifest lock",
			HasManifestLock: false,
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = true
				opts.RenderBrackets = false
			},
			ExpectedFiles: []*FilenameAndData{
				{path: "environments/development/applications/myapp/manifests/manifests.yaml", fileData: []byte("normal manifest")},
			},
		},
		{
			Name:            "bracket file is written when no manifest lock",
			HasManifestLock: false,
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = false
				opts.RenderBrackets = true
				opts.PointToBrackets = true
			},
			ExpectedFiles: []*FilenameAndData{
				{path: "environments/development/brackets/myapp/myapp.yaml", fileData: []byte("normal manifest")},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t, tc.ArgoRenderOptions)
			ctx := AddGeneratorToContext(testutilauth.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())
			dbHandler := repo.State().DBHandler

			setupTransformers := []Transformer{
				&CreateEnvironment{
					Environment: envName,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{Latest: true},
						ArgoCd: &config.EnvironmentConfigArgoCd{
							Destination: config.ArgoCdDestination{Server: string(envName)},
						},
					},
					TransformerEslVersion: 1,
					TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
				},
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						envName: "normal manifest",
					},
					WriteCommitData:       false,
					Version:               1,
					Revision:              0,
					TransformerEslVersion: 2,
					Team:                  "myteam",
					TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
				},
			}
			deployTransformer := &DeployApplicationVersion{
				Authentication:        Authentication{},
				Environment:           envName,
				Application:           appName,
				Version:               1,
				Revision:              0,
				LockBehaviour:         1,
				WriteCommitData:       false,
				TransformerEslVersion: 3,
				TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
			}

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				if err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction); err != nil {
					return err
				}
				for _, tr := range setupTransformers {
					if err := dbHandler.DBWriteEslEventInternal(ctx, tr.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tr.GetMetadata().AuthorName, AuthorEmail: tr.GetMetadata().AuthorEmail}); err != nil {
						return err
					}
					prepareDatabaseLikeCdService(ctx, transaction, tr, dbHandler, t, authorEmail, authorName)
				}
				if err := dbHandler.DBWriteEslEventInternal(ctx, deployTransformer.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: authorName, AuthorEmail: authorEmail}); err != nil {
					return err
				}
				prepareDatabaseLikeCdService(ctx, transaction, deployTransformer, dbHandler, t, authorEmail, authorName)

				if tc.HasManifestLock {
					if err := dbHandler.DBWriteManifestLock(ctx, transaction, types.AppName(appName), envName, db.LockMetadata{
						CreatedByName:  authorName,
						CreatedByEmail: authorEmail,
						Message:        "test lock",
					}); err != nil {
						return err
					}
				}

				for _, tr := range setupTransformers {
					if err := repo.Apply(ctx, transaction, tr); err != nil {
						return err
					}
				}
				return repo.Apply(ctx, transaction, deployTransformer)
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			updatedState := repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFiles); err != nil {
				t.Fatalf("error verifying content: %v\nFiles:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			if err := verifyMissing(updatedState.Filesystem, tc.UnexpectedFiles); err != nil {
				t.Fatalf("error verifying missing files: %v\nFiles:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
		})
	}
}

func TestDeleteEnvFromAppRemovesBracketFile(t *testing.T) {
	const appName = "myapp"
	const envName types.EnvName = "development"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name              string
		DeleteVia         string // "undeploy" or "deleteEnvFromApp"
		ArgoRenderOptions func(*argocd.RenderOptions)
		// FilesBeforeDelete must exist after the deploy, i.e. before the delete transformer runs.
		FilesBeforeDelete []*FilenameAndData
		// RemovedFile must be gone after the delete transformer runs.
		RemovedFile *FilenameAndData
	}{
		{
			Name:      "bracket manifest is removed by UndeployApplication",
			DeleteVia: "undeploy",
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = true
				opts.RenderBrackets = true
				opts.PointToBrackets = true
			},
			FilesBeforeDelete: []*FilenameAndData{
				{path: "environments/development/brackets/myapp/myapp.yaml", fileData: []byte("normal manifest")},
			},
			RemovedFile: &FilenameAndData{path: "environments/development/brackets/myapp/myapp.yaml"},
		},
		{
			Name:      "application manifest is removed by UndeployApplication",
			DeleteVia: "undeploy",
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = true
				opts.RenderBrackets = false
			},
			FilesBeforeDelete: []*FilenameAndData{
				{path: "environments/development/applications/myapp/manifests/manifests.yaml", fileData: []byte("normal manifest")},
			},
			RemovedFile: &FilenameAndData{path: "environments/development/applications/myapp/manifests/manifests.yaml"},
		},
		{
			Name:      "bracket manifest is removed by DeleteEnvFromApp",
			DeleteVia: "deleteEnvFromApp",
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = true
				opts.RenderBrackets = true
				opts.PointToBrackets = true
			},
			FilesBeforeDelete: []*FilenameAndData{
				{path: "environments/development/brackets/myapp/myapp.yaml", fileData: []byte("normal manifest")},
			},
			RemovedFile: &FilenameAndData{path: "environments/development/brackets/myapp/myapp.yaml"},
		},
		{
			Name:      "application manifest is removed by DeleteEnvFromApp",
			DeleteVia: "deleteEnvFromApp",
			ArgoRenderOptions: func(opts *argocd.RenderOptions) {
				opts.RenderApps = true
				opts.RenderBrackets = false
			},
			FilesBeforeDelete: []*FilenameAndData{
				{path: "environments/development/applications/myapp/manifests/manifests.yaml", fileData: []byte("normal manifest")},
			},
			RemovedFile: &FilenameAndData{path: "environments/development/applications/myapp/manifests/manifests.yaml"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t, tc.ArgoRenderOptions)
			ctx := AddGeneratorToContext(testutilauth.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())
			dbHandler := repo.State().DBHandler

			setupTransformers := []Transformer{
				&CreateEnvironment{
					Environment: envName,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{Latest: true},
						ArgoCd: &config.EnvironmentConfigArgoCd{
							Destination: config.ArgoCdDestination{Server: string(envName)},
						},
					},
					TransformerEslVersion: 1,
					TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
				},
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[types.EnvName]string{
						envName: "normal manifest",
					},
					WriteCommitData:       false,
					Version:               1,
					Revision:              0,
					TransformerEslVersion: 2,
					Team:                  "myteam",
					TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
				},
			}
			deployTransformer := &DeployApplicationVersion{
				Authentication:        Authentication{},
				Environment:           envName,
				Application:           appName,
				Version:               1,
				Revision:              0,
				LockBehaviour:         1,
				WriteCommitData:       false,
				TransformerEslVersion: 3,
				TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
			}
			var deleteTransformer Transformer
			switch tc.DeleteVia {
			case "undeploy":
				deleteTransformer = &UndeployApplication{
					Application:           appName,
					TransformerEslVersion: 4,
					TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
				}
			case "deleteEnvFromApp":
				deleteTransformer = &DeleteEnvFromApp{
					Application:           appName,
					Environment:           envName,
					TransformerEslVersion: 4,
					TransformerMetadata:   TransformerMetadata{AuthorName: authorName, AuthorEmail: authorEmail},
				}
			default:
				t.Fatalf("unknown DeleteVia: %q", tc.DeleteVia)
			}

			// Phase 1: create the environment, the app version and deploy it, so the manifest exists.
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				if err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction); err != nil {
					return err
				}
				for _, tr := range setupTransformers {
					if err := dbHandler.DBWriteEslEventInternal(ctx, tr.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tr.GetMetadata().AuthorName, AuthorEmail: tr.GetMetadata().AuthorEmail}); err != nil {
						return err
					}
					prepareDatabaseLikeCdService(ctx, transaction, tr, dbHandler, t, authorEmail, authorName)
				}
				if err := dbHandler.DBWriteEslEventInternal(ctx, deployTransformer.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: authorName, AuthorEmail: authorEmail}); err != nil {
					return err
				}
				prepareDatabaseLikeCdService(ctx, transaction, deployTransformer, dbHandler, t, authorEmail, authorName)

				for _, tr := range setupTransformers {
					if err := repo.Apply(ctx, transaction, tr); err != nil {
						return err
					}
				}
				return repo.Apply(ctx, transaction, deployTransformer)
			})
			t.Logf("repo root: %s", repo.State().Filesystem.Root())
			if err != nil {
				t.Fatalf("unexpected error during setup: %v", err)
			}
			// precondition: the manifest we are about to delete must actually exist first
			if err := verifyContent(repo.State().Filesystem, tc.FilesBeforeDelete); err != nil {
				t.Fatalf("precondition failed, manifest was not rendered before delete: %v\nFiles:\n%s", err, strings.Join(listFiles(repo.State().Filesystem), "\n"))
			}

			// Phase 2: delete the environment from the app, which must remove the manifest.
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				if err := dbHandler.DBWriteEslEventInternal(ctx, deleteTransformer.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: authorName, AuthorEmail: authorEmail}); err != nil {
					return err
				}
				return repo.Apply(ctx, transaction, deleteTransformer)
			})
			if err != nil {
				t.Fatalf("unexpected error during delete: %v", err)
			}

			updatedState := repo.State()
			if err := verifyMissing(updatedState.Filesystem, []*FilenameAndData{tc.RemovedFile}); err != nil {
				t.Fatalf("error verifying missing files: %v\nFiles:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
		})
	}
}
