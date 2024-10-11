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
	"sync"
)

type Notify struct {
	mx                 sync.Mutex
	listener           map[chan struct{}]struct{}
	changeAppsListener map[chan []string][]string
}

type Unsubscribe = func()

func (n *Notify) Subscribe() (<-chan struct{}, Unsubscribe) {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}

	n.mx.Lock()
	defer n.mx.Unlock()
	if n.listener == nil {
		n.listener = map[chan struct{}]struct{}{}
	}

	n.listener[ch] = struct{}{}
	return ch, func() {
		n.mx.Lock()
		defer n.mx.Unlock()
		delete(n.listener, ch)
	}
}

func (n *Notify) Notify() {
	n.mx.Lock()
	defer n.mx.Unlock()
	for ch := range n.listener {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (n *Notify) SubscribeChangesApps() (<-chan []string, Unsubscribe) {
	ch := make(chan []string, 1)
	ch <- []string{}

	n.mx.Lock()
	defer n.mx.Unlock()
	if n.changeAppsListener == nil {
		n.changeAppsListener = map[chan []string][]string{}
	}
	n.changeAppsListener[ch] = []string{}
	return ch, func() {
		n.mx.Lock()
		defer n.mx.Unlock()
		delete(n.changeAppsListener, ch)
	}
}

func (n *Notify) NotifyChangedApps(changedApps []string) {
	n.mx.Lock()
	defer n.mx.Unlock()
	for ch := range n.changeAppsListener {
		select {
		case ch <- changedApps:
		default:
		}
	}
}
