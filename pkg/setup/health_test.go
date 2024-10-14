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

package setup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// Used to compare two error message strings, needed because errors.Is(fmt.Errorf(text),fmt.Errorf(text)) == false
type errMatcher struct {
	msg string
}

func (e errMatcher) Error() string {
	return e.msg
}

func (e errMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}

type mockClock struct {
	t time.Time
}

func (m *mockClock) now() time.Time {
	return m.t
}

func (m *mockClock) sleep(d time.Duration) {
	m.t = m.t.Add(d)
}

func TestHealthReporterBasics(t *testing.T) {
	var veryQuick = time.Nanosecond * 1
	tcs := []struct {
		Name               string
		ReportHealth       Health
		ReportMessage      string
		ReportTtl          *time.Duration
		ExpectedHttpStatus int
	}{

		{
			Name:               "reports error with TTL",
			ReportHealth:       HealthReady,
			ReportMessage:      "should work",
			ReportTtl:          &veryQuick,
			ExpectedHttpStatus: 500,
		},
		{
			Name:               "works without ttl",
			ReportHealth:       HealthReady,
			ReportMessage:      "should work",
			ReportTtl:          nil,
			ExpectedHttpStatus: 200,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			bo := &mockBackoff{}
			fakeClock := &mockClock{}
			hs := HealthServer{
				parts:          nil,
				mx:             sync.Mutex{},
				BackOffFactory: nil,
				Clock: func() time.Time {
					return fakeClock.now()
				},
			}
			hs.BackOffFactory = func() backoff.BackOff { return bo }
			reporter := hs.Reporter("Clark")
			reporter.ReportHealthTtl(tc.ReportHealth, tc.ReportMessage, tc.ReportTtl)
			//reporter.ReportHealth(HealthFailed, "testing")
			fakeClock.sleep(time.Millisecond * 1)

			testRequest := httptest.NewRequest("GET", "http://localhost/healthz", nil)
			testResponse := httptest.NewRecorder()
			hs.ServeHTTP(testResponse, testRequest)
			if testResponse.Code != tc.ExpectedHttpStatus {
				t.Errorf("wrong http status, expected %d, got %d", tc.ExpectedHttpStatus, testResponse.Code)
			}
		})
	}
}

func TestHealthReporter(t *testing.T) {
	tcs := []struct {
		Name               string
		ReportHealth       Health
		ReportMessage      string
		ReportTtl          *time.Duration
		ExpectedHealthBody string
		ExpectedStatus     int
		ExpectedMetricBody string
	}{
		{
			Name:               "reports starting",
			ReportTtl:          nil,
			ExpectedStatus:     500,
			ExpectedHealthBody: `{"a":{"health":"starting"}}`,
			ExpectedMetricBody: `# HELP background_job_ready 
# TYPE background_job_ready gauge
background_job_ready{name="a"} 0
`,
		},
		{
			Name:               "reports ready",
			ReportHealth:       HealthReady,
			ReportMessage:      "running",
			ReportTtl:          nil,
			ExpectedStatus:     200,
			ExpectedHealthBody: `{"a":{"health":"ready","message":"running"}}`,
			ExpectedMetricBody: `# HELP background_job_ready 
# TYPE background_job_ready gauge
background_job_ready{name="a"} 1
`,
		},
		{
			Name:               "reports failed",
			ReportHealth:       HealthFailed,
			ReportMessage:      "didnt work",
			ReportTtl:          nil,
			ExpectedStatus:     500,
			ExpectedHealthBody: `{"a":{"health":"failed","message":"didnt work"}}`,
			ExpectedMetricBody: `# HELP background_job_ready 
# TYPE background_job_ready gauge
background_job_ready{name="a"} 0
`,
		},
	}
	type Deadline struct {
		deadline *time.Time
		hr       *HealthReporter
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			stateChange := make(chan Deadline)
			cfg := ServerConfig{
				HTTP: []HTTPConfig{
					{
						Port: "18883",
					},
				},
				Background: []BackgroundTaskConfig{
					{
						Name: "a",
						Run: func(ctx context.Context, hr *HealthReporter) error {
							actualDeadline := hr.ReportHealthTtl(tc.ReportHealth, tc.ReportMessage, tc.ReportTtl)
							stateChange <- Deadline{deadline: actualDeadline, hr: hr}
							<-ctx.Done()
							return nil
						},
					},
				},
			}
			ctx, cancel := context.WithCancel(context.Background())
			doneCh := make(chan struct{})
			go func() {
				Run(ctx, cfg)
				doneCh <- struct{}{}
			}()
			actualDeadline := <-stateChange
			actualDeadline.hr.server.IsReady(actualDeadline.hr.name)
			status, body := getHttp(t, "http://localhost:18883/healthz")
			if status != tc.ExpectedStatus {
				t.Errorf("wrong http status, expected %d, got %d", tc.ExpectedStatus, status)
			}

			d := cmp.Diff(body, tc.ExpectedHealthBody)
			if d != "" {
				t.Errorf("wrong body, diff: %s", d)
			}
			_, metricBody := getHttp(t, "http://localhost:18883/metrics")
			if status != tc.ExpectedStatus {
				t.Errorf("wrong http status, expected %d, got %d", tc.ExpectedStatus, status)
			}
			d = cmp.Diff(metricBody, tc.ExpectedMetricBody)
			if d != "" {
				t.Errorf("wrong metric body, diff: %s\ngot:\n%s\nwant:\n%s\n", d, metricBody, tc.ExpectedMetricBody)
			}
			cancel()
			<-doneCh
		})
	}
}

type mockBackoff struct {
	called        uint
	resetted      uint
	backOffcalled chan bool
}

func (b *mockBackoff) NextBackOff() time.Duration {
	b.called = b.called + 1
	b.backOffcalled <- true
	return 1 * time.Nanosecond
}

func (b *mockBackoff) Reset() {
	b.resetted = b.resetted + 1
	return
}

func TestHealthReporterRetry(t *testing.T) {
	type step struct {
		ReportHealth  Health
		ReportMessage string
		ReturnError   error

		ExpectReady         bool
		ExpectBackoffCalled uint
		ExpectResetCalled   uint
	}
	tcs := []struct {
		Name              string
		BackoffChanLength uint

		Steps []step

		ExpectError error
	}{
		{
			Name:              "reports healthy",
			BackoffChanLength: 1,
			Steps: []step{
				{
					ReportHealth: HealthReady,

					ExpectReady:       true,
					ExpectResetCalled: 1,
				},
			},
		},
		{
			Name:              "reports unhealthy if there is an error",
			BackoffChanLength: 1,
			Steps: []step{
				{
					ReturnError: fmt.Errorf("no"),

					ExpectReady:         false,
					ExpectBackoffCalled: 1,
				},
			},
		},
		{
			Name:              "doesnt retry permanent errors",
			BackoffChanLength: 1,
			Steps: []step{
				{
					ReturnError: Permanent(fmt.Errorf("no")),

					ExpectReady:         false,
					ExpectBackoffCalled: 0,
				},
			},
			ExpectError: errMatcher{"no"},
		},
		{
			Name:              "retries some times and resets once it's healthy",
			BackoffChanLength: 3,
			Steps: []step{
				{
					ReturnError: fmt.Errorf("no"),

					ExpectReady:         false,
					ExpectBackoffCalled: 1,
				},
				{
					ReturnError: fmt.Errorf("no"),

					ExpectReady:         false,
					ExpectBackoffCalled: 2,
				},
				{
					ReturnError: fmt.Errorf("no"),

					ExpectReady:         false,
					ExpectBackoffCalled: 3,
				},
				{
					ReportHealth: HealthReady,

					ExpectReady:         true,
					ExpectBackoffCalled: 3,
					ExpectResetCalled:   1,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			stepCh := make(chan step)
			stateChange := make(chan struct{}, len(tc.Steps))
			bo := &mockBackoff{
				backOffcalled: make(chan bool, tc.BackoffChanLength),
			}
			hs := HealthServer{}
			hs.BackOffFactory = func() backoff.BackOff { return bo }
			ctx, cancel := context.WithCancel(context.Background())
			errCh := make(chan error)
			go func() {
				hr := hs.Reporter("a")
				errCh <- hr.Retry(ctx, func() error {
					for {
						select {
						case <-ctx.Done():
							return nil
						case st := <-stepCh:
							if st.ReturnError != nil {

								stateChange <- struct{}{}
								return st.ReturnError
							}
							hr.ReportHealth(st.ReportHealth, st.ReportMessage)
							stateChange <- struct{}{}
						}
					}
				})
			}()
			for _, st := range tc.Steps {
				stepCh <- st
				<-stateChange
				if st.ReturnError == nil && !st.ExpectReady {
					<-bo.backOffcalled
				}
				ready := hs.IsReady("a")
				if st.ExpectReady != ready {
					t.Errorf("expected ready status to %t but got %t", st.ExpectReady, ready)
				}
				if st.ExpectBackoffCalled != bo.called {
					t.Errorf("wrong number of backoffs called, expected %d, but got %d", st.ExpectBackoffCalled, bo.called)
				}
				if st.ExpectResetCalled != bo.resetted {
					t.Errorf("wrong number of backoff resets, expected %d, but got %d", st.ExpectResetCalled, bo.resetted)
				}

			}
			cancel()
			err := <-errCh
			if diff := cmp.Diff(tc.ExpectError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			close(stepCh)

		})
	}
}

func getHttp(t *testing.T, url string) (int, string) {
	for i := 0; i < 10; i = i + 1 {
		resp, err := http.Get(url)
		if err != nil {
			t.Log(err)
			<-time.After(time.Second)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}
	t.FailNow()
	return 0, ""
}
