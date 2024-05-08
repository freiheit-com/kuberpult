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

package sorting

import (
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestSortKeys(t *testing.T) {
	tcs := []struct {
		Name         string
		Input        map[string]string
		ExpectedKeys []string
	}{
		{
			Name:         "empty map",
			Input:        map[string]string{},
			ExpectedKeys: []string{},
		},
		{
			Name: "one element",
			Input: map[string]string{
				"a": "one",
			},
			ExpectedKeys: []string{"a"},
		},
		{
			Name: "many elements",
			Input: map[string]string{
				"a": "x",
				"d": "y",
				"c": "z",
				"b": "w",
			},
			ExpectedKeys: []string{"a", "b", "c", "d"},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			actualKeys := SortKeys(tc.Input)
			if diff := cmp.Diff(tc.ExpectedKeys, actualKeys); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
