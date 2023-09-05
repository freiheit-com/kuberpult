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

Copyright 2023 freiheit.com*/

package notifier

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
)

var errChannelClosed error = fmt.Errorf("subscriber: channel closed")

var backOffFactory func() backoff.BackOff = func() backoff.BackOff {
	return backoff.NewExponentialBackOff()
}

func Subscribe(ctx context.Context, notifier Notifier, broadcast *service.Broadcast) error {
	s := subscriber{notifier: notifier, notifyStatus: map[key]*notifyStatus{}}
	bo := backOffFactory()
	for {
		err := s.subscribeOnce(ctx, broadcast)
		select {
		case <-ctx.Done():
			// the channel closed error is irrelevant when we shutdown
			if err == errChannelClosed {
				return nil
			}
			return err
		default:
			nb := bo.NextBackOff()
			if nb == backoff.Stop {
				return err
			} else {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(nb):
				}
			}
		}
	}
}

type key struct {
	environment string
	application string
}

type notifyStatus struct {
	targetVersion uint64
}

type subscriber struct {
	notifier     Notifier
	notifyStatus map[key]*notifyStatus
}

func (s *subscriber) subscribeOnce(ctx context.Context, broadcast *service.Broadcast) error {
	initial, ch, unsubscribe := broadcast.Start()
	defer unsubscribe()
	for _, ev := range initial {
		s.maybeSend(ctx, ev)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-ch:
			if !ok {
				// channel closed
				return errChannelClosed
			}
			s.maybeSend(ctx, ev)
		}
	}
}

func (s *subscriber) maybeSend(ctx context.Context, ev *service.BroadcastEvent) {
	// skip cases where we don't know the kuberpult version
	if ev.KuberpultVersion == 0 {
		return
	}
	// also don't notify when the version in argocd is already the right one
	if ev.ArgocdVersion == ev.KuberpultVersion {
		return
	}
	// also don't send events for the same version again
	k := key{ev.Environment, ev.Application}
	ns := s.notifyStatus[k]
	if ns != nil && ns.targetVersion == ev.KuberpultVersion {
		return
	}
	s.notifyStatus[k] = &notifyStatus{
		targetVersion: ev.KuberpultVersion,
	}
	// finally send the request
	s.notifier.NotifyArgoCd(ctx, ev.Environment, ev.Application)
}
