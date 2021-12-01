/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
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
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/go-git/go-billy/v5/util"
	godebug "github.com/kylelemons/godebug/diff"
)

const (
	envAcceptance      = "acceptance"
	envProduction      = "production"
	additionalVersions = 7
)

// Tests various error cases in the Undeploy endpoint, specifically the error messages returned.
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
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatal(err)
			}

			commitMsg, _, err := repo.ApplyTransformersInternal(tc.Transformers...)
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
					Environment: "doesnotexistenvironment",
				},
			},
			expectedError:     "could not find environment config for 'doesnotexistenvironment'",
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
							Latest:      true,
						},
					},
				},
				&CreateEnvironmentLock{
					Environment: envAcceptance,
					Message:     "don't",
					LockId:      "care",
				},
				&ReleaseTrain{
					Environment: envAcceptance,
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
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatal(err)
			}

			commitMsg, _, err := repo.ApplyTransformersInternal(tc.Transformers...)
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

func TestTransformer(t *testing.T) {
	c1 := config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}}

	tcs := []struct {
		Name         string
		Transformers []Transformer
		Test         func(t *testing.T, s *State)
		ErrorTest    func(t *testing.T, err error)
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
					Environment: envProduction,
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
					},
				}
				if !reflect.DeepEqual(locks, expected) {
					t.Fatalf("mismatched locks. expected: %#v, actual: %#v", expected, locks)
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
					if !reflect.DeepEqual(expectedEnvLocks, lockErr.EnvironmentLocks) {
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
			Transformers: makeTransformersDeployTestEnvLock(api.LockBehavior_Queue),
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
			Transformers: makeTransformersDeployTestAppLock(api.LockBehavior_Queue),
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
					if !reflect.DeepEqual(expectedEnvLocks, lockErr.EnvironmentApplicationLocks) {
						t.Errorf("unexpected environment locks: expected %q, actual: %q", expectedEnvLocks, lockErr.EnvironmentApplicationLocks)
					}
				}
			},
		},
		{
			Name:         "Deploy twice LockBehavior=Queue and LockBehavior=Queue",
			Transformers: makeTransformersTwoDeploymentsWriteToQueue(api.LockBehavior_Queue, api.LockBehavior_Queue),
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
			Transformers: makeTransformersTwoDeploymentsWriteToQueue(api.LockBehavior_Queue, api.LockBehavior_Ignore),
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
			Transformers: makeTransformersTwoDeploymentsWriteToQueue(api.LockBehavior_Ignore, api.LockBehavior_Queue),
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
			Transformers: makeTransformersDoubleLock(api.LockBehavior_Queue, false),
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
			Transformers: makeTransformersDoubleLock(api.LockBehavior_Queue, true),
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
				if q != nil {
					t.Errorf("unexpected version: expected nil, actual %d", *q)
				}
			},
		},
		{
			Name: "It creates an ArgoCd AppProject",
			Transformers: []Transformer{
				&CreateEnvironment{Environment: "staging", Config: config.EnvironmentConfig{
					ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Namespace: "staging",
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
							Namespace: "not-staging",
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
    clusters:
    - '*'
    duration: 1h
    kind: deny
    manualSync: true
    namespaces:
    - '*'
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
							Namespace: "not-staging",
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
							Namespace: "staging",
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
				},
				&CreateApplicationVersion{
					Application: "test2",
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
      prune: true
      selfHeal: true
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
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
							Namespace: "staging",
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
							Namespace: "staging",
							Server:    "localhost:8080",
						},
						IgnoreDifferences: []config.ArgoCdIgnoreDifference{
							{
								Group: "apps",
								Kind:  "Deployment",
								JSONPointers: []string{
									"/spec/replicas",
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
  name: staging-test
spec:
  destination:
    namespace: staging
    server: localhost:8080
  ignoreDifferences:
  - group: apps
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
      prune: true
      selfHeal: true
`, repoURL)
				if string(content) != expected {
					t.Fatalf("unexpected argocd manifest:\ndiff:\n%s\n\n", godebug.Diff(expected, string(content)))
				}
			},
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
			repo, err := NewWait(
				context.Background(),
				Config{
					URL:            remoteDir,
					Path:           localDir,
					CommitterEmail: "kuberpult@freiheit.com",
					CommitterName:  "kuberpult",
				},
			)
			if err != nil {
				t.Fatal(err)
			}

			for i, tf := range tc.Transformers {
				err = repo.Apply(context.Background(), tf)
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

func setupRepositoryTest(t *testing.T) (Repository, error) {
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Start()
	cmd.Wait()
	repo, err := NewWait(
		context.Background(),
		Config{
			URL:            remoteDir,
			Path:           localDir,
			CommitterEmail: "kuberpult@freiheit.com",
			CommitterName:  "kuberpult",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, nil
}
