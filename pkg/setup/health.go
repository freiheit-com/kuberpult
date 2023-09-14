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
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Health uint

const (
	HealthStarting Health = iota
	HealthReady
	HealthFailed
)

func (h Health) String() string {
	switch h {
	case HealthStarting:
		return "starting"
	case HealthReady:
		return "ready"
	case HealthFailed:
		return "failed"
	}
	return "unknown"
}

func (h Health) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.String())
}

type HealthReporter struct {
	server *HealthServer
	name   string
}

type report struct {
	Health  Health        `json:"health"`
	Message string        `json:"message,omitempty"`
}

func (r *HealthReporter) ReportReady(message string) {
	r.ReportHealth(HealthReady, message)
}

func (r *HealthReporter) ReportHealth(health Health, message string) {
	if r == nil {
		return
	}
	r.server.mx.Lock()
	defer r.server.mx.Unlock()
	if r.server.parts == nil {
		r.server.parts = map[string]report{}
	}
	r.server.parts[r.name] = report{
		Health:  health,
		Message: message,
	}
}

type HealthServer struct {
	parts map[string]report
	mx    sync.Mutex
}

func (h *HealthServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reports := h.reports()
	success := true
	for _, r := range reports {
		if r.Health != HealthReady {
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
	return report.Health == HealthReady
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

func (h *HealthServer) Reporter(name string) *HealthReporter {
	r := &HealthReporter{
		server: h,
		name:   name,
	}
	r.ReportHealth(HealthStarting, "starting")
	return r
}

var (
	_ http.Handler = (*HealthServer)(nil)
)
