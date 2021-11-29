/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package notify

import "sync"

type Notify struct {
	mx       sync.Mutex
	listener map[chan struct{}]struct{}
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
