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

package sqlitestore

// #cgo pkg-config: sqlite3 libgit2
/*
#include <git2.h>
#include <sqlite3.h>
#include "sqlite.h"
*/
import "C"
import (
	"fmt"
	"unsafe"

	git "github.com/libgit2/git2go/v34"
)

func NewOdbBackend(name string) (*git.OdbBackend, error) {
	var (
		result  *C.git_odb_backend
		err_out *C.char
	)
	err := C.kp_backend_sqlite(&result, C.CString(name), &err_out)
	if err != C.SQLITE_OK {
		str := C.GoString(C.sqlite3_errstr(err))
		return nil, fmt.Errorf("sqlitestore: %d %s %s", err, str, C.GoString(err_out))
	}
	return git.NewOdbBackendFromC(unsafe.Pointer(result)), nil
}
