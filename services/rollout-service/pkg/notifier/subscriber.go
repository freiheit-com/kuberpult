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

	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
)

func Subscribe(ctx context.Context, notifier Notifier, broadcast *service.Broadcast) error {
	s := subscriber{notifier: notifier, notifyStatus: map[key]*notifyStatus{}}
	initial, ch, unsubscribe := broadcast.Start()
	defer unsubscribe()
	for _, ev := range initial {
		s.maybeSend(ctx, ev)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-ch:
			s.maybeSend(ctx, ev)
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
