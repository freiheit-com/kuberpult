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
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testssh"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-git/go-billy/v5/util"
	"github.com/google/go-cmp/cmp"
	git "github.com/libgit2/git2go/v34"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNew(t *testing.T) {
	tcs := []struct {
		Name   string
		Branch string
		Setup  func(t *testing.T, remoteDir, localDir string)
		Test   func(t *testing.T, repo Repository, remoteDir string)
	}{
		{
			Name:  "new in empty directory works",
			Setup: func(_ *testing.T, _, _ string) {},
		},
		{
			Name: "new in initialized repository works",
			Setup: func(t *testing.T, remoteDir, localDir string) {
				// run the initialization code once
				_, err := New(
					testutil.MakeTestContext(),
					RepositoryConfig{
						URL:  "file://" + remoteDir,
						Path: localDir,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
			},
			Test: func(t *testing.T, repo Repository, remoteDir string) {
				state := repo.State()
				entries, err := state.Filesystem.ReadDir("")
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) > 0 {
					t.Errorf("repository is not empty but contains %d entries", len(entries))
				}
			},
		},
		{
			Name: "new in initialized repository with data works",
			Setup: func(t *testing.T, remoteDir, localDir string) {
				// run the initialization code once
				repo, err := New(
					testutil.MakeTestContext(),
					RepositoryConfig{
						URL:  remoteDir,
						Path: localDir,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				err = repo.Apply(testutil.MakeTestContext(), &CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						"development": "foo",
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			Test: func(t *testing.T, repo Repository, remoteDir string) {
				state := repo.State()
				entries, err := state.Filesystem.ReadDir("applications/foo/releases")
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 1 {
					t.Errorf("applications/foo/releases doesn't contain 1 but %d entries", len(entries))
				}
			},
		},
		{
			Name: "new with empty repository but non-empty remote works",
			Setup: func(t *testing.T, remoteDir, localDir string) {
				// run the initialization code once
				repo, err := New(
					testutil.MakeTestContext(),
					RepositoryConfig{
						URL:  remoteDir,
						Path: t.TempDir(),
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				err = repo.Apply(testutil.MakeTestContext(), &CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						"development": "foo",
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			Test: func(t *testing.T, repo Repository, remoteDir string) {
				state := repo.State()
				entries, err := state.Filesystem.ReadDir("applications/foo/releases")
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 1 {
					t.Errorf("applications/foo/releases doesn't contain 1 but %d entries", len(entries))
				}
			},
		},
		{
			Name:   "new with changed branch works",
			Branch: "not-master",
			Setup:  func(t *testing.T, remoteDir, localDir string) {},
			Test: func(t *testing.T, repo Repository, remoteDir string) {
				err := repo.Apply(testutil.MakeTestContext(), &CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						"development": "foo",
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("git", "--git-dir="+remoteDir, "rev-parse", "not-master")
				out, err := cmd.Output()
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						t.Logf("stderr: %s\n", exitErr.Stderr)
					}
					t.Fatal(err)
				}
				state := repo.State()
				localRev := state.Commit.Id().String()
				if localRev != strings.TrimSpace(string(out)) {
					t.Errorf("mismatched revision. expected %q but got %q", localRev, strings.TrimSpace(string(out)))
				}
			},
		},
		{
			Name:   "old with changed branch works",
			Branch: "master",
			Setup:  func(t *testing.T, remoteDir, localDir string) {},
			Test: func(t *testing.T, repo Repository, remoteDir string) {
				workdir := t.TempDir()
				cmd := exec.Command("git", "clone", remoteDir, workdir) // Clone git dir
				out, err := cmd.Output()
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						t.Logf("stderr: %s\n", exitErr.Stderr)
					}
					t.Fatal(err)
				}

				if err := os.WriteFile(filepath.Join(workdir, "hello.txt"), []byte("hello"), 0666); err != nil {
					t.Fatal(err)
				}
				cmd = exec.Command("git", "add", "hello.txt") // Add a new file to git
				cmd.Dir = workdir
				out, err = cmd.Output()
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						t.Logf("stderr: %s\n", exitErr.Stderr)
					}
					t.Fatal(err)
				}
				cmd = exec.Command("git", "commit", "-m", "new-file") // commit the new file
				cmd.Dir = workdir
				cmd.Env = []string{
					"GIT_AUTHOR_NAME=kuberpult",
					"GIT_COMMITTER_NAME=kuberpult",
					"EMAIL=test@kuberpult.com",
				}
				out, err = cmd.Output()
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						t.Logf("stderr: %s\n", exitErr.Stderr)
					}
					t.Fatal(err)
				}
				cmd = exec.Command("git", "push", "origin", "HEAD") // push the new commit
				cmd.Dir = workdir
				out, err = cmd.Output()
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						t.Logf("stderr: %s\n", exitErr.Stderr)
					}
					t.Fatal(err)
				}
				err = repo.Apply(testutil.MakeTestContext(), &CreateApplicationVersion{
					Application: "foo",
					Manifests: map[string]string{
						"development": "foo",
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				cmd = exec.Command("git", "--git-dir="+remoteDir, "rev-parse", "master")
				out, err = cmd.Output()
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						t.Logf("stderr: %s\n", exitErr.Stderr)
					}
					t.Fatal(err)
				}
				state := repo.State()
				localRev := state.Commit.Id().String()
				if localRev != strings.TrimSpace(string(out)) {
					t.Errorf("mismatched revision. expected %q but got %q", localRev, strings.TrimSpace(string(out)))
				}

				content, err := util.ReadFile(state.Filesystem, "hello.txt")
				if err != nil {
					t.Fatal(err)
				}
				if string(content) != "hello" {
					t.Errorf("mismatched file content, expected %s, got %s", "hello", content)
				}
			},
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
			tc.Setup(t, remoteDir, localDir)
			repo, err := New(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:    "file://" + remoteDir,
					Path:   localDir,
					Branch: tc.Branch,
				},
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}
			if tc.Test != nil {
				tc.Test(t, repo, remoteDir)
			}
		})
	}
}

func TestGetTags(t *testing.T) {
	tcs := []struct {
		Name            string
		Setup           func(t *testing.T, remoteDir, localDir string)
		expectedTags    []string
		expectedCommits []string
	}{
		{
			Name: "No tags to be returned at all",
			Setup: func(t *testing.T, remoteDir, localDir string) {
				// run the initialization code once
				// run the initialization code once
				_, err := New(
					testutil.MakeTestContext(),
					RepositoryConfig{
						URL:  "file://" + remoteDir,
						Path: localDir,
					},
				)
				if err != nil {
					t.Fatal(err)
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
			tc.Setup(t, remoteDir, localDir)
			tags, commits, err := GetTags(
				RepositoryConfig{
					URL:  "file://" + remoteDir,
					Path: localDir,
				},
				"./repository",
				testutil.MakeTestContext(),
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}
			if len(tags) != len(tc.expectedTags) {
				t.Fatalf("expected %v tags but got %v", len(tc.expectedTags), len(tags))
			}
			if len(commits) != len(tc.expectedCommits) {
				t.Fatalf("expected %v commits but got %v", len(tc.expectedTags), len(tags))
			}
			for i, _ := range tags {
				if tags[i] != tc.expectedTags[i] {
					t.Fatalf("the values of the tags does not match the expected, got %v but want %v", tags, tc.expectedTags)
				}
				if commits[i] != tc.expectedCommits[i] {
					t.Fatalf("the values of the commits does not match the expected, got %v but want %v", commits, tc.expectedCommits)
				}
			}
		})
	}
}

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
						URL:  "file://" + remoteDir,
						Path: localDir,
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
				},
			)
			if err == nil {
				t.Fatalf("New: Expected error but no error was thrown")
			}
		})
	}
}

func TestConfigReload(t *testing.T) {
	configFiles := []struct {
		ConfigContent string
		ErrorExpected bool
	}{
		{
			ConfigContent: "{\"upstream\": {\"latest\": true }}",
			ErrorExpected: false,
		},
		{
			ConfigContent: "{\"upstream\": \"latest\": true }}",
			ErrorExpected: true,
		},
		{
			ConfigContent: "{\"upstream\": {\"latest\": true }}",
			ErrorExpected: false,
		},
	}
	t.Run("Config file reload on change", func(t *testing.T) {
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
		cmd = exec.Command("git", "config", "pull.rebase", "false") // Add a new file to git
		cmd.Dir = workdir
		_, err = cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				t.Logf("stderr: %s\n", exitErr.Stderr)
			}
			t.Fatal(err)
		}

		if err := os.MkdirAll(path.Join(workdir, "environments", "development"), 0700); err != nil {
			t.Fatal(err)
		}

		updateConfigFile := func(configFileContent string) error {
			configFilePath := path.Join(workdir, "environments", "development", "config.json")
			if err := os.WriteFile(configFilePath, []byte(configFileContent), 0666); err != nil {
				return err
			}
			cmd = exec.Command("git", "add", configFilePath) // Add a new file to git
			cmd.Dir = workdir
			_, err = cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Logf("stderr: %s\n", exitErr.Stderr)
				}
				return err
			}
			cmd = exec.Command("git", "commit", "-m", "valid config") // commit the new file
			cmd.Dir = workdir
			cmd.Env = []string{
				"GIT_AUTHOR_NAME=kuberpult",
				"GIT_COMMITTER_NAME=kuberpult",
				"EMAIL=test@kuberpult.com",
			}
			out, err := cmd.Output()
			fmt.Println(string(out))
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Logf("stderr: %s\n", exitErr.Stderr)
					t.Logf("stderr: %s\n", err)
				}
				return err
			}
			cmd = exec.Command("git", "push", "origin", "HEAD") // push the new commit
			cmd.Dir = workdir
			_, err = cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Logf("stderr: %s\n", exitErr.Stderr)
				}
				return err
			}
			return nil
		}

		repo, err := New(
			testutil.MakeTestContext(),
			RepositoryConfig{
				URL:  remoteDir,
				Path: t.TempDir(),
			},
		)

		if err != nil {
			t.Fatal(err)
		}

		for _, configFile := range configFiles {
			err = updateConfigFile(configFile.ConfigContent)
			if err != nil {
				t.Fatal(err)
			}
			err := repo.Apply(testutil.MakeTestContext(), &CreateApplicationVersion{
				Application: "foo",
				Manifests: map[string]string{
					"development": "foo",
				},
			})
			if configFile.ErrorExpected {
				if err == nil {
					t.Errorf("Apply gave error even though config.json was incorrect")
				}
			} else {
				if err != nil {
					fmt.Println(err)
					t.Errorf("Initialization failed with valid config.json")
				}
				cmd = exec.Command("git", "pull") // Add a new file to git
				cmd.Dir = workdir
				_, err = cmd.Output()
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						t.Logf("stderr: %s\n", exitErr.Stderr)
					}
					t.Fatal(err)
				}
			}
		}
	})
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
					URL:  remoteDir,
					Path: t.TempDir(),
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

func TestGc(t *testing.T) {
	tcs := []struct {
		Name               string
		GcFrequency        uint
		StorageBackend     StorageBackend
		ExpectedGarbageMin uint64
		ExpectedGarbageMax uint64
	}{
		{
			// 0 disables GC entirely
			// we are reasonably expecting some additional files around
			Name:               "gc disabled",
			GcFrequency:        0,
			StorageBackend:     GitBackend,
			ExpectedGarbageMin: 906,
			ExpectedGarbageMax: 913,
		},
		{
			// we are going to perform 101 requests, that should trigger a gc
			// the number of additional files should be lower than in the case above
			Name:               "gc enabled",
			GcFrequency:        100,
			StorageBackend:     GitBackend,
			ExpectedGarbageMin: 9,
			ExpectedGarbageMax: 10,
		},
		{
			// enabling sqlite should bring the number of loose files down to 0
			Name:               "sqlite",
			GcFrequency:        0, // the actual number here doesn't matter. GC is not run when sqlite is in use
			StorageBackend:     SqliteBackend,
			ExpectedGarbageMin: 0,
			ExpectedGarbageMax: 0,
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
			ctx := testutil.MakeTestContext()
			repo, err := New(
				ctx,
				RepositoryConfig{
					URL:            "file://" + remoteDir,
					Path:           localDir,
					GcFrequency:    tc.GcFrequency,
					StorageBackend: tc.StorageBackend,
				},
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}

			err = repo.Apply(ctx, &CreateEnvironment{
				Environment: "test",
			})
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 100; i++ {
				err := repo.Apply(ctx, &CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"test": fmt.Sprintf("test%d", i),
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			stats, err := repo.(*repository).countObjects(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if stats.Count > tc.ExpectedGarbageMax {
				t.Errorf("expected object count to be lower than %d, but got %d", tc.ExpectedGarbageMax, stats.Count)
			}
			if stats.Count < tc.ExpectedGarbageMin {
				t.Errorf("expected object count to be higher than %d, but got %d", tc.ExpectedGarbageMin, stats.Count)
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

type SlowTransformer struct {
	finished chan struct{}
	started  chan struct{}
}

func (s *SlowTransformer) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	s.started <- struct{}{}
	<-s.finished
	return "ok", &TransformerResult{}, nil
}

type EmptyTransformer struct{}

func (p *EmptyTransformer) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	return "nothing happened", &TransformerResult{}, nil
}

type PanicTransformer struct{}

func (p *PanicTransformer) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	panic("panic tranformer")
}

var TransformerError = errors.New("error transformer")

type ErrorTransformer struct{}

func (p *ErrorTransformer) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	return "error", nil, TransformerError
}

type InvalidJsonTransformer struct{}

func (p *InvalidJsonTransformer) Transform(ctx context.Context, state *State) (string, *TransformerResult, error) {
	return "error", nil, invalidJson
}

func convertToSet(list []uint64) map[int]bool {
	set := make(map[int]bool)
	for _, i := range list {
		set[int(i)] = true
	}
	return set
}

func TestApplyQueuePanic(t *testing.T) {
	type action struct {
		Transformer Transformer
		// Tests
		ExpectedError error
	}
	tcs := []struct {
		Name    string
		Actions []action
	}{
		{
			Name: "panic at the start",
			Actions: []action{
				{
					Transformer:   &PanicTransformer{},
					ExpectedError: panicError,
				}, {
					ExpectedError: panicError,
				}, {
					ExpectedError: panicError,
				},
			},
		},
		{
			Name: "panic at the middle",
			Actions: []action{
				{
					ExpectedError: panicError,
				}, {
					Transformer:   &PanicTransformer{},
					ExpectedError: panicError,
				}, {
					ExpectedError: panicError,
				},
			},
		},
		{
			Name: "panic at the end",
			Actions: []action{
				{
					ExpectedError: panicError,
				}, {
					ExpectedError: panicError,
				}, {
					Transformer:   &PanicTransformer{},
					ExpectedError: panicError,
				},
			},
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
			repo, err := New(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:  "file://" + remoteDir,
					Path: localDir,
				},
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}
			repoInternal := repo.(*repository)
			// Block the worker so that we have multiple items in the queue
			finished := make(chan struct{})
			started := make(chan struct{}, 1)
			go func() {
				repo.Apply(testutil.MakeTestContext(), &SlowTransformer{finished, started})
			}()
			<-started
			// The worker go routine is now blocked. We can move some items into the queue now.
			results := make([]<-chan error, len(tc.Actions))
			for i, action := range tc.Actions {
				results[i] = repoInternal.applyDeferred(testutil.MakeTestContext(), action.Transformer)
			}
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("The code did not panic")
				}
				// Check for the correct errors
				for i, action := range tc.Actions {
					if err := <-results[i]; err != action.ExpectedError {
						t.Errorf("result[%d] error is not \"%v\" but got \"%v\"", i, action.ExpectedError, err)
					}
				}
			}()
			repoInternal.ProcessQueue(testutil.MakeTestContext())
		})
	}
}

func TestApplyQueue(t *testing.T) {
	type action struct {
		CancelBeforeAdd bool
		CancelAfterAdd  bool
		Transformer     Transformer
		// Tests
		ExpectedError error
	}
	tcs := []struct {
		Name             string
		Actions          []action
		ExpectedReleases []uint64
	}{
		{
			Name: "simple",
			Actions: []action{
				{}, {}, {},
			},
			ExpectedReleases: []uint64{
				1, 2, 3,
			},
		},
		{
			Name: "cancellation in the middle (after)",
			Actions: []action{
				{}, {
					CancelAfterAdd: true,
					ExpectedError:  context.Canceled,
				}, {},
			},
			ExpectedReleases: []uint64{
				1, 3,
			},
		},
		{
			Name: "cancellation at the start (after)",
			Actions: []action{
				{
					CancelAfterAdd: true,
					ExpectedError:  context.Canceled,
				}, {}, {},
			},
			ExpectedReleases: []uint64{
				2, 3,
			},
		},
		{
			Name: "cancellation at the end (after)",
			Actions: []action{
				{}, {},
				{
					CancelAfterAdd: true,
					ExpectedError:  context.Canceled,
				},
			},
			ExpectedReleases: []uint64{
				1, 2,
			},
		},
		{
			Name: "cancellation in the middle (before)",
			Actions: []action{
				{}, {
					CancelBeforeAdd: true,
					ExpectedError:   context.Canceled,
				}, {},
			},
			ExpectedReleases: []uint64{
				1, 3,
			},
		},
		{
			Name: "cancellation at the start (before)",
			Actions: []action{
				{
					CancelBeforeAdd: true,
					ExpectedError:   context.Canceled,
				}, {}, {},
			},
			ExpectedReleases: []uint64{
				2, 3,
			},
		},
		{
			Name: "cancellation at the end (before)",
			Actions: []action{
				{}, {},
				{
					CancelBeforeAdd: true,
					ExpectedError:   context.Canceled,
				},
			},
			ExpectedReleases: []uint64{
				1, 2,
			},
		},
		{
			Name: "error at the start",
			Actions: []action{
				{
					ExpectedError: TransformerError,
					Transformer:   &ErrorTransformer{},
				}, {}, {},
			},
			ExpectedReleases: []uint64{
				2, 3,
			},
		},
		{
			Name: "error at the middle",
			Actions: []action{
				{},
				{
					ExpectedError: TransformerError,
					Transformer:   &ErrorTransformer{},
				}, {},
			},
			ExpectedReleases: []uint64{
				1, 3,
			},
		},
		{
			Name: "error at the end",
			Actions: []action{
				{}, {},
				{
					ExpectedError: TransformerError,
					Transformer:   &ErrorTransformer{},
				},
			},
			ExpectedReleases: []uint64{
				1, 2,
			},
		},
		{
			Name: "Invalid json error at start",
			Actions: []action{
				{
					ExpectedError: invalidJson,
					Transformer:   &InvalidJsonTransformer{},
				},
				{}, {},
			},
			ExpectedReleases: []uint64{
				2, 3,
			},
		},
		{
			Name: "Invalid json error at middle",
			Actions: []action{
				{},
				{
					ExpectedError: invalidJson,
					Transformer:   &InvalidJsonTransformer{},
				},
				{},
			},
			ExpectedReleases: []uint64{
				1, 3,
			},
		},
		{
			Name: "Invalid json error at end",
			Actions: []action{
				{}, {},
				{
					ExpectedError: invalidJson,
					Transformer:   &InvalidJsonTransformer{},
				},
			},
			ExpectedReleases: []uint64{
				1, 2,
			},
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
			repo, err := New(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:  "file://" + remoteDir,
					Path: localDir,
				},
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}
			repoInternal := repo.(*repository)
			// Block the worker so that we have multiple items in the queue
			finished := make(chan struct{})
			started := make(chan struct{}, 1)
			go func() {
				repo.Apply(testutil.MakeTestContext(), &SlowTransformer{finished, started})
			}()
			<-started
			// The worker go routine is now blocked. We can move some items into the queue now.
			results := make([]<-chan error, len(tc.Actions))
			for i, action := range tc.Actions {
				ctx, cancel := context.WithCancel(testutil.MakeTestContext())
				if action.CancelBeforeAdd {
					cancel()
				}
				if action.Transformer != nil {
					results[i] = repoInternal.applyDeferred(ctx, action.Transformer)
				} else {
					tf := &CreateApplicationVersion{
						Application: "foo",
						Manifests: map[string]string{
							"development": fmt.Sprintf("%d", i),
						},
						Version: uint64(i + 1),
					}
					results[i] = repoInternal.applyDeferred(ctx, tf)
				}
				if action.CancelAfterAdd {
					cancel()
				}
			}
			// Now release the slow transformer
			finished <- struct{}{}
			// Check for the correct errors
			for i, action := range tc.Actions {
				if err := <-results[i]; err != action.ExpectedError {
					t.Errorf("result[%d] error is not \"%v\" but got \"%v\"", i, action.ExpectedError, err)
				}
			}
			releases, _ := repo.State().Releases("foo")
			if !cmp.Equal(convertToSet(tc.ExpectedReleases), convertToSet(releases)) {
				t.Fatal("Output mismatch (-want +got):\n", cmp.Diff(tc.ExpectedReleases, releases))
			}

		})
	}
}

func getTransformer(i int) (Transformer, error) {
	transformerType := i % 5
	switch transformerType {
	case 0:
	case 1:
	case 2:
		return &CreateApplicationVersion{
			Application: "foo",
			Manifests: map[string]string{
				"development": fmt.Sprintf("%d", i),
			},
			Version: uint64(i + 1),
		}, nil
	case 3:
		return &ErrorTransformer{}, TransformerError
	case 4:
		return &InvalidJsonTransformer{}, invalidJson
	}
	return &ErrorTransformer{}, TransformerError
}

func createGitWithCommit(remote string, local string, t *testing.B) {
	cmd := exec.Command("git", "init", "--bare", remote)
	cmd.Start()
	cmd.Wait()

	cmd = exec.Command("git", "clone", remote, local) // Clone git dir
	_, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("touch", "a") // Add a new file to git
	cmd.Dir = local
	_, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "add", "a") // Add a new file to git
	cmd.Dir = local
	_, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "commit", "-m", "adding") // commit the new file
	cmd.Dir = local
	cmd.Env = []string{
		"GIT_AUTHOR_NAME=kuberpult",
		"GIT_COMMITTER_NAME=kuberpult",
		"EMAIL=test@kuberpult.com",
	}
	_, err = cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Logf("stderr: %s\n", exitErr.Stderr)
			t.Logf("stderr: %s\n", err)
		}
		t.Fatal(err)
	}
	cmd = exec.Command("git", "push", "origin", "HEAD") // push the new commit
	cmd.Dir = local
	_, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkApplyQueue(t *testing.B) {
	t.StopTimer()
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	createGitWithCommit(remoteDir, localDir, t)

	repo, err := New(
		testutil.MakeTestContext(),
		RepositoryConfig{
			URL:  "file://" + remoteDir,
			Path: localDir,
		},
	)
	if err != nil {
		t.Fatalf("new: expected no error, got '%e'", err)
	}
	repoInternal := repo.(*repository)
	// The worker go routine is now blocked. We can move some items into the queue now.
	results := make([]<-chan error, t.N)
	expectedResults := make([]error, t.N)
	expectedReleases := make(map[int]bool, t.N)
	tf, _ := getTransformer(0)
	repoInternal.Apply(testutil.MakeTestContext(), tf)

	t.StartTimer()
	for i := 0; i < t.N; i++ {
		tf, expectedResult := getTransformer(i)
		results[i] = repoInternal.applyDeferred(testutil.MakeTestContext(), tf)
		expectedResults[i] = expectedResult
		if expectedResult == nil {
			expectedReleases[i+1] = true
		}
	}

	for i := 0; i < t.N; i++ {
		if err := <-results[i]; err != expectedResults[i] {
			t.Errorf("result[%d] expected error \"%v\" but got \"%v\"", i, expectedResults[i], err)
		}
	}
	releases, _ := repo.State().Releases("foo")
	if !cmp.Equal(expectedReleases, convertToSet(releases)) {
		t.Fatal("Output mismatch (-want +got):\n", cmp.Diff(expectedReleases, convertToSet(releases)))
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

func TestProcessQueueOnce(t *testing.T) {
	tcs := []struct {
		Name           string
		Element        element
		PushUpdateFunc PushUpdateFunc
		PushActionFunc PushActionCallbackFunc
		ExpectedError  error
	}{
		{
			Name:           "success",
			PushUpdateFunc: defaultPushUpdate,
			PushActionFunc: DefaultPushActionCallback,
			Element: element{
				ctx: testutil.MakeTestContext(),
				transformers: []Transformer{
					&EmptyTransformer{},
				},
				result: make(chan error, 1),
			},
			ExpectedError: nil,
		},
		{
			Name: "failure because DefaultPushUpdate is wrong (branch protection)",
			PushUpdateFunc: func(s string, success *bool) git.PushUpdateReferenceCallback {
				*success = false
				return nil
			},
			PushActionFunc: DefaultPushActionCallback,
			Element: element{
				ctx: testutil.MakeTestContext(),
				transformers: []Transformer{
					&EmptyTransformer{},
				},
				result: make(chan error, 1),
			},
			ExpectedError: errors.New("failed to push - this indicates that branch protection is enabled in 'file://$DIR/remote' on branch 'master'"),
		},
		{
			Name: "failure because error is returned in push (ssh key has read only access)",
			PushUpdateFunc: func(s string, success *bool) git.PushUpdateReferenceCallback {
				return nil
			},
			PushActionFunc: func(options git.PushOptions, r *repository) PushActionFunc {
				return func() error {
					return git.MakeGitError(1)
				}
			},
			Element: element{
				ctx: testutil.MakeTestContext(),
				transformers: []Transformer{
					&EmptyTransformer{},
				},
				result: make(chan error, 1),
			},
			ExpectedError: errors.New("rpc error: code = InvalidArgument desc = error: could not push to manifest repository 'file://$DIR/remote' on branch 'master' - this indicates that the ssh key does not have write access"),
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
			repo, actualError := New(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:  "file://" + remoteDir,
					Path: localDir,
				},
			)
			if actualError != nil {
				t.Fatalf("new: expected no error, got '%e'", actualError)
			}
			repoInternal := repo.(*repository)
			repoInternal.ProcessQueueOnce(testutil.MakeTestContext(), tc.Element, tc.PushUpdateFunc, tc.PushActionFunc)

			result := tc.Element.result
			actualError = <-result
			if tc.ExpectedError == nil && actualError == nil {
				return
			}
			if tc.ExpectedError == nil && actualError != nil {
				t.Fatalf("result error is not:\n\"%v\"\nbut got:\n\"%v\"", nil, actualError.Error())
			}
			if tc.ExpectedError != nil && actualError == nil {
				t.Fatalf("result error is not:\n\"%v\"\nbut got:\n\"%v\"", tc.ExpectedError, nil)
			}
			expectedError := strings.ReplaceAll(tc.ExpectedError.Error(), "$DIR", dir)
			if actualError.Error() != expectedError {
				t.Errorf("result error is not:\n\"%v\"\nbut got:\n\"%v\"", expectedError, actualError.Error())
			}
		})
	}
}

func TestGitPushDoesntGetStuck(t *testing.T) {
	tcs := []struct {
		Name string
	}{
		{
			Name: "it doesnt get stuck",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// create a remote
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Run()
			ts := testssh.New(remoteDir)
			defer ts.Close()
			repo, err := New(
				ctx,
				RepositoryConfig{
					URL: ts.Url,
					Certificates: Certificates{
						KnownHostsFile: ts.KnownHosts,
					},
					Credentials: Credentials{
						SshKey: ts.ClientKey,
					},
					Path:           localDir,
					NetworkTimeout: time.Second,
				},
			)
			if err != nil {
				t.Errorf("expected no error, got %q ( %#v )", err, err)
			}
			err = repo.Apply(testutil.MakeTestContext(),
				&CreateEnvironment{Environment: "dev"},
			)
			if err != nil {
				t.Errorf("expected no error, got %q ( %#v )", err, err)
			}
			// This will prevent the next push from working
			ts.DelayExecs(15 * time.Second)
			err = repo.Apply(testutil.MakeTestContext(),
				&CreateEnvironment{Environment: "stg"},
			)
			if err == nil {
				t.Errorf("expected an error, but didn't get one")
			}
			if status.Code(err) != codes.Canceled {
				t.Errorf("expected status code cancelled, but got %q", status.Code(err))
			}
			// This will make the next push work
			ts.DelayExecs(0 * time.Second)
			err = repo.Apply(testutil.MakeTestContext(),
				&CreateEnvironment{Environment: "stg"},
			)
			if err != nil {
				t.Errorf("expected no error, got %q ( %#v )", err, err)
			}
		})
	}
}
