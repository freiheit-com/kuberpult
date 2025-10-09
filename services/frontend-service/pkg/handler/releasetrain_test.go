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

package handler

import (
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestIsTagEqual(t *testing.T) {
	tcs := []struct {
		name             string
		inputTagA        string
		inputTagB        string
		expectedResponse bool
	}{
		{
			name:             "must prefix refs/tags/",
			inputTagA:        "refs/tags/foo",
			inputTagB:        "foo",
			expectedResponse: true,
		},
		{
			name:             "absolutely identical, long form",
			inputTagA:        "refs/tags/foo",
			inputTagB:        "refs/tags/foo",
			expectedResponse: true,
		},
		{
			name:             "absolutely identical, short form",
			inputTagA:        "bar",
			inputTagB:        "bar",
			expectedResponse: true,
		},
		{
			name:             "not identical",
			inputTagA:        "bar",
			inputTagB:        "foo",
			expectedResponse: false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResponse1 := IsTagEqual(tc.inputTagA, tc.inputTagB)
			if diff := cmp.Diff(tc.expectedResponse, actualResponse1); diff != "" {
				t.Fatal(diff)
			}
			// it should work the same with different order of parameters:
			actualResponse2 := IsTagEqual(tc.inputTagB, tc.inputTagA)
			if diff := cmp.Diff(tc.expectedResponse, actualResponse2); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
