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
	"github.com/go-git/go-billy/v5/util"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tcs := []struct {
		Name   string
		Branch string
		Setup  func(t *testing.T, remoteDir, localDir string)
		Test   func(t *testing.T, repo *Repository, remoteDir string)
	}{
		{
			Name:  "new in empty directory works",
			Setup: func(_ *testing.T, _, _ string) {},
		},
		{
			Name: "new in initialized repository works",
			Setup: func(t *testing.T, remoteDir, localDir string) {
				// run the initialization code once
				_, err := NewWait(
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
			Test: func(t *testing.T, repo *Repository, remoteDir string) {
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
				repo, err := NewWait(
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
			Test: func(t *testing.T, repo *Repository, remoteDir string) {
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
				repo, err := NewWait(
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
			Test: func(t *testing.T, repo *Repository, remoteDir string) {
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
			Test: func(t *testing.T, repo *Repository, remoteDir string) {
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
			Test: func(t *testing.T, repo *Repository, remoteDir string) {
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
					URL:    "file://" + remoteDir,
					Path:   localDir,
					Branch: tc.Branch,
				},
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}
			if waitErr := repo.WaitReady(); waitErr != nil {
				t.Fatalf("wait: expected no error, got '%s'", waitErr)
			}
			if tc.Test != nil {
				tc.Test(t, repo, remoteDir)
			}
		})
	}
}
