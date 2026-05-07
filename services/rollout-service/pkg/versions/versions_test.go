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
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

type step struct {
	ChangedApps         *api.GetChangedAppsResponse
	ConnectErr          error
	RecvErr             error
	CancelContext       bool
	OverviewResponse    *api.GetOverviewResponse
	AppDetailsResponses map[string]*api.GetAppDetailsResponse
	ExpectReady         bool
	ExpectedEvents      []KuberpultEvent
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

var _ api.OverviewServiceClient = (*mockOverviewClient)(nil)

type mockVersionClient struct {
	LastMetadata metadata.MD
}

func (m *mockVersionClient) GetManifests(ctx context.Context, in *api.GetManifestsRequest, opts ...grpc.CallOption) (*api.GetManifestsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

type mockVersionEventProcessor struct {
	events []KuberpultEvent
}

func (m *mockVersionEventProcessor) ProcessKuberpultEvent(ctx context.Context, ev KuberpultEvent) {
	m.events = append(m.events, ev)
}

func assertStep(t *testing.T, i int, s step, vp *mockVersionEventProcessor, hs *setup.HealthServer) {
	if hs.IsReady("versions") != s.ExpectReady {
		t.Errorf("wrong readyness in step %d, expected %t but got %t", i, s.ExpectReady, hs.IsReady("versions"))
	}
	//Sort this to avoid flakeyness based on order
	sort.Slice(vp.events, func(i, j int) bool {
		return vp.events[i].Environment < vp.events[j].Environment
	})
	//Sort this to avoid flakeyness based on order
	sort.Slice(s.ExpectedEvents, func(i, j int) bool {
		return s.ExpectedEvents[i].Environment < s.ExpectedEvents[j].Environment
	})
	if !cmp.Equal(s.ExpectedEvents, vp.events) {
		t.Errorf("version events differ: %s", cmp.Diff(s.ExpectedEvents, vp.events))
	}
	vp.events = nil
}

func assertExpectedVersions(t *testing.T, expectedVersions []expectedVersion, vc VersionClient, mc *mockOverviewClient, mvc *mockVersionClient) {
	for _, ev := range expectedVersions {
		version, err := vc.GetVersion(context.Background(), ev.Revision, ev.Environment, ev.Application)
		if err != nil {
			t.Errorf("expected no error for %s/%s@%s, but got %q", ev.Environment, ev.Application, ev.Revision, err)
			continue
		}
		//We ignore the timestamp as it is based on test execution. Everything else we check

		if version.Version != ev.DeployedVersion {
			t.Errorf("expected version %d to be deployed for %s/%s@%s but got %d", ev.DeployedVersion, ev.Environment, ev.Application, ev.Revision, version.Version)
		}

		if version.SourceCommitId != ev.SourceCommitId {
			t.Errorf("expected source commit id to be %q for %s/%s@%s but got %q", ev.SourceCommitId, ev.Environment, ev.Application, ev.Revision, version.SourceCommitId)
		}
		if !cmp.Equal(mc.LastMetadata, ev.OverviewMetadata) {
			t.Errorf("mismachted version metadata %s", cmp.Diff(mc.LastMetadata, ev.OverviewMetadata))
		}
		if !cmp.Equal(mvc.LastMetadata, ev.VersionMetadata) {
			t.Errorf("mismachted version metadata %s", cmp.Diff(mvc.LastMetadata, ev.VersionMetadata))
		}

	}
}

// setupDB returns a new DBHandler with a tmp directory every time, so tests can are completely independent
func setupDB(t *testing.T) *db.DBHandler {
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatal(err)
	}
	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, t.Name())
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	t.Logf("directory for DB migrations: %s", migrationsPath)
	t.Logf("tmp dir for DB data: %s", tmpDir)

	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	setupErr := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
		if err != nil {
			return err
		}
		err = dbHandler.DBWriteEnvironment(ctx, transaction, "staging", config.EnvironmentConfig{})
		if err != nil {
			return err
		}
		err = dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "foo", db.AppStateChangeCreate, db.DBAppMetaData{}, "foo")
		if err != nil {
			return err
		}
		var version uint64 = 1234
		err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
			ReleaseNumbers: types.ReleaseNumbers{
				Revision: 0,
				Version:  &version,
			},
			Created: time.Unix(123456789, 0).UTC(),
			App:     "foo",
			Manifests: db.DBReleaseManifests{
				Manifests: map[types.EnvName]string{"staging": ""},
			},
			Metadata: db.DBReleaseMetaData{},
		})
		if err != nil {
			return err
		}

		err = dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, db.Deployment{
			Created: time.Unix(123456789, 0).UTC(),
			App:     "foo",
			Env:     "staging",
			ReleaseNumbers: types.ReleaseNumbers{
				Revision: 0,
				Version:  &version,
			},
			TransformerID: 0,
		})

		return err
	})

	if setupErr != nil {
		t.Fatal(setupErr)
	}
	return dbHandler
}
