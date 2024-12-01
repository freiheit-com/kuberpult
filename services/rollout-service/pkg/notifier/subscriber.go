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

package notifier

import (
	"context"
	"fmt"

	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
)

var errChannelClosed error = fmt.Errorf("subscriber: channel closed")

func Subscribe(ctx context.Context, notifier Notifier, broadcast *service.Broadcast, health *setup.HealthReporter) error {
	s := subscriber{notifier: notifier, notifyStatus: map[key]*notifyStatus{}}
	return health.Retry(ctx, func() error {
		initial, ch, unsubscribe := broadcast.Start()
		health.ReportReady("notifying")
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
					// this can happen in two cases
					select {
					// 1. we are shutting down. Then it's expected and not an error
					case <-ctx.Done():
						return nil
					// 2. when this subscriber fell behind too much when consuming. Then it's an error and should be handled
					default:
						return errChannelClosed
					}
				}
				go s.maybeSend(ctx, ev)
			}
		}
	})
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
	if ev.KuberpultVersion == nil {
		return
	}
	// also don't notify when the version in argocd is already the right one
	if ev.ArgocdVersion == ev.KuberpultVersion {
		return
	}
	// also don't send events for the same version again
	k := key{ev.Environment, ev.Application}
	ns := s.notifyStatus[k]
	if ns != nil && ns.targetVersion == ev.KuberpultVersion.Version {
		return
	}
	s.notifyStatus[k] = &notifyStatus{
		targetVersion: ev.KuberpultVersion.Version,
	}
	// finally send the request
	s.notifier.NotifyArgoCd(ctx, ev.Environment, ev.Application)
}
