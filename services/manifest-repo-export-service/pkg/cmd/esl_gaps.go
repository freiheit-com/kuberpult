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
	"time"

	"go.uber.org/zap"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logging"
)

// trimAtGap prevents the export from moving the cutoff past ESL versions that are still being committed.
// The parameter `events` is the list of unprocessed events, with the oldest event at index 0.
//
// A slow transaction (e.g. a release train) can commit after transactions with higher versions,
// and the export would skip it forever once the cutoff has moved past it.
//
// It returns the events up to the first gap. A gap directly after the cutoff is skipped
// once the event after it is at least gracePeriod old. A gracePeriod of 0 disables the guard.
//
// The missing event has no timestamp, but cd-service writes the esl event first in its transaction,
// and "Created" is the start of that transaction. So the missing transaction has been open
// about as long as the event after it exists. "now" must be the database clock, like "Created".
func trimAtGap(ctx context.Context, gracePeriod time.Duration, cutoff *db.EslVersion, events []*db.EslEventRow, now time.Time) []*db.EslEventRow {
	if gracePeriod == 0 || len(events) == 0 {
		return events
	}
	// Without a cutoff (fresh database), the first event must be version 1, like in DBReadEslEventBatch.
	var cutoffVersion db.EslVersion
	if cutoff != nil {
		cutoffVersion = *cutoff
	}
	nextCutoff := cutoffVersion + 1
	prefix := consecutivePrefixSlice(events)
	nextEvent := events[0]
	if nextEvent.EslVersion == nextCutoff { // if there is no gap at the beginning
		return prefix // then we just return all consecutive events
	}

	// A gap exists at the front: calculate the age by looking at when the next Event was written (transaction start time)
	age := now.Sub(nextEvent.Created)
	if age < gracePeriod {
		// We are still within the gracePeriod, so we wait:
		return nil
	}
	// The missing versions were most likely rolled back. Not an Error/Warn because rollbacks should not spam the logs.
	logging.Info(ctx, "esl version gap accepted",
		zap.Any("missingFrom", nextCutoff),
		zap.Any("missingTo", nextEvent.EslVersion-1),
		zap.String("ageOfNextEvent", age.Round(time.Second).String()),
		zap.String("configuredGracePeriod", gracePeriod.String()),
	)
	return prefix
}

// consecutivePrefixSlice returns the events from the start of the slice up to the first gap.
// The slice must not be empty.
func consecutivePrefixSlice(events []*db.EslEventRow) []*db.EslEventRow {
	for i, event := range events {
		if i != 0 && (event.EslVersion != events[0].EslVersion+db.EslVersion(i)) {
			return events[:i]
		}
	}
	return events
}
