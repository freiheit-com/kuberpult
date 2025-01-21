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

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
)

// The dispatcher is responsible for enriching argo events with version data from kuberpult. It also maintains a backlog of applications where adding this data failed.
// The backlog is retried frequently so that missing data eventually can be resolved.
type Dispatcher struct {
	sink          ArgoEventProcessor
	versionClient versions.VersionClient
}

func NewDispatcher(sink ArgoEventProcessor, vc versions.VersionClient) *Dispatcher {
	rs := &Dispatcher{
		sink:          sink,
		versionClient: vc,
	}
	return rs
}

func (r *Dispatcher) Dispatch(ctx context.Context, k Key, ev *v1alpha1.ApplicationWatchEvent) {
	revision := ev.Application.Status.Sync.Revision
	version, _ := r.versionClient.GetVersion(ctx, revision, k.Environment, k.Application)
	if version != nil {
		r.sendEvent(ctx, k, version, ev)
	}
}

func (r *Dispatcher) sendEvent(ctx context.Context, k Key, version *versions.VersionInfo, ev *v1alpha1.ApplicationWatchEvent) {
	r.sink.ProcessArgoEvent(ctx, ArgoEvent{
		Application:      k.Application,
		Environment:      k.Environment,
		SyncStatusCode:   ev.Application.Status.Sync.Status,
		HealthStatusCode: ev.Application.Status.Health.Status,
		OperationState:   ev.Application.Status.OperationState,
		Version:          version,
	})
}
