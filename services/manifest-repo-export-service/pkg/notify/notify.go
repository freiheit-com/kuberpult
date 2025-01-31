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
	syncStatusListener map[chan struct{}]struct{}
}

type Unsubscribe = func()

// Meant to be called when service wants to subscribe to any updates
func (n *Notify) Subscribe() (<-chan struct{}, Unsubscribe) {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}

	n.mx.Lock()
	defer n.mx.Unlock()
	if n.syncStatusListener == nil {
		n.syncStatusListener = map[chan struct{}]struct{}{}
	}

	n.syncStatusListener[ch] = struct{}{}
	return ch, func() {
		n.mx.Lock()
		defer n.mx.Unlock()
		delete(n.syncStatusListener, ch)
	}
}

// Meant to be called when service needs to be notified on new change
func (n *Notify) Notify() {
	n.mx.Lock()
	defer n.mx.Unlock()
	for ch := range n.syncStatusListener {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
