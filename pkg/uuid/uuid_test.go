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

package uuid

import (
	"github.com/onokonem/sillyQueueServer/timeuuid"
	"testing"
	"time"
)

// this tests that we can get the time out of a time-uuid.
func TestUuidTimeRoundTrip(t *testing.T) {
	tcs := []struct {
		Name      string
		InputTime time.Time
	}{
		{
			Name: "current time",
			// note the 0 at the end: we do not support nanosecond precision (milliseconds are good enough)
			InputTime: time.Date(2024, 1, 1, 1, 1, 1, 0, time.UTC),
		},
		{
			Name:      "future time",
			InputTime: time.Date(2042, 7, 1, 1, 1, 1, 0, time.UTC),
		},
		{
			Name:      "past time",
			InputTime: time.Date(1999, 12, 7, 1, 1, 1, 0, time.UTC),
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			uuidFromTime := timeuuid.UUIDFromTime(tc.InputTime)
			timestamp := GetTime(&uuidFromTime)
			actualTime := timestamp.AsTime()

			if actualTime != tc.InputTime {
				t.Fatalf("expected a different time.\nExpected: %v\nGot %v", tc.InputTime, actualTime)
			}
		})
	}
}
