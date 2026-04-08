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

func setupRepositoryTestWithPath(t *testing.T) (Repository, string) {
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
			expectedError: errMatcher{msg: "error within transaction: first apply failed, aborting: error at index 0 of transformer batch: UndeployApplication: error last release is not un-deployed application version of 'app1'"},
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
			expectedError: errMatcher{msg: "error within transaction: first apply failed, aborting: error at index 0 of transformer batch: UndeployApplication(repo):" +
				" error cannot un-deploy application '" + appName + "' the release on 'acceptance2' is not un-deployed: 'environments/acceptance2/applications/" + appName + "/version/undeploy'"},
			expectedData: []*FilenameAndData{},
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
			//t.Parallel()
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

			if err != nil {
				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
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
			Name: "create same environment twice updates it",
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
					Environment: envAcceptance,
					Config:      envAcceptance2Config,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
			},
			expectedData: []*FilenameAndData{
				{
					path: "/environments/acceptance/config.json",
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
			},
			expectedMessage: "create environment \"acceptance\"",
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
		err2 := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, types.AppName(concreteTransformer.Application), db.AppStateChangeCreate, db.DBAppMetaData{Team: concreteTransformer.Team}, types.ArgoBracketName(concreteTransformer.Application))
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
