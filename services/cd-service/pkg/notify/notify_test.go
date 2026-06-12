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

package notify

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestNotifyChangedAppsMergesWhenSubscriberIsSlow verifies that change
// notifications are never lost: when the subscriber has not consumed the
// previous notification yet, the next one is merged into it instead of being
// dropped. The fast path sends each change exactly once, so a dropped
// notification would leave e.g. a bracket Argo app pinned to a stale snapshot
// until an unrelated later change happens to touch it again.
func TestNotifyChangedAppsMergesWhenSubscriberIsSlow(t *testing.T) {
	tcs := []struct {
		Name string
		// ConsumeSeed drains the initial "all apps" sentinel before notifying.
		ConsumeSeed   bool
		Notifications []ChangedAppNames
		WantReceived  ChangedAppNames
	}{
		{
			Name:          "single notification is delivered as-is",
			ConsumeSeed:   true,
			Notifications: []ChangedAppNames{{"app1"}},
			WantReceived:  ChangedAppNames{"app1"},
		},
		{
			Name:          "second notification merges instead of dropping",
			ConsumeSeed:   true,
			Notifications: []ChangedAppNames{{"app1"}, {"app2"}},
			WantReceived:  ChangedAppNames{"app1", "app2"},
		},
		{
			Name:          "merging dedupes and sorts",
			ConsumeSeed:   true,
			Notifications: []ChangedAppNames{{"b-app"}, {"a-app"}, {"b-app"}},
			WantReceived:  ChangedAppNames{"a-app", "b-app"},
		},
		{
			Name:          "pending all-apps sentinel absorbs concrete notifications",
			ConsumeSeed:   false,
			Notifications: []ChangedAppNames{{"app1"}},
			WantReceived:  ChangedAppNames{},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			var n Notify
			ch, unsubscribe := n.SubscribeChangesApps()
			defer unsubscribe()
			if tc.ConsumeSeed {
				<-ch
			}
			for _, notification := range tc.Notifications {
				n.NotifyChangedApps(notification)
			}
			got := <-ch
			if diff := cmp.Diff(tc.WantReceived, got); diff != "" {
				t.Errorf("received changed apps mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
