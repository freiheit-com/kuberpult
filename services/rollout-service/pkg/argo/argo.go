package argo

import (
	"context"
	"mime/quotedprintable"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/service"
)

type ArgoAppProcessor struct {
	trigger      chan struct{}
	lastOverview *api.GetOverviewResponse
	argoApps     chan *v1alpha1.ApplicationWatchEvent
}

func (a *ArgoAppProcessor) Push(last *api.GetOverviewResponse) {
	a.lastOverview = last
	select {
	case a.trigger <- struct{}{}:
	default:
	}
}

func (a *ArgoAppProcessor) Consume(ctx context.Context) error {
	seen := map[service.Key]struct{}{}

	for {
		select {
		case <-a.trigger:
			// run one loop
		case ev := <-a.argoApps:
			// process one argo event
		case <-ctx.Done():
			return nil
		}
		overview := a.lastOverview
		for _, env := range overview.EnvironmentGroups {
		}
	}
	return nil
}

func (a *ArgoAppProcessor) ConsumeArgo(ctx context.Context, argo service.SimplifiedApplicationServiceClient) error {
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
		environment, application := getEnvironmentAndName(ev.Application.Annotations)
		if application == "" {
			continue
		}
		k := service.Key{Application: application, Environment: environment}
		switch ev.Type {
		case "ADDED", "MODIFIED", "DELETED":
			a.argoApps <- ev
		default:
		}
	}
}

func getEnvironmentAndName(annotations map[string]string) (string, string) {
	return annotations["com.freiheit.kuberpult/environment"], annotations["com.freiheit.kuberpult/application"]
}
