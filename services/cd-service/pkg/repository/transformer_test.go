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

package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/testutil"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/ptr"
	"github.com/freiheit-com/kuberpult/pkg/testfs"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/go-git/go-billy/v5/util"
	"github.com/google/go-cmp/cmp"
	godebug "github.com/kylelemons/godebug/diff"
)

const (
	envAcceptance      = "acceptance"
	envProduction      = "production"
	additionalVersions = 7
)

var timeNowOld = time.Date(1999, 01, 02, 03, 04, 05, 0, time.UTC)

func TestUndeployApplicationErrors(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedError     string
		expectedCommitMsg string
		shouldSucceed     bool
	}{
		{
			Name: "Delete non-existent application",
			Transformers: []Transformer{
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedError:     "UndeployApplication: error cannot undeploy non-existing application 'app1'",
			expectedCommitMsg: "",
			shouldSucceed:     false,
		},
		{
			Name: "Success",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "application 'app1' was deleted successfully",
			shouldSucceed:     true,
		},
		{
			Name: "Create un-deploy Version for un-deployed application should not work",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&UndeployApplication{
					Application: "app1",
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
			},
			expectedError:     "cannot undeploy non-existing application 'app1'",
			expectedCommitMsg: "",
			shouldSucceed:     false,
		},
		{
			Name: "Undeploy application where there is an application lock should not work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateEnvironmentApplicationLock{
					Environment: "acceptance",
					Application: "app1",
					LockId:      "22133",
					Message:     "test",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "application 'app1' was deleted successfully",
			shouldSucceed:     true,
		},
		{
			Name: "Undeploy application where there is an application lock created after the un-deploy version creation should",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateEnvironmentApplicationLock{
					Environment: "acceptance",
					Application: "app1",
					LockId:      "22133",
					Message:     "test",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "application 'app1' was deleted successfully",
			shouldSucceed:     true,
		},
		{
			Name: "Undeploy application where there current releases are not undeploy shouldn't work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
				},
				&CreateEnvironmentLock{
					Environment: "acceptance",
					LockId:      "22133",
					Message:     "test",
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedError:     "UndeployApplication: error cannot un-deploy application 'app1' the release 'acceptance' is not un-deployed: 'environments/acceptance/applications/app1/version/undeploy'",
			expectedCommitMsg: "",
			shouldSucceed:     false,
		},
		{
			Name: "Undeploy application where the app does not have a release in all envs must work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironment{
					Environment: "production",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "application 'app1' was deleted successfully",
			shouldSucceed:     true,
		},
		{
			Name: "Undeploy application where there is an environment lock should work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateEnvironmentLock{
					Environment: "acceptance",
					LockId:      "22133",
					Message:     "test",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "application 'app1' was deleted successfully",
			shouldSucceed:     true,
		},
		{
			Name: "Undeploy application where the last release is not Undeploy shouldn't work",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateApplicationVersion{
					Application:    "app1",
					Manifests:      nil,
					SourceCommitId: "",
					SourceAuthor:   "",
					SourceMessage:  "",
				},
				&UndeployApplication{
					Application: "app1",
				},
			},
			expectedError:     "UndeployApplication: error last release is not un-deployed application version of 'app1'",
			expectedCommitMsg: "",
			shouldSucceed:     false,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := setupRepositoryTest(t)
			commitMsg, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), tc.Transformers...)
			// note that we only check the LAST error here:
			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
				}
				actualMsg := commitMsg[len(commitMsg)-1]
				if actualMsg != tc.expectedCommitMsg {
					t.Fatalf("expected a different message.\nExpected: %q\nGot %q", tc.expectedCommitMsg, actualMsg)
				}
			} else {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				} else {
					actualMsg := err.Error()
					if actualMsg != tc.expectedError {
						t.Fatalf("expected a different error.\nExpected: %q\nGot %q", tc.expectedError, actualMsg)
					}
				}
			}
		})
	}
}

func TestCreateUndeployApplicationVersionErrors(t *testing.T) {
	tcs := []struct {
		Name             string
		Transformers     []Transformer
		expectedError    string
		expectedPath     string
		shouldSucceed    bool
		expectedFileData []byte
	}{
		{
			Name: "successfully undeploy - should work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
			},
			expectedError:    "",
			expectedPath:     "applications/app1/releases/2/environments/acceptance/manifests.yaml",
			expectedFileData: []byte(" "),
			shouldSucceed:    true,
		},
		{
			Name: "Does not undeploy - should not succeed",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
				},
			},
			expectedError:    "file does not exist",
			expectedPath:     "",
			expectedFileData: []byte(""),
			shouldSucceed:    false,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := setupRepositoryTest(t)
			_, updatedState, _ := repo.ApplyTransformersInternal(testutil.MakeTestContext(), tc.Transformers...)

			fileData, err := util.ReadFile(updatedState.Filesystem, updatedState.Filesystem.Join(updatedState.Filesystem.Root(), tc.expectedPath))

			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
				}
				if !cmp.Equal(fileData, tc.expectedFileData) {
					t.Fatalf("Expected %v, got %v", tc.expectedFileData, fileData)
				}
			} else {
				if err == nil {
					t.Fatal("Expected error but got none")
				} else {
					actualMsg := err.Error()
					if actualMsg != tc.expectedError {
						t.Fatalf("expected a different error.\nExpected: %q\nGot %q", tc.expectedError, actualMsg)
					}
				}
			}
		})
	}
}

func TestDeployApplicationVersion(t *testing.T) {
	tcs := []struct {
		Name                        string
		Transformers                []Transformer
		expectedError               string
		expectedPath                string
		expectedFileData            []byte
		expectedDeployedByPath      string
		expectedDeployedByData      []byte
		expectedDeployedByEmailPath string
		expectedDeployedByEmailData []byte
		expectedDeployedAtPath      string
		expectedDeployedAtData      []byte
	}{
		{
			Name: "successfully deploy a full manifest",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance", // not empty
					},
				},
				&DeployApplicationVersion{
					Environment:   envAcceptance,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_Fail,
				},
			},
			expectedError:               "",
			expectedPath:                "environments/acceptance/applications/app1/manifests/manifests.yaml",
			expectedFileData:            []byte("acceptance"),
			expectedDeployedByPath:      "environments/acceptance/applications/app1/deployed_by",
			expectedDeployedByData:      []byte("test tester"),
			expectedDeployedAtPath:      "environments/acceptance/applications/app1/deployed_at_utc",
			expectedDeployedAtData:      []byte(timeNowOld.UTC().String()),
			expectedDeployedByEmailPath: "environments/acceptance/applications/app1/deployed_by_email",
			expectedDeployedByEmailData: []byte("testmail@example.com"),
		},
		{
			Name: "successfully deploy an empty manifest",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: false}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "", // empty!
					},
				},
				&DeployApplicationVersion{
					Environment:   envAcceptance,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_Fail,
				},
			},
			expectedError:               "",
			expectedPath:                "environments/acceptance/applications/app1/manifests/manifests.yaml",
			expectedFileData:            []byte(" "),
			expectedDeployedByPath:      "environments/acceptance/applications/app1/deployed_by",
			expectedDeployedByData:      []byte("test tester"),
			expectedDeployedAtPath:      "environments/acceptance/applications/app1/deployed_at_utc",
			expectedDeployedAtData:      []byte(timeNowOld.UTC().String()),
			expectedDeployedByEmailPath: "environments/acceptance/applications/app1/deployed_by_email",
			expectedDeployedByEmailData: []byte("testmail@example.com"),
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctxWithTime := withTimeNow(testutil.MakeTestContext(), timeNowOld)
			t.Parallel()
			repo := setupRepositoryTest(t)
			_, updatedState, err := repo.ApplyTransformersInternal(ctxWithTime, tc.Transformers...)
			if err != nil {
				t.Fatalf("Expected no error when applying: %v", err)
			}

			fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), tc.expectedPath)
			fileData, err := util.ReadFile(updatedState.Filesystem, fullPath)

			if err != nil {
				t.Fatalf("Expected no error: %v path=%s", err, fullPath)
			}
			if !cmp.Equal(fileData, tc.expectedFileData) {
				t.Fatalf("Expected '%v', got '%v'", string(tc.expectedFileData), string(fileData))
			}

			fullDeployedByPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), tc.expectedDeployedByPath)
			deployedByData, err := util.ReadFile(updatedState.Filesystem, fullDeployedByPath)

			if err != nil {
				t.Fatalf("Expected no error: %v path=%s", err, fullDeployedByPath)
			}
			if !cmp.Equal(deployedByData, tc.expectedDeployedByData) {
				t.Fatalf("Expected '%v', got '%v'", string(tc.expectedDeployedByData), string(deployedByData))
			}

			fullDeployedByEmailPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), tc.expectedDeployedByEmailPath)
			deployedByEmailData, err := util.ReadFile(updatedState.Filesystem, fullDeployedByEmailPath)

			if err != nil {
				t.Fatalf("Expected no error: %v path=%s", err, fullDeployedByEmailPath)
			}
			if !cmp.Equal(deployedByEmailData, tc.expectedDeployedByEmailData) {
				t.Fatalf("Expected '%v', got '%v'", string(tc.expectedDeployedByEmailData), string(deployedByEmailData))
			}

			fullDeployedAtPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), tc.expectedDeployedAtPath)
			DeployedAtData, err := util.ReadFile(updatedState.Filesystem, fullDeployedAtPath)

			if err != nil {
				t.Fatalf("Expected no error: %v path=%s", err, fullDeployedAtPath)
			}
			if !cmp.Equal(DeployedAtData, tc.expectedDeployedAtData) {
				t.Fatalf("Expected '%v', got '%v'", string(tc.expectedDeployedAtData), string(DeployedAtData))
			}
		})
	}
}

func TestCreateApplicationVersionWithVersion(t *testing.T) {
	tcs := []struct {
		Name             string
		Transformers     []Transformer
		expectedPath     string
		expectedFileData []byte
	}{
		{
			Name: "successfully create app version with right order - should work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "first version (100) manifest",
					},
					Version: 100,
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "second version (101) manifest",
					},
					Version: 101,
				},
			},
			expectedPath:     "applications/app1/releases/101/environments/acceptance/manifests.yaml",
			expectedFileData: []byte("second version (101) manifest"),
		},
		{
			Name: "successfully create 2 app versions in wrong order - should work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "first version (100) manifest",
					},
					Version: 100,
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "second version (99) manifest",
					},
					Version: 99,
				},
			},
			expectedPath:     "applications/app1/releases/99/environments/acceptance/manifests.yaml",
			expectedFileData: []byte("second version (99) manifest"),
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := setupRepositoryTest(t)
			_, updatedState, _ := repo.ApplyTransformersInternal(testutil.MakeTestContext(), tc.Transformers...)

			fileData, err := util.ReadFile(updatedState.Filesystem, updatedState.Filesystem.Join(updatedState.Filesystem.Root(), tc.expectedPath))

			if err != nil {
				t.Fatalf("Expected no error: %v", err)
			}
			if !cmp.Equal(fileData, tc.expectedFileData) {
				t.Fatalf("Expected %v, got %v", string(tc.expectedFileData), string(fileData))
			}
		})
	}
}

// Tests various error cases in the prepare-Undeploy endpoint, specifically the error messages returned.
func TestUndeployErrors(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedError     string
		expectedCommitMsg string
		shouldSucceed     bool
	}{
		{
			Name: "Access non-existent application",
			Transformers: []Transformer{
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
			},
			expectedError:     "cannot undeploy non-existing application 'app1'",
			expectedCommitMsg: "",
			shouldSucceed:     false,
		},
		{
			Name: "Success",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "created undeploy-version 2 of 'app1'\n",
			shouldSucceed:     true,
		},
		{
			Name: "Deploy after Undeploy should work",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateApplicationVersion{
					Application:    "app1",
					Manifests:      nil,
					SourceCommitId: "",
					SourceAuthor:   "",
					SourceMessage:  "",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "created version 3 of \"app1\"\n",
			shouldSucceed:     true,
		},
		{
			Name: "Undeploy twice should succeed",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
				&CreateUndeployApplicationVersion{
					Application: "app1",
				},
			},
			shouldSucceed:     true,
			expectedError:     "",
			expectedCommitMsg: "created undeploy-version 3 of 'app1'\n",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := setupRepositoryTest(t)
			commitMsg, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), tc.Transformers...)
			// note that we only check the LAST error here:
			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
				}
				actualMsg := commitMsg[len(commitMsg)-1]
				if actualMsg != tc.expectedCommitMsg {
					t.Fatalf("expected a different message.\nExpected: %q\nGot %q", tc.expectedCommitMsg, actualMsg)
				}
			} else {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				} else {
					actualMsg := err.Error()
					if actualMsg != tc.expectedError {
						t.Fatalf("expected a different error.\nExpected: %q\nGot %q", tc.expectedError, actualMsg)
					}
				}
			}
		})
	}
}

// Tests various error cases in the release train, specifically the error messages returned.
func TestReleaseTrainErrors(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedError     string
		expectedCommitMsg string
		shouldSucceed     bool
	}{
		{
			Name: "Access non-existent environment",
			Transformers: []Transformer{
				&ReleaseTrain{
					Target: "doesnotexistenvironment",
				},
			},
			expectedError:     "rpc error: code = InvalidArgument desc = error: could not find environment group or environment configs for 'doesnotexistenvironment'",
			expectedCommitMsg: "",
			shouldSucceed:     false,
		},
		{
			Name: "Environment is locked",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance,
						},
					},
				},
				&CreateEnvironmentLock{
					Environment: envAcceptance,
					Message:     "don't",
					LockId:      "care",
				},
				&ReleaseTrain{
					Target: envAcceptance,
				},
			},
			shouldSucceed:     true,
			expectedError:     "",
			expectedCommitMsg: "Target Environment 'acceptance' is locked - exiting.",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := setupRepositoryTest(t)
			commitMsg, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), tc.Transformers...)
			// note that we only check the LAST error here:
			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
				}
				actualMsg := commitMsg[len(commitMsg)-1]
				if actualMsg != tc.expectedCommitMsg {
					t.Fatalf("expected a different message.\nExpected: %q\nGot %q", tc.expectedCommitMsg, actualMsg)
				}

			} else {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				} else {
					actualMsg := err.Error()
					if actualMsg != tc.expectedError {
						t.Fatalf("expected a different error.\nExpected: %q\nGot %q", tc.expectedError, actualMsg)
					}
				}
			}
		})
	}
}

func TestRbacTransformerTest(t *testing.T) {
	tcs := []struct {
		Name          string
		Transformers  []Transformer
		ExpectedError string
	}{
		{
			Name: "able to create environment lock with permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateEnvironmentLock{
					Environment: "production",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{
						"developer,CreateLock,production:production,*,allow": {Role: "developer"}}}},
				},
			},
		},
		{
			Name: "unable to create environment lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateEnvironmentLock{
					Environment:    "production",
					Message:        "don't",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{}}},
				},
			},
			ExpectedError: "user does not have permissions for: developer,CreateLock,production:production,*,allow",
		},
		{
			Name: "unable to delete environment lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateEnvironmentLock{
					Environment: "production",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{
						"developer,CreateLock,production:production,*,allow": {Role: "developer"}}}},
				},
				&DeleteEnvironmentLock{
					Environment:    "production",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{}}},
				},
			},
			ExpectedError: "user does not have permissions for: developer,DeleteLock,production:production,*,allow",
		},
		{
			Name: "able to delete environment lock with permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateEnvironmentLock{
					Environment: "production",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{
						"developer,CreateLock,production:production,*,allow": {Role: "developer"}}}},
				},
				&DeleteEnvironmentLock{
					Environment: "production",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{
						"developer,CreateLock,production:production,*,allow": {Role: "developer"},
						"developer,DeleteLock,production:production,*,allow": {Role: "developer"}}}},
				},
			},
		},
		{
			Name: "unable to create environment application lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
				},
				&CreateEnvironmentApplicationLock{
					Environment:    "production",
					Application:    "test",
					Message:        "don't",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{}}},
				},
			},
			ExpectedError: "user does not have permissions for: developer,CreateLock,production:production,test,allow",
		},
		{
			Name: "able to create environment application lock with correct permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
				},
				&CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{
						"developer,CreateLock,production:production,*,allow": {Role: "Developer"},
					}}},
				},
			},
		},
		{
			Name: "unable to delete environment application lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
				},
				&CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{
						"developer,CreateLock,production:production,*,allow": {Role: "developer"},
					}}},
				},
				&DeleteEnvironmentApplicationLock{
					Environment:    "production",
					Application:    "test",
					LockId:         "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{}}},
				},
			},
			ExpectedError: "user does not have permissions for: developer,DeleteLock,production:production,test,allow",
		},
		{
			Name: "able to delete environment application lock without permissions policy",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
				},
				&CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					Message:     "don't",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{
						"developer,CreateLock,production:production,*,allow": {Role: "developer"},
					}}},
				},
				&DeleteEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					LockId:      "manual",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: true, Policy: map[string]*auth.Permission{
						"developer,DeleteLock,production:production,*,allow": {Role: "developer"},
					}}},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()
			repo, err := New(
				testutil.MakeTestContextDexEnabled(),
				RepositoryConfig{
					URL:            remoteDir,
					Path:           localDir,
					CommitterEmail: "kuberpult@freiheit.com",
					CommitterName:  "kuberpult",
					BootstrapMode:  false,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			for _, tf := range tc.Transformers {
				err = repo.Apply(testutil.MakeTestContextDexEnabled(), tf)
				if err != nil {
					break
				}
			}
			if err != nil {
				if !(strings.Contains(err.Error(), tc.ExpectedError)) {
					t.Errorf("want :\n\"%v\"\nbut got:\n\"%v\"", tc.ExpectedError, err.Error())
				}
				if tc.ExpectedError == "" {
					t.Errorf("expected success but got: %v", err.Error())
				}
			} else if tc.ExpectedError != "" {
				t.Errorf("expected error but got: none found")
			}
		})
	}
}

func TestTransformer(t *testing.T) {
	c1 := config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}}

	tcs := []struct {
		Name          string
		Transformers  []Transformer
		Test          func(t *testing.T, s *State)
		ErrorTest     func(t *testing.T, err error)
		BootstrapMode bool
	}{
		{
			Name:         "Create Versions and do not clean up because not enough versions",
			Transformers: makeTransformersForDelete(3),
			Test: func(t *testing.T, s *State) {
				{
					prodVersion, err := s.GetEnvironmentApplicationVersion(envProduction, "test")
					if err != nil {
						t.Fatal(err)
					}
					if prodVersion == nil || *prodVersion != 3 {
						t.Errorf("unexpected version: expected 3, actual %d", *prodVersion)
					}
					checkReleaseExists := func(v uint64) {
						_, err := s.GetApplicationRelease("test", v)
						if err != nil {
							t.Fatal(err)
						}
					}
					var v uint64
					for v = 1; v <= 3; v++ {
						checkReleaseExists(v)
					}
				}
			},
		},
		{
			Name:         "Create Versions and clean up because too many version",
			Transformers: makeTransformersForDelete(keptVersionsOnCleanup),
			Test: func(t *testing.T, s *State) {
				{
					prodVersion, err := s.GetEnvironmentApplicationVersion(envProduction, "test")
					if err != nil {
						t.Fatal(err)
					}
					if prodVersion == nil || *prodVersion != keptVersionsOnCleanup {
						t.Errorf("unexpected version: actual %d", *prodVersion)
					}
					checkReleaseExists := func(v uint64) {
						_, err := s.GetApplicationRelease("test", v)
						if err != nil {
							t.Fatal(err)
						}
					}
					var v uint64
					for v = 1; v <= keptVersionsOnCleanup; v++ {
						checkReleaseExists(v)
					}
				}
			},
		},
		{
			Name:         "Create Versions and clean up because too many version",
			Transformers: makeTransformersForDelete(keptVersionsOnCleanup + additionalVersions),
			Test: func(t *testing.T, s *State) {
				{
					prodVersion, err := s.GetEnvironmentApplicationVersion(envProduction, "test")
					if err != nil {
						t.Fatal(err)
					}
					if prodVersion == nil || *prodVersion != keptVersionsOnCleanup+additionalVersions {
						t.Errorf("unexpected version: actual %d", *prodVersion)
					}
					checkReleaseExists := func(v uint64) {
						_, err := s.GetApplicationRelease("test", v)
						if err != nil {
							t.Fatal(err)
						}
					}
					checkReleaseDoesNotExists := func(v uint64) {
						release, err := s.GetApplicationRelease("test", v)
						if err == nil {
							t.Fatalf("expected release to not exist. release: %d, actual: %d", v, release.Version)
						} else {
							expectedError := fmt.Sprintf("could not call stat 'applications/test/releases/%d': file does not exist", v)
							if err.Error() != expectedError {
								t.Errorf("unexpected error while checking release: \n%v\nExpected:\n%s", err.Error(), expectedError)
							}
						}
					}
					var v uint64
					for v = 1; v <= additionalVersions; v++ {
						checkReleaseDoesNotExists(v)
					}
					for v = additionalVersions + 1; v <= keptVersionsOnCleanup+additionalVersions; v++ {
						checkReleaseExists(v)
					}
				}
			},
		},
		{
			Name: "Release train",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance, // train drives from acceptance to production
						},
					},
				},
				&CreateEnvironment{
					Environment: envAcceptance,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance,
							Latest:      true,
						},
					},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
				},
				&DeployApplicationVersion{
					Environment: envProduction,
					Application: "test",
					Version:     1,
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
				},
				&DeployApplicationVersion{
					Environment: envAcceptance,
					Application: "test",
					Version:     1,
				},
				&DeployApplicationVersion{
					Environment: envAcceptance,
					Application: "test",
					Version:     2,
				},
				&ReleaseTrain{
					Target: envProduction,
				},
			},
			Test: func(t *testing.T, s *State) {
				{
					prodVersion, err := s.GetEnvironmentApplicationVersion(envProduction, "test")
					if err != nil {
						t.Fatal(err)
					}
					acceptanceVersion, err := s.GetEnvironmentApplicationVersion(envAcceptance, "test")
					if err != nil {
						t.Fatal(err)
					}
					if *acceptanceVersion != 2 {
						t.Errorf("unexpected version: expected 2, actual %d", acceptanceVersion)
					}
					if *prodVersion != 2 {
						t.Errorf("unexpected version: expected 2, actual %d", *prodVersion)
					}
				}
			},
		},
		{
			Name: "Release train from Latest",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						envAcceptance: "acceptancenmanifest",
					},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						envAcceptance: "acceptancenmanifest",
					},
				},
				&ReleaseTrain{
					Target: envAcceptance,
				},
			},
			Test: func(t *testing.T, s *State) {
				{
					acceptanceVersion, err := s.GetEnvironmentApplicationVersion(envAcceptance, "test")
					if err != nil {
						t.Fatal(err)
					}
					if *acceptanceVersion != 2 {
						t.Errorf("unexpected version: expected 2, actual %d", acceptanceVersion)
					}
				}
			},
		},
		{
			Name: "Release train for a Team",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance, // train drives from acceptance to production
						},
					},
				},
				&CreateEnvironment{
					Environment: envAcceptance,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: envAcceptance,
							Latest:      true,
						},
					},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
					Team: "test",
				},
				&DeployApplicationVersion{
					Environment: envProduction,
					Application: "test",
					Version:     1,
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
						envAcceptance: "acceptancenmanifest",
					},
				},
				&DeployApplicationVersion{
					Environment: envAcceptance,
					Application: "test",
					Version:     1,
				},
				&DeployApplicationVersion{
					Environment: envAcceptance,
					Application: "test",
					Version:     2,
				},
				&ReleaseTrain{
					Target: envProduction,
					Team:   "test",
				},
			},
			Test: func(t *testing.T, s *State) {
				{
					prodVersion, err := s.GetEnvironmentApplicationVersion(envProduction, "test")
					if err != nil {
						t.Fatal(err)
					}
					acceptanceVersion, err := s.GetEnvironmentApplicationVersion(envAcceptance, "test")
					if err != nil {
						t.Fatal(err)
					}
					if *acceptanceVersion != 2 {
						t.Errorf("unexpected version: expected 2, actual %d", acceptanceVersion)
					}
					if *prodVersion != 2 {
						t.Errorf("unexpected version: expected 2, actual %d", *prodVersion)
					}
				}
			},
		},
		{
			Name: "Lock environment",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateEnvironmentLock{
					Environment: "production",
					Message:     "don't",
					LockId:      "manual",
				},
			},
			Test: func(t *testing.T, s *State) {
				locks, err := s.GetEnvironmentLocks("production")
				if err != nil {
					t.Fatal(err)
				}
				expected := map[string]Lock{
					"manual": {
						Message: "don't",
						CreatedBy: Actor{
							Name:  "test tester",
							Email: "testmail@example.com",
						},
						CreatedAt: timeNowOld,
					},
				}
				if !reflect.DeepEqual(locks, expected) {
					t.Fatalf("mismatched locks. expected:\n%#v\nactual:\n%#v", expected, locks)
				}
			},
		},
		{
			Name: "Lock application",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
				},
				&CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					Message:     "don't",
					LockId:      "manual",
				},
			},
			Test: func(t *testing.T, s *State) {
				locks, err := s.GetEnvironmentApplicationLocks("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				expected := map[string]Lock{
					"manual": {
						Message: "don't",
						CreatedBy: Actor{
							Name:  "test tester",
							Email: "testmail@example.com",
						},
						CreatedAt: timeNowOld,
					},
				}
				if !reflect.DeepEqual(locks, expected) {
					t.Fatalf("mismatched locks. expected:\n%#v\n, actual:\n%#v", expected, locks)
				}
			},
		},
		{
			Name: "Overwriting lock environment",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateEnvironmentLock{
					Environment: "production",
					Message:     "don't",
					LockId:      "manual",
				},
				&CreateEnvironmentLock{
					Environment: "production",
					Message:     "just don't",
					LockId:      "manual",
				},
			},
			Test: func(t *testing.T, s *State) {
				locks, err := s.GetEnvironmentLocks("production")
				if err != nil {
					t.Fatal(err)
				}
				expected := map[string]Lock{
					"manual": {
						Message: "just don't",
						CreatedBy: Actor{
							Name:  "test tester",
							Email: "testmail@example.com",
						},
						CreatedAt: timeNowOld,
					},
				}
				if !reflect.DeepEqual(locks, expected) {
					t.Fatalf("mismatched locks. expected: %#v, actual: %#v", expected, locks)
				}
			},
		},
		{
			Name: "Unlocking a locked environment",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateEnvironmentLock{
					Environment: "production",
					Message:     "don't",
					LockId:      "manual",
				},
				&DeleteEnvironmentLock{
					Environment: "production",
					LockId:      "manual",
				},
			},
			Test: func(t *testing.T, s *State) {
				locks, err := s.GetEnvironmentLocks("production")
				if err != nil {
					t.Fatal(err)
				}
				expected := map[string]Lock{}
				if !reflect.DeepEqual(locks, expected) {
					t.Fatalf("mismatched locks. expected: %#v, actual: %#v", expected, locks)
				}
			},
		},
		{
			Name: "Unlocking an already unlocked environment",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&DeleteEnvironmentLock{
					Environment: "production",
					LockId:      "manual",
				},
			},
			Test: func(t *testing.T, s *State) {
				locks, err := s.GetEnvironmentLocks("production")
				if err != nil {
					t.Fatal(err)
				}
				expected := map[string]Lock{}
				if !reflect.DeepEqual(locks, expected) {
					t.Fatalf("mismatched locks. expected: %#v, actual: %#v", expected, locks)
				}
			},
		},
		{
			Name: "Deploy version",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
				},
				&DeployApplicationVersion{
					Environment: "production",
					Application: "test",
					Version:     1,
				},
			},
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				{
					i, err := s.GetEnvironmentApplicationVersion("production", "test")
					if err != nil {
						t.Fatal(err)
					}
					if *i != 1 {
						t.Errorf("unexpected version: expected 1, actual %d", i)
					}
				}
				// check that the manifest is in place for argocd
				{
					m, err := s.Filesystem.Open("environments/production/applications/test/manifests/manifests.yaml")
					if err != nil {
						t.Fatal(err)
					}
					content, err := io.ReadAll(m)
					expected := "productionmanifest"
					actual := string(content)
					if actual != expected {
						t.Errorf("unexpected manifest: expected %q, actual: %q", expected, actual)
					}
				}
				// Check that reading is possible
				{
					rel, err := s.GetApplicationRelease("test", 1)
					if err != nil {
						t.Fatal(err)
					}
					if rel.Version != 1 {
						t.Errorf("unexpected version: expected 1, actual: %d", rel.Version)
					}
					if rel.SourceAuthor != "" {
						t.Errorf("unexpected source author: expected \"\", actual: %q", rel.SourceAuthor)
					}
					if rel.SourceCommitId != "" {
						t.Errorf("unexpected source commit id: expected \"\", actual: %q", rel.SourceCommitId)
					}
					if rel.SourceMessage != "" {
						t.Errorf("unexpected source author: expected \"\", actual: %q", rel.SourceMessage)
					}
				}
			},
		},
		{
			Name: "Create version with source information",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateApplicationVersion{
					Application:    "test",
					SourceAuthor:   "test <test@example.com>",
					SourceCommitId: "deadbeef",
					SourceMessage:  "changed something",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
				},
			},
			Test: func(t *testing.T, s *State) {
				// Check that reading is possible
				{
					rel, err := s.GetApplicationRelease("test", 1)
					if err != nil {
						t.Fatal(err)
					}
					if rel.Version != 1 {
						t.Errorf("unexpected version: expected 1, actual: %d", rel.Version)
					}
					if rel.SourceAuthor != "test <test@example.com>" {
						t.Errorf("unexpected source author: expected \"test <test@example.com>\", actual: %q", rel.SourceAuthor)
					}
					if rel.SourceCommitId != "deadbeef" {
						t.Errorf("unexpected source commit id: expected \"deadbeef\", actual: %q", rel.SourceCommitId)
					}
					if rel.SourceMessage != "changed something" {
						t.Errorf("unexpected source author: expected \"changed something\", actual: %q", rel.SourceMessage)
					}
					if rel.CreatedAt != timeNowOld {
						t.Errorf("unexpected created at: expected: %q, actual: %q", timeNowOld, rel.SourceMessage)
					}
				}
			},
		}, {
			Name: "Create version with team name",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
					Team: "test-team",
				},
			},
			Test: func(t *testing.T, s *State) {
				// Check that team is written
				{
					team, err := s.GetApplicationTeamOwner("test")
					if err != nil {
						t.Fatal(err)
					}
					if team != "test-team" {
						t.Errorf("expected team name to be test-team, but got %q", team)
					}
				}
			},
		}, {
			Name: "Create version with version number",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateApplicationVersion{
					Version:     42,
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
				},
			},
			Test: func(t *testing.T, s *State) {
				// Check that reading is possible
				{
					rel, err := s.GetApplicationReleases("test")
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(rel, []uint64{42}) {
						t.Errorf("expected release list to be exaclty [42], but got %q", rel)
					}

				}
			},
		}, {
			Name: "Creating a version with same version number yields the correct error",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production"},
				&CreateApplicationVersion{
					Version:     42,
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
				},
				&CreateApplicationVersion{
					Version:     42,
					Application: "test",
					Manifests: map[string]string{
						"production": "productionmanifest",
					},
				},
			},
			ErrorTest: func(t *testing.T, err error) {
				if err == nil || (!strings.Contains(err.Error(), ErrReleaseAlreadyExist.Error())) {
					t.Errorf("expected %q, got %q", ErrReleaseAlreadyExist, err)
				}
			},
		}, {
			Name: "Creating an older version doesn't auto deploy",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production", Config: c1},
				&CreateApplicationVersion{
					Version:     42,
					Application: "test",
					Manifests: map[string]string{
						"production": "42",
					},
				},
				&CreateApplicationVersion{
					Version:     41,
					Application: "test",
					Manifests: map[string]string{
						"production": "41",
					},
				},
			},
			Test: func(t *testing.T, s *State) {
				i, err := s.GetEnvironmentApplicationVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if *i != 42 {
					t.Errorf("unexpected version: expected 42, actual %d", i)
				}
			},
		}, {
			Name: "Creating a version that is much too old yields the correct error",
			Transformers: func() []Transformer {
				t := make([]Transformer, 0, keptVersionsOnCleanup+1)
				t = append(t, &CreateEnvironment{Environment: "production"})
				for i := keptVersionsOnCleanup + 1; i > 0; i-- {
					t = append(t, &CreateApplicationVersion{
						Version:     uint64(i),
						Application: "test",
						Manifests: map[string]string{
							"production": "42",
						},
					})
				}
				return t
			}(),
			ErrorTest: func(t *testing.T, err error) {
				if err != ErrReleaseTooOld {
					t.Errorf("expected %q, got %q", ErrReleaseTooOld, err)
				}
			},
		}, {
			Name: "Auto Deploy version to second env",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "one", Config: c1},
				&CreateEnvironment{Environment: "two", Config: c1},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"one": "productionmanifest",
						"two": "productionmanifest",
					},
				},
				&DeployApplicationVersion{
					Environment: "one",
					Application: "test",
					Version:     1,
				},
			},
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				{
					i, err := s.GetEnvironmentApplicationVersion("one", "test")
					if err != nil {
						t.Fatal(err)
					}
					if *i != 1 {
						t.Errorf("unexpected version: expected 1, actual %d", i)
					}
				}
				for _, env := range []string{"one", "two"} {
					// check that the manifest is in place for BOTH envs

					m, err := s.Filesystem.Open(fmt.Sprintf("environments/%s/applications/test/manifests/manifests.yaml", env))
					if err != nil {
						t.Fatal(err)
					}
					content, err := io.ReadAll(m)
					expected := "productionmanifest"
					actual := string(content)
					if actual != expected {
						t.Errorf("unexpected manifest: expected %q, actual: %q", expected, actual)
					}
				}
			},
		},
		{
			Name: "Skip Auto Deploy if env is locked",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "one", Config: c1},
				&CreateEnvironment{Environment: "two", Config: c1},
				&CreateEnvironmentLock{
					Environment: "one",
					Message:     "don't!",
					LockId:      "manual123",
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"one": "productionmanifest",
						"two": "productionmanifest",
					},
				},
			},
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				{
					// version should only exist for "two"
					i, err := s.GetEnvironmentApplicationVersion("two", "test")
					if err != nil {
						t.Fatal(err)
					}
					if *i != 1 {
						t.Errorf("unexpected version: expected 1, actual %d", i)
					}
					i, err = s.GetEnvironmentApplicationVersion("one", "test")
					if i != nil || err != nil {
						t.Fatalf("expect file to not exist, because the env is locked.")
					}
				}
				// manifests should be written either way:
				for _, env := range []string{"one", "two"} {
					m, err := s.Filesystem.Open(fmt.Sprintf("applications/test/releases/1/environments/%s/manifests.yaml", env))
					if err != nil {
						t.Fatal(err)
					}
					content, err := io.ReadAll(m)
					expected := "productionmanifest"
					actual := string(content)
					if actual != expected {
						t.Errorf("unexpected manifest: expected %q, actual: %q", expected, actual)
					}
				}
			},
		},
		{
			Name: "Skip Auto Deploy version to second env if it's not latest",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "one", Config: c1},
				&CreateEnvironment{Environment: "two", Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{
					Environment: "two",
					Latest:      false,
				}}},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"one": "productionmanifest",
						"two": "productionmanifest",
					},
				},
				&DeployApplicationVersion{
					Environment: "one",
					Application: "test",
					Version:     1,
				},
			},
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				{
					i, err := s.GetEnvironmentApplicationVersion("one", "test")
					if err != nil {
						t.Fatal(err)
					}
					if *i != 1 {
						t.Errorf("unexpected version: expected 1, actual %d", i)
					}
				}
				_, err := s.Filesystem.Open(fmt.Sprintf("environments/%s/applications/test/manifests/manifests.yaml", "two"))
				if err == nil {
					t.Fatal("expected not to find this file!")
				}
			},
		},
		{
			Name:         "Deploy version when environment is locked fails LockBehavior=Fail",
			Transformers: makeTransformersDeployTestEnvLock(api.LockBehavior_Fail),
			ErrorTest: func(t *testing.T, err error) {
				var lockErr *LockedError
				if !errors.As(err, &lockErr) {
					t.Errorf("error must be a LockError, but got %#v", err)
				} else {
					expectedEnvLocks := map[string]Lock{
						"manual": {
							Message: "don't",
						},
					}
					if !reflect.DeepEqual(expectedEnvLocks["manual"].Message, lockErr.EnvironmentLocks["manual"].Message) {
						t.Errorf("unexpected environment locks: expected %q, actual: %q", expectedEnvLocks, lockErr.EnvironmentLocks)
					}
				}
			},
		},
		{
			Name:         "Deploy version ignoring locks when environment is locked LockBehavior=Ignore",
			Transformers: makeTransformersDeployTestEnvLock(api.LockBehavior_Ignore),
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				i, err := s.GetEnvironmentApplicationVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if *i != 1 {
					t.Errorf("unexpected version: expected 1, actual %d", i)
				}
			},
		},
		{
			Name:         "Deploy version ignoring locks when environment is locked LockBehavior=Queue",
			Transformers: makeTransformersDeployTestEnvLock(api.LockBehavior_Record),
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				i, err := s.GetEnvironmentApplicationVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if i != nil {
					t.Errorf("unexpected version: expected nil, actual %d", i)
				}
			},
		},
		{
			Name:         "Deploy version when application in environment is locked and config=LockBehaviourIgnoreAllLocks",
			Transformers: makeTransformersDeployTestAppLock(api.LockBehavior_Ignore),
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				i, err := s.GetEnvironmentApplicationVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if *i != 1 {
					t.Errorf("unexpected version: expected 1, actual %d", i)
				}
			},
		},
		{
			Name:         "Deploy version when application in environment is locked and config=LockBehaviourQueue",
			Transformers: makeTransformersDeployTestAppLock(api.LockBehavior_Record),
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				i, err := s.GetEnvironmentApplicationVersion("production", "test")
				if err != nil && err.Error() != "file does not exist" {
					t.Fatalf("unexpected error: %v", err.Error())
				}
				if i != nil {
					t.Errorf("unexpected version: expected nil, actual %d", i)
				}

				actualQueued, err := s.GetQueuedVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if *actualQueued != 1 {
					t.Errorf("unexpected version: expected 1, actual %d", i)
				}
			},
		},
		{
			Name:         "Deploy version ignoring locks when application in environment is locked and LockBehaviourFail",
			Transformers: makeTransformersDeployTestAppLock(api.LockBehavior_Fail),
			ErrorTest: func(t *testing.T, err error) {
				var lockErr *LockedError
				if !errors.As(err, &lockErr) {
					t.Errorf("error must be a LockError, but got %#v", err)
				} else {
					expectedEnvLocks := map[string]Lock{
						"manual": {
							Message: "don't",
						},
					}
					if !reflect.DeepEqual(expectedEnvLocks["manual"].Message, lockErr.EnvironmentApplicationLocks["manual"].Message) {
						t.Errorf("unexpected environment locks: expected %q, actual: %q", expectedEnvLocks, lockErr.EnvironmentApplicationLocks)
					}
				}
			},
		},
		{
			Name:         "Deploy twice LockBehavior=Queue and LockBehavior=Queue",
			Transformers: makeTransformersTwoDeploymentsWriteToQueue(api.LockBehavior_Record, api.LockBehavior_Record),
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				i, err := s.GetEnvironmentApplicationVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if i != nil {
					t.Errorf("unexpected version: expected nil, actual %d", i)
				}
				q, err := s.GetQueuedVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if q == nil {
					t.Errorf("unexpected version: expected 2, actual nil")
				} else {
					if *q != 2 {
						t.Errorf("unexpected version: expected 2, actual %d", *q)
					}
				}
			},
		},
		{
			Name:         "Deploy twice LockBehavior=Queue and LockBehavior=Ignore",
			Transformers: makeTransformersTwoDeploymentsWriteToQueue(api.LockBehavior_Record, api.LockBehavior_Ignore),
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				i, err := s.GetEnvironmentApplicationVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if i == nil {
					t.Errorf("unexpected version: expected 2, actual nil")
				} else {
					if *i != 2 {
						t.Errorf("unexpected version: expected 2, actual %d", *i)
					}
				}
				q, err := s.GetQueuedVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if q != nil {
					t.Errorf("unexpected version: expected nil, actual %d The queue should have been removed at this point!", *q)
				}
			},
		},
		{
			Name:         "Deploy twice LockBehavior=Ignore and LockBehavior=Queue",
			Transformers: makeTransformersTwoDeploymentsWriteToQueue(api.LockBehavior_Ignore, api.LockBehavior_Record),
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				i, err := s.GetEnvironmentApplicationVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if i == nil {
					t.Errorf("unexpected version: expected 1, actual nil")
				} else {
					if *i != 1 {
						t.Errorf("unexpected version: expected 1, actual %d", *i)
					}
				}
				q, err := s.GetQueuedVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if q == nil {
					t.Errorf("unexpected version: expected 2, actual nil")
				} else {
					if *q != 2 {
						t.Errorf("unexpected version: expected 2, actual %d", *q)
					}
				}
			},
		},
		{
			Name:         "Lock env AND app and then Deploy and unlock one lock ",
			Transformers: makeTransformersDoubleLock(api.LockBehavior_Record, false),
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				i, err := s.GetEnvironmentApplicationVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if i != nil {
					t.Errorf("unexpected version: expected nil, actual %d", *i)
				}
				q, err := s.GetQueuedVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if q == nil {
					t.Errorf("unexpected version: expected 1, actual nil")
				} else {
					if *q != 1 {
						t.Errorf("unexpected version: expected 1, actual %d", *q)
					}
				}
			},
		},
		{
			Name:         "Lock env AND app and then Deploy and unlock both locks",
			Transformers: makeTransformersDoubleLock(api.LockBehavior_Record, true),
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				i, err := s.GetEnvironmentApplicationVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if i != nil {
					t.Errorf("unexpected version %d: expected: nil", *i)
				}
				q, err := s.GetQueuedVersion("production", "test")
				if err != nil {
					t.Fatal(err)
				}
				if q == nil {
					t.Errorf("unexpected version: expected 1, actual nil")
				}
			},
		},
		{
			Name: "It creates an ArgoCd AppProject",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "staging", Config: config.EnvironmentConfig{
					ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Namespace: ptr.FromString("staging"),
							Server:    "localhost:8080",
						},
					},
				}},
				&CreateEnvironment{Environment: "production", Config: config.EnvironmentConfig{
					ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Name: "production",
						},
					},
				}},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"staging":    "stagingmanifest",
						"production": "stagingmanifest",
					},
				},
			},
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				{
					content, err := util.ReadFile(s.Filesystem, "argocd/v1alpha1/staging.yaml")
					if err != nil {
						t.Fatalf("unexpected error reading argocd manifest: %q", err)
					}
					expected := `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: staging
spec:
  description: staging
  destinations:
  - namespace: staging
    server: localhost:8080
  sourceRepos:
  - '*'
`
					if string(content) != expected {
						t.Fatalf("unexpected argocd manifest:\nexpected:\n%s\n\nactual:\n%s", expected, string(content))
					}
				}

				{

					content, err := util.ReadFile(s.Filesystem, "argocd/v1alpha1/production.yaml")
					if err != nil {
						t.Fatalf("unexpected error reading argocd manifest: %q", err)
					}
					expected := `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: production
spec:
  description: production
  destinations:
  - name: production
  sourceRepos:
  - '*'
`
					if string(content) != expected {
						t.Fatalf("unexpected argocd manifest:\nexpected:\n%s\n\nactual:\n%s", expected, string(content))
					}
				}
			},
		},
		{
			Name: "It creates an ArgoCd AppProject With Sync Windows",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "staging", Config: config.EnvironmentConfig{
					ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Namespace: ptr.FromString("not-staging"),
							Server:    "localhost:8080",
						},
						SyncWindows: []config.ArgoCdSyncWindow{
							{
								Schedule: "* * * * *",
								Duration: "1h",
								Kind:     "deny",
							},
						},
					},
				}},
			},
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				content, err := util.ReadFile(s.Filesystem, "argocd/v1alpha1/staging.yaml")
				if err != nil {
					t.Fatalf("unexpected error reading argocd manifest: %q", err)
				}
				expected := `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: staging
spec:
  description: staging
  destinations:
  - namespace: not-staging
    server: localhost:8080
  sourceRepos:
  - '*'
  syncWindows:
  - applications:
    - '*'
    duration: 1h
    kind: deny
    manualSync: true
    schedule: '* * * * *'
`
				if string(content) != expected {
					t.Fatalf("unexpected argocd manifest:\nexpected:\n%s\n\nactual:\n%s", expected, string(content))
				}
			},
		},
		{
			Name: "It creates an ArgoCd AppProject With global resources",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "staging", Config: config.EnvironmentConfig{
					ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Namespace: ptr.FromString("not-staging"),
							Server:    "localhost:8080",
						},
						ClusterResourceWhitelist: []config.AccessEntry{
							{
								Group: "*",
								Kind:  "MyClusterWideResource",
							},
							{
								Group: "*",
								Kind:  "ClusterSecretStore",
							},
						},
					},
				}},
			},
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				content, err := util.ReadFile(s.Filesystem, "argocd/v1alpha1/staging.yaml")
				if err != nil {
					t.Fatalf("unexpected error reading argocd manifest: %q", err)
				}
				expected := `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: staging
spec:
  clusterResourceWhitelist:
  - group: '*'
    kind: MyClusterWideResource
  - group: '*'
    kind: ClusterSecretStore
  description: staging
  destinations:
  - namespace: not-staging
    server: localhost:8080
  sourceRepos:
  - '*'
`
				if string(content) != expected {
					t.Fatalf("unexpected argocd manifest:\ndiff:\n%s\n\n", godebug.Diff(expected, string(content)))
				}
			},
		},
		{
			Name: "It creates ArgoCd Applications",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "staging", Config: config.EnvironmentConfig{
					ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Namespace: ptr.FromString("staging"),
							Server:    "localhost:8080",
						},
					},
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				}},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"staging": "stagingmanifest",
					},
					Team: "team1",
				},
				&CreateApplicationVersion{
					Application: "test2",
					Manifests: map[string]string{
						"staging": "stagingmanifest",
					},
					Team: "team2",
				},
			},
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				content, err := util.ReadFile(s.Filesystem, "argocd/v1alpha1/staging.yaml")
				if err != nil {
					t.Fatalf("unexpected error reading argocd manifest: %q", err)
				}
				// The repository URL changes every time because the repository is in a tmp dir.
				repoURL := regexp.MustCompile(`repoURL: ([^\n]+)\n`).FindStringSubmatch(string(content))[1]
				expected := fmt.Sprintf(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: staging
spec:
  description: staging
  destinations:
  - namespace: staging
    server: localhost:8080
  sourceRepos:
  - '*'
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    com.freiheit.kuberpult/application: test
    com.freiheit.kuberpult/environment: staging
    com.freiheit.kuberpult/team: team1
  finalizers:
  - resources-finalizer.argocd.argoproj.io
  labels:
    com.freiheit.kuberpult/team: team1
  name: staging-test
spec:
  destination:
    namespace: staging
    server: localhost:8080
  project: staging
  source:
    path: environments/staging/applications/test/manifests
    repoURL: %s
    targetRevision: master
  syncPolicy:
    automated:
      allowEmpty: true
      prune: true
      selfHeal: true
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    com.freiheit.kuberpult/application: test2
    com.freiheit.kuberpult/environment: staging
    com.freiheit.kuberpult/team: team2
  finalizers:
  - resources-finalizer.argocd.argoproj.io
  labels:
    com.freiheit.kuberpult/team: team2
  name: staging-test2
spec:
  destination:
    namespace: staging
    server: localhost:8080
  project: staging
  source:
    path: environments/staging/applications/test2/manifests
    repoURL: %s
    targetRevision: master
  syncPolicy:
    automated:
      allowEmpty: true
      prune: true
      selfHeal: true
`, repoURL, repoURL)
				if string(content) != expected {
					t.Fatalf("unexpected argocd manifest:\n%s", godebug.Diff(expected, string(content)))
				}
			},
		},
		{
			Name: "It creates ArgoCd Applications with labels",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "staging", Config: config.EnvironmentConfig{
					ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Namespace: ptr.FromString("staging"),
							Server:    "localhost:8080",
						},
						ApplicationAnnotations: map[string]string{
							"b": "foo",
							"a": "bar",
						},
					},
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				}},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"staging": "stagingmanifest",
					},
				},
			},
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				content, err := util.ReadFile(s.Filesystem, "argocd/v1alpha1/staging.yaml")
				if err != nil {
					t.Fatalf("unexpected error reading argocd manifest: %q", err)
				}
				// The repository URL changes every time because the repository is in a tmp dir.
				repoURL := regexp.MustCompile(`repoURL: ([^\n]+)\n`).FindStringSubmatch(string(content))[1]
				expected := fmt.Sprintf(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: staging
spec:
  description: staging
  destinations:
  - namespace: staging
    server: localhost:8080
  sourceRepos:
  - '*'
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    a: bar
    b: foo
    com.freiheit.kuberpult/application: test
    com.freiheit.kuberpult/environment: staging
    com.freiheit.kuberpult/team: ""
  finalizers:
  - resources-finalizer.argocd.argoproj.io
  labels:
    com.freiheit.kuberpult/team: ""
  name: staging-test
spec:
  destination:
    namespace: staging
    server: localhost:8080
  project: staging
  source:
    path: environments/staging/applications/test/manifests
    repoURL: %s
    targetRevision: master
  syncPolicy:
    automated:
      allowEmpty: true
      prune: true
      selfHeal: true
`, repoURL)
				if string(content) != expected {
					t.Fatalf("unexpected argocd manifest:\ndiff:\n%s\n\n", godebug.Diff(expected, string(content)))
				}
			},
		},
		{
			Name: "It creates ArgoCd Applications with ignore differences",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "staging", Config: config.EnvironmentConfig{
					ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Namespace: ptr.FromString("staging"),
							Server:    "localhost:8080",
						},
						IgnoreDifferences: []config.ArgoCdIgnoreDifference{
							{
								Group: "apps",
								Kind:  "Deployment",
								JSONPointers: []string{
									"/spec/replicas",
								},
								JqPathExpressions: []string{
									".foo.bar",
								},
							},
						},
					},
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				}},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"staging": "stagingmanifest",
					},
				},
			},
			Test: func(t *testing.T, s *State) {
				// check that the state reads the correct versions
				content, err := util.ReadFile(s.Filesystem, "argocd/v1alpha1/staging.yaml")
				if err != nil {
					t.Fatalf("unexpected error reading argocd manifest: %q", err)
				}
				// The repository URL changes every time because the repository is in a tmp dir.
				repoURL := regexp.MustCompile(`repoURL: ([^\n]+)\n`).FindStringSubmatch(string(content))[1]
				expected := fmt.Sprintf(`apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: staging
spec:
  description: staging
  destinations:
  - namespace: staging
    server: localhost:8080
  sourceRepos:
  - '*'
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    com.freiheit.kuberpult/application: test
    com.freiheit.kuberpult/environment: staging
    com.freiheit.kuberpult/team: ""
  finalizers:
  - resources-finalizer.argocd.argoproj.io
  labels:
    com.freiheit.kuberpult/team: ""
  name: staging-test
spec:
  destination:
    namespace: staging
    server: localhost:8080
  ignoreDifferences:
  - group: apps
    jqPathExpressions:
    - .foo.bar
    jsonPointers:
    - /spec/replicas
    kind: Deployment
  project: staging
  source:
    path: environments/staging/applications/test/manifests
    repoURL: %s
    targetRevision: master
  syncPolicy:
    automated:
      allowEmpty: true
      prune: true
      selfHeal: true
`, repoURL)
				if string(content) != expected {
					t.Fatalf("unexpected argocd manifest:\ndiff:\n%s\n\n", godebug.Diff(expected, string(content)))
				}
			},
		},
		{
			Name:          "CreateEnvironment errors in bootstrap mode",
			BootstrapMode: true,
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "production", Config: c1},
			},
			ErrorTest: func(t *testing.T, err error) {
				expectedError := "Cannot create or update configuration in bootstrap mode. Please update configuration in config map instead."
				if err.Error() != expectedError {
					t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
				}
			},
		},
		{
			Name:          "CreateEnvironment does not error in bootstrap mode without configuration",
			BootstrapMode: true,
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production",
				},
			},
			Test: func(t *testing.T, s *State) {},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()
			repo, err := New(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:            remoteDir,
					Path:           localDir,
					CommitterEmail: "kuberpult@freiheit.com",
					CommitterName:  "kuberpult",
					BootstrapMode:  tc.BootstrapMode,
				},
			)
			if err != nil {
				t.Fatal(err)
			}

			for i, tf := range tc.Transformers {
				ctxWithTime := withTimeNow(testutil.MakeTestContext(), timeNowOld)
				err = repo.Apply(ctxWithTime, tf)
				if err != nil {
					if tc.ErrorTest != nil && i == len(tc.Transformers)-1 {
						tc.ErrorTest(t, err)
						return
					} else {
						t.Fatalf("error applying transformations %q: %s", tf, err.Error())
					}
				}
			}
			if tc.ErrorTest != nil {
				t.Fatalf("expected an error but got none")
			}
			tc.Test(t, repo.State())
		})
	}
}

func makeTransformersDeployTestEnvLock(lock api.LockBehavior) []Transformer {
	return []Transformer{
		&CreateEnvironment{Environment: "production"},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "productionmanifest",
			},
		},
		&CreateEnvironmentLock{
			Environment: "production",
			Message:     "don't",
			LockId:      "manual",
		},
		&DeployApplicationVersion{
			Environment:   "production",
			Application:   "test",
			Version:       1,
			LockBehaviour: lock,
		},
	}
}

func makeTransformersDeployTestAppLock(lock api.LockBehavior) []Transformer {
	return []Transformer{
		&CreateEnvironment{Environment: "production"},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "productionmanifest",
			},
		},
		&CreateEnvironmentApplicationLock{
			Environment: "production",
			Application: "test",
			Message:     "don't",
			LockId:      "manual",
		},
		&DeployApplicationVersion{
			Environment:   "production",
			Application:   "test",
			Version:       1,
			LockBehaviour: lock,
		},
	}
}

func makeTransformersTwoDeploymentsWriteToQueue(lockA api.LockBehavior, lockB api.LockBehavior) []Transformer {
	return []Transformer{
		&CreateEnvironment{Environment: "production"},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "productionmanifest",
			},
		},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "productionmanifest",
			},
		},
		&CreateEnvironmentLock{
			Environment: "production",
			Message:     "stop",
			LockId:      "test",
		},
		&DeployApplicationVersion{
			Environment:   "production",
			Application:   "test",
			Version:       1,
			LockBehaviour: lockA,
		},
		&DeployApplicationVersion{
			Environment:   "production",
			Application:   "test",
			Version:       2,
			LockBehaviour: lockB,
		},
	}
}

func makeTransformersDoubleLock(lock api.LockBehavior, unlockBoth bool) []Transformer {
	res := []Transformer{
		&CreateEnvironment{Environment: "production"},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "productionmanifest",
			},
		},
		&CreateEnvironmentLock{
			Environment: "production",
			Message:     "stop",
			LockId:      "test",
		},
		&CreateEnvironmentApplicationLock{
			Environment: "production",
			Application: "test",
			LockId:      "test",
			Message:     "stop",
		},
		&DeployApplicationVersion{
			Environment:   "production",
			Application:   "test",
			Version:       1,
			LockBehaviour: lock,
		},
		&DeleteEnvironmentLock{
			Environment: "production",
			LockId:      "test",
		},
		// we still have an app lock here, so no deployment should happen!
	}
	if unlockBoth {
		res = append(res, &DeleteEnvironmentApplicationLock{
			Environment: "production",
			Application: "test",
			LockId:      "test",
		})
	}
	return res
}

func makeTransformersForDelete(numVersions uint64) []Transformer {
	res := []Transformer{
		&CreateEnvironment{Environment: envProduction},
	}
	var v uint64
	for v = 1; v <= numVersions; v++ {
		res = append(res, &CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				envProduction: "productionmanifest",
			},
		})
		res = append(res, &DeployApplicationVersion{
			Environment:   envProduction,
			Application:   "test",
			Version:       v,
			LockBehaviour: api.LockBehavior_Fail,
		})
	}
	return res
}

func setupRepositoryTest(t *testing.T) Repository {
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Start()
	cmd.Wait()
	repo, err := New(
		testutil.MakeTestContext(),
		RepositoryConfig{
			URL:            remoteDir,
			Path:           localDir,
			CommitterEmail: "kuberpult@freiheit.com",
			CommitterName:  "kuberpult",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

// Injects an error in the filesystem of the state
type injectErr struct {
	Transformer
	collector *testfs.UsageCollector
	operation testfs.Operation
	filename  string
	err       error
}

func (i *injectErr) Transform(ctx context.Context, state *State) (string, error) {
	original := state.Filesystem
	state.Filesystem = i.collector.WithError(state.Filesystem, i.operation, i.filename, i.err)
	s, err := i.Transformer.Transform(ctx, state)
	state.Filesystem = original
	return s, err
}

func TestAllErrorsHandledDeleteEnvironmentLock(t *testing.T) {
	t.Parallel()
	collector := &testfs.UsageCollector{}
	tcs := []struct {
		name          string
		operation     testfs.Operation
		filename      string
		expectedError string
	}{
		{
			name: "delete lock succeeds",
		},
		{
			name:          "delete lock fails",
			operation:     testfs.REMOVE,
			filename:      "environments/dev/locks/foo",
			expectedError: "failed to delete directory \"environments/dev/locks/foo\": obscure error",
		},
		{
			name:          "readdir fails",
			operation:     testfs.READDIR,
			filename:      "environments/dev/applications",
			expectedError: "environment applications for \"dev\" not found: obscure error",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := setupRepositoryTest(t)
			err := repo.Apply(testutil.MakeTestContext(), &CreateEnvironment{
				Environment: "dev",
			})
			if err != nil {
				t.Fatal(err)
			}
			err = repo.Apply(testutil.MakeTestContext(), &injectErr{
				Transformer: &DeleteEnvironmentLock{
					Environment:    "dev",
					LockId:         "foo",
					Authentication: Authentication{RBACConfig: auth.RBACConfig{DexEnabled: false}},
				},
				collector: collector,
				operation: tc.operation,
				filename:  tc.filename,
				err:       fmt.Errorf("obscure error"),
			})
			if tc.expectedError != "" {
				if err.Error() != tc.expectedError {
					t.Errorf("expected error to be %q but got %q", tc.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, but got %q", err)
				}
			}
		})
	}
	untested := collector.UntestedOps()
	for _, op := range untested {
		t.Errorf("Untested operations %s %s", op.Operation, op.Filename)
	}
}

func TestAllErrorsHandledDeleteEnvironmentApplicationLock(t *testing.T) {
	t.Parallel()
	collector := &testfs.UsageCollector{}
	tcs := []struct {
		name          string
		operation     testfs.Operation
		filename      string
		expectedError string
	}{
		{
			name: "delete lock succedes",
		},
		{
			name:          "delete lock fails",
			operation:     testfs.REMOVE,
			filename:      "environments/dev/applications/bar/locks/foo",
			expectedError: "failed to delete directory \"environments/dev/applications/bar/locks/foo\": obscure error",
		},
		{
			name:          "stat queue failes",
			operation:     testfs.READLINK,
			filename:      "environments/dev/applications/bar/queued_version",
			expectedError: "failed reading symlink \"environments/dev/applications/bar/queued_version\": obscure error",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := setupRepositoryTest(t)
			err := repo.Apply(testutil.MakeTestContext(), &CreateEnvironment{
				Environment: "dev",
			})
			if err != nil {
				t.Fatal(err)
			}
			err = repo.Apply(testutil.MakeTestContext(), &injectErr{
				Transformer: &DeleteEnvironmentApplicationLock{
					Environment: "dev",
					Application: "bar",
					LockId:      "foo",
				},
				collector: collector,
				operation: tc.operation,
				filename:  tc.filename,
				err:       fmt.Errorf("obscure error"),
			})
			if tc.expectedError != "" {
				if err == nil {
					t.Errorf("expected error to be %q but got <nil>", tc.expectedError)
				} else if err.Error() != tc.expectedError {

					t.Errorf("expected error to be %q but got %q", tc.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, but got %q", err)
				}
			}
		})
	}
	untested := collector.UntestedOps()
	for _, op := range untested {
		t.Errorf("Untested operations %s %s", op.Operation, op.Filename)
	}
}

func mockSendMetrics(repo Repository, interval time.Duration) <-chan bool {
	ch := make(chan bool, 1)
	go RegularlySendDatadogMetrics(repo, interval, func(repo Repository) { ch <- true })
	return ch
}

func TestSendRegularlyDatadogMetrics(t *testing.T) {
	tcs := []struct {
		Name          string
		shouldSucceed bool
	}{
		{
			Name: "Testing ticker",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo := setupRepositoryTest(t)

			select {
			case <-mockSendMetrics(repo, 1):
			case <-time.After(4 * time.Second):
				t.Fatal("An error occurred during the go routine")
			}

		})
	}
}

func TestUpdateDatadogMetrics(t *testing.T) {
	tcs := []struct {
		Name          string
		Transformers  []Transformer
		expectedError string
		shouldSucceed bool
	}{
		{
			Name: "Application Lock metric is sent",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
				},
				&CreateEnvironmentApplicationLock{
					Environment: "acceptance",
					Application: "app1",
					LockId:      "22133",
					Message:     "test",
				},
			},
			shouldSucceed: true,
		},
		{
			Name: "Application Lock metric is sent",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
				},
				&CreateEnvironmentLock{
					Environment: "acceptance",
					LockId:      "22133",
					Message:     "test",
				},
			},
			shouldSucceed: true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := setupRepositoryTest(t)
			_, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), tc.Transformers...)

			if err != nil {
				t.Fatalf("Got an unexpected error: %v", err)
			}

			err = UpdateDatadogMetrics(repo.State())

			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				} else {
					actualMsg := err.Error()
					if actualMsg != tc.expectedError {
						t.Fatalf("expected a different error.\nExpected: %q\nGot %q", tc.expectedError, actualMsg)
					}
				}
			}
		})
	}
}

func TestDeleteEnvFromApp(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedError     string
		expectedCommitMsg string
		shouldSucceed     bool
	}{
		{
			Name: "Success",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
				},
				&DeployApplicationVersion{
					Environment:   envProduction,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_Fail,
				},
				&DeleteEnvFromApp{
					Application: "app1",
					Environment: envProduction,
				},
			},
			expectedError:     "",
			expectedCommitMsg: "Environment 'production' was removed from application 'app1' successfully.",
			shouldSucceed:     true,
		},
		{
			Name: "Success Double Delete",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
				},
				&DeployApplicationVersion{
					Environment:   envProduction,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_Fail,
				},
				&DeleteEnvFromApp{
					Application: "app1",
					Environment: envProduction,
				},
				&DeleteEnvFromApp{
					Application: "app1",
					Environment: envProduction,
				},
			},
			expectedError:     "",
			expectedCommitMsg: "Attempted to remove environment 'production' from application 'app1' but it did not exist.",
			shouldSucceed:     true,
		},
		{
			Name: "fail to provide app name",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
				},
				&DeployApplicationVersion{
					Environment:   envProduction,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_Fail,
				},
				&DeleteEnvFromApp{
					Environment: envProduction,
				},
			},
			expectedError:     "DeleteEnvFromApp app '' on env 'production': Need to provide the application",
			expectedCommitMsg: "",
			shouldSucceed:     false,
		},
		{
			Name: "fail to provide env name",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "productionmanifest",
					},
				},
				&DeployApplicationVersion{
					Environment:   envProduction,
					Application:   "app1",
					Version:       1,
					LockBehaviour: api.LockBehavior_Fail,
				},
				&DeleteEnvFromApp{
					Application: "app1",
				},
			},
			expectedError:     "DeleteEnvFromApp app 'app1' on env '': Need to provide the environment",
			expectedCommitMsg: "",
			shouldSucceed:     false,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := setupRepositoryTest(t)
			commitMsg, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), tc.Transformers...)
			// note that we only check the LAST error here:
			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
				}
				actualMsg := commitMsg[len(commitMsg)-1]
				if actualMsg != tc.expectedCommitMsg {
					t.Fatalf("expected a different message.\nExpected: %q\nGot %q", tc.expectedCommitMsg, actualMsg)
				}
			} else {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				} else {
					actualMsg := err.Error()
					if actualMsg != tc.expectedError {
						t.Fatalf("expected a different error.\nExpected: %q\nGot %q", tc.expectedError, actualMsg)
					}
				}
			}
		})
	}
}

func TestDeleteLocks(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []Transformer
		expectedError     string
		expectedCommitMsg string
		shouldSucceed     bool
	}{
		{
			Name: "Success delete env lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&DeleteEnvironmentLock{
					Environment: envProduction,
					LockId:      "l123",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "Deleted lock \"l123\" on environment \"production\"",
			shouldSucceed:     true,
		},
		{
			Name: "Success delete app lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&DeleteEnvironmentApplicationLock{
					Environment: envProduction,
					Application: "app1",
					LockId:      "l123",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "Deleted lock \"l123\" on environment \"production\" for application \"app1\"",
			shouldSucceed:     true,
		},
		{
			Name: "Success create env lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironmentLock{
					Environment: envProduction,
					LockId:      "l123",
					Message:     "my lock",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "Created lock \"l123\" on environment \"production\"",
			shouldSucceed:     true,
		},
		{
			Name: "Success create app lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&CreateEnvironmentApplicationLock{
					Environment: envProduction,
					Application: "app1",
					LockId:      "l123",
					Message:     "my lock",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "Created lock \"l123\" on environment \"production\" for application \"app1\"",
			shouldSucceed:     true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := setupRepositoryTest(t)
			commitMsg, _, err := repo.ApplyTransformersInternal(testutil.MakeTestContext(), tc.Transformers...)
			// note that we only check the LAST error here:
			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
				}
				actualMsg := commitMsg[len(commitMsg)-1]
				if actualMsg != tc.expectedCommitMsg {
					t.Fatalf("expected a different message.\nExpected: %q\nGot %q", tc.expectedCommitMsg, actualMsg)
				}
			} else {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				} else {
					actualMsg := err.Error()
					if actualMsg != tc.expectedError {
						t.Fatalf("expected a different error.\nExpected: %q\nGot %q", tc.expectedError, actualMsg)
					}
				}
			}
		})
	}
}
