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

package fs

import (
	"io"
	"io/fs"
	"os"
	"path"
	"reflect"
	"testing"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	git "github.com/libgit2/git2go/v34"
)

func TestFs(t *testing.T) {
	tcs := []struct {
		Name string
		Data func(fs billy.Filesystem) error
	}{
		{
			Name: "Writing one file",
			Data: func(fs billy.Filesystem) error {
				err := fs.MkdirAll("foo", 0777)
				if err != nil {
					return err
				}
				return util.WriteFile(fs, "foo/bar", []byte("baz"), 0666)
			},
		},
		{
			Name: "Deep path",
			Data: func(fs billy.Filesystem) error {
				err := fs.MkdirAll("foo/bar/baz/buz", 0777)
				if err != nil {
					return err
				}
				err = fs.MkdirAll("foo/bar/baz/boz/", 0777)
				if err != nil {
					return err
				}
				err = util.WriteFile(fs, "foo/bar/baz/buz/zup", []byte("baz"), 0666)
				if err != nil {
					return err
				}
				err = util.WriteFile(fs, "foo/bar/baz/boz/zup", []byte("baz"), 0666)
				if err != nil {
					return err
				}
				return nil
			},
		},
		{
			Name: "Symlink",
			Data: func(fs billy.Filesystem) error {
				err := fs.MkdirAll("foo", 0777)
				if err != nil {
					return err
				}
				err = fs.Symlink("foo", "bar")
				if err != nil {
					return err
				}
				err = util.WriteFile(fs, "bar/baz", []byte("baz"), 0666)
				if err != nil {
					return err
				}
				return nil
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, err := git.InitRepository(t.TempDir(), true)
			if err != nil {
				t.Fatal(err)
			}
			// create an actual file directory
			actual := osfs.New(t.TempDir())
			err = tc.Data(actual)
			if err != nil {
				t.Fatal(err)
			}

			// build a tree fs and compare the result
			tree := NewEmptyTreeBuildFS(repo)
			err = tc.Data(tree)
			if err != nil {
				t.Fatal(err)
			}
			compareDir(t, actual, tree, ".")

			// write the tree into git
			oid, _, err := tree.insert()
			if err != nil {
				t.Fatal(err)
			}

			// read the tree from git again
			tree2 := NewTreeBuildFS(repo, oid)
			compareDir(t, actual, tree2, ".")

			// checkout the tree into a folder an compare again
			gitTree, err := repo.LookupTree(oid)
			if err != nil {
				t.Fatal(err)
			}
			tmpDir := t.TempDir()
			err = repo.CheckoutTree(gitTree, &git.CheckoutOpts{
				Strategy:        git.CheckoutForce,
				TargetDirectory: tmpDir,
			})
			checkedOut := osfs.New(tmpDir)
			compareDir(t, tree, checkedOut, ".")
		})
	}
}

func compareDir(t *testing.T, expected, actual billy.Filesystem, dir string) {
	t.Helper()
	ar, err := actual.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	br, err := expected.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	type mapEntry struct{ a, b os.FileInfo }
	files := map[string]*mapEntry{}
	for _, af := range ar {
		files[af.Name()] = &mapEntry{a: af}
	}
	for _, bf := range br {
		if e, ok := files[bf.Name()]; ok {
			e.b = bf
		} else {
			files[bf.Name()] = &mapEntry{b: bf}
		}
	}
	for name, entry := range files {
		p := path.Join(dir, name)
		if entry.a == nil {
			t.Errorf("missing file: %s", p)
		} else if entry.b == nil {
			t.Errorf("unexpected file: %s", p)
		} else {
			if entry.a.Mode()&fs.ModeType != entry.b.Mode()&fs.ModeType {
				t.Errorf("mismatched mode for %s: expected %q, actual %q", p, entry.b.Mode(), entry.a.Mode())
			} else {
				if entry.a.IsDir() {
					compareDir(t, expected, actual, p)
				} else if entry.a.Mode().IsRegular() {
					compareContent(t, expected, actual, p)
				}
			}
			astat, err := actual.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			bstat, err := expected.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if astat.Name() != bstat.Name() {
				t.Errorf("mismateched stat name for %s: expected %q, actual %q", p, bstat.Name(), astat.Name())
			}
		}
	}
}

func compareContent(t *testing.T, expected, actual billy.Filesystem, file string) {
	t.Helper()
	af, err := actual.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer af.Close()
	ac, err := io.ReadAll(af)
	if err != nil {
		t.Fatal(err)
	}

	bf, err := expected.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer bf.Close()
	bc, err := io.ReadAll(bf)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ac, bc) {
		t.Errorf("content differs for %s:\n\texpected: %q\n\tactual: %q", file, bc, ac)
	}
}

func dumpFs(t *testing.T, fs billy.Filesystem, indent string) {
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
