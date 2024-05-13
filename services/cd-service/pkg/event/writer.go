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

package event

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
)

func write(
	filesystem billy.Filesystem,
	dir string,
	val any,
) error {
	return writeFile(filesystem, dir, reflect.ValueOf(val), encodingDefault)
}

func writeFile(
	fs billy.Filesystem,
	file string,
	value reflect.Value,
	encoding encoding,
) error {
	tp := value.Type()
	switch tp.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		return writeFile(fs, file, value.Elem(), encoding)
	case reflect.String:
		val := value.Interface().(string)
		return util.WriteFile(fs, file, []byte(val), 0666)
	case reflect.Struct:
		if err := fs.MkdirAll(file, 0777); err != nil {
			return err
		}
		if tp.NumField() == 0 {
			return util.WriteFile(fs, fs.Join(file, ".gitkeep"), []byte{}, 0666)
		}
		for i := 0; i < tp.NumField(); i++ {
			f := tp.Field(i)
			name, enc := parseTag(f.Tag.Get("fs"))
			if name == "" {
				name = f.Name
			}
			if err := writeFile(fs,
				fs.Join(file, name),
				value.Field(i),
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
		if err := fs.MkdirAll(file, 0777); err != nil {
			return err
		}
		if value.Len() == 0 {
			return util.WriteFile(fs, fs.Join(file, ".gitkeep"), []byte{}, 0666)
		}
		iter := value.MapRange()
		for iter.Next() {
			fileName := iter.Key().Interface().(string)
			if err := writeFile(fs,
				fs.Join(file, fileName),
				iter.Value(),
				encodingDefault,
			); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		if err := fs.MkdirAll(file, 0777); err != nil {
			return err
		}
		if value.Len() == 0 {
			return util.WriteFile(fs, fs.Join(file, ".gitkeep"), []byte{}, 0666)
		}
		for i := 0; i < value.Len(); i++ {
			if err := writeFile(fs,
				fs.Join(file, fmt.Sprintf("%d", i)),
				value.Index(i),
				encodingDefault,
			); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("cannot write type %v", tp)
	}
}

type encoding int

const (
	encodingDefault encoding = iota
)

func parseTag(tag string) (name string, encoding encoding) {
	elems := strings.Split(tag, ",")
	if len(elems) == 0 {
		return "", encodingDefault
	}
	name = elems[0]
	if len(elems) == 1 {
		return name, encodingDefault
	}
	switch elems[1] {
	case "default", "":
		encoding = encodingDefault
	default:
		panic(fmt.Sprintf("unknown encoding: %q", elems[1]))
	}
	return name, encoding
}
