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

package fs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	billy "github.com/go-git/go-billy/v5"
	git "github.com/libgit2/git2go/v34"
)

type fileInfo struct {
	name string
	size int64
	mode os.FileMode
}

var _ os.FileInfo = (*fileInfo)(nil)

func (f *fileInfo) Name() string {
	return f.name
}

func (f *fileInfo) Size() int64 {
	return f.size
}

func (f *fileInfo) Mode() os.FileMode {
	return f.mode
}

func (f *fileInfo) IsDir() bool {
	return f.mode.IsDir()
}

func (f *fileInfo) ModTime() time.Time {
	return time.Time{}
}

func (f *fileInfo) Sys() interface{} {
	return nil
}

type treeBuilderEntry interface {
	load() error
	osInfo() os.FileInfo
	insert() (*git.Oid, git.Filemode, error)
}

type openMode int

const (
	closedMode openMode = iota
	readMode
	writeMode
)

type treeBuilderBlob struct {
	name       string
	repository *git.Repository
	oid        *git.Oid
	content    []byte
	mode       openMode
	pos        int
}

func (b *treeBuilderBlob) load() error {
	if b.content == nil {
		if b.oid != nil {
			c, err := b.repository.LookupBlob(b.oid)
			if err != nil {
				return err
			}
			b.content = c.Contents()
		}
	}
	return nil
}

func (b *treeBuilderBlob) insert() (*git.Oid, git.Filemode, error) {
	if b.content == nil {
		return b.oid, git.FilemodeBlob, nil
	} else {
		odb, err := b.repository.Odb()
		if err != nil {
			return nil, 0, err
		}
		oid, err := odb.Write(b.content, git.ObjectBlob)
		if err != nil {
			return nil, 0, err
		}
		return oid, git.FilemodeBlob, nil
	}
}

func (b *treeBuilderBlob) osInfo() os.FileInfo {
	b.load() //nolint: errcheck
	return &fileInfo{
		name: b.name,
		size: int64(len(b.content)),
		mode: 0666,
	}
}

func (b *treeBuilderBlob) Name() string {
	return b.name
}

func (b *treeBuilderBlob) Write(p []byte) (int, error) {
	if b.mode != writeMode {
		return 0, billy.ErrReadOnly
	}
	b.content = append(b.content[b.pos:], p...)
	b.pos += len(p)
	return len(p), nil
}

func (b *treeBuilderBlob) Read(p []byte) (int, error) {
	err := b.load()
	if err != nil {
		return 0, err
	}
	n := copy(p, b.content[b.pos:])
	b.pos += n
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (b *treeBuilderBlob) ReadAt(p []byte, off int64) (n int, err error) {
	panic("ReadAt is not implemented")
}

func (b *treeBuilderBlob) Seek(offset int64, whence int) (int64, error) {
	panic("Seek is not implemented")
}

func (b *treeBuilderBlob) Lock() error {
	return billy.ErrNotSupported
}

func (b *treeBuilderBlob) Unlock() error {
	return billy.ErrNotSupported
}

func (b *treeBuilderBlob) Truncate(s int64) error {
	if b.mode != writeMode {
		return billy.ErrReadOnly
	}
	b.content = b.content[0:s]
	return nil
}

func (b *treeBuilderBlob) Close() error {
	b.mode = closedMode
	return nil
}

type treeBuilderSymlink struct {
	name       string
	target     string
	oid        *git.Oid
	repository *git.Repository
}

func (b *treeBuilderSymlink) load() error {
	if b.target == "" {
		if b.oid != nil {
			bld, err := b.repository.LookupBlob(b.oid)
			if err != nil {
				return err
			}
			data := bld.Contents()
			b.target = string(data)
		}
	}
	return nil
}

func (b *treeBuilderSymlink) insert() (*git.Oid, git.Filemode, error) {
	if b.target == "" {
		return b.oid, git.FilemodeLink, nil
	} else {
		odb, err := b.repository.Odb()
		if err != nil {
			return nil, 0, err
		}
		oid, err := odb.Write([]byte(b.target), git.ObjectBlob)
		if err != nil {
			return nil, 0, err
		}
		return oid, git.FilemodeLink, nil
	}
}

func (b *treeBuilderSymlink) osInfo() os.FileInfo {
	return &fileInfo{
		size: 0,
		name: b.name,
		mode: os.ModeSymlink | os.ModePerm,
	}
}

type TreeBuilderFS struct {
	info       fileInfo
	entries    map[string]treeBuilderEntry
	repository *git.Repository
	oid        *git.Oid
	parent     *TreeBuilderFS
}

func (t *TreeBuilderFS) load() error {
	if t.entries == nil {
		tree, err := t.repository.LookupTree(t.oid)
		if err != nil {
			return err
		}
		result := map[string]treeBuilderEntry{}
		count := tree.EntryCount()
		for i := uint64(0); i < count; i++ {
			entry := tree.EntryByIndex(i)
			switch entry.Filemode {
			case git.FilemodeTree:
				result[entry.Name] = &TreeBuilderFS{
					entries: nil,
					info: fileInfo{
						size: 0,
						name: entry.Name,
						mode: os.ModeDir | os.ModePerm,
					},
					repository: t.repository,
					parent:     t,
					oid:        entry.Id,
				}
			case git.FilemodeBlob:
				result[entry.Name] = &treeBuilderBlob{
					content:    nil,
					mode:       0,
					pos:        0,
					name:       entry.Name,
					repository: t.repository,
					oid:        entry.Id,
				}
			case git.FilemodeLink:
				result[entry.Name] = &treeBuilderSymlink{
					target:     "",
					name:       entry.Name,
					repository: t.repository,
					oid:        entry.Id,
				}
			default:
				return fmt.Errorf("unsupported file mode %d", entry.Filemode)
			}
		}
		t.entries = result
	}
	return nil
}

func (t *TreeBuilderFS) insert() (*git.Oid, git.Filemode, error) {
	if t.entries == nil {
		// this tree wasn't modified, we can short-circuit it
		return t.oid, git.FilemodeTree, nil
	} else {
		bld, err := t.repository.TreeBuilder()
		if err != nil {
			return nil, 0, err
		}
		defer bld.Free()
		for name, entry := range t.entries {
			oid, mode, err := entry.insert()
			if err != nil {
				return nil, 0, err
			}
			if oid == nil {
				return nil, 0, fmt.Errorf("Oid is zero for %s %#v", name, entry)
			}
			err = bld.Insert(name, oid, mode)
			if err != nil {
				return nil, 0, err
			}
		}
		oid, err := bld.Write()
		if err != nil {
			return nil, 0, err
		}
		return oid, git.FilemodeTree, nil
	}
}

func (t *TreeBuilderFS) Insert() (*git.Oid, error) {
	oid, _, err := t.insert()
	return oid, err
}

func (t *TreeBuilderFS) osInfo() os.FileInfo {
	return &t.info
}

func (t *TreeBuilderFS) Create(filename string) (billy.File, error) {
	return t.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0666)
}

func (t *TreeBuilderFS) Open(filename string) (billy.File, error) {
	return t.OpenFile(filename, os.O_RDONLY, 0666)
}

func (t *TreeBuilderFS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	node, name, err := t.traverse(filename, false)
	if err != nil {
		return nil, err
	}
	if err := node.load(); err != nil {
		return nil, err
	}
	if entry, ok := node.entries[name]; ok {
		// found
		if file, ok := entry.(*treeBuilderBlob); ok {
			if file.mode == closedMode {
				if flag&os.O_RDONLY != 0 {
					file.mode = readMode
				} else {
					file.mode = writeMode
				}
				file.pos = 0
			} else {
				return nil, fmt.Errorf("file is already opened %q", filename)
			}
			return file, nil
		} else {
			return nil, fs.ErrInvalid
		}
	} else {
		if flag&os.O_CREATE != 0 {
			file := &treeBuilderBlob{
				oid:        nil,
				pos:        0,
				name:       name,
				repository: t.repository,
				mode:       writeMode,
				content:    []byte{},
			}
			node.entries[name] = file
			return file, nil
		} else {
			return nil, fs.ErrNotExist
		}
	}
}

func (t *TreeBuilderFS) Stat(filename string) (os.FileInfo, error) {
	node, rest, err := t.traverse(filename, false)
	if err != nil {
		return nil, err
	}
	if err := node.load(); err != nil {
		return nil, err
	}
	if entry, ok := node.entries[rest]; ok {
		return entry.osInfo(), nil
	} else {
		return nil, os.ErrNotExist
	}
}

func (t *TreeBuilderFS) Rename(oldpath, newpath string) error {
	panic("Rename is not implemented")
}

func (t *TreeBuilderFS) Remove(filename string) error {
	node, rest, err := t.traverse(filename, false)
	if err != nil {
		return err
	}
	if err := node.load(); err != nil {
		return err
	}
	delete(node.entries, rest)
	return nil
}

func (t *TreeBuilderFS) Join(elem ...string) string {
	return path.Join(elem...)
}

func (t *TreeBuilderFS) Root() string {
	return ""
}

func removeTrailingSlashes(in string) string {
	return strings.TrimRight(in, "/")
}

func (t *TreeBuilderFS) traverse(d string, createMissing bool) (*TreeBuilderFS, string, error) {
	parts := strings.Split(removeTrailingSlashes(d), "/")
	depth := 0
	current := t
	for {
		d := parts[0]
		parts = parts[1:]
		if len(parts) == 0 {
			return current, d, nil
		}
		if d == "." || d == "" {
			continue
		} else if d == ".." {
			if current.parent != nil {
				current = current.parent
				depth -= 1
			}
		} else {
			if err := current.load(); err != nil {
				return nil, "", err
			}
			s := current.entries[d]
			if s == nil {
				if createMissing {
					tree := NewEmptyTreeBuildFS(t.repository)
					tree.info.name = d
					tree.parent = current
					current.entries[d] = tree
					s = tree
				} else {
					return nil, "", fs.ErrNotExist
				}
			}
			if u, ok := s.(*TreeBuilderFS); ok {
				current = u
				depth += 1
			} else if l, ok := s.(*treeBuilderSymlink); ok {
				if err := l.load(); err != nil {
					return nil, "", err
				}
				ts := strings.Split(removeTrailingSlashes(l.target), "/")
				parts = append(ts, parts...)
				depth += 1
			} else {
				return nil, "", fs.ErrInvalid
			}
		}
	}
}

func (t *TreeBuilderFS) Chroot(dir string) (billy.Filesystem, error) {
	node, _, err := t.traverse(dir+"/.", false)
	return node, err
}

func (t *TreeBuilderFS) TempFile(dir, prefix string) (billy.File, error) {
	return nil, billy.ErrReadOnly
}

func (t *TreeBuilderFS) ReadDir(dir string) ([]os.FileInfo, error) {
	node, _, err := t.traverse(dir+"/.", false)
	if err != nil {
		if err == fs.ErrNotExist {
			return nil, nil
		}
		return nil, err
	}
	if err := node.load(); err != nil {
		return nil, err
	}
	result := make([]os.FileInfo, 0, len(node.entries))
	for _, entry := range node.entries {
		result = append(result, entry.osInfo())
	}
	return result, nil

}

func (t *TreeBuilderFS) MkdirAll(dir string, perm os.FileMode) error {
	_, _, err := t.traverse(dir+"/.", true)
	return err
}

func (t *TreeBuilderFS) Lstat(path string) (os.FileInfo, error) {
	// TODO(HVG): implement this to support actual symlinkk (https://github.com/freiheit-com/kuberpult/issues/1046)
	return t.Stat(path)
}

func (t *TreeBuilderFS) Readlink(path string) (string, error) {
	node, rest, err := t.traverse(path, false)
	if err != nil {
		return "", err
	}
	if err = node.load(); err != nil {
		return "", err
	}
	if entry, ok := node.entries[rest]; ok {
		if lnk, ok := entry.(*treeBuilderSymlink); ok {
			if err := lnk.load(); err != nil {
				return "", err
			} else {
				return lnk.target, nil
			}
		} else {
			return "", fs.ErrInvalid
		}
	} else {
		return "", fs.ErrNotExist
	}
}

func (t *TreeBuilderFS) Symlink(target, filename string) error {
	node, name, err := t.traverse(filename, false)
	if err != nil {
		return err
	}
	if err := node.load(); err != nil {
		return err
	}
	link := &treeBuilderSymlink{
		oid:        nil,
		name:       name,
		target:     target,
		repository: t.repository,
	}
	node.entries[name] = link
	return nil
}

func NewEmptyTreeBuildFS(repo *git.Repository) *TreeBuilderFS {
	return &TreeBuilderFS{
		oid:    nil,
		parent: nil,
		info: fileInfo{
			name: "",
			size: 0,
			mode: os.ModeDir | os.ModePerm,
		},
		repository: repo,
		entries:    map[string]treeBuilderEntry{},
	}
}

func NewTreeBuildFS(repo *git.Repository, oid *git.Oid) *TreeBuilderFS {
	return &TreeBuilderFS{
		entries: nil,
		parent:  nil,
		info: fileInfo{
			name: "",
			size: 0,
			mode: os.ModeDir | os.ModePerm,
		},
		repository: repo,
		oid:        oid,
	}
}

var _ billy.Filesystem = (*TreeBuilderFS)(nil)
