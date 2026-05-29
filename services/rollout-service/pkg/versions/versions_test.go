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

package versions

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"testing"
	"time"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-cmp/cmp"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/argo"
	"k8s.io/utils/lru"
)

type step struct {
	ChangedApps            *api.GetChangedAppsResponse
	ConnectErr             error
	RecvErr                error
	CancelContext          bool
	OverviewResponse       *api.GetOverviewResponse
	AppDetailsResponses    map[string]*api.GetAppDetailsResponse
	ExpectReady            bool
	ExpectedEvents         []KuberpultEvent
	ExpectedArgoAppDetails map[string]*api.GetAppDetailsResponse
}

type expectedVersion struct {
	Revision         string
	Environment      string
	Application      string
	DeployedVersion  uint64
	DeployTime       time.Time
	SourceCommitId   string
	OverviewMetadata metadata.MD
	VersionMetadata  metadata.MD
	IsProduction     bool
}

type mockOverviewClient struct {
	grpc.ClientStream
	OverviewResponse           *api.GetOverviewResponse
	GetAllAppLocksResponse     *api.GetAllAppLocksResponse
	GetAllEnvTeamLocksResponse *api.GetAllEnvTeamLocksResponse
	AppDetailsResponses        map[string]*api.GetAppDetailsResponse
	LastMetadata               metadata.MD
	StartStep                  chan struct{}
	Steps                      chan step
	savedStep                  *step
}

// GetOverview implements api.OverviewServiceClient
func (m *mockOverviewClient) GetOverview(ctx context.Context, in *api.GetOverviewRequest, opts ...grpc.CallOption) (*api.GetOverviewResponse, error) {
	return m.OverviewResponse, nil
}

// GetOverview implements api.GetAllAppLocks
func (m *mockOverviewClient) GetAllAppLocks(ctx context.Context, in *api.GetAllAppLocksRequest, opts ...grpc.CallOption) (*api.GetAllAppLocksResponse, error) {
	return m.GetAllAppLocksResponse, nil
}

// GetOverview implements api.GetAllEnvLocks
func (m *mockOverviewClient) GetAllEnvTeamLocks(ctx context.Context, in *api.GetAllEnvTeamLocksRequest, opts ...grpc.CallOption) (*api.GetAllEnvTeamLocksResponse, error) {
	return m.GetAllEnvTeamLocksResponse, nil
}

// GetOverview implements api.GetAppDetails
func (m *mockOverviewClient) GetAppDetails(ctx context.Context, in *api.GetAppDetailsRequest, opts ...grpc.CallOption) (*api.GetAppDetailsResponse, error) {
	if resp := m.AppDetailsResponses[in.AppName]; resp != nil {
		return resp, nil
	}
	return nil, status.Error(codes.Unknown, "no")
}

// StreamOverview implements api.OverviewServiceClient
func (m *mockOverviewClient) StreamOverview(ctx context.Context, in *api.GetOverviewRequest, opts ...grpc.CallOption) (api.OverviewService_StreamOverviewClient, error) {
	return nil, nil
}

// StreamOverview implements api.OverviewServiceClient
func (m *mockOverviewClient) StreamChangedApps(ctx context.Context, in *api.GetChangedAppsRequest, opts ...grpc.CallOption) (api.OverviewService_StreamChangedAppsClient, error) {
	m.StartStep <- struct{}{}
	reply, ok := <-m.Steps
	if !ok {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	if reply.ConnectErr != nil {
		return nil, reply.ConnectErr
	}
	m.savedStep = &reply
	return m, nil
}

// StreamOverview implements api.OverviewServiceClient
func (m *mockOverviewClient) StreamDeploymentHistory(ctx context.Context, in *api.DeploymentHistoryRequest, opts ...grpc.CallOption) (api.OverviewService_StreamDeploymentHistoryClient, error) {
	return nil, nil
}

func (m *mockOverviewClient) Recv() (*api.GetChangedAppsResponse, error) {
	var reply step
	var ok bool
	if m.savedStep != nil {
		reply = *m.savedStep
		m.savedStep = nil
		ok = true
	} else {
		m.StartStep <- struct{}{}
		reply, ok = <-m.Steps

	}
	if !ok {
		return nil, fmt.Errorf("exhausted: %w", io.EOF)
	}
	m.OverviewResponse = reply.OverviewResponse
	m.AppDetailsResponses = reply.AppDetailsResponses //Endpoint responses at different steps
	return reply.ChangedApps, reply.RecvErr
}

func (m *mockOverviewClient) GetAllManifestLocks(ctx context.Context, in *api.GetAllManifestLocksRequest, opts ...grpc.CallOption) (*api.GetAllManifestLocksResponse, error) {
	return nil, nil
}

var _ api.OverviewServiceClient = (*mockOverviewClient)(nil)

func TestGetVersion_Bracket(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatal(err)
	}
	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("directory for DB migrations: %s", migrationsPath)
	if err := db.RunDBMigrations(ctx, *dbConfig); err != nil {
		t.Fatal(err)
	}
	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}

	const bracketEnv = "bracket-env"
	const bracketName types.ArgoBracketName = "my-bracket"
	var versionA uint64 = 5
	var versionB uint64 = 3

	// Use two separate transactions so deployments get distinct transaction timestamps.
	setupErr := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
		if err := dbHandler.DBWriteMigrationsTransformer(ctx, tx); err != nil {
			return err
		}
		if err := dbHandler.DBWriteEnvironment(ctx, tx, bracketEnv, config.EnvironmentConfig{}); err != nil {
			return err
		}
		for _, appName := range []types.AppName{"app-a", "app-b"} {
			if err := dbHandler.DBInsertOrUpdateApplication(ctx, tx, appName, db.AppStateChangeCreate, db.DBAppMetaData{}, bracketName); err != nil {
				return err
			}
		}
		// releases
		for _, r := range []db.DBReleaseWithMetaData{
			{
				ReleaseNumbers: types.ReleaseNumbers{Version: &versionA, Revision: 0},
				App:            "app-a",
				Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{bracketEnv: ""}},
				Metadata:       db.DBReleaseMetaData{SourceCommitId: "commit-a"},
			},
			{
				ReleaseNumbers: types.ReleaseNumbers{Version: &versionB, Revision: 0},
				App:            "app-b",
				Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{bracketEnv: ""}},
				Metadata:       db.DBReleaseMetaData{SourceCommitId: "commit-b"},
			},
		} {
			if err := dbHandler.DBUpdateOrCreateRelease(ctx, tx, r); err != nil {
				return err
			}
		}
		// deploy app-a in this transaction
		if err := dbHandler.DBUpdateOrCreateDeployment(ctx, tx, db.Deployment{
			App: "app-a", Env: bracketEnv,
			ReleaseNumbers: types.ReleaseNumbers{Version: &versionA, Revision: 0},
		}); err != nil {
			return err
		}
		// bracket history: my-bracket → [app-a, app-b]
		return db.DBInsertBracketHistory(ctx, dbHandler, tx, db.BracketRow{
			CreatedAt: time.Now(),
			AllBracketsJsonBlob: db.BracketJsonBlob{
				BracketMap: map[types.ArgoBracketName]db.AppNames{
					bracketName: {"app-a", "app-b"},
				},
			},
		}, 0)
	})
	if setupErr != nil {
		t.Fatal(setupErr)
	}

	// Deploy app-b in a separate transaction so it gets a later timestamp.
	if err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
		return dbHandler.DBUpdateOrCreateDeployment(ctx, tx, db.Deployment{
			App: "app-b", Env: bracketEnv,
			ReleaseNumbers: types.ReleaseNumbers{Version: &versionB, Revision: 0},
		})
	}); err != nil {
		t.Fatal(err)
	}

	vc := New(nil, nil, nil, false, false, false, []string{}, *dbHandler, 50, 50, nil, []string{bracketEnv})

	version, err := vc.GetVersion(ctx, "5:3", bracketEnv, string(bracketName))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version == nil {
		t.Fatal("expected non-nil VersionInfo")
	}
	// The revision string is returned as-is as the Version field.
	if version.Version != types.RolloutAppBracketVersion("5:3") {
		t.Errorf("expected Version=%q, got %q", "5:3", version.Version)
	}
	// DeployedAt must be non-zero since both apps have deployments.
	if version.DeployedAt.IsZero() {
		t.Errorf("expected non-zero DeployedAt")
	}
	// app-b is deployed later (separate transaction), so its SourceCommitId should be returned.
	if version.SourceCommitId != "commit-b" {
		t.Errorf("expected SourceCommitId=%q, got %q", "commit-b", version.SourceCommitId)
	}

	// Verify fallback: env not in experimentalBracketsClusters but revision contains ":"
	vcNoCluster := New(nil, nil, nil, false, false, false, []string{}, *dbHandler, 50, 50, nil, []string{})
	version2, err2 := vcNoCluster.GetVersion(ctx, "5:3:", bracketEnv, string(bracketName))
	if err2 != nil {
		t.Fatalf("expected no error for bracket revision when env not in clusters list, got: %v", err2)
	}
	if version2 == nil || version2.Version != types.RolloutAppBracketVersion("5:3:") {
		t.Errorf("expected Version=%q, got %v", "5:3:", version2)
	}
}

type recordingProcessor struct {
	events chan KuberpultEvent
}

func (r *recordingProcessor) ProcessKuberpultEvent(_ context.Context, ev KuberpultEvent) {
	r.events <- ev
}

func devOverview() *api.GetOverviewResponse {
	return &api.GetOverviewResponse{
		//exhaustruct:ignore
		EnvironmentGroups: []*api.EnvironmentGroup{{
			EnvironmentGroupName: "dev-group",
			Priority:             api.Priority_UPSTREAM,
			//exhaustruct:ignore
			Environments: []*api.Environment{{Name: "dev"}},
		}},
	}
}

func bracketOverview(envName string) *api.GetOverviewResponse {
	return &api.GetOverviewResponse{
		//exhaustruct:ignore
		EnvironmentGroups: []*api.EnvironmentGroup{{
			EnvironmentGroupName: "bracket-group",
			Priority:             api.Priority_UPSTREAM,
			//exhaustruct:ignore
			Environments: []*api.Environment{{Name: envName}},
		}},
	}
}

func TestConsumeEvents(t *testing.T) {
	tcs := []struct {
		Name            string
		Steps           []step
		BracketClusters []string
		WantEvents      []KuberpultEvent
	}{
		{
			Name: "normal app change triggers ProcessKuberpultEvent",
			Steps: []step{{
				OverviewResponse: devOverview(),
				ChangedApps: &api.GetChangedAppsResponse{
					//exhaustruct:ignore
					ChangedApps: []*api.GetAppDetailsResponse{{
						//exhaustruct:ignore
						Application: &api.Application{Name: "my-app"},
						Deployments: map[string]*api.Deployment{
							"dev": {Version: 42}, //exhaustruct:ignore
						},
					}},
				},
			}},
			WantEvents: []KuberpultEvent{{
				Application:       "my-app",
				Environment:       "dev",
				ParentEnvironment: "dev",
				EnvironmentGroup:  "dev-group",
				IsProduction:      false,
				Team:              "",
				Version: &VersionInfo{
					Version:        types.RolloutAppBracketVersionFromUint64(42),
					SourceCommitId: "",
					DeployedAt:     time.Time{},
				},
			}},
		},
		{
			Name:            "bracket BracketVersionDelete triggers ProcessKuberpultEvent",
			BracketClusters: []string{"benv"},
			Steps: []step{{
				OverviewResponse: bracketOverview("benv"),
				ChangedApps: &api.GetChangedAppsResponse{
					//exhaustruct:ignore
					ChangedBrackets: []*api.GetBracketDetailsResponse{{
						BracketName: "my-bracket",
						Deployments: map[string]*api.BracketDeployment{
							"benv": {Version: string(types.BracketVersionDelete)}, //exhaustruct:ignore
						},
					}},
				},
			}},
			WantEvents: []KuberpultEvent{{
				Application:       "my-bracket",
				Environment:       "benv",
				ParentEnvironment: "benv",
				EnvironmentGroup:  "bracket-group",
				IsProduction:      false,
				Team:              "",
				Version: &VersionInfo{
					Version:        types.BracketVersionDelete,
					SourceCommitId: "",
					DeployedAt:     time.Time{},
				},
			}},
		},
		{
			Name:            "bracket normal version triggers ProcessKuberpultEvent",
			BracketClusters: []string{"benv"},
			Steps: []step{{
				OverviewResponse: bracketOverview("benv"),
				ChangedApps: &api.GetChangedAppsResponse{
					//exhaustruct:ignore
					ChangedBrackets: []*api.GetBracketDetailsResponse{{
						BracketName: "my-bracket",
						Deployments: map[string]*api.BracketDeployment{
							"benv": {Version: "1:2"}, //exhaustruct:ignore
						},
					}},
				},
			}},
			WantEvents: []KuberpultEvent{{
				Application:       "my-bracket",
				Environment:       "benv",
				ParentEnvironment: "benv",
				EnvironmentGroup:  "bracket-group",
				IsProduction:      false,
				Team:              "",
				Version: &VersionInfo{
					Version:        types.RolloutAppBracketVersion("1:2"),
					SourceCommitId: "",
					DeployedAt:     time.Time{},
				},
			}},
		},
		{
			Name:            "bracket proto zero-value still triggers ProcessKuberpultEvent",
			BracketClusters: []string{"benv"},
			Steps: []step{{
				OverviewResponse: bracketOverview("benv"),
				ChangedApps: &api.GetChangedAppsResponse{
					//exhaustruct:ignore
					ChangedBrackets: []*api.GetBracketDetailsResponse{{
						BracketName: "my-bracket",
						Deployments: map[string]*api.BracketDeployment{
							"benv": {Version: ""}, //exhaustruct:ignore
						},
					}},
				},
			}},
			WantEvents: []KuberpultEvent{{
				Application:       "my-bracket",
				Environment:       "benv",
				ParentEnvironment: "benv",
				EnvironmentGroup:  "bracket-group",
				IsProduction:      false,
				Team:              "",
				Version: &VersionInfo{
					Version:        types.RolloutAppBracketVersion(""),
					SourceCommitId: "",
					DeployedAt:     time.Time{},
				},
			}},
		},
		{
			Name: "same app version in second step is deduplicated",
			Steps: []step{
				{
					OverviewResponse: devOverview(),
					ChangedApps: &api.GetChangedAppsResponse{
						//exhaustruct:ignore
						ChangedApps: []*api.GetAppDetailsResponse{{
							//exhaustruct:ignore
							Application: &api.Application{Name: "my-app"},
							Deployments: map[string]*api.Deployment{
								"dev": {Version: 42}, //exhaustruct:ignore
							},
						}},
					},
				},
				{
					OverviewResponse: devOverview(),
					ChangedApps: &api.GetChangedAppsResponse{
						//exhaustruct:ignore
						ChangedApps: []*api.GetAppDetailsResponse{{
							//exhaustruct:ignore
							Application: &api.Application{Name: "my-app"},
							Deployments: map[string]*api.Deployment{
								"dev": {Version: 42}, //exhaustruct:ignore
							},
						}},
					},
				},
			},
			WantEvents: []KuberpultEvent{{
				Application:       "my-app",
				Environment:       "dev",
				ParentEnvironment: "dev",
				EnvironmentGroup:  "dev-group",
				IsProduction:      false,
				Team:              "",
				Version: &VersionInfo{
					Version:        types.RolloutAppBracketVersionFromUint64(42),
					SourceCommitId: "",
					DeployedAt:     time.Time{},
				},
			}},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stepsChannel := make(chan step, len(tc.Steps)+1)
			for _, s := range tc.Steps {
				stepsChannel <- s
			}

			mockClient := &mockOverviewClient{
				StartStep: make(chan struct{}, len(tc.Steps)+10),
				Steps:     stepsChannel,
			}

			processor := &recordingProcessor{
				events: make(chan KuberpultEvent, len(tc.WantEvents)+5),
			}

			vc := &versionClient{
				overviewClient:               mockClient,
				ArgoProcessor:                argo.New(nil, false, false, false, []string{}, 10, 10, nil, tc.BracketClusters, nil),
				cache:                        lru.New(20),
				experimentalBracketsClusters: tc.BracketClusters,
			}

			hlth := &setup.HealthServer{}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }

			done := make(chan error, 1)
			go func() {
				done <- vc.ConsumeEvents(ctx, processor, hlth.Reporter("test"))
			}()

			gotEvents := make([]KuberpultEvent, 0, len(tc.WantEvents))
			for range tc.WantEvents {
				select {
				case ev := <-processor.events:
					gotEvents = append(gotEvents, ev)
				case <-time.After(5 * time.Second):
					t.Fatalf("timed out waiting for event %d/%d", len(gotEvents)+1, len(tc.WantEvents))
				}
			}

			close(stepsChannel)
			cancel()
			<-done

			if diff := cmp.Diff(tc.WantEvents, gotEvents); diff != "" {
				t.Errorf("events mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
