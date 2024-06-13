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

package ptr

import (
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestSlice(t *testing.T) {
	tcs := []struct {
		Name     string
		Input    []int64
		Expected []uint64
	}{
		{
			Name:     "Empty Case",
			Input:    []int64{},
			Expected: []uint64{},
		},
		{
			Name:     "Simple Case",
			Input:    []int64{2, 4, 6, 99},
			Expected: []uint64{2, 4, 6, 99},
		},
		{
			Name:     "Complex Case",
			Input:    []int64{2, 4, 6, 99, 3, 5, 7},
			Expected: []uint64{2, 4, 6, 99, 3, 5, 7},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			actual := ToUint64Slice(tc.Input)
			if diff := cmp.Diff(tc.Expected, actual); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

		})
	}
}
