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

package valid

import (
	"strings"
	"testing"
)

func TestValidLabel(t *testing.T) {
	tcs := []struct {
		Name           string
		InputLabel     string
		ExpectedResult bool
	}{
		{
			Name:           "empty",
			InputLabel:     "",
			ExpectedResult: true,
		},
		{
			Name:           "start with underscore",
			InputLabel:     "_foo",
			ExpectedResult: false,
		},
		{
			Name:           "end with underscore",
			InputLabel:     "foo_",
			ExpectedResult: false,
		},
		{
			Name:           "underscore in the middle",
			InputLabel:     "foo_bar",
			ExpectedResult: true,
		},
		{
			Name:           "exactly full",
			InputLabel:     strings.Repeat("x", 63),
			ExpectedResult: true,
		},
		{
			Name:           "too much",
			InputLabel:     strings.Repeat("x", 64),
			ExpectedResult: false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			actual := KubernetesLabelValue(tc.InputLabel)

			if actual != tc.ExpectedResult {
				t.Errorf("Expected %v, got %v for input %s", tc.ExpectedResult, actual, tc.InputLabel)
			}

		})
	}
}
