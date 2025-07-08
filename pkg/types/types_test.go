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

package types

import (
	"github.com/google/go-cmp/cmp"
	"sort"
	"testing"
)

var versionOne = uint64(1)

func TestRevision(t *testing.T) {
	tcs := []struct {
		Name               string
		InitialCollection  ReleaseNumberCollection
		ExpectedCollection ReleaseNumberCollection
	}{
		{
			Name: "takes the default name without dd service name",
			InitialCollection: ReleaseNumberCollection{
				{
					Version:  &versionOne,
					Revision: "1.0",
				},
				{
					Version:  &versionOne,
					Revision: "3.0",
				},
				{
					Version:  &versionOne,
					Revision: "2.0",
				},
			},
			ExpectedCollection: ReleaseNumberCollection{

				{
					Version:  &versionOne,
					Revision: "1.0",
				},
				{
					Version:  &versionOne,
					Revision: "2.0",
				},
				{
					Version:  &versionOne,
					Revision: "3.0",
				},
			},
		},
		{
			Name: "takes the default name without dd service name",
			InitialCollection: ReleaseNumberCollection{
				{
					Version:  &versionOne,
					Revision: "1.3.1",
				},
				{
					Version:  &versionOne,
					Revision: "2.3.0",
				},
				{
					Version:  &versionOne,
					Revision: "1.1",
				},
			},
			ExpectedCollection: ReleaseNumberCollection{
				{
					Version:  &versionOne,
					Revision: "1.1",
				},
				{
					Version:  &versionOne,
					Revision: "1.3.1",
				},
				{
					Version:  &versionOne,
					Revision: "2.3.0",
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			sort.Sort(tc.InitialCollection)
			if diff := cmp.Diff(tc.InitialCollection, tc.ExpectedCollection); diff != "" {
				t.Fatalf("collection mismatch mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
