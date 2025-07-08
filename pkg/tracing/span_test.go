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

import (
	"context"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"testing"
)

func TestMarkAsDB(t *testing.T) {
	tcs := []struct {
		Name string
	}{
		{
			Name: "takes the default name without dd service name",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			span, err := tracer.StartSpanFromContext(context.Background(), "db:test")
			if err != nil {
				t.Fatalf("error creating span: %v", err)
			}

			result := ServiceName(tc.DefaultName)
			if result != tc.ExpectedName {
				t.Errorf("wrong service name, expected %q, got %q", tc.ExpectedName, result)
			}
		})
	}

}
