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

package cmd

import (
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/freiheit-com/kuberpult/pkg/errors"
	"github.com/google/go-cmp/cmp"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
)

func TestCalculateProcessDelay(t *testing.T) {
	exampleTime, err := time.Parse("2006-01-02 15:04:05", "2024-06-18 16:14:07")
	if err != nil {
		t.Fatal(err)
	}
	exampleTime10SecondsBefore := exampleTime.Add(-10 * time.Second)
	tcs := []struct {
		Name          string
		eslEvent      *db.EslEventRow
		currentTime   time.Time
		ExpectedDelay float64
	}{
		{
			Name:          "Should return 0 if there are no events",
			eslEvent:      nil,
			currentTime:   time.Now(),
			ExpectedDelay: 0,
		},
		{
			Name:          "Should return 0 if time created is not set",
			eslEvent:      &db.EslEventRow{},
			currentTime:   time.Now(),
			ExpectedDelay: 0,
		},
		{
			Name: "With one Event",
			eslEvent: &db.EslEventRow{
				EslVersion: 1,
				Created:    exampleTime10SecondsBefore,
				EventType:  "CreateApplicationVersion",
				EventJson:  "{}",
			},
			currentTime:   exampleTime,
			ExpectedDelay: 10,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := testutil.MakeTestContext()
			delay, err := calculateProcessDelay(ctx, tc.eslEvent, tc.currentTime)
			if err != nil {
				t.Fatal(err)
			}
			if delay != tc.ExpectedDelay {
				t.Errorf("expected %f, got %f", tc.ExpectedDelay, delay)
			}
		})
	}
}

func TestCalcSleep(t *testing.T) {
	tcs := []struct {
		Name              string
		inputError        error
		inputBackOff      backoff.BackOff
		eslEventSkipped   bool
		eslTableEmpty     bool
		expectedSleepData *SleepData
	}{
		{
			Name:              "Should return 0 if there is no error and eslEventSkipped=false",
			inputError:        nil,
			inputBackOff:      backoff.NewConstantBackOff(0),
			eslEventSkipped:   true,
			eslTableEmpty:     false,
			expectedSleepData: nil,
		},
		{
			Name:            "Should return resetTimer/500 if there is no error and eslEventSkipped=true",
			inputError:      nil,
			inputBackOff:    backoff.NewExponentialBackOff(backoff.WithRandomizationFactor(0)),
			eslEventSkipped: false,
			eslTableEmpty:   true,
			expectedSleepData: &SleepData{
				WarnMessage:   "",
				InfoMessage:   "sleeping for 500ms before looking for the first event again",
				SleepDuration: time.Millisecond * 500,
				FetchRepo:     false,
				ResetTimer:    false,
			},
		},
		{
			Name:            "Should return 'reset' if there is no error everything's false",
			inputError:      nil,
			inputBackOff:    backoff.NewConstantBackOff(0),
			eslEventSkipped: false,
			eslTableEmpty:   false,
			expectedSleepData: &SleepData{
				WarnMessage:   "",
				InfoMessage:   "",
				SleepDuration: 0,
				FetchRepo:     false,
				ResetTimer:    true,
			},
		},
		{
			Name:            "transaction error",
			inputError:      errors.RetryTransaction(fmt.Errorf("hello")),
			inputBackOff:    backoff.NewConstantBackOff(time.Millisecond * 250),
			eslEventSkipped: false,
			eslTableEmpty:   false,
			expectedSleepData: &SleepData{
				WarnMessage:   "transactional error: retry error for kind 'transaction': hello",
				InfoMessage:   "",
				SleepDuration: time.Millisecond * 250,
				FetchRepo:     false,
				ResetTimer:    false,
			},
		},
		{
			Name:            "git error",
			inputError:      errors.RetryGitRepo(fmt.Errorf("holla")),
			inputBackOff:    backoff.NewConstantBackOff(time.Millisecond * 123),
			eslEventSkipped: false,
			eslTableEmpty:   false,
			expectedSleepData: &SleepData{
				WarnMessage:   "could not update git repo",
				InfoMessage:   "",
				SleepDuration: time.Millisecond * 123,
				FetchRepo:     true,
				ResetTimer:    false,
			},
		},
		{
			Name:            "else",
			inputError:      nil,
			inputBackOff:    backoff.NewConstantBackOff(0),
			eslEventSkipped: false,
			eslTableEmpty:   false,
			expectedSleepData: &SleepData{
				WarnMessage:   "",
				InfoMessage:   "",
				SleepDuration: 0,
				FetchRepo:     false,
				ResetTimer:    true,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			actualSleepData := calcSleep(tc.inputError, tc.inputBackOff, tc.eslEventSkipped, tc.eslTableEmpty)
			if diff := cmp.Diff(actualSleepData, tc.expectedSleepData); diff != "" {
				t.Errorf("expected %v, got %v, diff:\n%s", tc.expectedSleepData, actualSleepData, diff)
			}
		})
	}
}
