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
	"fmt"
	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/argo"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
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

type ArgoEventProcessor interface {
	ProcessArgoEvent(ctx context.Context, ev ArgoEvent)
}

func ConsumeEvents(ctx context.Context, appClient SimplifiedApplicationServiceClient, dispatcher *Dispatcher, hlth *setup.HealthReporter, a *argo.ArgoAppProcessor, ddMetrics statsd.ClientInterface) error {
	return hlth.Retry(ctx, func() error {
		//exhaustruct:ignore
		watch, err := appClient.Watch(ctx, &application.ApplicationQuery{})
		if err != nil {
			if status.Code(err) == codes.Canceled {
				// context is cancelled -> we are shutting down
				return setup.Permanent(nil)
			}
			return fmt.Errorf("watching applications: %w", err)
		}
		hlth.ReportReady("consuming events")
		for {
			ev, err := watch.Recv()
			if err != nil {
				if status.Code(err) == codes.Canceled {
					// context is cancelled -> we are shutting down
					return setup.Permanent(nil)
				}
				return err
			}
			environment, application := getEnvironmentAndName(ev.Application.Annotations)
			if application == "" {
				continue
			}
			k := Key{Application: application, Environment: environment}
			var ddError error = nil
			var eventDiscarded = false
			switch ev.Type {
			case "ADDED", "MODIFIED", "DELETED":
				dispatcher.Dispatch(ctx, k, ev)
				select {
				case a.ArgoApps <- ev:
				default:
					eventDiscarded = true
					logger.FromContext(ctx).Sugar().Warnf("argo apps channel at full capacity of %d. Discarding event: %v", cap(a.ArgoApps), ev)
				}

				if ddMetrics != nil { //If DD is enabled, send metrics
					if eventDiscarded {
						ddError = ddMetrics.Gauge("argo_discarded_events", 1, []string{}, 1)
						if ddError != nil {
							logger.FromContext(ctx).Sugar().Warnf("could not send argo_discarded_events metric to datadog! Err: %v", ddError)
						}
					}
					fillRate := 0.0
					if cap(a.ArgoApps) != 0 {
						fillRate = float64(len(a.ArgoApps)) / float64(cap(a.ArgoApps))
					} else {
						fillRate = 1 // If capacity is 0, we are always at 100%
					}
					ddError = ddMetrics.Gauge("argo_events_fill_rate", fillRate, []string{}, 1)
					if ddError != nil {
						logger.FromContext(ctx).Sugar().Warnf("could not send argo_events_fill_rate metric to datadog! Err: %v", ddError)
					}
				}
			case "BOOKMARK":
				// ignore this event
			default:
				logger.FromContext(ctx).Warn("argocd.application.unknown_type", zap.String("event.type", string(ev.Type)))
			}
		}
	})
}

func getEnvironmentAndName(annotations map[string]string) (string, string) {
	return annotations["com.freiheit.kuberpult/environment"], annotations["com.freiheit.kuberpult/application"]
}

type ArgoEvent struct {
	Environment      string
	Application      string
	SyncStatusCode   v1alpha1.SyncStatusCode
	HealthStatusCode health.HealthStatusCode
	OperationState   *v1alpha1.OperationState
	Version          *versions.VersionInfo
}
