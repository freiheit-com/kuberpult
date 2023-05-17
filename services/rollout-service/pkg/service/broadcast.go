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
	"sync"

	"github.com/freiheit-com/kuberpult/pkg/api"

	"github.com/argoproj/gitops-engine/pkg/health"
)

type key struct {
	Application string
	Environment string
}

type Broadcast struct {
	state map[key]Event
	mx    sync.Mutex
}

func New() *Broadcast {
	return &Broadcast{
		state: map[key]Event{},
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
	b.state[k] = ev
}

func (b *Broadcast) StreamStatus(req *api.StreamStatusRequest, svc api.RolloutService_StreamStatusServer) error {
	for _, ev := range b.state {
		msg := &api.StreamStatusResponse{
			Environment:   ev.Environment,
			Application:   ev.Application,
			Version:       ev.Version,
			RolloutStatus: rolloutStatus(&ev),
		}
		svc.Send(msg)
	}
	return nil
}

type unsubscribe interface {
}

func (b *Broadcast) start() ([]api.StreamStatusResponse, <-chan api.StreamStatusResponse, unsubscribe) {
	panic("unimplemented")
}

func rolloutStatus(ev *Event) api.RolloutStatus {
	switch ev.HealthStatusCode {
	case health.HealthStatusDegraded, health.HealthStatusMissing:
		return api.RolloutStatus_RolloutStatusError
	case health.HealthStatusProgressing, health.HealthStatusSuspended:
		return api.RolloutStatus_RolloutStatusProgressing
	case health.HealthStatusHealthy:
		return api.RolloutStatus_RolloutStatusSuccesful
	}
	return api.RolloutStatus_RolloutStatusUnknown
}

var _ EventProcessor = (*Broadcast)(nil)
var _ api.RolloutServiceServer = (*Broadcast)(nil)
