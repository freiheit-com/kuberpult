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
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	git "github.com/libgit2/git2go/v34"
)

func TestBootstrapModeNew(t *testing.T) {
	tcs := []struct {
		Name          string
		PreInitialize bool
	}{
		{
			Name:          "New in empty repo",
			PreInitialize: false,
		},
		{
			Name:          "New in existing repo",
			PreInitialize: true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			// create a remote
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")

			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()

			if tc.PreInitialize {
				_, err := New(
					testutil.MakeTestContext(),
					RepositoryConfig{
						URL:                 "file://" + remoteDir,
						Path:                localDir,
						ArgoCdGenerateFiles: true,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
			}

			environmentConfigsPath := filepath.Join(remoteDir, "..", "environment_configs.json")

			repo, err := New(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:                    "file://" + remoteDir,
					Path:                   localDir,
					BootstrapMode:          true,
					EnvironmentConfigsPath: environmentConfigsPath,
					ArgoCdGenerateFiles:    true,
				},
			)
			if err != nil {
				t.Fatalf("New: Expected no error, error %e was thrown", err)
			}

			state := repo.State()
			if !state.BootstrapMode {
				t.Fatalf("Bootstrap mode not preserved")
			}
		})
	}
}

func TestBootstrapModeReadConfig(t *testing.T) {
	tcs := []struct {
		Name string
	}{
		{
			Name: "Config read correctly",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			// create a remote
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")

			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()

			environmentConfigsPath := filepath.Join(remoteDir, "..", "environment_configs.json")
			if err := os.WriteFile(environmentConfigsPath, []byte(`{"uniqueEnv": {"environmentGroup": "testgroup321", "upstream": {"latest": true}}}`), fs.FileMode(0644)); err != nil {
				t.Fatal(err)
			}

			repo, err := New(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:                    "file://" + remoteDir,
					Path:                   localDir,
					BootstrapMode:          true,
					EnvironmentConfigsPath: environmentConfigsPath,
					ArgoCdGenerateFiles:    true,
				},
			)
			if err != nil {
				t.Fatalf("New: Expected no error, error %e was thrown", err)
			}

			state := repo.State()
			if !state.BootstrapMode {
				t.Fatalf("Bootstrap mode not preserved")
			}
			configs, err := state.GetEnvironmentConfigs()
			if err != nil {
				t.Fatal(err)
			}
			if len(configs) != 1 {
				t.Fatal("Configuration not read properly")
			}
			if configs["uniqueEnv"].Upstream.Latest != true {
				t.Fatal("Configuration not read properly")
			}
			if configs["uniqueEnv"].EnvironmentGroup == nil {
				t.Fatalf("EnvironmentGroup not read, found nil")
			}
			if *configs["uniqueEnv"].EnvironmentGroup != "testgroup321" {
				t.Fatalf("EnvironmentGroup not read, found '%s' instead", *configs["uniqueEnv"].EnvironmentGroup)
			}
		})
	}
}

func TestBootstrapError(t *testing.T) {
	tcs := []struct {
		Name          string
		ConfigContent string
	}{
		{
			Name:          "Invalid json in bootstrap configuration",
			ConfigContent: `{"development": "upstream": {"latest": true}}}`,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			// create a remote
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()

			environmentConfigsPath := filepath.Join(remoteDir, "..", "environment_configs.json")
			if err := os.WriteFile(environmentConfigsPath, []byte(tc.ConfigContent), fs.FileMode(0644)); err != nil {
				t.Fatal(err)
			}

			_, err := New(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:                    "file://" + remoteDir,
					Path:                   localDir,
					BootstrapMode:          true,
					EnvironmentConfigsPath: environmentConfigsPath,
					ArgoCdGenerateFiles:    true,
				},
			)
			if err == nil {
				t.Fatalf("New: Expected error but no error was thrown")
			}
		})
	}
}

func TestConfigValidity(t *testing.T) {
	tcs := []struct {
		Name          string
		ConfigContent string
		ErrorExpected bool
	}{
		{
			Name:          "Initialization with valid config.json file works",
			ConfigContent: "{\"upstream\": {\"latest\": true }}",
			ErrorExpected: false,
		},
		{
			Name:          "Initialization with invalid config.json file throws error",
			ConfigContent: "{\"upstream\": \"latest\": true }}",
			ErrorExpected: true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			// create a remote
			workdir := t.TempDir()
			remoteDir := path.Join(workdir, "remote")
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()

			workdir = t.TempDir()
			cmd = exec.Command("git", "clone", remoteDir, workdir) // Clone git dir
			_, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Logf("stderr: %s\n", exitErr.Stderr)
				}
				t.Fatal(err)
			}

			if err := os.MkdirAll(path.Join(workdir, "environments", "development"), 0700); err != nil {
				t.Fatal(err)
			}

			configFilePath := path.Join(workdir, "environments", "development", "config.json")
			if err := os.WriteFile(configFilePath, []byte(tc.ConfigContent), 0666); err != nil {
				t.Fatal(err)
			}
			cmd = exec.Command("git", "add", configFilePath) // Add a new file to git
			cmd.Dir = workdir
			_, err = cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Logf("stderr: %s\n", exitErr.Stderr)
				}
				t.Fatal(err)
			}
			cmd = exec.Command("git", "commit", "-m", "valid config") // commit the new file
			cmd.Dir = workdir
			cmd.Env = []string{
				"GIT_AUTHOR_NAME=kuberpult",
				"GIT_COMMITTER_NAME=kuberpult",
				"EMAIL=test@kuberpult.com",
			}
			_, err = cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Logf("stderr: %s\n", exitErr.Stderr)
				}
				t.Fatal(err)
			}
			cmd = exec.Command("git", "push", "origin", "HEAD") // push the new commit
			cmd.Dir = workdir
			_, err = cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Logf("stderr: %s\n", exitErr.Stderr)
				}
				t.Fatal(err)
			}

			_, err = New(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:                 remoteDir,
					Path:                t.TempDir(),
					ArgoCdGenerateFiles: true,
				},
			)

			if tc.ErrorExpected {
				if err == nil {
					t.Errorf("Initialized even though config.json was incorrect")
				}
			} else {
				if err != nil {
					t.Errorf("Initialization failed with valid config.json")
				}
			}

		})
	}
}

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
			resp := repo.Push(testutil.MakeTestContext(), func() error {
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
			actualError := defaultPushUpdate(tc.InputBranch, &success)(tc.InputRefName, tc.InputStatus)
			if success != tc.ExpectedSuccess {
				t.Fatal(fmt.Sprintf("expected sucess=%t but got %t", tc.ExpectedSuccess, success))
			}
			if actualError != nil {
				t.Fatal(fmt.Sprintf("expected no error but got %s but got none", actualError))
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
			repo := setupRepositoryTest(t)
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

func setupRepositoryTest(t *testing.T) Repository {
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Start()
	cmd.Wait()
	t.Logf("test created dir: %s", localDir)
	repo, err := New(
		testutil.MakeTestContext(),
		RepositoryConfig{
			URL:                    remoteDir,
			Path:                   localDir,
			CommitterEmail:         "kuberpult@freiheit.com",
			CommitterName:          "kuberpult",
			EnvironmentConfigsPath: filepath.Join(remoteDir, "..", "environment_configs.json"),
			ArgoCdGenerateFiles:    true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}
