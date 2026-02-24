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
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/argo"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
)

const ARGO_APP_ENVIRONMENT_TAG = "com.freiheit.kuberpult/environment"
const ARGO_APP_PARENT_ENVIRONMENT_TAG = "com.freiheit.kuberpult/aa-parent-environment"
const ARGO_APP_APPLICATION_TAG = "com.freiheit.kuberpult/application"

// this is a simpler version of ApplicationServiceClient from the application package
type SimplifiedApplicationServiceClient interface {
	Watch(ctx context.Context, qry *application.ApplicationQuery, opts ...grpc.CallOption) (application.ApplicationService_WatchClient, error)
}

// type assertion
var (
	_ SimplifiedApplicationServiceClient = (application.ApplicationServiceClient)(nil)
)

type ArgoEventProcessor interface {
	ProcessArgoEvent(ctx context.Context, ev ArgoEvent) *ArgoEvent
}

type ArgoEventConsumer struct {
	AppClient           SimplifiedApplicationServiceClient
	Dispatcher          *Dispatcher
	HealthReporter      *setup.HealthReporter
	ArgoAppProcessor    *argo.ArgoAppProcessor
	DDMetrics           statsd.ClientInterface
	DBHandler           *db.DBHandler
	PersistArgoEvents   bool
	ArgoEventsBatchSize int
}

func (e *ArgoEventConsumer) ConsumeEvents(ctx context.Context) error {
	var argoEventBatch []*db.ArgoEvent

	return e.HealthReporter.Retry(ctx, func() error {
		if e.ArgoAppProcessor == nil {
			return fmt.Errorf("argo app processor is not configured (nil)")
		}
		//exhaustruct:ignore
		watch, err := e.AppClient.Watch(ctx, &application.ApplicationQuery{})
		if err != nil {
			if status.Code(err) == codes.Canceled {
				// context is cancelled -> we are shutting down
				return setup.Permanent(nil)
			}
			return fmt.Errorf("watching applications: %w", err)
		}

		e.HealthReporter.ReportReady("consuming events")
		for {
			ev, err := watch.Recv()
			if err != nil {
				if status.Code(err) == codes.Canceled {
					// context is cancelled -> we are shutting down
					return setup.Permanent(nil)
				}
				return err
			}
			concreteEnvironment, app, parentEnvironment := getArgoApplicationData(ev.Application.Annotations)
			if app == "" {
				continue
			}
			if parentEnvironment == "" {
				parentEnvironment = concreteEnvironment //For backwards compatibility, older apps might not have the aa-parent-environment tag
			}

			k := ArgoAppData{Application: app, Environment: concreteEnvironment, ParentEnvironment: parentEnvironment}

			var eventDiscarded = false
			switch ev.Type {
			case "ADDED", "MODIFIED", "DELETED":
				argoEvent := e.Dispatcher.Dispatch(ctx, k, ev)
				select {
				case e.ArgoAppProcessor.ArgoApps <- ev:
				default:
					eventDiscarded = true
					logger.FromContext(ctx).Sugar().Warnf("argo apps channel at full capacity of %d. Discarding event: %v", cap(e.ArgoAppProcessor.ArgoApps), ev)
				}

				if e.ArgoAppProcessor.ShouldSendArgoAppsMetrics() {
					if eventDiscarded {
						ddError := e.DDMetrics.Incr("argo_discarded_events", []string{}, 1)
						if ddError != nil {
							logger.FromContext(ctx).Sugar().Warnf("could not send argo_discarded_events metric to datadog! Err: %v", ddError)
						}
					}
					e.ArgoAppProcessor.GaugeArgoAppsQueueFillRate(ctx)
				}

				if argoEvent != nil && e.PersistArgoEvents {
					jsonEvent, err := json.Marshal(argoEvent)
					if err != nil {
						return err
					}
					dbEvent := &db.ArgoEvent{
						App:       k.Application,
						Env:       k.Environment,
						JsonEvent: jsonEvent,
						Discarded: eventDiscarded,
					}
					argoEventBatch = append(argoEventBatch, dbEvent)
					if len(argoEventBatch) <= e.ArgoEventsBatchSize {
						err := e.DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
							return e.DBHandler.UpsertArgoEvents(ctx, transaction, argoEventBatch)
						})
						if err != nil {
							return err
						}
						argoEventBatch = []*db.ArgoEvent{}
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

func getArgoApplicationData(annotations map[string]string) (string, string, string) {
	return annotations[ARGO_APP_ENVIRONMENT_TAG], annotations[ARGO_APP_APPLICATION_TAG], annotations[ARGO_APP_PARENT_ENVIRONMENT_TAG]
}

type ArgoEvent struct {
	Environment      string
	Application      string
	SyncStatusCode   v1alpha1.SyncStatusCode
	HealthStatusCode health.HealthStatusCode
	OperationState   *v1alpha1.OperationState
	Version          *versions.VersionInfo
}

func ToArgoEvent(k Key, ev *v1alpha1.ApplicationWatchEvent, version *versions.VersionInfo) ArgoEvent {
	return ArgoEvent{
		Application:      k.Application,
		Environment:      k.Environment,
		SyncStatusCode:   ev.Application.Status.Sync.Status,
		HealthStatusCode: ev.Application.Status.Health.Status,
		OperationState:   ev.Application.Status.OperationState,
		Version:          version,
	}
}

func ToDBEvent(k Key, ev *v1alpha1.ApplicationWatchEvent, version *versions.VersionInfo, discarded bool) (db.ArgoEvent, error) {
	argoEvent := ToArgoEvent(k, ev, version)
	jsonEvent, err := json.Marshal(argoEvent)
	if err != nil {
		return db.ArgoEvent{}, err
	}
	return db.ArgoEvent{App: k.Application, Env: k.Environment, JsonEvent: jsonEvent, Discarded: discarded}, nil
}
