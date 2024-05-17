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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testutil"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/freiheit-com/kuberpult/pkg/setup"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testssh"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-git/go-billy/v5/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	git "github.com/libgit2/git2go/v34"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
						URL:                 "file://" + remoteDir,
						Path:                localDir,
						ArgoCdGenerateFiles: true,
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
						URL:                 remoteDir,
						Path:                localDir,
						ArgoCdGenerateFiles: true,
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
						URL:                 remoteDir,
						Path:                t.TempDir(),
						ArgoCdGenerateFiles: true,
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
				if diff := cmp.Diff(localRev, strings.TrimSpace(string(out))); diff != "" {
					t.Errorf("mismatched revision (-want, +got):\n%s", diff)
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
				if diff := cmp.Diff(localRev, strings.TrimSpace(string(out))); diff != "" {
					t.Errorf("mismatched revision (-want, +got):\n%s", diff)
				}

				content, err := util.ReadFile(state.Filesystem, "hello.txt")
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff("hello", string(content)); diff != "" {
					t.Errorf("mismatched file content (-want, +got):\n%s", diff)
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
					URL:                 "file://" + remoteDir,
					Path:                localDir,
					Branch:              tc.Branch,
					ArgoCdGenerateFiles: true,
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

func TestGetTagsNoTags(t *testing.T) {
	name := "No tags to be returned at all"

	t.Run(name, func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		remoteDir := path.Join(dir, "remote")
		localDir := path.Join(dir, "local")
		repoConfig := RepositoryConfig{
			StorageBackend:      0,
			URL:                 "file://" + remoteDir,
			Path:                localDir,
			Branch:              "master",
			ArgoCdGenerateFiles: true,
		}
		cmd := exec.Command("git", "init", "--bare", remoteDir)
		cmd.Start()
		cmd.Wait()
		_, err := New(
			testutil.MakeTestContext(),
			repoConfig,
		)

		if err != nil {
			t.Fatal(err)
		}
		tags, err := GetTags(
			repoConfig,
			localDir,
			testutil.MakeTestContext(),
		)
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
			Name:         "Tags added to be returned",
			tagsToAdd:    []string{"v1.0.0"},
			expectedTags: []api.TagData{{Tag: "refs/tags/v1.0.0", CommitId: ""}},
		},
		{
			Name:         "Tags added in opposite order and are sorted",
			tagsToAdd:    []string{"v1.0.1", "v0.0.1"},
			expectedTags: []api.TagData{{Tag: "refs/tags/v0.0.1", CommitId: ""}, {Tag: "refs/tags/v1.0.1", CommitId: ""}},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")
			repoConfig := RepositoryConfig{
				StorageBackend:      0,
				URL:                 "file://" + remoteDir,
				Path:                localDir,
				Branch:              "master",
				ArgoCdGenerateFiles: true,
			}
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()
			_, err := New(
				testutil.MakeTestContext(),
				repoConfig,
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
				_, err := repo.Tags.Create(tc.tagsToAdd[addTag], commit, &git.Signature{Name: "SRE", Email: "testing@gmail"}, "testing")
				expectedCommits = append(expectedCommits, api.TagData{Tag: tc.tagsToAdd[addTag], CommitId: commit.Id().String()})
				if err != nil {
					t.Fatal(err)
				}
			}
			tags, err := GetTags(
				repoConfig,
				localDir,
				testutil.MakeTestContext(),
			)
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
				URL:                 remoteDir,
				Path:                t.TempDir(),
				ArgoCdGenerateFiles: true,
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
					t.Errorf("Apply gave no error even though config.json was incorrect")
				}
			} else {
				if err != nil {
					t.Errorf("Initialization failed with valid config.json: %s", err.Error())
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
			ExpectedGarbageMax: 1500,
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
					URL:                 "file://" + remoteDir,
					Path:                localDir,
					GcFrequency:         tc.GcFrequency,
					StorageBackend:      tc.StorageBackend,
					ArgoCdGenerateFiles: true,
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

func (s *SlowTransformer) GetDBEventType() db.EventType {
	return "invalid"
}

func (s *SlowTransformer) Transform(ctx context.Context, state *State, transformerContext TransformerContext, transaction *sql.Tx) (string, error) {
	s.started <- struct{}{}
	<-s.finished
	return "ok", nil
}

type EmptyTransformer struct{}

func (p *EmptyTransformer) GetDBEventType() db.EventType {
	return "invalid"
}

func (p *EmptyTransformer) Transform(ctx context.Context, state *State, transformerContext TransformerContext, transaction *sql.Tx) (string, error) {
	return "nothing happened", nil
}

type PanicTransformer struct{}

func (p *PanicTransformer) GetDBEventType() db.EventType {
	return "invalid"
}

func (p *PanicTransformer) Transform(ctx context.Context, state *State, transformerContext TransformerContext, transaction *sql.Tx) (string, error) {
	panic("panic tranformer")
}

var TransformerError = errors.New("error transformer")

type ErrorTransformer struct{}

func (p *ErrorTransformer) GetDBEventType() db.EventType {
	return "invalid"
}

func (p *ErrorTransformer) Transform(ctx context.Context, state *State, transformerContext TransformerContext, transaction *sql.Tx) (string, error) {
	return "error", TransformerError
}

type InvalidJsonTransformer struct{}

func (p *InvalidJsonTransformer) GetDBEventType() db.EventType {
	return "invalid"
}

func (p *InvalidJsonTransformer) Transform(ctx context.Context, state *State, transformerContext TransformerContext, transaction *sql.Tx) (string, error) {
	return "error", InvalidJson
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
			repo, processQueue, err := New2(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:                   "file://" + remoteDir,
					Path:                  localDir,
					MaximumCommitsPerPush: 3,
					ArgoCdGenerateFiles:   true,
				},
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}
			// The worker go routine is not started. We can move some items into the queue now.
			results := make([]<-chan error, len(tc.Actions))
			for i, action := range tc.Actions {
				// We are using the internal interface here as an optimization to avoid spinning up one go-routine per action
				t := action.Transformer
				if t == nil {
					t = &EmptyTransformer{}
				}
				results[i] = repo.(*repository).applyDeferred(testutil.MakeTestContext(), t)
			}
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("The code did not panic")
				} else if r != "panic tranformer" {
					t.Logf("The code did not panic with the correct string but %#v", r)
					panic(r)
				}
				// Check for the correct errors
				for i, action := range tc.Actions {
					if err := <-results[i]; err != action.ExpectedError {
						t.Errorf("result[%d] error is not \"%v\" but got \"%v\"", i, action.ExpectedError, err)
					}
				}
			}()
			ctx, cancel := context.WithTimeout(testutil.MakeTestContext(), 10*time.Second)
			defer cancel()
			processQueue(ctx, nil)
		})
	}
}

type mockClock struct {
	t time.Time
}

func (m *mockClock) now() time.Time {
	return m.t
}

func (m *mockClock) sleep(d time.Duration) {
	m.t = m.t.Add(d)
}

func TestApplyQueueTtlForHealth(t *testing.T) {
	// we set the networkTimeout to something low, so that it doesn't interfere with other processes e.g like once per second:
	networkTimeout := 1 * time.Second

	tcs := []struct {
		Name string
	}{
		{
			Name: "sleeps way too long, so health should fail",
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
			repo, processQueue, err := New2(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:                 "file://" + remoteDir,
					Path:                localDir,
					NetworkTimeout:      networkTimeout,
					ArgoCdGenerateFiles: true,
				},
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}
			ctx, cancel := context.WithTimeout(testutil.MakeTestContext(), 10*time.Second)

			mc := mockClock{}
			hlth := &setup.HealthServer{}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
			hlth.Clock = mc.now
			reporterName := "ClarkKent"
			reporter := hlth.Reporter(reporterName)
			isReady := func() bool {
				return hlth.IsReady(reporterName)
			}
			errChan := make(chan error)
			go func() {
				err = processQueue(ctx, reporter)
				errChan <- err
			}()
			defer func() {
				cancel()
				chanError := <-errChan
				if chanError != nil {
					t.Errorf("Expected no error in processQueue but got: %v", chanError)
				}
			}()

			finished := make(chan struct{})
			started := make(chan struct{})
			var transformer Transformer = &SlowTransformer{
				finished: finished,
				started:  started,
			}

			go repo.Apply(ctx, transformer)

			// first, wait, until the transformer has started:
			<-started
			// health should be reporting as ready now
			if !isReady() {
				t.Error("Expected health to be ready after transformer was started, but it was not")
			}
			// now advance the clock time
			mc.sleep(4 * networkTimeout)

			// now that the transformer is started, we should get a failed health check immediately, because the networkTimeout is tiny:
			if isReady() {
				t.Error("Expected health to be not ready after transformer took too long, but it was")
			}

			// let the transformer finish:
			finished <- struct{}{}

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
					ExpectedError: &TransformerBatchApplyError{TransformerError: TransformerError, Index: 0},
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
					ExpectedError: &TransformerBatchApplyError{TransformerError: TransformerError, Index: 0},
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
					ExpectedError: &TransformerBatchApplyError{TransformerError: TransformerError, Index: 0},
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
					ExpectedError: &TransformerBatchApplyError{TransformerError: InvalidJson, Index: 0},
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
					ExpectedError: &TransformerBatchApplyError{TransformerError: InvalidJson, Index: 0},
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
					ExpectedError: &TransformerBatchApplyError{TransformerError: InvalidJson, Index: 0},
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
					URL:                   "file://" + remoteDir,
					Path:                  localDir,
					MaximumCommitsPerPush: 10,
					ArgoCdGenerateFiles:   true,
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
				repo.Apply(testutil.MakeTestContext(), &SlowTransformer{finished: finished, started: started})
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
				if err := <-results[i]; err != nil && err.Error() != action.ExpectedError.Error() {
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

func TestMaximumCommitsPerPush(t *testing.T) {
	tcs := []struct {
		NumberOfCommits       uint
		MaximumCommitsPerPush uint
		ExpectedAtLeastPushes uint
	}{
		{
			NumberOfCommits:       7,
			MaximumCommitsPerPush: 5,
			ExpectedAtLeastPushes: 2,
		},
		{
			NumberOfCommits:       5,
			MaximumCommitsPerPush: 0,
			ExpectedAtLeastPushes: 5,
		},
		{
			NumberOfCommits:       5,
			MaximumCommitsPerPush: 10,
			ExpectedAtLeastPushes: 1,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(fmt.Sprintf("with %d commits and %d per push", tc.NumberOfCommits, tc.MaximumCommitsPerPush), func(t *testing.T) {
			// create a remote
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Run()
			ts := testssh.New(remoteDir)
			defer ts.Close()
			repo, processor, err := New2(
				testutil.MakeTestContext(),
				RepositoryConfig{
					URL:  ts.Url,
					Path: localDir,
					Certificates: Certificates{
						KnownHostsFile: ts.KnownHosts,
					},
					Credentials: Credentials{
						SshKey: ts.ClientKey,
					},

					MaximumCommitsPerPush: tc.MaximumCommitsPerPush,
					ArgoCdGenerateFiles:   true,
				},
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}
			var eg errgroup.Group
			for i := uint(0); i < tc.NumberOfCommits; i++ {
				eg.Go(func() error {
					return repo.Apply(testutil.MakeTestContext(), &CreateApplicationVersion{
						Application: "foo",
						Manifests:   map[string]string{"development": "foo"},
					})
				})
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				processor(ctx, nil)
			}()
			eg.Wait()
			if ts.Pushes < tc.ExpectedAtLeastPushes {
				t.Errorf("expected at least %d pushes, but %d happened", tc.ExpectedAtLeastPushes, ts.Pushes)
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
		return &InvalidJsonTransformer{}, InvalidJson
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
			URL:                 "file://" + remoteDir,
			Path:                localDir,
			ArgoCdGenerateFiles: true,
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

func TestProcessQueueOnce(t *testing.T) {
	tcs := []struct {
		Name           string
		Element        transformerBatch
		PushUpdateFunc PushUpdateFunc
		PushActionFunc PushActionCallbackFunc
		ExpectedError  error
	}{
		{
			Name:           "success",
			PushUpdateFunc: defaultPushUpdate,
			PushActionFunc: DefaultPushActionCallback,
			Element: transformerBatch{
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
			Element: transformerBatch{
				ctx: testutil.MakeTestContext(),
				transformers: []Transformer{
					&EmptyTransformer{},
				},
				result: make(chan error, 1),
			},
			ExpectedError: errMatcher{"failed to push - this indicates that branch protection is enabled in 'file://$DIR/remote' on branch 'master'"},
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
			Element: transformerBatch{
				ctx: testutil.MakeTestContext(),
				transformers: []Transformer{
					&EmptyTransformer{},
				},
				result: make(chan error, 1),
			},
			ExpectedError: errMatcher{"rpc error: code = InvalidArgument desc = error: could not push to manifest repository 'file://$DIR/remote' on branch 'master' - this indicates that the ssh key does not have write access"},
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
					URL:                 "file://" + remoteDir,
					Path:                localDir,
					ArgoCdGenerateFiles: true,
				},
			)
			if actualError != nil {
				t.Fatalf("new: expected no error, got '%e'", actualError)
			}
			repoInternal := repo.(*repository)
			repoInternal.ProcessQueueOnce(testutil.MakeTestContext(), tc.Element, tc.PushUpdateFunc, tc.PushActionFunc)

			result := tc.Element.result
			actualError = <-result

			var expectedError error
			if tc.ExpectedError != nil {
				expectedError = errMatcher{strings.ReplaceAll(tc.ExpectedError.Error(), "$DIR", dir)}
			}
			if diff := cmp.Diff(expectedError, actualError, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
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
					Path:                localDir,
					NetworkTimeout:      time.Second,
					ArgoCdGenerateFiles: true,
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

type TestWebhookResolver struct {
	t        *testing.T
	rec      *httptest.ResponseRecorder
	requests chan *http.Request
}

func (resolver TestWebhookResolver) Resolve(insecure bool, req *http.Request) (*http.Response, error) {
	testhandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resolver.t.Logf("called with request: %v", *r)
		resolver.requests <- req
		close(resolver.requests)
	})
	testhandler.ServeHTTP(resolver.rec, req)
	response := resolver.rec.Result()
	resolver.t.Logf("responded with: %v", response)
	return response, nil
}

func TestSendWebhookToArgoCd(t *testing.T) {
	tcs := []struct {
		Name    string
		Changes TransformerResult
		webUrl  string
		branch  string
	}{
		{
			Name: "webhook",
			Changes: TransformerResult{

				Commits: &CommitIds{
					Current:  git.NewOidFromBytes([]byte{'C', 'U', 'R', 'R', 'E', 'N', 'T', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}),
					Previous: git.NewOidFromBytes([]byte{'P', 'R', 'E', 'V', 'I', 'O', 'U', 'S', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}),
				},
			},
			webUrl: "http://example.com",
			branch: "examplebranch",
		},
	}
	for _, tc := range tcs {
		tc := tc
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			// given
			logger, err := zap.NewDevelopment()
			if err != nil {
				t.Fatalf("error creating logger: %v", err)
			}
			dir := t.TempDir()
			path := path.Join(dir, "repo")
			repo, _, err := New2(
				ctx,
				RepositoryConfig{
					URL:                 fmt.Sprintf("file://%s", path),
					Path:                path,
					ArgoCdGenerateFiles: true,
				},
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%v'", err)
			}
			repoInternal := repo.(*repository)
			repoInternal.config.ArgoWebhookUrl = "http://argo.example.com"
			rec := httptest.NewRecorder()
			resolver := TestWebhookResolver{
				t:        t,
				rec:      rec,
				requests: make(chan *http.Request, 1),
			}
			repoInternal.config.WebhookResolver = resolver

			// when
			repoInternal.config.WebURL = tc.webUrl
			repoInternal.config.Branch = tc.branch
			repoInternal.sendWebhookToArgoCd(ctx, logger, &tc.Changes)

			// then
			req := <-resolver.requests
			buf := make([]byte, req.ContentLength)
			if _, err = io.ReadFull(req.Body, buf); err != nil {
				t.Errorf("error reading request body: %v", err)
			}
			var jsonRequest map[string]any
			if err = json.Unmarshal(buf, &jsonRequest); err != nil {
				t.Errorf("Error parsing request body '%s' as json: %v", string(buf), err)
			}
			after := jsonRequest["after"].(string)
			if after != tc.Changes.Commits.Current.String() {
				t.Fatalf("after '%s' does not match current '%s'", after, tc.Changes.Commits.Current)
			}
			before := jsonRequest["before"].(string)
			if before != tc.Changes.Commits.Previous.String() {
				t.Fatalf("before '%s' does not match previous '%s'", before, tc.Changes.Commits.Previous)
			}
			ref := jsonRequest["ref"].(string)
			if ref != fmt.Sprintf("refs/heads/%s", tc.branch) {
				t.Fatalf("refs '%s' does not match expected for branch given as '%s'", ref, tc.branch)
			}
			repository := jsonRequest["repository"].(map[string]any)
			htmlUrl := repository["html_url"].(string)
			if htmlUrl != tc.webUrl {
				t.Fatalf("repository/html_url '%s' does not match expected for webUrl given as '%s'", htmlUrl, tc.webUrl)
			}
		})
	}
}
func TestLimit(t *testing.T) {
	transformers := []Transformer{
		&CreateEnvironment{
			Environment: "production",
			Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
		},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "manifest",
			},
		},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "manifest2",
			},
		},
	}
	tcs := []struct {
		Name               string
		numberBatchActions int
		ShouldSucceed      bool
		limit              int
		Setup              []Transformer
		ExpectedError      error
	}{
		{
			Name:               "less than maximum number of requests",
			ShouldSucceed:      true,
			limit:              5,
			numberBatchActions: 1,
			Setup:              transformers,
			ExpectedError:      nil,
		},
		{
			Name:               "more than the maximum number of requests",
			numberBatchActions: 10,
			limit:              5,
			ShouldSucceed:      false,
			Setup:              transformers,
			ExpectedError:      errMatcher{"queue is full. Queue Capacity: 5."},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {

			repo, err := setupRepositoryTestAux(t, 3)
			ctx := testutil.MakeTestContext()
			if err != nil {
				t.Fatal(err)
			}
			for _, tr := range tc.Setup {
				errCh := repo.(*repository).applyDeferred(ctx, tr)
				select {
				case e := <-repo.(*repository).queue.transformerBatches:
					dummyPushUpdateFunction := func(string, *bool) git.PushUpdateReferenceCallback { return nil }
					dummyPushActionFunction := func(options git.PushOptions, r *repository) PushActionFunc {
						return func() error {
							return nil
						}
					}
					repo.(*repository).ProcessQueueOnce(ctx, e, dummyPushUpdateFunction, dummyPushActionFunction)
				default:
				}
				select {
				case err := <-errCh:
					if err != nil {
						t.Fatal(err)
					}
				default:
				}
			}

			expectedErrorNumber := tc.numberBatchActions - tc.limit
			actualErrorNumber := 0
			for i := 0; i < tc.numberBatchActions; i++ {
				errCh := repo.(*repository).applyDeferred(ctx, transformers[0])
				select {
				case err := <-errCh:
					if tc.ShouldSucceed {
						t.Fatalf("Got an error at iteration %d and was not expecting it %v\n", i, err)
					}
					//Should get some errors, check if they are the ones we expect
					if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
						t.Errorf("error mismatch (-want, +got):\n%s", diff)
					}
					actualErrorNumber += 1
				default:
					// If there is no error,
				}
			}
			if expectedErrorNumber > 0 && expectedErrorNumber != actualErrorNumber {
				t.Errorf("error number mismatch expected: %d, got %d", expectedErrorNumber, actualErrorNumber)
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
		},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "manifest",
			},
		},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "manifest2",
			},
		},
	}
	tcs := []struct {
		Name                string
		shouldGenerateFiles bool
	}{
		{
			Name:                "ArgoCD files should NOT be generated",
			shouldGenerateFiles: false,
		},
		{
			Name:                "Argo CD files should be generated",
			shouldGenerateFiles: true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			cfg := RepositoryConfig{
				MaximumCommitsPerPush: 5,
				ArgoCdGenerateFiles:   tc.shouldGenerateFiles,
			}
			repo, err := setupRepository(t, cfg)
			state := repo.State()
			ctx := testutil.MakeTestContext()

			if err != nil {
				t.Fatal(err)
			}

			_, applyErr := repo.(*repository).ApplyTransformers(ctx, nil, transformers...)

			state = repo.State() //update state
			if applyErr != nil {
				t.Fatalf("Unexpected error applying transformers: Error: %v", applyErr)
			}

			if _, err := state.Filesystem.Stat("argocd"); errors.Is(err, os.ErrNotExist) {
				if tc.shouldGenerateFiles {
					t.Fatalf("Expected ArgoCD directory, but none was found. %v\n", err)
				}
			} else { //Argo CD dir exists
				if !tc.shouldGenerateFiles {
					t.Fatalf("ArgoCD files should not have been generated. Found ArgoCD directory.")
				}
			}

		})
	}
}

func setupRepository(t *testing.T, config RepositoryConfig) (Repository, error) {
	//t.Parallel()
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Start()
	cmd.Wait()

	config.URL = remoteDir
	config.Path = localDir
	config.CommitterEmail = "kuberpult@freiheit.com"
	config.CommitterName = "kuberpult"
	config.EnvironmentConfigsPath = filepath.Join(remoteDir, "..", "environment_configs.json")
	t.Logf("test created dir: %s", localDir)
	repo, _, err := New2(
		testutil.MakeTestContext(),
		config,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, nil
}

func setupRepositoryTestAux(t *testing.T, commits uint) (Repository, error) {
	t.Parallel()
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Start()
	cmd.Wait()
	t.Logf("test created dir: %s", localDir)
	repo, _, err := New2(
		testutil.MakeTestContext(),
		RepositoryConfig{
			URL:                    remoteDir,
			Path:                   localDir,
			CommitterEmail:         "kuberpult@freiheit.com",
			CommitterName:          "kuberpult",
			EnvironmentConfigsPath: filepath.Join(remoteDir, "..", "environment_configs.json"),
			MaximumCommitsPerPush:  commits,
			ArgoCdGenerateFiles:    true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, nil
}
