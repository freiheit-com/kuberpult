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

// Setup implementation shared between all microservices.
// If this file is changed it will affect _all_ microservices in the monorepo (and this
// is deliberately so).
package setup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
)

type Health uint

const (
	HealthStarting Health = iota
	HealthReady
	HealthBackoff
	HealthFailed
)

func (h Health) String() string {
	switch h {
	case HealthStarting:
		return "starting"
	case HealthReady:
		return "ready"
	case HealthBackoff:
		return "backoff"
	case HealthFailed:
		return "failed"
	}
	return "unknown"
}

func (h Health) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.String())
}

type HealthReporter struct {
	server  *HealthServer
	name    string
	backoff backoff.BackOff
}

type report struct {
	Health  Health `json:"health"`
	Message string `json:"message,omitempty"`

	// a nil Deadline is interpreted as "valid forever"
	Deadline *time.Time `json:"deadline,omitempty"`
}

func (r *report) isReady(now time.Time) bool {
	if r.Health != HealthReady {
		return false
	}
	if r.Deadline == nil {
		return true
	}
	return now.Before(*r.Deadline)
}

func (r *HealthReporter) ReportReady(message string) {
	r.ReportHealth(HealthReady, message)
}

func (r *HealthReporter) ReportHealth(health Health, message string) {
	r.ReportHealthTtl(health, message, nil)
}

// ReportHealthTtl returns the deadline (for testing)
func (r *HealthReporter) ReportHealthTtl(health Health, message string, ttl *time.Duration) *time.Time {
	if r == nil {
		return nil
	}
	if health == HealthReady {
		r.backoff.Reset()
	}
	r.server.mx.Lock()
	defer r.server.mx.Unlock()
	if r.server.parts == nil {
		r.server.parts = map[string]report{}
	}
	var deadline *time.Time
	if ttl != nil {
		dl := r.server.now().Add(*ttl)
		deadline = &dl
	}
	r.server.parts[r.name] = report{
		Health:   health,
		Message:  message,
		Deadline: deadline,
	}
	return deadline
}

/*
Retry allows background services to set up reliable streaming with backoff.

This can be used to create background tasks that look like this:

	func Consume(ctx context.Context, hr *setup.HealthReporter) error {
		state := initState()
		return hr.Retry(ctx, func() error {
			stream, err := startConsumer()
			if err != nil {
				return err
			}
			hr.ReportReady("receiving")
			for {
				select {
				case <-ctx.Done(): return nil
				case ev := <-stream: handleEvent(state, event)
				}
			}
	  })
	}

In the example above, connecting to  the consumer will be retried a few times with backoff.
The number of retries is reset whenever ReportReady is called so that successful connection heal the service.
*/
func (r *HealthReporter) Retry(ctx context.Context, fn func() error) error {
	bo := r.backoff
	for {
		err := fn()
		select {
		case <-ctx.Done():
			return err
		default:
		}
		if err != nil {
			var perr *backoff.PermanentError
			if errors.As(err, &perr) {
				return perr.Unwrap()
			}
			r.ReportHealth(HealthBackoff, err.Error())
		} else {
			r.ReportHealth(HealthBackoff, "")
		}
		next := bo.NextBackOff()
		if next == backoff.Stop {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(next):
			continue
		}
	}
}

type HealthServer struct {
	parts          map[string]report
	mx             sync.Mutex
	BackOffFactory func() backoff.BackOff
	Clock          func() time.Time
}

func (h *HealthServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reports := h.reports()
	success := true
	for _, r := range reports {
		if !r.isReady(h.now()) {
			success = false
		}
	}
	body, err := json.Marshal(reports)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	if success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	fmt.Fprint(w, string(body))
}

func (h *HealthServer) IsReady(name string) bool {
	h.mx.Lock()
	defer h.mx.Unlock()
	if h.parts == nil {
		return false
	}
	report := h.parts[name]
	return report.isReady(h.now())
}

func (h *HealthServer) reports() map[string]report {
	h.mx.Lock()
	defer h.mx.Unlock()
	result := make(map[string]report, len(h.parts))
	for k, v := range h.parts {
		result[k] = v
	}
	return result
}

func (h *HealthServer) now() time.Time {
	if h.Clock != nil {
		return h.Clock()
	}
	return time.Now()
}

func (h *HealthServer) Reporter(name string) *HealthReporter {
	var bo backoff.BackOff
	if h.BackOffFactory != nil {
		bo = h.BackOffFactory()
	} else {
		bo = backoff.NewExponentialBackOff()
	}
	r := &HealthReporter{
		server:  h,
		name:    name,
		backoff: bo,
	}
	r.ReportHealth(HealthStarting, "starting")
	return r
}

func Permanent(err error) error {
	return backoff.Permanent(err)
}

var (
	_ http.Handler = (*HealthServer)(nil)
)
