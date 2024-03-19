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
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/google/go-cmp/cmp"
)

func Test_roundtrip(t *testing.T) {
	for _, test := range []struct {
		Name  string
		Event Event
	}{
		{
			Name: "new-release",
			Event: &NewRelease{
				Environments: map[string]struct{}{
					"env1": {},
					"env2": {},
				},
			},
		},
		{
			Name: "deployment-basic",
			Event: &Deployment{
				Application: "app",
				Environment: "env",
			},
		},
		{
			Name: "deployment-1",
			Event: &Deployment{
				Application:                 "app1",
				Environment:                 "env1",
				SourceTrainEnvironmentGroup: ptr("A"),
			},
		},
		{
			Name: "deployment-2",
			Event: &Deployment{
				Application:                 "app1",
				Environment:                 "env1",
				SourceTrainEnvironmentGroup: ptr("A"),
				SourceTrainUpstream:         ptr("B"),
			},
		},
		{
			Name: "lock-prevented-deployment",
			Event: &LockPreventedDeployment{
				Application: "app",
				Environment: "env",
				LockMessage: "msg",
				LockType:    "application",
			},
		},
	} {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			fs := memfs.New()
			if err := Write(fs, "test", test.Event); err != nil {
				t.Fatal("writing  event:", err)
			}
			result, err := Read(fs, "test")
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(test.Event, result); diff != "" {
				t.Error("wrong result:\n", diff)
			}
		})
	}
}

func ptr[T any](x T) *T {
	return &x
}
