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
	"github.com/DataDog/dd-trace-go/v2/ddtrace/ext"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/mocktracer"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"testing"
)

func TestMarkAsDB(t *testing.T) {
	var toIgnoreKeys = []string{
		// these are automatically added by the dd lib, so we ignore them
		"_dd.base_service",
		"_dd.p.tid",
		"_dd.profiling.enabled",
		"_dd.top_level",
		"language",
		"span.name",
	}
	tcs := []struct {
		Name  string
		Query string
	}{
		{
			Name:  "takes the default name without dd service name",
			Query: "SELECT * FROM BRAIN",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			mt := mocktracer.Start()
			defer mt.Stop()

			{
				span := tracer.StartSpan("db:test")
				MarkSpanAsDB(span, tc.Query)
				span.Finish()
			}

			actualSpans := mt.FinishedSpans()

			if len(actualSpans) != 1 {
				t.Errorf("expected 1 span, got %d: %v", len(actualSpans), actualSpans)
			}
			expectedTags := map[string]string{
				"sql.query":      tc.Query,
				ext.ResourceName: tc.Query,
				ext.ServiceName:  "postgres-client",
				ext.SpanType:     ext.SpanTypeSQL,
				ext.DBType:       "postgres",
				ext.DBSystem:     "postgres",
			}

			span := actualSpans[0]
			actualTags := span.Tags()
			for _, toIgnoreValue := range toIgnoreKeys {
				delete(actualTags, toIgnoreValue)
			}

			for key, value := range expectedTags {
				actualValue, ok := actualTags[key]
				if !ok {
					t.Fatalf("expected tag %s=%s to exist", key, value)
				}
				if actualValue != value {
					t.Errorf("expected tag %s=%s got %s=%s", key, value, key, actualValue)
				}
			}
			if len(actualTags) != len(expectedTags) {
				t.Errorf("expected %d tags, got %d:\n%v", len(expectedTags), len(actualTags), actualTags)
			}
		})
	}

}
