package argo

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
)

type ArgoAppProcessor struct {
	trigger           chan *v1alpha1.Application
	lastOverview      *api.GetOverviewResponse
	argoApps          chan *v1alpha1.ApplicationWatchEvent
	ApplicationClient application.ApplicationServiceClient
	HealthReporter    *setup.HealthReporter
}

func (a *ArgoAppProcessor) Push(last *api.GetOverviewResponse, appToPush *v1alpha1.Application) {
	a.lastOverview = last
	select {
	case a.trigger <- appToPush:
	default:
	}
}

func (a *ArgoAppProcessor) Consume(ctx context.Context) error {
	seen := map[service.Key]*api.Environment_Application{}

	for {
		select {
		case <-a.trigger:
			err := a.ConsumeArgo(ctx, a.ApplicationClient, a.HealthReporter)
			if err != nil {
				return err
			}
		case ev := <-a.argoApps:

			switch ev.Type {
			case "ADDED":
				upsert := false
				validate := false

				appCreateRequest := &application.ApplicationCreateRequest{
					Application: &ev.Application,
					Upsert:      &upsert,
					Validate:    &validate,
				}
				_, err := a.ApplicationClient.Create(ctx, appCreateRequest)
				if err != nil {
					return fmt.Errorf(err.Error())
				}
			case "MODIFIED":
				appUpdateRequest := &application.ApplicationUpdateSpecRequest{
					Name:         &ev.Application.Name,
					Spec:         &ev.Application.Spec,
					AppNamespace: &ev.Application.Namespace,
				}
				_, err := a.ApplicationClient.UpdateSpec(ctx, appUpdateRequest)
				if err != nil {
					return fmt.Errorf(err.Error())
				}
			case "DELETED":
				appDeleteRequest := &application.ApplicationDeleteRequest{
					Name:         &ev.Application.Name,
					AppNamespace: &ev.Application.Namespace,
				}
				_, err := a.ApplicationClient.Delete(ctx, appDeleteRequest)
				if err != nil {
					return fmt.Errorf(err.Error())
				}
			}

		case <-ctx.Done():
			return nil
		}

		overview := a.lastOverview
		for _, envGroup := range overview.EnvironmentGroups {
			for _, env := range envGroup.Environments {
				for _, app := range env.Applications {
					k := service.Key{Application: app.Name, Environment: env.Name}
					if ok := seen[k]; ok != nil {
						seen[k] = app
					}
				}
			}
		}
	}
}

func (a *ArgoAppProcessor) ConsumeArgo(ctx context.Context, argo service.SimplifiedApplicationServiceClient, hlth *setup.HealthReporter) error {
	watch, err := argo.Watch(ctx, &application.ApplicationQuery{})
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

		if ev.Type == "ADDED" || ev.Type == "MODIFIED" || ev.Type == "DELETED" {
			a.argoApps <- ev
		}
	}
}
