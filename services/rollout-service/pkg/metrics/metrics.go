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

package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func Metrics(ctx context.Context, bc *service.Broadcast, meterProvider metric.MeterProvider, clock func() time.Time) error {
	if clock == nil {
		clock = time.Now
	}
	var err error
	meter := meterProvider.Meter("kuberpult")
	argoLag, err := meter.Float64ObservableGauge("argocd_lag")
	if err != nil {
		return fmt.Errorf("registering meter: %w", err)
	}

	state := map[string]*appState{}

	reg, err := meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			now := clock()
			for _, st := range state {
				if st != nil {
					o.ObserveFloat64(argoLag, st.value(now), metric.WithAttributeSet(st.Attributes))
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

	for _, ev := range st {
		k := fmt.Sprintf("%s|%s", ev.Environment, ev.Application)
		state[k] = state[k].update(ev)
	}
	for {
		select {
		case ev := <-ch:
			k := fmt.Sprintf("%s|%s", ev.Environment, ev.Application)
			state[k] = state[k].update(ev)
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

func (a *appState) value(now time.Time) float64 {
	if a.Successful {
		return 0
	} else {
		return now.Sub(a.DeployedAt).Seconds()
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
	sc := ev.RolloutStatus == api.RolloutStatus_RolloutStatusSuccesful
	var (
		as attribute.Set
	)
	if a != nil {
		as = a.Attributes
	} else {
		as = attribute.NewSet(
			attribute.String("kuberpult_application", ev.Application),
			attribute.String("kuberpult_environment", ev.Environment),
		)
	}
	return &appState{
		Attributes: as,
		Successful: sc,
		DeployedAt: ev.KuberpultVersion.DeployedAt,
	}
}
