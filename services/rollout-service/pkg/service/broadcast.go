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
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
)

type Key struct {
	Application string
	Environment string
}

type appState struct {
	argocdVersion    *versions.VersionInfo
	kuberpultVersion *versions.VersionInfo
	rolloutStatus    api.RolloutStatus
	environmentGroup string
}

func (a *appState) applyArgoEvent(ev *ArgoEvent) *BroadcastEvent {
	status := rolloutStatus(ev)
	if a.rolloutStatus != status || a.argocdVersion == nil || a.argocdVersion.Version != ev.Version.Version {
		a.rolloutStatus = status
		a.argocdVersion = ev.Version
		return a.getEvent(ev.Application, ev.Environment)
	}
	return nil
}

func (a *appState) applyKuberpultEvent(ev *versions.KuberpultEvent) *BroadcastEvent {
	if a.kuberpultVersion == nil || a.kuberpultVersion.Version != ev.Version.Version {
		a.kuberpultVersion = ev.Version
		a.environmentGroup = ev.EnvironmentGroup
		return a.getEvent(ev.Application, ev.Environment)
	}
	return nil
}

func (a *appState) getEvent(application, environment string) *BroadcastEvent {
	rs := a.rolloutStatus
	if a.kuberpultVersion != nil && a.argocdVersion != nil && a.kuberpultVersion.Version != a.argocdVersion.Version {
		rs = api.RolloutStatus_RolloutStatusPending
	}
	return &BroadcastEvent{
		Key: Key{
			Environment: environment,
			Application: application,
		},
		EnvironmentGroup: a.environmentGroup,
		ArgocdVersion:    a.argocdVersion,
		RolloutStatus:    rs,
		KuberpultVersion: a.kuberpultVersion,
	}
}

type Broadcast struct {
	state    map[Key]*appState
	mx       sync.Mutex
	listener map[chan *BroadcastEvent]struct{}
}

func New() *Broadcast {
	return &Broadcast{
		state:    map[Key]*appState{},
		listener: map[chan *BroadcastEvent]struct{}{},
	}
}

// ProcessArgoEvent implements service.EventProcessor
func (b *Broadcast) ProcessArgoEvent(ctx context.Context, ev ArgoEvent) {
	b.mx.Lock()
	defer b.mx.Unlock()
	k := Key{
		Application: ev.Application,
		Environment: ev.Environment,
	}
	if b.state[k] == nil {
		b.state[k] = &appState{}
	}
	msg := b.state[k].applyArgoEvent(&ev)
	if msg == nil {
		return
	}
	desub := []chan *BroadcastEvent{}
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

func (b *Broadcast) ProcessKuberpultEvent(ctx context.Context, ev versions.KuberpultEvent) {
	b.mx.Lock()
	defer b.mx.Unlock()
	k := Key{
		Application: ev.Application,
		Environment: ev.Environment,
	}
	if b.state[k] == nil {
		b.state[k] = &appState{}
	}
	msg := b.state[k].applyKuberpultEvent(&ev)
	if msg == nil {
		return
	}
	desub := []chan *BroadcastEvent{}
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

// Disconnects all listeners. This is used in tests to check wheter subscribers handle reconnects
func (b *Broadcast) DisconnectAll() {
	b.mx.Lock()
	defer b.mx.Unlock()
	for l := range b.listener {
		close(l)
	}
	b.listener = make(map[chan *BroadcastEvent]struct{})
}

func (b *Broadcast) StreamStatus(req *api.StreamStatusRequest, svc api.RolloutService_StreamStatusServer) error {
	resp, ch, unsubscribe := b.Start()
	defer unsubscribe()
	for _, r := range resp {
		svc.Send(streamStatus(r))
	}
	for {
		select {
		case r := <-ch:
			if r == nil {
				// closed
				return nil
			}
			err := svc.Send(streamStatus(r))
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

func (b *Broadcast) Start() ([]*BroadcastEvent, <-chan *BroadcastEvent, unsubscribe) {
	b.mx.Lock()
	defer b.mx.Unlock()
	result := make([]*BroadcastEvent, 0, len(b.state))
	for key, app := range b.state {
		result = append(result, app.getEvent(key.Application, key.Environment))
	}
	ch := make(chan *BroadcastEvent, 100)
	b.listener[ch] = struct{}{}
	return result, ch, func() {
		b.mx.Lock()
		defer b.mx.Unlock()
		delete(b.listener, ch)
	}
}

type BroadcastEvent struct {
	Key
	EnvironmentGroup string
	ArgocdVersion    *versions.VersionInfo
	KuberpultVersion *versions.VersionInfo
	RolloutStatus    api.RolloutStatus
}

func streamStatus(b *BroadcastEvent) *api.StreamStatusResponse {
	version := uint64(0)
	if b.ArgocdVersion != nil {
		version = b.ArgocdVersion.Version
	}
	return &api.StreamStatusResponse{
		Environment:   b.Environment,
		Application:   b.Application,
		Version:       version,
		RolloutStatus: b.RolloutStatus,
	}
}

func rolloutStatus(ev *ArgoEvent) api.RolloutStatus {
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
	switch ev.HealthStatusCode {
	case health.HealthStatusDegraded, health.HealthStatusMissing:
		return api.RolloutStatus_RolloutStatusUnhealthy
	case health.HealthStatusProgressing, health.HealthStatusSuspended:
		return api.RolloutStatus_RolloutStatusProgressing
	case health.HealthStatusHealthy:
		return api.RolloutStatus_RolloutStatusSuccesful
	}
	return api.RolloutStatus_RolloutStatusUnknown
}

var _ ArgoEventProcessor = (*Broadcast)(nil)
var _ api.RolloutServiceServer = (*Broadcast)(nil)
