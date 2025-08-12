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
	"github.com/freiheit-com/kuberpult/pkg/types"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"

	"github.com/freiheit-com/kuberpult/pkg/setup"

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

func TestApplyQueuePanic(t *testing.T) {
	type action struct {
		Transformer Transformer
		// Tests
		ExpectedError error
	}
	tcs := []struct {
		Name    string
		Actions []action
	}{
		{
			Name: "panic at the start",
			Actions: []action{
				{
					Transformer:   &PanicTransformer{},
					ExpectedError: errPanic,
				}, {
					ExpectedError: errPanic,
				}, {
					ExpectedError: errPanic,
				},
			},
		},
		{
			Name: "panic at the middle",
			Actions: []action{
				{
					ExpectedError: errPanic,
				}, {
					Transformer:   &PanicTransformer{},
					ExpectedError: errPanic,
				}, {
					ExpectedError: errPanic,
				},
			},
		},
		{
			Name: "panic at the end",
			Actions: []action{
				{
					ExpectedError: errPanic,
				}, {
					ExpectedError: errPanic,
				}, {
					Transformer:   &PanicTransformer{},
					ExpectedError: errPanic,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			// create a remote
			ctx := testutil.MakeTestContext()
			migrationsPath, err := db.CreateMigrationsPath(4)
			if err != nil {
				t.Fatalf("CreateMigrationsPath error: %v", err)
			}
			dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, false, t.Name())
			if err != nil {
				t.Fatalf("SetupPostgres: %v", err)
			}

			repoCfg := RepositoryConfig{
				ArgoCdGenerateFiles:   true,
				MaximumCommitsPerPush: 3,
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

			repo, processQueue, err := New2(
				ctx,
				repoCfg,
			)
			if err != nil {
				t.Fatal(err)
			}
			// The worker go routine is not started. We can move some items into the queue now.
			results := make([]<-chan error, len(tc.Actions))
			for i, action := range tc.Actions {
				// We are using the internal interface here as an optimization to avoid spinning up one go-routine per action
				t := action.Transformer
				if t == nil {
					t = &EmptyTransformer{}
				}
				results[i] = repo.(*repository).applyDeferred(testutil.MakeTestContext(), t)
			}
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("The code did not panic")
				} else if r != "panic tranformer" {
					t.Logf("The code did not panic with the correct string but %#v", r)
					panic(r)
				}
				// Check for the correct errors
				for i, action := range tc.Actions {
					if err := <-results[i]; err != action.ExpectedError {
						t.Errorf("result[%d] error is not \"%v\" but got \"%v\"", i, action.ExpectedError, err)
					}
				}
			}()
			ctx, cancel := context.WithTimeout(testutil.MakeTestContext(), 10*time.Second)
			defer cancel()
			err = processQueue(ctx, nil)
			if err != nil {
				t.Fatalf("processQueue: %v", err)
			}
		})
	}
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

func TestApplyQueueTtlForHealth(t *testing.T) {
	// we set the networkTimeout to something low, so that it doesn't interfere with other processes e.g like once per second:
	networkTimeout := 1 * time.Second

	tcs := []struct {
		Name string
	}{
		{
			Name: "sleeps way too long, so health should fail",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(testutil.MakeTestContext(), 10*time.Second)
			migrationsPath, err := db.CreateMigrationsPath(4)
			if err != nil {
				t.Fatalf("CreateMigrationsPath error: %v", err)
			}
			dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, false, t.Name())
			if err != nil {
				t.Fatalf("SetupPostgres: %v", err)
			}

			repoCfg := RepositoryConfig{
				ArgoCdGenerateFiles:   true,
				MaximumCommitsPerPush: 3,
				NetworkTimeout:        networkTimeout,
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

			repo, processQueue, err := New2(
				ctx,
				repoCfg,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}

			mc := mockClock{}
			hlth := &setup.HealthServer{}
			hlth.BackOffFactory = func() backoff.BackOff { return backoff.NewConstantBackOff(0) }
			hlth.Clock = mc.now
			reporterName := "ClarkKent"
			reporter := hlth.Reporter(reporterName)
			isReady := func() bool {
				return hlth.IsReady(reporterName)
			}
			errChan := make(chan error)
			go func() {
				err = processQueue(ctx, reporter)
				errChan <- err
			}()
			defer func() {
				cancel()
				chanError := <-errChan
				if chanError != nil {
					t.Errorf("Expected no error in processQueue but got: %v", chanError)
				}
			}()

			finished := make(chan struct{})
			started := make(chan struct{})
			var transformer Transformer = &SlowTransformer{
				finished: finished,
				started:  started,
			}

			go func() {
				_ = repo.Apply(ctx, transformer)
			}()

			// first, wait, until the transformer has started:
			<-started
			// health should be reporting as ready now
			if !isReady() {
				t.Error("Expected health to be ready after transformer was started, but it was not")
			}
			// now advance the clock time
			mc.sleep(4 * networkTimeout)

			// now that the transformer is started, we should get a failed health check immediately, because the networkTimeout is tiny:
			if isReady() {
				t.Error("Expected health to be not ready after transformer took too long, but it was")
			}

			// let the transformer finish:
			finished <- struct{}{}

		})
	}
}

func TestApplyQueue(t *testing.T) {
	type action struct {
		CancelBeforeAdd bool
		CancelAfterAdd  bool
		Transformer     Transformer
		// Tests
		ExpectedError error
	}
	tcs := []struct {
		Name             string
		Actions          []action
		ExpectedReleases []types.ReleaseNumbers
	}{
		{
			Name: "simple",
			Actions: []action{
				{}, {}, {},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(1),
				types.MakeReleaseNumberVersion(2),
				types.MakeReleaseNumberVersion(3),
			},
		},
		{
			Name: "cancellation in the middle (after)",
			Actions: []action{
				{}, {
					CancelAfterAdd: true,
					ExpectedError:  context.Canceled,
				}, {},
			},
			ExpectedReleases: []types.ReleaseNumbers{

				types.MakeReleaseNumberVersion(1),
				types.MakeReleaseNumberVersion(3),
			},
		},
		{
			Name: "cancellation at the start (after)",
			Actions: []action{
				{
					CancelAfterAdd: true,
					ExpectedError:  context.Canceled,
				}, {}, {},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(2),
				types.MakeReleaseNumberVersion(3),
			},
		},
		{
			Name: "cancellation at the end (after)",
			Actions: []action{
				{}, {},
				{
					CancelAfterAdd: true,
					ExpectedError:  context.Canceled,
				},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(1),
				types.MakeReleaseNumberVersion(2),
			},
		},
		{
			Name: "cancellation in the middle (before)",
			Actions: []action{
				{}, {
					CancelBeforeAdd: true,
					ExpectedError:   context.Canceled,
				}, {},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(1),
				types.MakeReleaseNumberVersion(3),
			},
		},
		{
			Name: "cancellation at the start (before)",
			Actions: []action{
				{
					CancelBeforeAdd: true,
					ExpectedError:   context.Canceled,
				}, {}, {},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(2),
				types.MakeReleaseNumberVersion(3),
			},
		},
		{
			Name: "cancellation at the end (before)",
			Actions: []action{
				{}, {},
				{
					CancelBeforeAdd: true,
					ExpectedError:   context.Canceled,
				},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(1),
				types.MakeReleaseNumberVersion(2),
			},
		},
		{
			Name: "error at the start",
			Actions: []action{
				{
					ExpectedError: &TransformerBatchApplyError{TransformerError: ErrTransformer, Index: 0},
					Transformer:   &ErrorTransformer{},
				}, {}, {},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(2),
				types.MakeReleaseNumberVersion(3),
			},
		},
		{
			Name: "error at the middle",
			Actions: []action{
				{},
				{
					ExpectedError: &TransformerBatchApplyError{TransformerError: ErrTransformer, Index: 0},
					Transformer:   &ErrorTransformer{},
				}, {},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(1),
				types.MakeReleaseNumberVersion(3),
			},
		},
		{
			Name: "error at the end",
			Actions: []action{
				{}, {},
				{
					ExpectedError: &TransformerBatchApplyError{TransformerError: ErrTransformer, Index: 0},
					Transformer:   &ErrorTransformer{},
				},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(1),
				types.MakeReleaseNumberVersion(2),
			},
		},
		{
			Name: "Invalid json error at start",
			Actions: []action{
				{
					ExpectedError: &TransformerBatchApplyError{TransformerError: ErrInvalidJson, Index: 0},
					Transformer:   &InvalidJsonTransformer{},
				},
				{}, {},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(2),
				types.MakeReleaseNumberVersion(3),
			},
		},
		{
			Name: "Invalid json error at middle",
			Actions: []action{
				{},
				{
					ExpectedError: &TransformerBatchApplyError{TransformerError: ErrInvalidJson, Index: 0},
					Transformer:   &InvalidJsonTransformer{},
				},
				{},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(1),
				types.MakeReleaseNumberVersion(3),
			},
		},
		{
			Name: "Invalid json error at end",
			Actions: []action{
				{}, {},
				{
					ExpectedError: &TransformerBatchApplyError{TransformerError: ErrInvalidJson, Index: 0},
					Transformer:   &InvalidJsonTransformer{},
				},
			},
			ExpectedReleases: []types.ReleaseNumbers{
				types.MakeReleaseNumberVersion(1),
				types.MakeReleaseNumberVersion(2),
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := SetupRepositoryTestWithAllOptions(t, false, 100, true, false)
			ctx := testutil.MakeTestContext()
			repoInternal := repo.(*repository)
			// Block the worker so that we have multiple items in the queue
			finished := make(chan struct{})
			started := make(chan struct{}, 1)
			var applyErr error = nil
			go func() {
				applyErr = repo.Apply(testutil.MakeTestContext(), &SlowTransformer{finished: finished, started: started})
			}()
			<-started
			// The worker go routine is now blocked. We can move some items into the queue now.
			results := make([]<-chan error, len(tc.Actions))
			for i, action := range tc.Actions {
				ctx, cancel := context.WithCancel(testutil.MakeTestContext()) //nolint:govet
				if action.CancelBeforeAdd {
					cancel()
				}
				if action.Transformer != nil {
					results[i] = repoInternal.applyDeferred(ctx, action.Transformer)
				} else {
					_ = repoInternal.applyDeferred(ctx, &CreateEnvironment{
						Environment: "development",
					})
					tf := &CreateApplicationVersion{
						Application: "foo",
						Manifests: map[types.EnvName]string{
							"development": fmt.Sprintf("%d", i),
						},
						Version: uint64(i + 1),
					}
					results[i] = repoInternal.applyDeferred(ctx, tf)
				}
				if action.CancelAfterAdd {
					cancel()
				}
			}
			// Now release the slow transformer
			finished <- struct{}{}
			// Check for the correct errors
			for i, action := range tc.Actions {
				err := <-results[i]
				expErrStr := ""
				if action.ExpectedError != nil {
					expErrStr = action.ExpectedError.Error()
				}
				if err != nil && err.Error() != expErrStr {
					t.Errorf("result[%d] error is not \"%s\" but got \"%v\"", i, expErrStr, err)
				}
			}
			_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				releases, _ := repo.State().GetAllApplicationReleases(ctx, transaction, "foo")
				if !cmp.Equal(tc.ExpectedReleases, releases) {
					t.Fatal("Output mismatch (-want +got):\n", cmp.Diff(tc.ExpectedReleases, releases))
				}
				return nil
			})
			if applyErr != nil {
				t.Fatalf("could not run slow transformer: %v", applyErr)
			}
		}) //nolint:govet
	}
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

func TestLimitTooSmall(t *testing.T) {
	tcs := []struct {
		Name          string
		QueueCapacity uint // max queue size
	}{
		{
			Name:          "size 1: queue full",
			QueueCapacity: 1,
		},
		{
			Name:          "size 2: queue full",
			QueueCapacity: 2,
		},
		{
			Name:          "size 5: queue full",
			QueueCapacity: 15,
		},
		{
			Name:          "size 1000: queue full",
			QueueCapacity: 1000,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := SetupRepositoryTestWithAllOptions(t, false, tc.QueueCapacity, false, false)
			ctx := testutil.MakeTestContext()
			var i = 0
			for range tc.QueueCapacity {
				noop := &nilTransformer{}
				errCh := repo.(*repository).applyDeferred(ctx, noop)
				if errCh == nil {
					t.Errorf("unexpected error in setup")
				}
				i++
			}

			var actualError error
			errCh := repo.(*repository).applyDeferred(ctx, noopTransformer)
			select {
			case e := <-repo.(*repository).queue.transformerBatches:
				repo.(*repository).ProcessQueueOnce(ctx, e)
			default:
			}
			actualError = <-errCh

			var expectedError = errMatcher{fmt.Sprintf("queue is full. Queue Capacity: %d", tc.QueueCapacity)}
			var expErrStr = fmt.Sprintf("%v", expectedError)
			var actErrStr = fmt.Sprintf("%v", actualError)
			if expErrStr != actErrStr {
				t.Errorf("error mismatch, expected '%s', got '%s'", expErrStr, actErrStr)
			}
		})
	}
}

func TestLimitFitsExactly(t *testing.T) {
	tcs := []struct {
		Name          string
		QueueCapacity uint // max queue size
	}{
		{
			Name:          "size 1: queue has room",
			QueueCapacity: 1,
		},
		{
			Name:          "size 2: queue has room",
			QueueCapacity: 2,
		},
		{
			Name:          "size 20: queue has room",
			QueueCapacity: 20,
		},
		{
			Name:          "size 1000: queue has room",
			QueueCapacity: 1000,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := SetupRepositoryTestWithAllOptions(t, false, tc.QueueCapacity, false, false)
			ctx := testutil.MakeTestContext()
			var errChannels = make([]<-chan error, 0)
			var i = 0
			for range tc.QueueCapacity - 1 { // we fill it so that 1 element still fits in
				noop := &nilTransformer{}
				errCh := repo.(*repository).applyDeferred(ctx, noop)
				if errCh == nil {
					t.Errorf("unexpected error in setup")
				}
				errChannels = append(errChannels, errCh)
				i++
			}

			var actualError error

			// first put int the new transformer, this should return a channel with error queue is full:
			errCh := repo.(*repository).applyDeferred(ctx, noopTransformer)

			// then make sure all previous error channels are waited for:
			for i2 := range errChannels {
				e := <-repo.(*repository).queue.transformerBatches
				repo.(*repository).ProcessQueueOnce(ctx, e)

				errCh := errChannels[i2]
				err := <-errCh
				if err != nil {
					t.Errorf("initial channel return an error: %v", err)
				}
			}

			// now process the new transformer:
			select {
			case e := <-repo.(*repository).queue.transformerBatches:
				t.Logf("go: ProcessQueueOnce start")
				repo.(*repository).ProcessQueueOnce(ctx, e)
				t.Logf("go: ProcessQueueOnce end")
			default:
			}
			actualError = <-errCh

			var expectedErr error = nil
			var expErrStr = fmt.Sprintf("%v", expectedErr)
			var actErrStr = fmt.Sprintf("%v", actualError)
			if expErrStr != actErrStr {
				t.Errorf("error mismatch, expected '%s', got '%s'", expErrStr, actErrStr)
			}

		})
	}
}

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

		err = dbHandler.DBWriteEnvironment(ctx, transaction, "development", config.EnvironmentConfig{}, []string{"foo"})
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
