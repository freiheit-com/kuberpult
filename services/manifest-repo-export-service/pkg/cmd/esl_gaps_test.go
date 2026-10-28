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
	"context"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

func TestEslGapGuardTrimAtGap(t *testing.T) {
	now := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	tcs := []struct {
		Name             string
		GivenCutoff      *db.EslVersion
		GivenDisabled    bool
		GivenVersions    []db.EslVersion
		GivenAge         time.Duration // how old the events are, relative to now
		ExpectedVersions []db.EslVersion
	}{
		{
			Name:             "contiguous events are returned unchanged",
			GivenCutoff:      types.Ptr[db.EslVersion](10),
			GivenVersions:    []db.EslVersion{11, 12, 13},
			ExpectedVersions: []db.EslVersion{11, 12, 13},
		},
		{
			Name:             "events in front of a gap are returned, even if the gap is old",
			GivenCutoff:      types.Ptr[db.EslVersion](10),
			GivenVersions:    []db.EslVersion{11, 13},
			GivenAge:         time.Hour,
			ExpectedVersions: []db.EslVersion{11},
		},
		{
			Name:             "a gap that is younger than the grace period is waited for",
			GivenCutoff:      types.Ptr[db.EslVersion](10),
			GivenVersions:    []db.EslVersion{12, 13},
			GivenAge:         time.Minute,
			ExpectedVersions: []db.EslVersion{},
		},
		{
			Name:             "a gap that is exactly as old as the grace period is skipped",
			GivenCutoff:      types.Ptr[db.EslVersion](10),
			GivenVersions:    []db.EslVersion{12, 13},
			GivenAge:         3 * time.Minute,
			ExpectedVersions: []db.EslVersion{12, 13},
		},
		{
			// For example after a restart of the export, when the export was behind, or after a failed git push:
			Name:             "a gap that is much older than the grace period is skipped",
			GivenCutoff:      types.Ptr[db.EslVersion](10),
			GivenVersions:    []db.EslVersion{12, 13},
			GivenAge:         time.Hour,
			ExpectedVersions: []db.EslVersion{12, 13},
		},
		{
			Name:             "after a skipped gap, events are returned up to the next gap",
			GivenCutoff:      types.Ptr[db.EslVersion](10),
			GivenVersions:    []db.EslVersion{12, 13, 15},
			GivenAge:         4 * time.Minute,
			ExpectedVersions: []db.EslVersion{12, 13},
		},
		{
			Name:             "a gap of several versions is skipped as a whole",
			GivenCutoff:      types.Ptr[db.EslVersion](10),
			GivenVersions:    []db.EslVersion{14, 15},
			GivenAge:         4 * time.Minute,
			ExpectedVersions: []db.EslVersion{14, 15},
		},
		{
			Name:             "a later gap is judged by the age of its own next event",
			GivenCutoff:      types.Ptr[db.EslVersion](13),
			GivenVersions:    []db.EslVersion{15},
			GivenAge:         time.Minute,
			ExpectedVersions: []db.EslVersion{},
		},
		{
			Name:             "without a cutoff, events that start at version 1 are returned unchanged",
			GivenCutoff:      nil,
			GivenVersions:    []db.EslVersion{1, 2},
			ExpectedVersions: []db.EslVersion{1, 2},
		},
		{
			Name:             "without a cutoff, a gap at the very beginning is waited for",
			GivenCutoff:      nil,
			GivenVersions:    []db.EslVersion{2},
			ExpectedVersions: []db.EslVersion{},
		},
		{
			Name:             "no events are returned unchanged",
			GivenCutoff:      types.Ptr[db.EslVersion](10),
			GivenVersions:    []db.EslVersion{},
			ExpectedVersions: []db.EslVersion{},
		},
		{
			Name:             "a grace period of 0 disables the guard",
			GivenCutoff:      types.Ptr[db.EslVersion](10),
			GivenDisabled:    true,
			GivenVersions:    []db.EslVersion{11, 13},
			ExpectedVersions: []db.EslVersion{11, 13},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			gracePeriod := 3 * time.Minute
			if tc.GivenDisabled {
				gracePeriod = 0
			}
			events := make([]*db.EslEventRow, 0, len(tc.GivenVersions))
			for _, version := range tc.GivenVersions {
				events = append(events, &db.EslEventRow{EslVersion: version, Created: now.Add(-tc.GivenAge)})
			}

			actual := trimAtGap(context.Background(), gracePeriod, tc.GivenCutoff, events, now)

			actualVersions := make([]db.EslVersion, 0, len(actual))
			for _, event := range actual {
				actualVersions = append(actualVersions, event.EslVersion)
			}
			if diff := testutil.CmpDiff(tc.ExpectedVersions, actualVersions); diff != "" {
				t.Errorf("versions mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
