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
	"github.com/freiheit-com/kuberpult/services/frontend-service/api"
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

func ConsumeEvents(ctx context.Context, appClient SimplifiedApplicationServiceClient, overview api.OverviewServiceClient) error {
	for {
		watch, err := appClient.Watch(ctx, &application.ApplicationQuery{})
		if err != nil {
			if status.Code(err) == codes.Canceled {

				// context is cancelled -> we are shutting down
				return nil
			}
			return fmt.Errorf("watching applications: %w", err)
		}
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
			fmt.Printf("%s %s %s %s %s %#v\n", ev.Type, ev.Application.Name, ev.Application.Spec.Project, ev.Application.Status.Sync.Status, ev.Application.Status.Sync.Revision, ev.Application.Status.Health.Status)
		}
	}
}

type argoEvent struct {
	Application      string
	SyncStatusCode   v1alpha1.SyncStatusCode
	Revision         string
	HealthStatusCode health.HealthStatusCode
}

type argoEventProcessor struct {
}

func (ep *argoEventProcessor) Process(ev *argoEvent) {

}
