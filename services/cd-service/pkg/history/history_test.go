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
package history

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	git "github.com/libgit2/git2go/v33"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/fs"
)

type testCommit struct {
	Files    map[string]string
	Symlinks map[string]string
}

func TestHistory(t *testing.T) {
	tcs := []struct {
		Name            string
		Commits         []testCommit
		AssertChangedAt map[string]int
		Test            func(t *testing.T, repo *git.Repository, commits []*git.Commit)
	}{
		{
			Name: "one simple file is considered changed at the first commit",
			Commits: []testCommit{
				{
					Files: map[string]string{
						"foo": "bar",
					},
				},
			},
			AssertChangedAt: map[string]int{
				"foo": 0,
			},
			Test: func(t *testing.T, repo *git.Repository, commits []*git.Commit) {
				h := NewHistory(repo)
				head := commits[len(commits)-1]
				// Verify that we get the correct error for missing files
				{
					ch, err := h.Of(head)
					if err != nil {
						t.Fatal(err)
					}
					c, err := ch.Change([]string{"non_existing"})
					if c != nil {
						t.Errorf("commit mismatch, expected nil, but got %q", c.Id())
					}
					var notExists *NotExists
					if !errors.As(err, &notExists) {
						t.Errorf("unexpected error, expected an instance of *NotExists, but got %q", err)
					} else {
						path := filepath.Join(notExists.Path...)
						if path != "non_existing" {
							t.Errorf("unexpected error path, expected %q, but got %q", "non_existing", path)
						}
					}
				}
				// Verify that we get the correct error for wrong file types
				{
					ch, err := h.Of(head)
					if err != nil {
						t.Fatal(err)
					}
					c, err := ch.Change([]string{"foo", "non_existing"})
					if c != nil {
						t.Errorf("commit mismatch, expected nil, but got %q", c.Id())
					}
					var notExists *NotExists
					if !errors.As(err, &notExists) {
						t.Errorf("unexpected error, expected an instance of *NotExists, but got %q", err)
					} else {
						path := filepath.Join(notExists.Path...)
						if path != "foo/non_existing" {
							t.Errorf("unexpected error path, expected %q, but got %q", "foo/non_existing", path)
						}
					}
				}
			},
		},
		{
			Name: "a file that stays the same should not be considered changed",
			Commits: []testCommit{
				{
					Files: map[string]string{
						"foo": "bar",
					},
				},
				{
					Files: map[string]string{
						"foo": "baz",
					},
				},
				{
					Files: map[string]string{
						"foo": "baz",
					},
				},
			},
			AssertChangedAt: map[string]int{
				"foo": 1,
			},
		},
		{
			Name: "a directory that stays the same should not be considered changed",
			Commits: []testCommit{
				{
					Files: map[string]string{
						"foo/bar": "bar",
					},
				},
				{
					Files: map[string]string{
						"foo/bar": "baz",
					},
				},
				{
					Files: map[string]string{
						"foo/bar": "baz",
					},
				},
			},
			AssertChangedAt: map[string]int{
				"foo":     1,
				"foo/bar": 1,
			},
		},
		{
			Name: "a symlink is considered changed if its target gets changed",
			Commits: []testCommit{
				{
					Files: map[string]string{
						"foo/bar": "baz",
					},
					Symlinks: map[string]string{
						"foo/baz": "buz",
					},
				},
				{
					Files: map[string]string{
						"foo/bar": "baz",
					},
					Symlinks: map[string]string{
						"foo/baz": "bar",
					},
				},
				{
					Files: map[string]string{
						"foo/bar": "baz",
					},
					Symlinks: map[string]string{
						"foo/baz": "bar",
					},
				},
				{
					Files: map[string]string{
						"foo/bar": "boz",
					},
					Symlinks: map[string]string{
						"foo/baz": "bar",
					},
				},
			},
			AssertChangedAt: map[string]int{
				"foo":     3,
				"foo/bar": 3,
				"foo/baz": 1,
			},
		},
		{
			Name: "change detection works for deep paths",
			Commits: []testCommit{
				{
					Files: map[string]string{
						"foo/bar": "baz",
					},
				},
				{
					Files: map[string]string{
						"foo/bar":             "baz",
						"foo/1/2/3/4/5/6/bar": "baz",
					},
				},
			},
			AssertChangedAt: map[string]int{
				"foo/1/2/3/4/5/6/bar": 1,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			repo, err := git.InitRepository(dir, true)
			if err != nil {
				t.Fatal(err)
			}
			parents := []*git.Oid{}
			commits := make([]*git.Commit, len(tc.Commits))
			for i, c := range tc.Commits {
				fs := fs.NewEmptyTreeBuildFS(repo)
				for name, content := range c.Files {
					if li := strings.LastIndex(name, "/"); li != -1 {
						fs.MkdirAll(name[0:li], 0o755)
					}
					err := util.WriteFile(fs, name, []byte(content), 0o666)
					if err != nil {
						t.Fatal(err)
					}
				}
				if c.Symlinks != nil {
					for name, target := range c.Symlinks {
						if li := strings.LastIndex(name, "/"); li != -1 {
							fs.MkdirAll(name[0:li], 0o755)
						}
						err := fs.Symlink(target, name)
						if err != nil {
							t.Fatal(err)
						}
					}
				}

				tree, err := fs.Insert()
				if err != nil {
					t.Fatal(err)
				}
				sig := git.Signature{
					Name:  "test",
					Email: "test@test.com",
					When:  time.Unix(int64(i), 0),
				}
				p, err := repo.CreateCommitFromIds("", &sig, &sig, "test", tree, parents...)
				if err != nil {
					t.Fatal(err)
				}
				parents = []*git.Oid{p}
				commit, err := repo.LookupCommit(p)
				if err != nil {
					t.Fatal(err)
				}
				commits[i] = commit
			}
			if tc.AssertChangedAt != nil {
				// Run all tests once without cache
				for name, changedAt := range tc.AssertChangedAt {
					h := NewHistory(repo)
					ch, err := h.Of(commits[len(commits)-1])
					if err != nil {
						t.Fatal(err)
					}
					c, err := ch.Change(strings.Split(name, "/"))
					if err != nil {
						t.Errorf("unexpected error: %q", err)
					}
					assertChangedAtNthCommit(t, name, c, changedAt, commits)
				}
				// Run all tests once with cache
				h := NewHistory(repo)
				// Warm cache before doing the actual run
				for _, commit := range commits {
					ch, err := h.Of(commit)
					if err != nil {
						t.Fatal(err)
					}
					for name := range tc.AssertChangedAt {
						ch.Change(strings.Split(name, "/"))
					}
				}
				for name, changedAt := range tc.AssertChangedAt {
					ch, err := h.Of(commits[len(commits)-1])
					if err != nil {
						t.Fatal(err)
					}
					c, err := ch.Change(strings.Split(name, "/"))
					if err != nil {
						t.Errorf("unexpected error: %q", err)
					}
					assertChangedAtNthCommit(t, name, c, changedAt, commits)
				}
			}
			if tc.Test != nil {
				tc.Test(t, repo, commits)
			}
		})
	}
}

func nthParent(t *testing.T, from *git.Commit, offset int) *git.Commit {
	current := from
	for i := 0; i < offset; i++ {
		current = current.Parent(0)
	}
	return current
}

func assertChangedAtNthCommit(t *testing.T, name string, actualCommit *git.Commit, expectedPosition int, commits []*git.Commit) {
	t.Helper()
	if actualCommit == nil {
		t.Errorf("commit was nil, but expected non-nil")
		return
	}
	for i, c := range commits {
		if c.Id().Equal(actualCommit.Id()) {
			if i != expectedPosition {
				t.Errorf("wrong changed commit for %q, expected %d, actual %d", name, expectedPosition, i)
			}
			return
		}
	}
	t.Errorf("wrong changed commit for %q, expected %d, actually not any known commit", name, expectedPosition)
}

func BenchmarkHistoryNoCache(b *testing.B) {
	benchmarkHistory(b, false)
}

func BenchmarkHistoryCache(b *testing.B) {
	benchmarkHistory(b, true)
}

func benchmarkHistory(b *testing.B, cache bool) {
	names := []string{"a", "b", "c", "d", "e", "f"}
	dir := b.TempDir()
	repo, err := git.InitRepository(dir, true)
	if err != nil {
		b.Fatal(err)
	}
	parent := []*git.Oid{}
	tree := fs.NewEmptyTreeBuildFS(repo)
	for i := 0; i < 100; i++ {
		sig := git.Signature{
			Name:  "test",
			Email: "test@test.com",
			When:  time.Unix(int64(i), 0),
		}
		for _, name := range names {

			err := tree.MkdirAll(tree.Join("applications", name, "versions", strconv.Itoa(i)), 0o755)
			if err != nil {
				b.Fatal(err)
			}
			err = util.WriteFile(tree, tree.Join("applications", name, "versions", strconv.Itoa(i), "manifest"), []byte{1}, 0o644)
			if err != nil {
				b.Fatal(err)
			}
			oid, err := tree.Insert()
			if err != nil {
				b.Fatal(err)
			}
			p, err := repo.CreateCommitFromIds("", &sig, &sig, "test", oid, parent...)
			if err != nil {
				b.Fatal(err)
			}

			parent = []*git.Oid{p}
		}
	}
	commit, err := repo.LookupCommit(parent[0])
	if err != nil {
		b.Fatal(err)
	}

	warmup := NewHistory(repo)

	if cache {
		p := commit.Parent(0)
		for i := 0; i < 99; i++ {
			for _, name := range names {
				ch, err := warmup.Of(p)
				if err != nil {
					b.Fatal(err)
				}
				_, err = ch.Change([]string{"applications", name, "versions", strconv.Itoa(i), "manifest"})
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	}

	b.ResetTimer()

	// Test it!
	for n := 0; n < b.N; n++ {
		h := NewHistory(repo)
		if cache {
			h.cache = warmup.cache
		} else {
			h.cache = nil
		}
		for i := 0; i < 100; i++ {
			for _, name := range names {
				ch, err := h.Of(commit)
				if err != nil {
					b.Fatal(err)
				}
				_, err = ch.Change([]string{"applications", name, "versions", strconv.Itoa(i), "manifest"})
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	}
}

func dumpFs(t *testing.B, fs billy.Filesystem, indent string) {
	infos, err := fs.ReadDir(".")
	if err != nil {
		t.Logf("%s err: %q\n", indent, err)
	} else {
		for _, i := range infos {
			t.Logf("%s - %s\n", indent, i.Name())
			if i.Mode()&os.ModeSymlink != 0 {
				lnk, _ := fs.Readlink(i.Name())
				t.Logf("%s   linked to: %s\n", indent, lnk)
			}
			if i.IsDir() {
				ch, _ := fs.Chroot(i.Name())
				dumpFs(t, ch, indent+"  ")
			}
		}
	}
}
