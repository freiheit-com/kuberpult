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
	"net/http"

	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
)

const (
	EventTagApplication      = "kuberpult_application"
	EventTagEnvironment      = "kuberpult_environment"
	EventTagEnvironmentGroup = "kuberpult_environment_group"
)

func Init() (metric.MeterProvider, http.Handler, error) {

	reg := prometheus.NewPedanticRegistry()

	promExp, err := otelprom.New(otelprom.WithRegisterer(reg), otelprom.WithoutScopeInfo(), otelprom.WithoutTargetInfo())
	if err != nil {
		return nil, nil, err
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExp),
	)

	//exhaustruct:ignore
	return meterProvider, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}), nil
}

type ctxKeyType struct{}

var ctxKey ctxKeyType = ctxKeyType{}

func FromContext(ctx context.Context) metric.MeterProvider {
	return ctx.Value(ctxKey).(metric.MeterProvider)
}

func WithProvider(ctx context.Context, pv metric.MeterProvider) context.Context {
	return context.WithValue(ctx, ctxKey, pv)
}
