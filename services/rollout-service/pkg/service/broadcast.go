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

package service

import (
	"context"
	"errors"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"sync"
	"time"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
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
	isProduction     *bool
	team             string
}

func (a *appState) applyArgoEvent(ctx context.Context, ev *ArgoEvent) *BroadcastEvent {
	l := logger.FromContext(ctx).Sugar()
	//l.Infof("applyArgoEvent: %v", ev)
	status := rolloutStatus(ev)
	l.Infof("applyArgoEvent: event=%v; status=%v", ev, status)
	if a.rolloutStatus != status || !a.argocdVersion.Equal(ev.Version) {
		a.rolloutStatus = status
		a.argocdVersion = ev.Version
		evt := a.getEvent(ev.Application, ev.Environment)
		l.Infof("applyArgoEvent: new event applied (%v, %v)=%v", ev.Application, ev.Environment, evt)
		return evt
	}
	l.Infof("applyArgoEvent: ignored. Status: %v,%v; version: %v %v", a.rolloutStatus, status, a.argocdVersion, ev.Version)
	return nil
}

func (a *appState) applyKuberpultEvent(ev *versions.KuberpultEvent) *BroadcastEvent {
	if !a.argocdVersion.Equal(ev.Version) || a.isProduction == nil || *a.isProduction != ev.IsProduction {
		a.kuberpultVersion = ev.Version
		a.environmentGroup = ev.EnvironmentGroup
		a.team = ev.Team
		a.isProduction = conversion.Bool(ev.IsProduction)
		return a.getEvent(ev.Application, ev.Environment)
	}
	return nil
}

func (a *appState) getEvent(application, environment string) *BroadcastEvent {
	rs := a.rolloutStatus
	if a.kuberpultVersion == nil || a.argocdVersion == nil {
		if rs == api.RolloutStatus_ROLLOUT_STATUS_SUCCESFUL {
			rs = api.RolloutStatus_ROLLOUT_STATUS_UNKNOWN
		}
	} else if a.kuberpultVersion.Version != a.argocdVersion.Version {
		rs = api.RolloutStatus_ROLLOUT_STATUS_PENDING
	}
	return &BroadcastEvent{
		Key: Key{
			Environment: environment,
			Application: application,
		},
		EnvironmentGroup: a.environmentGroup,
		IsProduction:     a.isProduction,
		ArgocdVersion:    a.argocdVersion,
		RolloutStatus:    rs,
		Team:             a.team,
		KuberpultVersion: a.kuberpultVersion,
	}
}

type Broadcast struct {
	state    map[Key]*appState
	mx       sync.Mutex
	listener map[chan *BroadcastEvent]struct{}

	// The waiting function is used in tests to trigger events after the subscription is set up.
	waiting func()
}

func New() *Broadcast {
	return &Broadcast{
		mx:       sync.Mutex{},
		waiting:  nil,
		state:    map[Key]*appState{},
		listener: map[chan *BroadcastEvent]struct{}{},
	}
}

// ProcessArgoEvent implements service.EventProcessor
func (b *Broadcast) ProcessArgoEvent(ctx context.Context, ev ArgoEvent) {
	l := logger.FromContext(ctx).Sugar()
	l.Info("ProcessArgoEvent: %v", ev)
	b.mx.Lock()
	defer b.mx.Unlock()
	k := Key{
		Application: ev.Application,
		Environment: ev.Environment,
	}
	if b.state[k] == nil {
		//exhaustruct:ignore
		b.state[k] = &appState{}
	}
	msg := b.state[k].applyArgoEvent(ctx, &ev)
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
		//exhaustruct:ignore
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
		err := svc.Send(streamStatus(r))
		if err != nil {
			return err
		}
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

func (b *Broadcast) GetStatus(ctx context.Context, req *api.GetStatusRequest) (*api.GetStatusResponse, error) {
	l := logger.FromContext(ctx).Sugar()
	l.Info("GetStatus...")
	var wait <-chan time.Time
	if req.WaitSeconds > 0 {
		wait = time.After(time.Duration(req.WaitSeconds) * time.Second)
	}
	resp, ch, unsubscribe := b.Start()
	defer unsubscribe()
	apps := map[Key]*api.GetStatusResponse_ApplicationStatus{}
	l.Infof("GetStatus response: %v: %v", len(resp), resp)
	for _, r := range resp {
		s := filterApplication(ctx, req, r)
		l.Infof("GetStatus filter for %v: %v", r, s == nil)
		if s != nil {
			apps[r.Key] = s
			l.Infof("GetStatus app[r.key]=s. key=%v, s=%v", r.Key, s)
		}
	}
	status := aggregateStatus(apps)
	l.Infof("GetStatus aggregate=%v", status)
	if wait != nil {
		// The waiting function is used in testing to make sure, we are really processing delayed events.
		if b.waiting != nil {
			b.waiting()
		}
	waiting:
		for {
			l.Info("GetStatus for...")

			status = aggregateStatus(apps)
			if status == api.RolloutStatus_ROLLOUT_STATUS_SUCCESFUL || status == api.RolloutStatus_ROLLOUT_STATUS_ERROR {
				break
			}
			select {
			case r, ok := <-ch:
				if !ok {
					break waiting
				}
				s := filterApplication(ctx, req, r)
				if s != nil {
					apps[r.Key] = s
				} else {
					delete(apps, r.Key)
				}
			case <-ctx.Done():
				break waiting
			case <-wait:
				break waiting
			}
		}
	}

	appList := make([]*api.GetStatusResponse_ApplicationStatus, 0, len(apps))
	for _, app := range apps {
		appList = append(appList, app)
	}

	return &api.GetStatusResponse{
		Status:       status,
		Applications: appList,
	}, nil
}

// Removes irrelevant app states from the list.
func filterApplication(ctx context.Context, req *api.GetStatusRequest, ev *BroadcastEvent) *api.GetStatusResponse_ApplicationStatus {
	l := logger.FromContext(ctx).Sugar()

	// Only apps that have the correct envgroup are considered
	if ev.EnvironmentGroup != req.EnvironmentGroup {
		l.Infof("filterApplication team ev.envgroup=%v, req.envgroup=%v", ev.EnvironmentGroup, req.EnvironmentGroup)
		return nil
	}
	// If it's filtered by team, then only apps with the correct team are considered.
	if req.Team != "" && req.Team != ev.Team {
		l.Infof("filterApplication team req.Team=%v, ev.Team=%v", req.Team, ev.Team)
		return nil
	}
	s := getStatus(ev)
	// Successful apps are also irrelevant.
	if s.RolloutStatus == api.RolloutStatus_ROLLOUT_STATUS_SUCCESFUL {
		l.Infof("filterApplication successful s=%v", s)
		return nil
	}
	l.Infof("filterApplication end s=%v", s)
	return s
}

// Calculates an aggregatted rollout status
func aggregateStatus(apps map[Key]*api.GetStatusResponse_ApplicationStatus) api.RolloutStatus {
	status := api.RolloutStatus_ROLLOUT_STATUS_SUCCESFUL
	for _, app := range apps {
		status = mostRelevantStatus(app.RolloutStatus, status)
	}
	return status
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
	Team             string
	IsProduction     *bool
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

func getStatus(b *BroadcastEvent) *api.GetStatusResponse_ApplicationStatus {
	return &api.GetStatusResponse_ApplicationStatus{
		Environment:   b.Environment,
		Application:   b.Application,
		RolloutStatus: b.RolloutStatus,
	}
}

func rolloutStatus(ev *ArgoEvent) api.RolloutStatus {
	if ev.OperationState != nil {
		switch ev.OperationState.Phase {
		case common.OperationError, common.OperationFailed:

			return api.RolloutStatus_ROLLOUT_STATUS_ERROR
		}
	}
	switch ev.SyncStatusCode {
	case v1alpha1.SyncStatusCodeOutOfSync:
		return api.RolloutStatus_ROLLOUT_STATUS_PROGRESSING
	}
	switch ev.HealthStatusCode {
	case health.HealthStatusDegraded, health.HealthStatusMissing:
		return api.RolloutStatus_ROLLOUT_STATUS_UNHEALTHY
	case health.HealthStatusProgressing, health.HealthStatusSuspended:
		return api.RolloutStatus_ROLLOUT_STATUS_PROGRESSING
	case health.HealthStatusHealthy:
		if ev.Version == nil {
			return api.RolloutStatus_ROLLOUT_STATUS_UNKNOWN
		}
		return api.RolloutStatus_ROLLOUT_STATUS_SUCCESFUL
	}
	return api.RolloutStatus_ROLLOUT_STATUS_UNKNOWN
}

// Depending on the rollout state, there are different things a user should do.
// 1. Nothing because everything is fine
// 2. Wait longer
// 3. Stop and call an operator
// The sorting is the same as in the UI.
var statusPriorities []api.RolloutStatus = []api.RolloutStatus{
	// Error is not recoverable by waiting and requires manual intervention
	api.RolloutStatus_ROLLOUT_STATUS_ERROR,

	// These states may resolve by waiting longer
	api.RolloutStatus_ROLLOUT_STATUS_PROGRESSING,
	api.RolloutStatus_ROLLOUT_STATUS_UNHEALTHY,
	api.RolloutStatus_ROLLOUT_STATUS_PENDING,
	api.RolloutStatus_ROLLOUT_STATUS_UNKNOWN,

	// This is the only successful state
	api.RolloutStatus_ROLLOUT_STATUS_SUCCESFUL,
}

// 0 is the highest priority - (RolloutStatusSuccesful) is the lowest priority
func statusPriority(a api.RolloutStatus) int {
	for i, p := range statusPriorities {
		if p == a {
			return i
		}
	}
	return len(statusPriorities) - 1
}

func mostRelevantStatus(a, b api.RolloutStatus) api.RolloutStatus {
	ap := statusPriority(a)
	bp := statusPriority(b)
	if ap < bp {
		return a
	} else {
		return b
	}
}

var _ ArgoEventProcessor = (*Broadcast)(nil)
var _ api.RolloutServiceServer = (*Broadcast)(nil)
