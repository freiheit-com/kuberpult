/*
This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com
*/
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

	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	URL                string
	Token              []byte
	Concurrency int
}

func New(config Config) *Subscriber {
	sub := &Subscriber{
		group: errgroup.Group{},
	}
	sub.group.SetLimit(config.Concurrency)
	sub.url = config.URL
	sub.token = config.Token
	sub.ready = func() {}
	return sub
}

type Subscriber struct {
	group errgroup.Group
	token []byte
	url   string
	// The ready function is needed to sync tests
	ready func()
}

func (s *Subscriber) Subscribe(ctx context.Context, b *service.Broadcast) error {
	event, ch, unsub := b.Start()
	defer unsub()
	state := map[service.Key]*service.BroadcastEvent{}
	for _, ev := range event {
		if ev.IsProduction != nil && *ev.IsProduction {
			state[ev.Key] = ev
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
			if shouldNotify(state[ev.Key], ev) {
				s.group.Go(s.notify(ctx, ev))
			}
			state[ev.Key] = ev
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
		Id:          uuidFor(ev.Application, ev.ArgocdVersion.SourceCommitId, ev.ArgocdVersion.DeployedAt.String()),
		CommitHash:  ev.ArgocdVersion.SourceCommitId,
		EventTime:   ev.ArgocdVersion.DeployedAt.Format(time.RFC3339),
		ServiceName: ev.Application,
	}
	return func() error {
		body, err := json.Marshal(event)
		h := hmac.New(sha256.New, s.token)
		h.Write([]byte(body))
		sha := "sha256=" + hex.EncodeToString(h.Sum(nil))
		r, err := http.NewRequest("POST", s.url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("creating http request: %w", err)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Hub-Signature-256", sha)
		r.Header.Set("User-Agent", "kuberpult")
		s, err := http.DefaultClient.Do(r)
		if err != nil {
			return fmt.Errorf("sending req: %w", err)
		}
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
