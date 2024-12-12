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
	URL         string
	Token       []byte
	Concurrency int
	MaxEventAge time.Duration
}

func New(config Config) *Subscriber {
	sub := &Subscriber{
		token:  nil,
		url:    "",
		ready:  nil,
		state:  nil,
		maxAge: 0,
		now:    nil,
		group:  errgroup.Group{},
	}
	sub.group.SetLimit(config.Concurrency)
	sub.url = config.URL
	sub.token = config.Token
	sub.ready = func() {}
	sub.maxAge = config.MaxEventAge
	sub.now = time.Now
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
			span, ctx := tracer.StartSpanFromContext(ctx, "RevolutionStart")
			if !ok {
				return s.group.Wait()
			}
			if ev.IsProduction == nil || !*ev.IsProduction {
				continue
			}
			span.SetTag("IsProduction", "true")
			if s.maxAge != 0 &&
				ev.ArgocdVersion != nil &&
				ev.ArgocdVersion.DeployedAt.Add(s.maxAge).Before(s.now()) {
				continue
			}
			span.SetTag("IsWithinMaxAge", "true")
			l.Info("registering event app: " + ev.Key.Application + ", environment: " + ev.Key.Environment)
			if shouldNotify(s.state[ev.Key], ev) {

				span.SetTag("Notified", "true")
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
		s, err := http.DefaultClient.Do(r)
		if err != nil {
			span.Finish(tracer.WithError(err))
			return nil
		}
		span.SetTag("http.status_code", s.Status)
		defer s.Body.Close()
		content, _ := io.ReadAll(s.Body)
		if s.StatusCode > 299 {
			return fmt.Errorf("http status (%d): %s", s.StatusCode, content)
		}
		return nil
	}
}

var kuberpultUuid uuid.UUID = uuid.NewSHA1(uuid.MustParse("00000000-0000-0000-0000-000000000000"), []byte("kuberpult"))

func uuidFor(application, commitHash, deployedAt string) string {
	return uuid.NewSHA1(kuberpultUuid, []byte(fmt.Sprintf("%s\n%s\n%s", application, commitHash, deployedAt))).String()
}
