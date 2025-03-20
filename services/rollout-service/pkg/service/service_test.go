/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/

package service

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"

	"io"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/argo"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/cenkalti/backoff/v4"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type step struct {
	Event         *v1alpha1.ApplicationWatchEvent
	WatchErr      error
	RecvErr       error
	CancelContext bool

	ExpectedEvent *ArgoEvent
}

func (m *mockApplicationServiceClient) Recv() (*v1alpha1.ApplicationWatchEvent, error) {
	if m.current >= len(m.Steps) {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	if m.current != 0 {
		lastReply := m.Steps[m.current-1]
		if lastReply.ExpectedEvent == nil {

		} else {
			select {
			case lastEvent := <-m.lastEvent:
				if !cmp.Equal(lastReply.ExpectedEvent, lastEvent) {
					m.t.Errorf("step %d did not generate the expected event, diff: %s", m.current-1, cmp.Diff(lastReply.ExpectedEvent, lastEvent))
				}
			case <-time.After(time.Second):
				m.t.Errorf("step %d timed out waiting for event", m.current-1)
			}
		}
	}
	reply := m.Steps[m.current]
	if reply.CancelContext {
		m.cancel()
	}
	m.current = m.current + 1
	return reply.Event, reply.RecvErr
}

type mockApplicationServiceClient struct {
	Steps     []step
	current   int
	t         *testing.T
	lastEvent chan *ArgoEvent
	cancel    context.CancelFunc
	grpc.ClientStream
}

func (m *mockApplicationServiceClient) Watch(ctx context.Context, qry *application.ApplicationQuery, opts ...grpc.CallOption) (application.ApplicationService_WatchClient, error) {
	if m.current >= len(m.Steps) {
		return nil, setup.Permanent(fmt.Errorf("exhausted: %w", io.EOF))
	}
	reply := m.Steps[m.current]
	if reply.WatchErr != nil {
		if reply.CancelContext {
			m.cancel()
		}
		m.current = m.current + 1
		return nil, reply.WatchErr
	}
	return m, nil
}

func (m *mockApplicationServiceClient) testAllConsumed(t *testing.T) {
	if m.current < len(m.Steps) {
		t.Errorf("expected to consume all %d replies, only consumed %d", len(m.Steps), m.current)
	}
}

// Process implements service.EventProcessor
func (m *mockApplicationServiceClient) ProcessArgoEvent(ctx context.Context, ev ArgoEvent) *ArgoEvent {
	m.lastEvent <- &ev
	return &ev
}

type version struct {
	Revision        string
	Environment     string
	Application     string
	Attempt         uint64
	DeployedVersion uint64
	Error           error
}

type mockVersionClient struct {
	versions.VersionClient
	versions     []version
	attemptCount map[string]uint64
}

// GetVersion implements versions.VersionClient
func (m *mockVersionClient) GetVersion(ctx context.Context, revision string, environment string, application string) (*versions.VersionInfo, error) {
	if m.attemptCount == nil {
		m.attemptCount = map[string]uint64{}
	}
	key := fmt.Sprintf("%s/%s@%s", environment, application, revision)
	current := m.attemptCount[key]
	m.attemptCount[key] = current + 1
	for _, v := range m.versions {
		if v.Revision == revision && v.Environment == environment && v.Application == application && v.Attempt == current {
			return &versions.VersionInfo{Version: v.DeployedVersion}, nil
		}
	}
	return nil, fmt.Errorf("no")
}

var _ versions.VersionClient = (*mockVersionClient)(nil)

type Gauge struct {
	Name  string
	Value float64
	Tags  []string
	Rate  float64
}

type MockClient struct {
	events []*statsd.Event
	Gauges []Gauge
	statsd.ClientInterface
}

func (c *MockClient) Gauge(name string, value float64, tags []string, rate float64) error {
	c.Gauges = append(c.Gauges, Gauge{
		Name:  name,
		Value: value,
		Tags:  tags,
		Rate:  rate,
	})
	return nil
}

func TestArgoConection(t *testing.T) {
	makeGauge := func(name string, val float64, tags []string, rate float64) Gauge {
		return Gauge{
			Name:  name,
			Value: val,
			Tags:  tags,
			Rate:  rate,
		}
	}
	tcs := []struct {
		Name          string
		KnownVersions []version
		Steps         []step

		ExpectedError error
		ExpectedReady bool

		expectedGauges []Gauge

		channelSize int
	}{
		{
			Name: "stops without error when ctx is closed on Recv call",
			Steps: []step{
				{
					WatchErr:      status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			channelSize:   10,
			ExpectedReady: false,
		},
		{
			Name: "does not stop for watch errors",
			Steps: []step{
				{
					WatchErr: fmt.Errorf("no"),
				},
				{
					WatchErr:      status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			channelSize:   10,
			ExpectedReady: false,
		},
		{
			Name: "stops when ctx closes in the watch call",
			Steps: []step{
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			channelSize:   10,
			ExpectedReady: true,
		},
		{
			Name: "retries when Recv fails",
			Steps: []step{
				{
					RecvErr: fmt.Errorf("no"),
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			channelSize:   10,
			ExpectedReady: true,
		},
		{
			Name: "ignore events for applications that were not generated by kuberpult",
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "ADDED",
						Application: v1alpha1.Application{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "foo",
								Annotations: map[string]string{},
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "",
							},
							Status: v1alpha1.ApplicationStatus{
								Sync:   v1alpha1.SyncStatus{Revision: "1234"},
								Health: v1alpha1.HealthStatus{},
							},
						},
					},
					// Applications generated by kuberpult have name = "<env>-<name>" and project = "<env>".
					// This application doesn't follow this scheme and must not create an event.
					ExpectedEvent: nil,
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			channelSize:   10,
			ExpectedReady: true,
		},
		{
			Name: "generates events for applications that were generated by kuberpult",
			KnownVersions: []version{
				{
					Revision:        "1234",
					Environment:     "foo",
					Application:     "bar",
					DeployedVersion: 42,
				},
			},
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "ADDED",
						Application: v1alpha1.Application{
							ObjectMeta: metav1.ObjectMeta{
								Name: "doesntmatter",
								Annotations: map[string]string{
									"com.freiheit.kuberpult/environment": "foo",
									"com.freiheit.kuberpult/application": "bar",
								},
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "foo",
							},
							Status: v1alpha1.ApplicationStatus{
								Sync:   v1alpha1.SyncStatus{Revision: "1234"},
								Health: v1alpha1.HealthStatus{},
							},
						},
					},
					ExpectedEvent: &ArgoEvent{
						Application: "bar",
						Environment: "foo",
						Version:     &versions.VersionInfo{Version: 42},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			channelSize:   10,
			ExpectedReady: true,
			expectedGauges: []Gauge{
				makeGauge("argo_events_fill_rate", 0.1, []string{}, 1),
			},
		},
		{
			Name: "doesnt generate events for deleted",
			KnownVersions: []version{
				{
					Revision:        "1234",
					Environment:     "foo",
					Application:     "bar",
					DeployedVersion: 42,
				},
			},
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "DELETED",
						Application: v1alpha1.Application{
							ObjectMeta: metav1.ObjectMeta{
								Name: "doesntmatter",
								Annotations: map[string]string{
									"com.freiheit.kuberpult/environment": "foo",
									"com.freiheit.kuberpult/application": "bar",
								},
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "foo",
							},
							Status: v1alpha1.ApplicationStatus{
								Sync:   v1alpha1.SyncStatus{Revision: "1234"},
								Health: v1alpha1.HealthStatus{},
							},
						},
					},
					ExpectedEvent: &ArgoEvent{
						Application: "bar",
						Environment: "foo",
						Version:     &versions.VersionInfo{Version: 42},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			channelSize:   10,
			ExpectedReady: true,
			expectedGauges: []Gauge{
				makeGauge("argo_events_fill_rate", 0.1, []string{}, 1),
			},
		},
		{
			Name: "recovers from errors",
			KnownVersions: []version{
				{
					Revision:    "1234",
					Environment: "foo",
					Application: "bar",
					Attempt:     0,
					Error:       fmt.Errorf("no"),
				},
				{
					Revision:        "1234",
					Environment:     "foo",
					Application:     "bar",
					Attempt:         1,
					DeployedVersion: 1,
				},
			},
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "MODIFIED",
						Application: v1alpha1.Application{
							ObjectMeta: metav1.ObjectMeta{
								Name: "doesntmatter",
								Annotations: map[string]string{
									"com.freiheit.kuberpult/environment": "foo",
									"com.freiheit.kuberpult/application": "bar",
								},
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "foo",
							},
							Status: v1alpha1.ApplicationStatus{
								Sync:   v1alpha1.SyncStatus{Revision: "1234"},
								Health: v1alpha1.HealthStatus{},
							},
						},
					},
					ExpectedEvent: &ArgoEvent{
						Application: "bar",
						Environment: "foo",
						Version:     &versions.VersionInfo{},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			ExpectedReady: true,
			channelSize:   0, //Ok to get discarded events, as there is nobody listening to them
			expectedGauges: []Gauge{
				makeGauge("argo_discarded_events", 1, []string{}, 1),
				makeGauge("argo_events_fill_rate", 1, []string{}, 1),
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			as := mockApplicationServiceClient{
				Steps:     tc.Steps,
				cancel:    cancel,
				t:         t,
				lastEvent: make(chan *ArgoEvent, 10),
			}
			dbHandler := SetupDB(t)
			var mockClient = &MockClient{}
			var client statsd.ClientInterface = mockClient
			hlth := &setup.HealthServer{}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
			dispatcher := NewDispatcher(&as, &mockVersionClient{versions: tc.KnownVersions})
			params := ConsumeEventsParameters{
				Dispatcher:     dispatcher,
				HealthReporter: hlth.Reporter("consume"),
				AppClient:      &as,
				ArgoAppProcessor: &argo.ArgoAppProcessor{
					ApplicationClient:     nil,
					ManageArgoAppsEnabled: true,
					ManageArgoAppsFilter:  []string{},
					ArgoApps:              make(chan *v1alpha1.ApplicationWatchEvent, tc.channelSize),
				},
				DDMetrics:         client,
				DBHandler:         dbHandler,
				PersistArgoEvents: false,
			}
			err := ConsumeEvents(ctx, &params)
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			ready := hlth.IsReady("consume")
			if tc.ExpectedReady != ready {
				t.Errorf("expected ready to be %t but got %t", tc.ExpectedReady, ready)
			}
			if diff := cmp.Diff(tc.expectedGauges, mockClient.Gauges); diff != "" {
				t.Errorf("gauges mismatch (-want, +got):\n%s", diff)
			}
			as.testAllConsumed(t)

		})
	}
}

func TestArgoEvents(t *testing.T) {
	const appName = "appName"
	const envName = "envName"
	const targetRevision = "1234"
	const deployedVersion = 42
	const envAnnotation = "com.freiheit.kuberpult/environment"
	const appAnnotation = "com.freiheit.kuberpult/application"

	sampleEvent, _ := ToDBEvent(Key{Application: appName, Environment: envName}, &v1alpha1.ApplicationWatchEvent{
		Type: "ADDED",
		Application: v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "doesntmatter",
				Annotations: map[string]string{
					envAnnotation: envName,
					appAnnotation: appName,
				},
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "foo",
			},
			Status: v1alpha1.ApplicationStatus{
				Sync:   v1alpha1.SyncStatus{Revision: targetRevision},
				Health: v1alpha1.HealthStatus{},
			},
		},
	}, &versions.VersionInfo{Version: deployedVersion}, false)
	tcs := []struct {
		Name          string
		KnownVersions []version
		Steps         []step

		ExpectedError error
		ExpectedReady bool

		persistArgoEvents bool
		argoBufferSize    int
		channelSize       int
	}{
		{
			Name: "Turned off does not generate events on the database",
			KnownVersions: []version{
				{
					Revision:        targetRevision,
					Environment:     envName,
					Application:     appName,
					DeployedVersion: deployedVersion,
				},
			},
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "ADDED",
						Application: v1alpha1.Application{
							ObjectMeta: metav1.ObjectMeta{
								Name: "doesntmatter",
								Annotations: map[string]string{
									envAnnotation: envName,
									appAnnotation: appName,
								},
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "foo",
							},
							Status: v1alpha1.ApplicationStatus{
								Sync:   v1alpha1.SyncStatus{Revision: targetRevision},
								Health: v1alpha1.HealthStatus{},
							},
						},
					},
					ExpectedEvent: &ArgoEvent{
						Application: appName,
						Environment: envName,
						Version:     &versions.VersionInfo{Version: deployedVersion},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			channelSize:       10,
			persistArgoEvents: true,
			argoBufferSize:    1,
			ExpectedReady:     true,
		},
		{
			Name: "generates events for applications that were generated by kuberpult",
			KnownVersions: []version{
				{
					Revision:        targetRevision,
					Environment:     envName,
					Application:     appName,
					DeployedVersion: deployedVersion,
				},
			},
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "ADDED",
						Application: v1alpha1.Application{
							ObjectMeta: metav1.ObjectMeta{
								Name: "doesntmatter",
								Annotations: map[string]string{
									envAnnotation: envName,
									appAnnotation: appName,
								},
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "foo",
							},
							Status: v1alpha1.ApplicationStatus{
								Sync:   v1alpha1.SyncStatus{Revision: targetRevision},
								Health: v1alpha1.HealthStatus{},
							},
						},
					},
					ExpectedEvent: &ArgoEvent{
						Application: appName,
						Environment: envName,
						Version:     &versions.VersionInfo{Version: deployedVersion},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			channelSize:       10,
			persistArgoEvents: true,
			argoBufferSize:    1,
			ExpectedReady:     true,
		},
		{
			Name: "doesnt generate events for deleted",
			KnownVersions: []version{
				{
					Revision:        targetRevision,
					Environment:     envName,
					Application:     appName,
					DeployedVersion: deployedVersion,
				},
			},
			Steps: []step{
				{
					Event: &v1alpha1.ApplicationWatchEvent{
						Type: "DELETED",
						Application: v1alpha1.Application{
							ObjectMeta: metav1.ObjectMeta{
								Name: "doesntmatter",
								Annotations: map[string]string{
									envAnnotation: envName,
									appAnnotation: appName,
								},
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "foo",
							},
							Status: v1alpha1.ApplicationStatus{
								Sync:   v1alpha1.SyncStatus{Revision: targetRevision},
								Health: v1alpha1.HealthStatus{},
							},
						},
					},
					ExpectedEvent: &ArgoEvent{
						Application: appName,
						Environment: envName,
						Version:     &versions.VersionInfo{Version: deployedVersion},
					},
				},
				{
					RecvErr:       status.Error(codes.Canceled, "context cancelled"),
					CancelContext: true,
				},
			},
			channelSize:       10,
			persistArgoEvents: true,
			argoBufferSize:    1,
			ExpectedReady:     true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			as := mockApplicationServiceClient{
				Steps:     tc.Steps,
				cancel:    cancel,
				t:         t,
				lastEvent: make(chan *ArgoEvent, 10),
			}
			dbHandler := SetupDB(t)
			var mockClient = &MockClient{}
			var client statsd.ClientInterface = mockClient
			hlth := &setup.HealthServer{}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
			dispatcher := NewDispatcher(&as, &mockVersionClient{versions: tc.KnownVersions})
			params := ConsumeEventsParameters{
				Dispatcher:     dispatcher,
				HealthReporter: hlth.Reporter("consume"),
				AppClient:      &as,
				ArgoAppProcessor: &argo.ArgoAppProcessor{
					ApplicationClient:     nil,
					ManageArgoAppsEnabled: true,
					ManageArgoAppsFilter:  []string{},
					ArgoApps:              make(chan *v1alpha1.ApplicationWatchEvent, tc.channelSize),
				},
				DDMetrics:           client,
				DBHandler:           dbHandler,
				PersistArgoEvents:   tc.persistArgoEvents,
				ArgoEventsBatchSize: tc.argoBufferSize,
			}
			err := ConsumeEvents(ctx, &params)
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			ready := hlth.IsReady("consume")
			if tc.ExpectedReady != ready {
				t.Errorf("expected ready to be %t but got %t", tc.ExpectedReady, ready)
			}

			dbCtx := testutil.MakeTestContext()
			result, err := db.WithTransactionT(dbHandler, dbCtx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*db.ArgoEvent, error) {
				r, err := dbHandler.DBReadArgoEvent(dbCtx, transaction, appName, envName)
				return r, err
			})
			if err != nil {
				t.Fatal(err)
			}
			if result == nil {
				if !tc.persistArgoEvents {
					return
				}
				t.Fatalf("no argo event found for app %q on env %q", appName, envName)
			}

			if diff := cmp.Diff(sampleEvent, *result); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}

			as.testAllConsumed(t)

		})
	}
}

// SetupDB returns a new DBHandler with a tmp directory every time, so tests can are completely independent
func SetupDB(t *testing.T) *db.DBHandler {
	ctx := context.Background()
	dir, err := testutil.CreateMigrationsPath(2)
	tmpDir := t.TempDir()
	t.Logf("directory for DB migrations: %s", dir)
	t.Logf("tmp dir for DB data: %s", tmpDir)
	cfg := db.DBConfig{
		MigrationsPath: dir,
		DriverName:     "sqlite3",
		DbHost:         tmpDir,
	}

	migErr := db.RunDBMigrations(ctx, cfg)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		err1 := dbHandler.DBWriteEslEventWithJson(ctx, transaction, db.EvtMigrationTransformer, "{}")
		if err1 != nil {
			return fmt.Errorf("error while writing EvtMigrationTransformer, error: %w", err)
		}

		err1 = dbHandler.DBWriteEnvironment(ctx, transaction, "staging", config.EnvironmentConfig{}, []string{})
		if err1 != nil {
			return err1
		}
		err1 = dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "foo", db.AppStateChangeCreate, db.DBAppMetaData{})
		if err1 != nil {
			return err1
		}
		err1 = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
			ReleaseNumber: 1234,
			Created:       time.Unix(123456789, 0).UTC(),
			App:           "foo",
			Manifests: db.DBReleaseManifests{
				Manifests: map[string]string{"staging": ""},
			},
			Metadata: db.DBReleaseMetaData{},
		})
		if err1 != nil {
			return err1
		}
		var version int64 = 1234
		err1 = dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, db.Deployment{
			Created:       time.Unix(123456789, 0).UTC(),
			App:           "foo",
			Env:           "staging",
			Version:       &version,
			TransformerID: 1,
		})
		return err1
	})
	if err != nil {
		t.Fatal(err)
	}
	return dbHandler
}
