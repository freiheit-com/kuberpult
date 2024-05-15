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

package tracing

import "testing"

func TestServiceName(t *testing.T) {
	tcs := []struct {
		Name         string
		DdServiceEnv string
		DefaultName  string
		ExpectedName string
	}{
		{
			Name:         "takes the default name without dd service name",
			DdServiceEnv: "",
			DefaultName:  "foo",
			ExpectedName: "foo",
		},
		{
			Name:         "prefers the dd service name over the default",
			DdServiceEnv: "bar",
			DefaultName:  "foo",
			ExpectedName: "bar",
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Setenv("DD_SERVICE", tc.DdServiceEnv)
			result := ServiceName(tc.DefaultName)
			if result != tc.ExpectedName {
				t.Errorf("wrong service name, expected %q, got %q", tc.ExpectedName, result)
			}
		})
	}

}
