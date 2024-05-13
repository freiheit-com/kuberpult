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
package datadogdora

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"golang.org/x/sync/errgroup"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type Config struct {
	URL         string
	APIKey      string
	Concurrency int
	MaxEventAge time.Duration
}

func New(config Config) *Subscriber {
	sub := &Subscriber{
		group: errgroup.Group{},
	}
	sub.group.SetLimit(config.Concurrency)
	sub.url = config.URL
	sub.apiKey = config.APIKey
	sub.ready = func() {}
	sub.maxAge = config.MaxEventAge
	sub.now = time.Now
	configuration := datadog.NewConfiguration()
	configuration.SetUnstableOperationEnabled("v2.CreateDORADeployment", true)
	apiClient := datadog.NewAPIClient(configuration)
	sub.doraAPI = datadogV2.NewDORAMetricsApi(apiClient)

	return sub
}

type Subscriber struct {
	group         errgroup.Group
	apiKey        string
	url           string
	RepositoryUrl string
	// The ready function is needed to sync tests
	ready func()
	state map[service.Key]*service.BroadcastEvent
	// The maximum age of events that should be considered. If 0,
	// all events are considered.
	maxAge time.Duration
	// Used to simulate the current time in tests
	now func() time.Time
	// used to report dora metrics
	doraAPI *datadogV2.DORAMetricsApi
}

func (s *Subscriber) Subscribe(ctx context.Context, b *service.Broadcast) error {
	if s.state == nil {
		s.state = map[service.Key]*service.BroadcastEvent{}
	}
	for {
		err := s.subscribeOnce(ctx, b)
		select {
		case <-ctx.Done():
			return err
		default:
		}
	}
}

func (s *Subscriber) subscribeOnce(ctx context.Context, b *service.Broadcast) error {
	event, ch, unsub := b.Start()
	defer unsub()
	for _, ev := range event {
		if ev.IsProduction != nil && *ev.IsProduction {
			s.state[ev.Key] = ev
		}
	}
	s.ready()
	for {
		select {
		case <-ctx.Done():
			return s.group.Wait()
		case ev, ok := <-ch:
			if !ok {
				return s.group.Wait()
			}
			if ev.IsProduction == nil || !*ev.IsProduction {
				continue
			}
			if s.maxAge != 0 &&
				ev.ArgocdVersion != nil &&
				ev.ArgocdVersion.DeployedAt.Add(s.maxAge).Before(s.now()) {
				continue
			}
			if shouldNotify(s.state[ev.Key], ev) {
				s.group.Go(s.notify(ctx, ev))
			}
			s.state[ev.Key] = ev
		}
	}
}

func shouldNotify(old *service.BroadcastEvent, nu *service.BroadcastEvent) bool {
	// check for fields that must be present to generate the request
	if nu.ArgocdVersion == nil || nu.IsProduction == nil || nu.ArgocdVersion.SourceCommitId == "" {
		return false
	}
	if old == nil || old.ArgocdVersion == nil || old.IsProduction == nil {
		return true
	}
	if old.ArgocdVersion.SourceCommitId != nu.ArgocdVersion.SourceCommitId || old.ArgocdVersion.DeployedAt != nu.ArgocdVersion.DeployedAt {
		return true
	}
	return false
}
func (s *Subscriber) notify(ctx context.Context, ev *service.BroadcastEvent) func() error {

	return func() error {
		span, _ := tracer.StartSpanFromContext(ctx, "datadogdora.notify")
		defer span.Finish()
		span.SetTag("datadogAPI.url", s.url)
		span.SetTag("environment", ev.Environment)
		span.SetTag("application", ev.Application)
		body := datadogV2.DORADeploymentRequest{
			Data: datadogV2.DORADeploymentRequestData{
				Attributes: datadogV2.DORADeploymentRequestAttributes{
					FinishedAt: ev.ArgocdVersion.DeployedAt.UnixNano(),
					Git: &datadogV2.DORAGitInfo{
						CommitSha:     ev.ArgocdVersion.SourceCommitId,
						RepositoryUrl: s.RepositoryUrl,
					},
					Service: ev.Application,
					// TODO(BJ) get the time the sync was triggered?
					StartedAt: ev.ArgocdVersion.DeployedAt.UnixNano() - 1,
					Version:   datadog.PtrString("v1.12.07"),
				},
			},
		}
		ctx = context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {
					Key: s.apiKey,
				},
			},
		)
		ctx = context.WithValue(ctx,
			datadog.ContextServerVariables,
			map[string]string{
				"site": s.url,
			})

		_, r, err := s.doraAPI.CreateDORADeployment(ctx, body)
		if err != nil {
			span.Finish(tracer.WithError(err))
			return nil
		}
		span.SetTag("http.status_code", r.Status)
		content, _ := io.ReadAll(r.Body)
		if r.StatusCode > 299 {
			span.Finish(tracer.WithError(err))
			return fmt.Errorf("http status (%d): %s", r.StatusCode, content)
		}
		return nil
	}
}
