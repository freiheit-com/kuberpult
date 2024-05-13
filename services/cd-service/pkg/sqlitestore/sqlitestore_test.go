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

package sqlitestore

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	git "github.com/libgit2/git2go/v34"
)

func TestWriteAndRead(t *testing.T) {
	be, err := NewOdbBackend(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	odb, err := git.NewOdb()
	if err != nil {
		t.Fatal(err)
	}
	err = odb.AddBackend(be, 0)
	if err != nil {
		t.Fatal(err)
	}
	data := "foo"
	oid, err := odb.Write([]byte(data), git.ObjectBlob)
	if err != nil {
		t.Fatal(err)
	}
	result, err := odb.Read(oid)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(data, string(result.Data())); diff != "" {
		t.Errorf("result mismatch (-want, +got):\n%s", diff)
	}
}
