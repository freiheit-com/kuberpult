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
	"slices"
	"sync"

	"github.com/freiheit-com/kuberpult/pkg/types"
)

type Notify struct {
	mx                    sync.Mutex
	oveviewListener       map[chan struct{}]struct{}
	changeAppsListener    map[chan ChangedAppNames]struct{}
	gitSyncStatusListener map[chan struct{}]struct{}
}

type Unsubscribe = func()

func (n *Notify) Subscribe() (<-chan struct{}, Unsubscribe) {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}

	n.mx.Lock()
	defer n.mx.Unlock()
	if n.oveviewListener == nil {
		n.oveviewListener = map[chan struct{}]struct{}{}
	}

	n.oveviewListener[ch] = struct{}{}
	return ch, func() {
		n.mx.Lock()
		defer n.mx.Unlock()
		delete(n.oveviewListener, ch)
	}
}

func (n *Notify) Notify() {
	n.mx.Lock()
	defer n.mx.Unlock()
	for ch := range n.oveviewListener {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

type ChangedAppNames []types.AppName

func (n *Notify) SubscribeChangesApps() (<-chan ChangedAppNames, Unsubscribe) {
	ch := make(chan ChangedAppNames, 1)
	ch <- ChangedAppNames{} // trigger complete reset by leaving changedApps empty

	n.mx.Lock()
	defer n.mx.Unlock()
	if n.changeAppsListener == nil {
		n.changeAppsListener = map[chan ChangedAppNames]struct{}{}
	}
	n.changeAppsListener[ch] = struct{}{}
	return ch, func() {
		n.mx.Lock()
		defer n.mx.Unlock()
		delete(n.changeAppsListener, ch)
	}
}

// mergeChangedAppNames unions a still-pending notification with the next one,
// in case one message was skipped.
// An empty list is the "all apps" sentinel (see SubscribeChangesApps), which
// already covers any concrete list. The result is deduped and sorted.
func mergeChangedAppNames(pending, next ChangedAppNames) ChangedAppNames {
	if len(pending) == 0 {
		return pending // sentinel: all apps
	}
	if len(next) == 0 {
		return next // sentinel: all apps
	}
	merged := slices.Clone(pending)
	for _, app := range next {
		if !slices.Contains(merged, app) {
			merged = append(merged, app)
		}
	}
	slices.Sort(merged)
	return merged
}

func (n *Notify) NotifyChangedApps(changedApps ChangedAppNames) {
	n.mx.Lock()
	defer n.mx.Unlock()
	for ch := range n.changeAppsListener {
		select {
		case ch <- changedApps:
		default:
			// The subscriber has not consumed the previous notification yet.
			// Dropping the new one would lose those app names for good — the
			// fast path sends each change exactly once — so merge the two
			// notifications instead.
			var pending ChangedAppNames
			hadPending := false
			select {
			case pending = <-ch:
				hadPending = true
			default:
				// The subscriber consumed it just now; the buffer is free again.
			}
			merged := changedApps
			if hadPending {
				merged = mergeChangedAppNames(pending, changedApps)
			}
			select {
			case ch <- merged:
			default:
				// Cannot happen: all senders hold n.mx and the only buffer slot
				// was just drained.
			}
		}
	}
}

func (n *Notify) SubscribeGitSyncStatus() (<-chan struct{}, Unsubscribe) {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}

	n.mx.Lock()
	defer n.mx.Unlock()
	if n.gitSyncStatusListener == nil {
		n.gitSyncStatusListener = map[chan struct{}]struct{}{}
	}

	n.gitSyncStatusListener[ch] = struct{}{}
	return ch, func() {
		n.mx.Lock()
		defer n.mx.Unlock()
		delete(n.gitSyncStatusListener, ch)
	}
}

func (n *Notify) NotifyGitSyncStatus() {
	n.mx.Lock()
	defer n.mx.Unlock()
	for ch := range n.gitSyncStatusListener {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
