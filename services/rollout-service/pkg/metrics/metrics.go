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

package metrics

import (
	"context"
	"fmt"
	pkgmetrics "github.com/freiheit-com/kuberpult/pkg/metrics"
	"math"
	"sync"
	"time"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func Metrics(ctx context.Context, bc *service.Broadcast, meterProvider metric.MeterProvider, clock func() time.Time, done func()) error {
	for {
		err := metrics(ctx, bc, meterProvider, clock, done)
		select {
		case <-ctx.Done():
			return err
		default:
		}
	}
}

func metrics(ctx context.Context, bc *service.Broadcast, meterProvider metric.MeterProvider, clock func() time.Time, done func()) error {
	if clock == nil {
		clock = time.Now
	}
	var err error
	meter := meterProvider.Meter("kuberpult")
	argoLag, err := meter.Int64ObservableGauge("rollout_lag_seconds")
	if err != nil {
		return fmt.Errorf("registering meter: %w", err)
	}
	var stateMx sync.Mutex
	state := map[service.Key]*appState{}

	reg, err := meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			stateMx.Lock()
			defer stateMx.Unlock()
			now := clock()
			for _, st := range state {
				if st != nil {
					o.ObserveInt64(argoLag, st.value(now), metric.WithAttributeSet(st.Attributes))
				}
			}
			return nil
		},
		argoLag,
	)
	if err != nil {
		return fmt.Errorf("registering callback: %w", err)
	}
	defer func() {
		err = reg.Unregister()
	}()

	st, ch, unsub := bc.Start()
	defer unsub()

	stateMx.Lock()
	for _, ev := range st {
		state[ev.Key] = state[ev.Key].update(ev)
	}
	done()
	stateMx.Unlock()
	for {
		select {
		case ev := <-ch:
			if ev == nil {
				return nil
			}
			stateMx.Lock()
			state[ev.Key] = state[ev.Key].update(ev)
			done()
			stateMx.Unlock()
		case <-ctx.Done():
			return err
		}
	}
}

type appState struct {
	Attributes attribute.Set
	DeployedAt time.Time
	Successful bool
}

func (a *appState) value(now time.Time) int64 {
	if a.Successful {
		return 0
	} else {
		return int64(math.Round(now.Sub(a.DeployedAt).Seconds()))
	}
}

func (a *appState) update(ev *service.BroadcastEvent) *appState {
	if ev.KuberpultVersion == nil {
		// If we don't know the kuberpult version at all, then we can't write this metric
		return nil
	}
	if ev.KuberpultVersion.DeployedAt.IsZero() {
		// Absent deployed at means the date is just missing.
		return nil
	}
	if ev.ArgocdVersion == nil {
		// We also need to know that something is in argocd
		return nil
	}
	sc := (ev.RolloutStatus == api.RolloutStatus_ROLLOUT_STATUS_SUCCESFUL || ev.RolloutStatus == api.RolloutStatus_ROLLOUT_STATUS_UNHEALTHY)
	// The environment group is the only thing that can change
	as := a.attributes(ev)
	return &appState{
		Attributes: as,
		Successful: sc,
		DeployedAt: ev.KuberpultVersion.DeployedAt,
	}
}

func (a *appState) attributes(ev *service.BroadcastEvent) attribute.Set {
	if a == nil {
		return buildAttributes(ev)
	}
	eg, _ := a.Attributes.Value("kuberpult_environment_group")
	if eg.AsString() == ev.EnvironmentGroup {
		return a.Attributes
	}
	return buildAttributes(ev)
}

func buildAttributes(ev *service.BroadcastEvent) attribute.Set {
	return attribute.NewSet(
		attribute.String(pkgmetrics.EventTagApplication, ev.Application),
		attribute.String(pkgmetrics.EventTagEnvironment, ev.Environment),
		attribute.String(pkgmetrics.EventTagEnvironmentGroup, ev.EnvironmentGroup),
	)
}
