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
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/google/go-cmp/cmp"
	git "github.com/libgit2/git2go/v33"
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
					context.Background(),
					Config{
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
					context.Background(),
					Config{
						URL:  remoteDir,
						Path: localDir,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				err = repo.Apply(context.Background(), &CreateApplicationVersion{
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
					context.Background(),
					Config{
						URL:  remoteDir,
						Path: t.TempDir(),
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				err = repo.Apply(context.Background(), &CreateApplicationVersion{
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
				err := repo.Apply(context.Background(), &CreateApplicationVersion{
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
				err = repo.Apply(context.Background(), &CreateApplicationVersion{
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
				context.Background(),
				Config{
					URL:                    "file://" + remoteDir,
					Path:                   localDir,
					Branch:                 tc.Branch,
					EnvironmentConfigsPath: filepath.Join(remoteDir, "..", "environment_configs.json"),
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
					context.Background(),
					Config{
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
				context.Background(),
				Config{
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
			if err := os.WriteFile(environmentConfigsPath, []byte(`{"uniqueEnv": {"upstream": {"latest": true}}}`), fs.FileMode(0644)); err != nil {
				t.Fatal(err)
			}

			repo, err := New(
				context.Background(),
				Config{
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
				context.Background(),
				Config{
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
			context.Background(),
			Config{
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
			err := repo.Apply(context.Background(), &CreateApplicationVersion{
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
				context.Background(),
				Config{
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
		Name          string
		GcFrequencies []uint
		CreateGarbage func(t *testing.T, repo *repository)
		Test          func(t *testing.T, repos []*repository)
	}{
		{
			Name: "simple",
			GcFrequencies: []uint{
				// 0 disables GC entirely
				0,
				// we are going to perform 101 requests, that should trigger a gc
				101,
			},
			CreateGarbage: func(t *testing.T, repo *repository) {
				ctx := context.Background()
				err := repo.Apply(ctx, &CreateEnvironment{
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
			},
			Test: func(t *testing.T, repos []*repository) {
				ctx := context.Background()
				stats0, err := repos[0].countObjects(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if stats0.Count == 0 {
					t.Errorf("expected object count to not be 0, but got %d", stats0.Count)
				}
				stats1, err := repos[1].countObjects(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if stats1.Count != 0 {
					t.Errorf("expected object count to be 0, but got %d", stats1.Count)
				}
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			// create a remote
			repos := make([]*repository, len(tc.GcFrequencies))
			for i, gcFrequency := range tc.GcFrequencies {
				dir := t.TempDir()
				remoteDir := path.Join(dir, "remote")
				localDir := path.Join(dir, "local")
				cmd := exec.Command("git", "init", "--bare", remoteDir)
				cmd.Start()
				cmd.Wait()
				repo, err := New(
					context.Background(),
					Config{
						URL:         "file://" + remoteDir,
						Path:        localDir,
						GcFrequency: gcFrequency,
					},
				)
				if err != nil {
					t.Fatalf("new: expected no error, got '%e'", err)
				}
				repoInternal := repo.(*repository)
				tc.CreateGarbage(t, repoInternal)
				repos[i] = repoInternal
			}
			tc.Test(t, repos)
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

			resp := repo.Push(context.Background(), func() error {
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
				return &git.GitError{Message: "mock error"}
			})

			if resp == nil || tc.ExpectedResponse == nil {
				if resp != tc.ExpectedResponse {
					t.Fatalf("new: expected '%e',  got '%e'", tc.ExpectedResponse, resp)
				}
			} else if resp.Error() != tc.ExpectedResponse.Error() {
				t.Fatalf("new: expected '%e',  got '%e'", tc.ExpectedResponse, resp)
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

func (s *SlowTransformer) Transform(ctx context.Context, state *State) (string, error) {
	s.started <- struct{}{}
	<-s.finished
	return "ok", nil
}

type PanicTransformer struct{}

func (p *PanicTransformer) Transform(ctx context.Context, state *State) (string, error) {
	panic("panic tranformer")
}

var TransformerError = errors.New("error transformer")

type ErrorTransformer struct{}

func (p *ErrorTransformer) Transform(ctx context.Context, state *State) (string, error) {
	return "error", TransformerError
}

type InvalidJsonTransformer struct{}

func (p *InvalidJsonTransformer) Transform(ctx context.Context, state *State) (string, error) {
	return "error", invalidJson
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
				context.Background(),
				Config{
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
				repo.Apply(context.Background(), &SlowTransformer{finished, started})
			}()
			<-started
			// The worker go routine is now blocked. We can move some items into the queue now.
			results := make([]<-chan error, len(tc.Actions))
			for i, action := range tc.Actions {
				results[i] = repoInternal.applyDeferred(context.Background(), action.Transformer)
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
			repoInternal.ProcessQueue(context.Background())
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
				context.Background(),
				Config{
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
				repo.Apply(context.Background(), &SlowTransformer{finished, started})
			}()
			<-started
			// The worker go routine is now blocked. We can move some items into the queue now.
			results := make([]<-chan error, len(tc.Actions))
			for i, action := range tc.Actions {
				ctx, cancel := context.WithCancel(context.Background())
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

func TestItPersistsTheIndex(t *testing.T) {
	tcs := []struct {
		Name string
	}{
		{
			Name: "It stores the index and reads it again",
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
				context.Background(),
				Config{
					URL:  "file://" + remoteDir,
					Path: localDir,
				},
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}

			err = repo.Apply(context.Background(), &CreateEnvironment{
				Environment: "development",
			})
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i < 10; i += 1 {
				err = repo.Apply(context.Background(), &CreateApplicationVersion{
					Version:     uint64(i),
					Application: "test",
					Manifests: map[string]string{
						"development": "test",
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				state := repo.State()
				state.GetApplicationReleaseCommit("test", 1)
			}

			// check that the index file is written
			_, err = os.Stat(filepath.Join(localDir, ".index", "v1"))
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					t.Errorf("index file doesn't exist")
				} else {
					t.Fatalf("error stating file: %s", err)
				}
			}

			// reopen the repo, check that the costs are considerably lower than 9
			repo2, err := New(
				context.Background(),
				Config{
					URL:  "file://" + remoteDir,
					Path: localDir,
				},
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}
			state2 := repo2.State()
			state2.GetApplicationReleaseCommit("test", 1)
			// This check here is the most important one.
			// Before adding the persistant index the cost of calculating this would have been 9 because we needed to check 9 commits before finding this one.
			if state2.CommitHistory.Cost() != 2 {
				t.Errorf("the cost for calculating the version is not 2 but %d", state2.CommitHistory.Cost())
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
		context.Background(),
		Config{
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
	repoInternal.Apply(context.Background(), tf)

	t.StartTimer()
	for i := 0; i < t.N; i++ {
		tf, expectedResult := getTransformer(i)
		results[i] = repoInternal.applyDeferred(context.Background(), tf)
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
