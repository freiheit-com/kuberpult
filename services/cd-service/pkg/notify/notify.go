
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
