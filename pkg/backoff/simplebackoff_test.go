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

package backoff

import (
	"math"
	"testing"
	"time"
)

func TestCreateBackOffProviderNanoSecondsBugfix(t *testing.T) {
	tcs := []struct {
		Name                string
		inputDuration       time.Duration
		inputMaxDuration    time.Duration
		expectedMinDuration time.Duration
		expectedMaxDuration time.Duration
	}{
		{
			Name:                "Should have values in range 1-10 sec",
			inputDuration:       time.Second * 1,
			inputMaxDuration:    time.Second * 10,
			expectedMinDuration: time.Second * 1,
			expectedMaxDuration: time.Second * 10,
		},
		{
			Name:                "Should have values in range 2-10 sec",
			inputDuration:       time.Second * 2,
			inputMaxDuration:    time.Second * 10,
			expectedMinDuration: time.Second * 2,
			expectedMaxDuration: time.Second * 10,
		},
		{
			Name:                "Should have values in range 2-600 sec",
			inputDuration:       time.Second * 1,
			inputMaxDuration:    time.Second * 600,
			expectedMinDuration: time.Second * 1,
			expectedMaxDuration: time.Second * 600,
		},
		{
			Name:                "Should have values in range 1-maxInt sec",
			inputDuration:       time.Nanosecond * 1,
			inputMaxDuration:    math.MaxInt64,
			expectedMinDuration: time.Nanosecond * 1,
			expectedMaxDuration: time.Nanosecond * math.MaxInt64,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			provider := MakeSimpleBackoff(tc.inputDuration, tc.inputMaxDuration)

			var previousDuration time.Duration = 0
			for i := 0; i < 1000; i++ {
				actualDuration := provider.NextBackOff()
				if actualDuration < tc.expectedMinDuration {
					t.Errorf("expected at least %v seconds in loop[%d], got %v", tc.expectedMinDuration, i, actualDuration)
				}
				if actualDuration > tc.expectedMaxDuration {
					t.Errorf("expected a maximum of %v seconds in loop[%d], got %v", tc.expectedMaxDuration, i, actualDuration)
				}

				if i > 0 {
					if actualDuration < previousDuration {
						t.Errorf("expected actualDuration(%d) at step [%d] to be greater than previousDuration (%d)",
							actualDuration, i, previousDuration)
					}
				}
				previousDuration = actualDuration
			}
			if previousDuration != tc.expectedMaxDuration {
				t.Errorf("last duration was %v but it should have been %v", previousDuration, tc.expectedMaxDuration)
			}
		})
	}
}
