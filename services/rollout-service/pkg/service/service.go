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
	"fmt"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// this is a simpler version of ApplicationServiceClient from the application package
type SimplifiedApplicationServiceClient interface {
	Watch(ctx context.Context, qry *application.ApplicationQuery, opts ...grpc.CallOption) (application.ApplicationService_WatchClient, error)
}

// type assertion
var (
	_ SimplifiedApplicationServiceClient = (application.ApplicationServiceClient)(nil)
)

type EventProcessor interface {
	Process(ctx context.Context, ev Event)
}

func ConsumeEvents(ctx context.Context, appClient SimplifiedApplicationServiceClient, version versions.VersionClient, sink EventProcessor, ready func()) error {
	for {
		watch, err := appClient.Watch(ctx, &application.ApplicationQuery{})
		if err != nil {
			if status.Code(err) == codes.Canceled {
				// context is cancelled -> we are shutting down
				return nil
			}
			return fmt.Errorf("watching applications: %w", err)
		}
		ready()
	recv:
		for {
			ev, err := watch.Recv()
			if err != nil {
				if status.Code(err) == codes.Canceled {
					// context is cancelled -> we are shutting down
					return nil
				}
				logger.FromContext(ctx).Warn("argocd.application.recv", zap.Error(err))
				break
			}
			environment, application := getEnvironmentAndName(ev.Application.Annotations)
			if application == "" {
				continue
			}
			switch ev.Type {
			case "ADDED", "MODIFIED":
				version, err := version.GetVersion(ctx, ev.Application.Status.Sync.Revision, environment, application)
				if err != nil {
					logger.FromContext(ctx).Warn("version.getversion", zap.String("revision", ev.Application.Status.Sync.Revision), zap.Error(err))
				}
				sink.Process(ctx, Event{
					Application:      application,
					Environment:      environment,
					SyncStatusCode:   ev.Application.Status.Sync.Status,
					HealthStatusCode: ev.Application.Status.Health.Status,
					OperationState:   ev.Application.Status.OperationState,
					Version:          version,
				})
			case "DELETED":
				sink.Process(ctx, Event{
					Application:      application,
					Environment:      environment,
					SyncStatusCode:   ev.Application.Status.Sync.Status,
					HealthStatusCode: ev.Application.Status.Health.Status,

					OperationState: ev.Application.Status.OperationState,
					Version:        0,
				})
				continue recv
			case "BOOKMARK":
				// ignore this event
			default:
				logger.FromContext(ctx).Warn("argocd.application.unknown_type", zap.String("event.type", string(ev.Type)))
			}
		}
	}
}

func getEnvironmentAndName(annotations map[string]string) (string, string) {
	return annotations["com.freiheit.kuberpult/environment"], annotations["com.freiheit.kuberpult/application"]
}

type Event struct {
	Environment      string
	Application      string
	SyncStatusCode   v1alpha1.SyncStatusCode
	HealthStatusCode health.HealthStatusCode
	OperationState   *v1alpha1.OperationState
	Version          uint64
}
