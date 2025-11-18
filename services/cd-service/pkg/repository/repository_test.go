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

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/types"

	"github.com/DataDog/datadog-go/v5/statsd"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"

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

type SlowTransformer struct {
	finished chan struct{}
	started  chan struct{}
}

func (s *SlowTransformer) GetDBEventType() db.EventType {
	return "invalid"
}
func (s *SlowTransformer) SetEslVersion(_ db.TransformerID) {
	//Does nothing
}

func (s *SlowTransformer) GetEslVersion() db.TransformerID {
	return 0
}

func (s *SlowTransformer) Transform(ctx context.Context, state *State, transformerContext TransformerContext, transaction *sql.Tx) (string, error) {
	s.started <- struct{}{}
	<-s.finished
	return "ok", nil
}

type EmptyTransformer struct{}

func (p *EmptyTransformer) SetEslVersion(_ db.TransformerID) {
	//Does nothing
}

func (p *EmptyTransformer) GetEslVersion() db.TransformerID {
	return 0
}

func (p *EmptyTransformer) GetDBEventType() db.EventType {
	return "invalid"
}

func (p *EmptyTransformer) Transform(ctx context.Context, state *State, transformerContext TransformerContext, transaction *sql.Tx) (string, error) {
	return "nothing happened", nil
}

type PanicTransformer struct{}

func (p *PanicTransformer) GetDBEventType() db.EventType {
	return "invalid"
}

func (p *PanicTransformer) GetEslVersion() db.TransformerID {
	return 0
}

func (p *PanicTransformer) SetEslVersion(_ db.TransformerID) {
	//Does nothing
}

func (p *PanicTransformer) Transform(ctx context.Context, state *State, transformerContext TransformerContext, transaction *sql.Tx) (string, error) {
	panic("panic tranformer")
}

var ErrTransformer = errors.New("error transformer")

type ErrorTransformer struct{}

func (p *ErrorTransformer) GetDBEventType() db.EventType {
	return "invalid"
}

func (p *ErrorTransformer) Transform(ctx context.Context, state *State, transformerContext TransformerContext, transaction *sql.Tx) (string, error) {
	return "error", ErrTransformer
}

func (p *ErrorTransformer) SetEslVersion(_ db.TransformerID) {
	//Does nothing
}

func (p *ErrorTransformer) GetEslVersion() db.TransformerID {
	return 0
}

type InvalidJsonTransformer struct{}

func (p *InvalidJsonTransformer) GetDBEventType() db.EventType {
	return "invalid"
}

func (p *InvalidJsonTransformer) SetEslVersion(_ db.TransformerID) {
	//Does nothing
}

func (p *InvalidJsonTransformer) GetEslVersion() db.TransformerID {
	return 0
}

func (p *InvalidJsonTransformer) Transform(ctx context.Context, state *State, transformerContext TransformerContext, transaction *sql.Tx) (string, error) {
	return "error", ErrInvalidJson
}

func convertToSet(list []types.ReleaseNumbers) map[TestStruct]bool {
	set := make(map[TestStruct]bool)
	for _, i := range list {
		set[TestStruct{Version: *i.Version, Revision: i.Revision}] = true
	}
	return set
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

func getTransformer(i int) (Transformer, error) {
	// transformerType := i % 5
	// switch transformerType {
	// case 0:
	// case 1:
	// case 2:
	return &CreateApplicationVersion{
		Application: "foo",
		Manifests: map[types.EnvName]string{
			"development": fmt.Sprintf("%d", i),
		},
		Version: uint64(i),
	}, nil
	// case 3:
	// 	return &ErrorTransformer{}, TransformerError
	// case 4:
	// 	return &InvalidJsonTransformer{}, InvalidJson
	// }
	// return &ErrorTransformer{}, TransformerError
}

func TestProcessQueueOnce(t *testing.T) {
	tcs := []struct {
		Name          string
		Element       transformerBatch
		ExpectedError error
	}{
		{
			Name: "success",
			Element: transformerBatch{
				ctx: testutil.MakeTestContext(),
				transformers: []Transformer{
					&EmptyTransformer{},
				},
				result: make(chan error, 1),
			},
			ExpectedError: nil,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			// create a remote
			dir := t.TempDir()
			repo := SetupRepositoryTestWithDB(t)
			repoInternal := repo.(*repository)
			repoInternal.ProcessQueueOnce(testutil.MakeTestContext(), tc.Element)

			result := tc.Element.result
			actualError := <-result

			var expectedError error
			if tc.ExpectedError != nil {
				expectedError = errMatcher{strings.ReplaceAll(tc.ExpectedError.Error(), "$DIR", dir)}
			}
			if diff := cmp.Diff(expectedError, actualError, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestApplyTransformerBatch(t *testing.T) {
	tcs := []struct {
		Name                string
		Batches             []transformerBatch
		failingBatchIndexes []int
	}{
		{
			Name: "One Batch One Transformer success",
			Batches: []transformerBatch{
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&EmptyTransformer{},
					},
					result: make(chan error, 1),
				},
			},
			failingBatchIndexes: nil,
		},
		{
			Name: "One Batch Multiple Transformer success",
			Batches: []transformerBatch{
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&EmptyTransformer{},
						&EmptyTransformer{},
					},
					result: make(chan error, 1),
				},
			},
			failingBatchIndexes: nil,
		},
		{
			Name: "Multiple Batches Multiple Transformer success",
			Batches: []transformerBatch{
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&EmptyTransformer{},
					},
					result: make(chan error, 1),
				},
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&EmptyTransformer{},
					},
					result: make(chan error, 1),
				},
			},
			failingBatchIndexes: nil,
		},
		{
			Name: "Multiple Batches Some fail",
			Batches: []transformerBatch{
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&EmptyTransformer{},
					},
					result: make(chan error, 1),
				},
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&ErrorTransformer{},
					},
					result: make(chan error, 1),
				},
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&EmptyTransformer{},
					},
					result: make(chan error, 1),
				},
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&ErrorTransformer{},
					},
					result: make(chan error, 1),
				},
			},
			failingBatchIndexes: []int{1, 3},
		},
		{
			Name: "Multiple Batches Multiple transformer Some fail",
			Batches: []transformerBatch{
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&EmptyTransformer{},
						&ErrorTransformer{},
					},
					result: make(chan error, 1),
				},
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&ErrorTransformer{},
						&EmptyTransformer{},
					},
					result: make(chan error, 1),
				},
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&EmptyTransformer{},
						&EmptyTransformer{},
					},
					result: make(chan error, 1),
				},
				{
					ctx: testutil.MakeTestContext(),
					transformers: []Transformer{
						&ErrorTransformer{},
						&ErrorTransformer{},
					},
					result: make(chan error, 1),
				},
			},
			failingBatchIndexes: []int{0, 1, 3},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			repo := SetupRepositoryTestWithDB(t)

			repoInternal := repo.(*repository)
			resultingBatches, _, err := repoInternal.applyTransformerBatches(tc.Batches)
			if err != nil {
				t.Errorf("Got error here but was not expecting: %v", err)
			}

			if tc.failingBatchIndexes == nil {
				if diff := cmp.Diff(tc.Batches, resultingBatches, cmpopts.IgnoreUnexported(transformerBatch{})); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(len(tc.Batches)-len(tc.failingBatchIndexes), len(resultingBatches), cmpopts.IgnoreUnexported(transformerBatch{})); diff != "" {
					t.Errorf("Number of resulting transformers mismatch (-want, +got):\n%s", diff)
				}
				batches := tc.Batches
				removedElements := 0
				for _, elem := range tc.failingBatchIndexes { //Filter out the supposed failed batches
					batches = append(batches[:elem-removedElements], batches[elem+1-removedElements:]...)
					removedElements++
				}

				if diff := cmp.Diff(batches, resultingBatches, cmpopts.IgnoreUnexported(transformerBatch{})); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

type TestWebhookResolver struct {
	t        *testing.T
	rec      *httptest.ResponseRecorder
	requests chan *http.Request
}

func (resolver TestWebhookResolver) Resolve(insecure bool, req *http.Request) (*http.Response, error) {
	testhandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resolver.t.Logf("called with request: %v", *r)
		resolver.requests <- req
		close(resolver.requests)
	})
	testhandler.ServeHTTP(resolver.rec, req)
	response := resolver.rec.Result()
	resolver.t.Logf("responded with: %v", response)
	return response, nil
}

type nilTransformer struct {
}

func (m *nilTransformer) Transform(_ context.Context, _ *State, _ TransformerContext, _ *sql.Tx) (commitMsg string, e error) {
	return "", nil
}
func (*nilTransformer) GetDBEventType() db.EventType {
	return "nilEvent"
}
func (*nilTransformer) SetEslVersion(_ db.TransformerID) {
	// nothing to do
}
func (*nilTransformer) GetEslVersion() db.TransformerID {
	panic("getEslVersion")
}

var noopTransformer = &nilTransformer{}

func TestMeasureGitSyncStatus(t *testing.T) {
	tcs := []struct {
		Name             string
		SyncedFailedApps []db.EnvApp
		UnsyncedApps     []db.EnvApp
		ExpectedGauges   []Gauge
	}{
		{
			Name:             "No unsynced or sync failed apps",
			SyncedFailedApps: []db.EnvApp{},
			UnsyncedApps:     []db.EnvApp{},
			ExpectedGauges: []Gauge{
				{Name: "git_sync_unsynced", Value: 0, Tags: []string{}, Rate: 1},
				{Name: "git_sync_failed", Value: 0, Tags: []string{}, Rate: 1},
			},
		},
		{
			Name: "Some sync failed apps",
			SyncedFailedApps: []db.EnvApp{
				{EnvName: "dev", AppName: "app"},
				{EnvName: "dev", AppName: "app2"},
			},
			UnsyncedApps: []db.EnvApp{
				{EnvName: "staging", AppName: "app"},
			},
			ExpectedGauges: []Gauge{
				{Name: "git_sync_unsynced", Value: 1, Tags: []string{}, Rate: 1},
				{Name: "git_sync_failed", Value: 2, Tags: []string{}, Rate: 1},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			var mockClient = &MockClient{}
			var client statsd.ClientInterface = mockClient

			ctx := testutil.MakeTestContext()
			repo := SetupRepositoryTestWithDB(t)
			ddMetrics = client
			dbHandler := repo.State().DBHandler

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteNewSyncEventBulk(ctx, transaction, 0, tc.SyncedFailedApps, db.SYNC_FAILED)
				if err != nil {
					return err
				}

				err = dbHandler.DBWriteNewSyncEventBulk(ctx, transaction, 0, tc.UnsyncedApps, db.UNSYNCED)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				t.Fatalf("failed to write sync events to db: %v", err)
			}

			err = MeasureGitSyncStatus(len(tc.UnsyncedApps), len(tc.SyncedFailedApps))
			if err != nil {
				t.Fatalf("failed to send git sync status metrics: %v", err)
			}

			cmpGauge := func(i, j Gauge) bool {
				if len(i.Tags) == 0 && len(j.Tags) == 0 {
					return i.Name > j.Name
				} else if len(i.Tags) != len(j.Tags) {
					return len(i.Tags) > len(j.Tags)
				} else {
					for tagIndex := range i.Tags {
						if i.Tags[tagIndex] != j.Tags[tagIndex] {
							return i.Tags[tagIndex] > j.Tags[tagIndex]
						}
					}
					return true
				}
			}
			if diff := cmp.Diff(tc.ExpectedGauges, mockClient.gauges, cmpopts.SortSlices(cmpGauge)); diff != "" {
				t.Errorf("gauges mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func SetupRepositoryBenchmark(t *testing.B) (Repository, *db.DBHandler) {
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}

	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, false, fmt.Sprintf("%s_%d", t.Name(), t.N))
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}

	repoCfg := RepositoryConfig{
		ArgoCdGenerateFiles: true,
	}

	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	repoCfg.DBHandler = dbHandler

	repo, err := New(
		testutil.MakeTestContext(),
		repoCfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, dbHandler
}

type TestStruct struct {
	Version  uint64
	Revision uint64
}

func BenchmarkApplyQueue(t *testing.B) {
	t.StopTimer()
	repo, _ := SetupRepositoryBenchmark(t)
	ctx := testutil.MakeTestContext()
	dbHandler := repo.State().DBHandler

	repoInternal := repo.(*repository)
	// The worker go routine is now blocked. We can move some items into the queue now.
	results := make([]error, t.N)
	expectedResults := make([]error, t.N)

	expectedReleases := make(map[TestStruct]bool, t.N)

	err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
		if err != nil {
			return err
		}

		err = dbHandler.DBWriteEnvironment(ctx, transaction, "development", config.EnvironmentConfig{}, []types.AppName{"foo"})
		if err != nil {
			return err
		}

		expectedResults[0] = nil
		results[0] = nil
		t.StartTimer()
		for i := 1; i < t.N; i++ {
			tf, expectedResult := getTransformer(i)
			_, _, _, err2 := repoInternal.ApplyTransformersInternal(ctx, transaction, tf)
			if err2 != nil {
				results[i] = err2.TransformerError
			} else {
				results[i] = nil
			}
			expectedResults[i] = expectedResult
			if expectedResult == nil {
				expectedReleases[TestStruct{Version: uint64(i), Revision: 0}] = true
			}
		}
		for i := 0; i < t.N; i++ {
			if diff := cmp.Diff(expectedResults[i], results[i], cmpopts.EquateErrors()); diff != "" {
				t.Errorf("result[%d] expected error \"%v\" but got \"%v\"", i, expectedResults[i], results[i])
			}
		}
		releases, _ := repo.State().GetAllApplicationReleases(ctx, transaction, "foo")

		if diff := cmp.Diff(expectedReleases, convertToSet(releases)); diff != "" {
			t.Fatalf("Output mismatch (-want +got): %s\n", diff)
		}

		return nil
	})
	if err != nil {
		t.Errorf("Error applying transformers: %v", err)
	}
}
