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

package testfs

import (
	"fmt"
	"os"
	"sync"

	"github.com/go-git/go-billy/v5"
)

type Operation int

const (
	NONE Operation = iota
	CREATE
	OPEN
	OPENFILE
	STAT
	RENAME
	REMOVE
	READDIR
	SYMLINK
	READLINK
	MKDIRALL
)

func (o Operation) String() string {
	switch o {
	case CREATE:
		return "Create"
	case OPEN:
		return "Open"
	case OPENFILE:
		return "OpenFile"
	case STAT:
		return "Stat"
	case RENAME:
		return "Rename"
	case REMOVE:
		return "Remove"
	case READDIR:
		return "ReadDir"
	case SYMLINK:
		return "Symlink"
	case READLINK:
		return "ReadLink"
	case MKDIRALL:
		return "MkdirAll"
	}
	return fmt.Sprintf("unknown(%d)", o)
}

type FileOperation struct {
	Operation Operation
	Filename  string
}

type value struct {
	errored bool
}

// UsageCollector tracks which file operations have been used in a test suite and reports which were not
// tested with an injected error.
type UsageCollector struct {
	mx    sync.Mutex
	usage map[FileOperation]value
}

func (u *UsageCollector) used(op Operation, filename string) {
	u.mx.Lock()
	defer u.mx.Unlock()
	if u.usage == nil {
		u.usage = map[FileOperation]value{}
	}
	_, ok := u.usage[FileOperation{op, filename}]
	if !ok {
		u.usage[FileOperation{op, filename}] = value{
			errored: false,
		}
	}
}

func (u *UsageCollector) errored(op Operation, filename string) {
	u.mx.Lock()
	defer u.mx.Unlock()
	if u.usage == nil {
		u.usage = map[FileOperation]value{}
	}
	u.usage[FileOperation{op, filename}] = value{errored: true}
}

func (u *UsageCollector) UntestedOps() []FileOperation {
	u.mx.Lock()
	defer u.mx.Unlock()
	result := []FileOperation{}
	for k, v := range u.usage {
		if !v.errored {
			result = append(result, k)
		}
	}
	return result
}

type errorInjector struct {
	operation Operation
	filename  string
	err       error
	used      bool
	collector *UsageCollector
}

func (e *errorInjector) inject(op Operation, filename string) error {
	if e.used {
		return nil
	} else if e.operation == op && e.filename == filename {
		e.used = true
		e.collector.errored(op, filename)
		return e.err
	}
	e.collector.used(op, filename)
	return nil
}

func (uc *UsageCollector) WithError(fs billy.Filesystem, op Operation, filename string, err error) *Filesystem {
	return &Filesystem{
		Inner: fs,
		errorInjector: errorInjector{
			used:      false,
			operation: op,
			filename:  filename,
			err:       err,
			collector: uc,
		},
	}
}

// A special filesystem that allows injecting errors at arbitrary operations.
type Filesystem struct {
	Inner         billy.Filesystem
	errorInjector errorInjector
}

func (f *Filesystem) Create(filename string) (billy.File, error) {
	err := f.errorInjector.inject(CREATE, filename)
	if err != nil {
		return nil, err
	}
	return f.Inner.Create(filename)
}

func (f *Filesystem) Open(filename string) (billy.File, error) {
	err := f.errorInjector.inject(OPEN, filename)
	if err != nil {
		return nil, err
	}
	return f.Inner.Open(filename)
}

func (f *Filesystem) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	err := f.errorInjector.inject(OPENFILE, filename)
	if err != nil {
		return nil, err
	}
	return f.Inner.OpenFile(filename, flag, perm)
}

func (f *Filesystem) Stat(filename string) (os.FileInfo, error) {
	err := f.errorInjector.inject(STAT, filename)
	if err != nil {
		return nil, err
	}
	return f.Inner.Stat(filename)
}

func (f *Filesystem) Rename(oldpath, newpath string) error {
	err := f.errorInjector.inject(RENAME, oldpath)
	if err != nil {
		return err
	}
	return f.Inner.Rename(oldpath, newpath)
}

func (f *Filesystem) Remove(filename string) error {
	err := f.errorInjector.inject(REMOVE, filename)
	if err != nil {
		return err
	}
	return f.Inner.Remove(filename)
}

func (f *Filesystem) Join(elem ...string) string {
	return f.Inner.Join(elem...)
}

func (f *Filesystem) TempFile(dir, prefix string) (billy.File, error) {
	return f.Inner.TempFile(dir, prefix)
}

func (f *Filesystem) Lstat(filename string) (os.FileInfo, error) {
	return f.Inner.Lstat(filename)
}

func (f *Filesystem) Symlink(target, link string) error {
	err := f.errorInjector.inject(SYMLINK, link)
	if err != nil {
		return err
	}
	return f.Inner.Symlink(target, link)
}

func (f *Filesystem) Readlink(link string) (string, error) {
	err := f.errorInjector.inject(READLINK, link)
	if err != nil {
		return "", err
	}
	return f.Inner.Readlink(link)
}

func (f *Filesystem) ReadDir(path string) ([]os.FileInfo, error) {
	err := f.errorInjector.inject(READDIR, path)
	if err != nil {
		return nil, err
	}
	return f.Inner.ReadDir(path)
}

func (f *Filesystem) MkdirAll(filename string, perm os.FileMode) error {
	err := f.errorInjector.inject(MKDIRALL, filename)
	if err != nil {
		return err
	}
	return f.Inner.MkdirAll(filename, perm)
}

func (f *Filesystem) Chroot(path string) (billy.Filesystem, error) {
	panic("Chroot not implemented")
}

func (f *Filesystem) Root() string {
	panic("Root not implemented")
}
