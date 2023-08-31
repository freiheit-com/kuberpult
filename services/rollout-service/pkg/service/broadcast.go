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

package service

import (
	"context"
	"errors"
	"sync"

	"github.com/freiheit-com/kuberpult/pkg/api"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
)

type key struct {
	Application string
	Environment string
}

type Broadcast struct {
	state    map[key]Event
	mx       sync.Mutex
	listener map[chan *api.StreamStatusResponse]struct{}
}

func New() *Broadcast {
	return &Broadcast{
		state:    map[key]Event{},
		listener: map[chan *api.StreamStatusResponse]struct{}{},
	}
}

// Process implements service.EventProcessor
func (b *Broadcast) Process(ctx context.Context, ev Event) {
	b.mx.Lock()
	defer b.mx.Unlock()
	k := key{
		Application: ev.Application,
		Environment: ev.Environment,
	}
	if b.state[k] == ev {
		return
	}
	msg := &api.StreamStatusResponse{
		Environment:   ev.Environment,
		Application:   ev.Application,
		Version:       ev.Version,
		RolloutStatus: rolloutStatus(&ev),
	}
	b.state[k] = ev
	desub := []chan *api.StreamStatusResponse{}
	for l := range b.listener {
		select {
		case l <- msg:
		default:
			close(l)
			desub = append(desub, l)
		}
	}
	for _, l := range desub {
		delete(b.listener, l)
	}
}

func (b *Broadcast) StreamStatus(req *api.StreamStatusRequest, svc api.RolloutService_StreamStatusServer) error {
	resp, ch, unsubscribe := b.start()
	defer unsubscribe()
	for _, r := range resp {
		svc.Send(r)
	}
	for {
		select {
		case r := <-ch:
			err := svc.Send(r)
			if err != nil {
				return err
			}
		case <-svc.Context().Done():
			err := svc.Context().Err()
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
	}
}

type unsubscribe func()

func (b *Broadcast) start() ([]*api.StreamStatusResponse, <-chan *api.StreamStatusResponse, unsubscribe) {
	b.mx.Lock()
	defer b.mx.Unlock()
	result := make([]*api.StreamStatusResponse, 0, len(b.state))
	for _, ev := range b.state {
		msg := &api.StreamStatusResponse{
			Environment:   ev.Environment,
			Application:   ev.Application,
			Version:       ev.Version,
			RolloutStatus: rolloutStatus(&ev),
		}
		result = append(result, msg)
	}
	ch := make(chan *api.StreamStatusResponse, 100)
	b.listener[ch] = struct{}{}
	return result, ch, func() {
		b.mx.Lock()
		defer b.mx.Unlock()
		delete(b.listener, ch)
	}
}

func rolloutStatus(ev *Event) api.RolloutStatus {
	switch ev.HealthStatusCode {
	case health.HealthStatusDegraded, health.HealthStatusMissing:
		return api.RolloutStatus_RolloutStatusError
	case health.HealthStatusProgressing, health.HealthStatusSuspended:
		return api.RolloutStatus_RolloutStatusProgressing
	case health.HealthStatusHealthy:
		if ev.OperationState != nil {
			switch ev.OperationState.Phase {
			case common.OperationError, common.OperationFailed:

				return api.RolloutStatus_RolloutStatusError
			}
		}
		switch ev.SyncStatusCode {
		case v1alpha1.SyncStatusCodeOutOfSync:
			return api.RolloutStatus_RolloutStatusProgressing
		}
		return api.RolloutStatus_RolloutStatusSuccesful
	}
	return api.RolloutStatus_RolloutStatusUnknown
}

var _ EventProcessor = (*Broadcast)(nil)
var _ api.RolloutServiceServer = (*Broadcast)(nil)
