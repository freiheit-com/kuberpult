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
	"strings"
	"time"

	argoapplication "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type SimplifiedApplicationInterface interface {
	Get(ctx context.Context, in *argoapplication.ApplicationQuery, opts ...grpc.CallOption) (*argoappv1.Application, error)
}

type Notifier interface {
	NotifyArgoCd(ctx context.Context, environment, application string)
}

func New(client SimplifiedApplicationInterface, concurrencyLimit int, timeout int) Notifier {
	n := &notifier{client, time.Duration(timeout), errgroup.Group{}}
	n.errGroup.SetLimit(concurrencyLimit)
	return n
}

type notifier struct {
	client   SimplifiedApplicationInterface
	timeout  time.Duration
	errGroup errgroup.Group
}

func (n *notifier) NotifyArgoCd(ctx context.Context, environment, application string) {
	n.errGroup.Go(func() error {
		var err error
		span, ctx := tracer.StartSpanFromContext(ctx, "argocd.refresh")
		span.SetTag("environment", environment)
		span.SetTag("application", application)
		ctx, cancel := context.WithTimeout(ctx, n.timeout*time.Second)
		defer cancel()
		l := logger.FromContext(ctx).With(zap.String("environment", environment), zap.String("application", application))
		//exhaustruct:ignore
		_, err = n.client.Get(ctx, &argoapplication.ApplicationQuery{
			Name:    conversion.FromString(fmt.Sprintf("%s-%s", environment, application)),
			Refresh: conversion.FromString(string(argoappv1.RefreshTypeNormal)),
		})
		if err != nil {
			if !strings.Contains(err.Error(), "DeadlineExceeded") ||
				!strings.Contains(err.Error(), "deadline exceeded") ||
				!strings.Contains(err.Error(), "context canceled") {
				l.Error("argocd.refresh", zap.Error(err))
			}
		}
		span.Finish(tracer.WithError(err))
		return nil
	})
}
