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

package event

import (
	"errors"
	"fmt"
	fserr "io/fs"
	"reflect"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
)

func read(
	filesystem billy.Filesystem,
	dir string,
	val any,
) error {
	v := reflect.ValueOf(val)
	return readFile(filesystem, dir, v.Elem(), v.Elem().Type(), encodingDefault)
}

func readFile(
	fs billy.Filesystem,
	file string,
	value reflect.Value,
	tp reflect.Type,
	encoding encoding,
) error {
	switch tp.Kind() {
	case reflect.Pointer:
		_, err := fs.Stat(file)
		if err != nil {
			if errors.Is(err, fserr.ErrNotExist) {
				return nil
			}
			return err
		}
		if value.IsNil() {
			value.Set(reflect.New(tp.Elem()))
		}
		return readFile(fs, file, value.Elem(), tp.Elem(), encoding)
	case reflect.String:
		cont, err := util.ReadFile(fs, file)
		if err != nil {
			return err
		}
		value.SetString(string(cont))
		return nil
	case reflect.Struct:
		for i := 0; i < tp.NumField(); i++ {
			f := tp.Field(i)
			name, enc := parseTag(f.Tag.Get("fs"))
			if name == "" {
				name = f.Name
			}
			if err := readFile(fs,
				fs.Join(file, name),
				value.Field(i),
				f.Type,
				enc,
			); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		if tp.Key().Kind() != reflect.String {
			return fmt.Errorf("can only use maps with strings as keys")
		}
		files, err := fs.ReadDir(file)
		if err != nil {
			return err
		}
		if value.IsNil() {
			value.Set(reflect.MakeMapWithSize(tp, len(files)))
		}
		for _, sub := range files {
			if sub.Name() == ".gitkeep" {
				continue
			}
			val := reflect.New(tp.Elem()).Elem()
			if err := readFile(fs,
				fs.Join(file, sub.Name()),
				val,
				tp.Elem(),
				encodingDefault,
			); err != nil {
				return err
			}
			value.SetMapIndex(reflect.ValueOf(sub.Name()), val)
		}
		return nil
	default:
		return fmt.Errorf("cannot read type %v", tp)
	}
}
