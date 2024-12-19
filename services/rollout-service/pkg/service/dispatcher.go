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
	"sync"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/cenkalti/backoff/v4"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
)

type knownRevision struct {
	revision string
	version  *versions.VersionInfo
}

// The dispatcher is responsible for enriching argo events with version data from kuberpult. It also maintains a backlog of applications where adding this data failed.
// The backlog is retried frequently so that missing data eventually can be resolved.
type Dispatcher struct {
	sink          ArgoEventProcessor
	versionClient versions.VersionClient
	mx            sync.Mutex
	known         map[Key]*knownRevision
	unknown       map[Key]*v1alpha1.ApplicationWatchEvent
	unknownCh     chan struct{}
	backoff       backoff.BackOff
}

func NewDispatcher(sink ArgoEventProcessor, vc versions.VersionClient) *Dispatcher {
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = 0
	bo.MaxInterval = 5 * time.Minute
	rs := &Dispatcher{
		mx:            sync.Mutex{},
		sink:          sink,
		versionClient: vc,
		known:         map[Key]*knownRevision{},
		unknown:       map[Key]*v1alpha1.ApplicationWatchEvent{},
		unknownCh:     make(chan struct{}, 1),
		backoff:       bo,
	}
	return rs
}

func (r *Dispatcher) Dispatch(ctx context.Context, k Key, ev *v1alpha1.ApplicationWatchEvent) {
	vs := r.tryResolve(ctx, k, ev)
	if vs != nil {
		r.sendEvent(ctx, k, vs, ev)
	}
}

func (r *Dispatcher) tryResolve(ctx context.Context, k Key, ev *v1alpha1.ApplicationWatchEvent) *versions.VersionInfo {
	r.mx.Lock()
	defer r.mx.Unlock()
	ddSpan, ctx := tracer.StartSpanFromContext(ctx, "tryResolve")
	defer ddSpan.Finish()
	revision := ev.Application.Status.Sync.Revision
	ddSpan.SetTag("argoSyncRevision", revision)

	// 0. Check if the app was deleted from kuberpult, if yes do nothing and wait for a delete event from argocd
	if revision == "" {
		return nil
	}
	// 1. Check if this is the delete event, if yes then we can delete the entry right away
	if ev.Type == "DELETED" {
		version := &versions.ZeroVersion
		r.known[k] = &knownRevision{
			revision: revision,
			version:  version,
		}
		delete(r.unknown, k)
		return version
	}
	// 2. Check if the revision has not changed
	if vi := r.known[k]; vi != nil && vi.revision == revision {
		delete(r.unknown, k)
		return vi.version
	}
	// 2. Check if the versions client knows this version already
	if version, err := r.versionClient.GetVersion(ctx, revision, k.Environment, k.Application); err == nil {
		r.known[k] = &knownRevision{
			revision: revision,
			version:  version,
		}
		delete(r.unknown, k)
		return version
	}
	// 4. Put this in the unknown queue and trigger the channel
	r.unknown[k] = ev
	select {
	case r.unknownCh <- struct{}{}:
	default:
	}
	return nil
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

func (r *Dispatcher) Work(ctx context.Context, hlth *setup.HealthReporter) error {
	hlth.ReportReady("dispatching")
	bo := backoff.WithContext(r.backoff, ctx)
	errored := false
	for {
		if errored {
			errored = false
			select {
			case <-ctx.Done():
				return nil
			case <-r.unknownCh:
			case <-time.After(bo.NextBackOff()):
			}
		} else {
			bo.Reset()
			select {
			case <-ctx.Done():
				return nil
			case <-r.unknownCh:
			}
		}

		keys := r.getUnknownKeys()
		for _, k := range keys {
			ev := r.getUnknown(k)
			if ev == nil {
				// The application was found in the meantime -> it's not unknown anymore
				continue
			}
			revision := ev.Application.Status.Sync.Revision
			version, err := r.versionClient.GetVersion(ctx, revision, k.Environment, k.Application)
			if err != nil {
				errored = true
				continue
			}
			r.foundUnknown(ctx, k, ev, version)
		}
	}
}

func (r *Dispatcher) getUnknownKeys() []Key {
	r.mx.Lock()
	defer r.mx.Unlock()
	keys := make([]Key, 0, len(r.unknown))
	for k := range r.unknown {
		keys = append(keys, k)
	}
	return keys
}
func (r *Dispatcher) getUnknown(k Key) *v1alpha1.ApplicationWatchEvent {
	r.mx.Lock()
	defer r.mx.Unlock()
	return r.unknown[k]
}

func (r *Dispatcher) foundUnknown(ctx context.Context, k Key, ev1 *v1alpha1.ApplicationWatchEvent, version *versions.VersionInfo) {
	r.mx.Lock()
	defer r.mx.Unlock()
	// We need to recheck here if a new event was observed while we were waiting.
	ev2 := r.unknown[k]
	if ev2 == nil {
		// Yes, there was a new event AND its version was resolved from cache. That means we don't need to do anything anymore.
		return
	}
	revision1 := ev1.Application.Status.Sync.Revision
	revision2 := ev2.Application.Status.Sync.Revision
	if revision1 != revision2 {
		// There was a new event AND its revision is different. We need to discard our version because it's for the wrong revision.
		return
	}
	delete(r.unknown, k)
	r.sendEvent(ctx, k, version, ev2)
}
