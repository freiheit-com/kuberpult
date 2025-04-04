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

package revolution

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/DataDog/datadog-go/v5/statsd"
	"io"
	"net/http"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type Config struct {
	URL            string
	Token          []byte
	Concurrency    int
	MaxEventAge    time.Duration
	MetricsEnabled bool
	DryRun         bool
}

func New(config Config, ddMetrics statsd.ClientInterface) *Subscriber {
	sub := &Subscriber{
		token:          nil,
		url:            "",
		ready:          nil,
		state:          nil,
		maxAge:         0,
		now:            nil,
		group:          errgroup.Group{},
		metricsEnabled: false,
		ddMetrics:      nil,
		dryRun:         false,
	}
	sub.group.SetLimit(config.Concurrency)
	sub.url = config.URL
	sub.token = config.Token
	sub.ready = func() {}
	sub.maxAge = config.MaxEventAge
	sub.now = time.Now
	sub.metricsEnabled = config.MetricsEnabled
	sub.ddMetrics = ddMetrics
	sub.dryRun = config.DryRun
	return sub
}

type Subscriber struct {
	group errgroup.Group
	token []byte
	url   string
	// The ready function is needed to sync tests
	ready func()
	state map[service.Key]*service.BroadcastEvent
	// The maximum age of events that should be considered. If 0,
	// all events are considered.
	maxAge time.Duration
	// Used to simulate the current time in tests
	now func() time.Time

	metricsEnabled bool
	dryRun         bool
	ddMetrics      statsd.ClientInterface
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
	l := logger.FromContext(ctx).With(zap.String("revolution", "processing"))
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
				l.Sugar().Warnf("discarded event for app %q on environment %q. Event too old: %s", ev.Key.Application, ev.Key.Environment, ev.ArgocdVersion.DeployedAt.String())
				continue
			}

			l.Info("registering event app: " + ev.Key.Application + ", environment: " + ev.Key.Environment)
			if shouldNotify(ctx, s.state[ev.Key], ev) {
				s.group.Go(s.notify(ctx, ev))
			}
			s.state[ev.Key] = ev
		}
	}
}
func shouldNotifyValidate(l *zap.Logger, e *service.BroadcastEvent) bool {
	versionIsNil := e.ArgocdVersion == nil
	isProductionIsNil := e.IsProduction == nil
	commitIdIsNil := e.ArgocdVersion == nil || e.ArgocdVersion.SourceCommitId == ""
	if versionIsNil || isProductionIsNil || commitIdIsNil {
		nilValues := []string{}
		if versionIsNil {
			nilValues = append(nilValues, "version")
		}
		if isProductionIsNil {
			nilValues = append(nilValues, "isProduction")
		}
		if commitIdIsNil {
			nilValues = append(nilValues, "commitId")
		}
		l.Info("Skipped notify as event has the following values nil: %s", zap.Strings("nil values", nilValues))
		return false
	}
	return true
}

func shouldNotify(ctx context.Context, old *service.BroadcastEvent, nu *service.BroadcastEvent) bool {
	l := logger.FromContext(ctx).With(zap.String("revolution", "processing"))
	// check for fields that must be present to generate the request
	if !shouldNotifyValidate(l, nu) {
		return false
	}
	if old == nil || old.ArgocdVersion == nil || old.IsProduction == nil {
		return true
	}
	sameCommitId := old.ArgocdVersion.SourceCommitId != nu.ArgocdVersion.SourceCommitId
	sameDeployAt := old.ArgocdVersion.DeployedAt != nu.ArgocdVersion.DeployedAt
	if sameCommitId && sameDeployAt {
		return true
	}
	mismatchingValues := []string{}
	if !sameCommitId {
		mismatchingValues = append(mismatchingValues, "CommitId")
	}
	if !sameDeployAt {
		mismatchingValues = append(mismatchingValues, "DeployAt")
	}
	l.Info("Skipped notify due to old vs. new mismatch: %v", zap.Strings("mismatches", mismatchingValues))
	return false
}

type kuberpultEvent struct {
	Id string `json:"id"`
	// Id/UUID to de-duplicate events
	CommitHash string `json:"commitHash"`
	EventTime  string `json:"eventTime"`
	// optimally in RFC3339 format
	URL string `json:"url,omitempty"`
	// where to see the logs/status of the deployment
	ServiceName string `json:"serviceName"`
}

func (s *Subscriber) notify(ctx context.Context, ev *service.BroadcastEvent) func() error {
	event := kuberpultEvent{
		URL:         "",
		Id:          uuidFor(ev.Application, ev.ArgocdVersion.SourceCommitId, ev.ArgocdVersion.DeployedAt.String()),
		CommitHash:  ev.ArgocdVersion.SourceCommitId,
		EventTime:   ev.ArgocdVersion.DeployedAt.Format(time.RFC3339),
		ServiceName: ev.Application,
	}

	return func() error {
		span, _ := tracer.StartSpanFromContext(ctx, "revolution.notify")
		defer span.Finish()
		span.SetTag("revolution.url", s.url)
		span.SetTag("revolution.id", event.Id)
		span.SetTag("environment", ev.Environment)
		span.SetTag("application", ev.Application)

		body, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}

		if s.dryRun {
			logger.FromContext(ctx).Sugar().Warnf("Dry Run enabled! Would send following DORA event to revolution: %s", string(body))
			return nil
		}
		h := hmac.New(sha256.New, s.token)
		h.Write([]byte(body))
		sha := "sha256=" + hex.EncodeToString(h.Sum(nil))
		r, err := http.NewRequest(http.MethodPost, s.url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("creating http request: %w", err)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Hub-Signature-256", sha)
		r.Header.Set("User-Agent", "kuberpult")
		requestResult, err := http.DefaultClient.Do(r)
		if err != nil {
			//Error issuing request
			s.GaugeDoraEvents(ctx, true)
			span.Finish(tracer.WithError(err))
			return nil
		}
		span.SetTag("http.status_code", requestResult.Status)
		defer requestResult.Body.Close()
		content, _ := io.ReadAll(requestResult.Body)
		if requestResult.StatusCode > 299 {
			//Error from Revolution
			s.GaugeDoraEvents(ctx, true)
			return fmt.Errorf("http status (%d): %s", requestResult.StatusCode, content)
		}
		s.GaugeDoraEvents(ctx, false)
		return nil
	}
}

func (s *Subscriber) GaugeDoraEvents(ctx context.Context, failed bool) {
	if s.ddMetrics != nil && s.metricsEnabled {
		var metric string
		if failed {
			metric = "dora_failed_events"
		} else {
			metric = "dora_successful_events"
		}
		ddError := s.ddMetrics.Incr(metric, []string{}, 1)
		if ddError != nil {
			logger.FromContext(ctx).Sugar().Warnf("could not send %s metric to datadog! Err: %v", metric, ddError)
		}
	}
}

var kuberpultUuid uuid.UUID = uuid.NewSHA1(uuid.MustParse("00000000-0000-0000-0000-000000000000"), []byte("kuberpult"))

func uuidFor(application, commitHash, deployedAt string) string {
	return uuid.NewSHA1(kuberpultUuid, []byte(fmt.Sprintf("%s\n%s\n%s", application, commitHash, deployedAt))).String()
}
