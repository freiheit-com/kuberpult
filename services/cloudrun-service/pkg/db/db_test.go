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

package db

import (
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

func TestReadWriteQueuedDeployments(t *testing.T) {
	tcs := []struct {
		name                      string
		queuedDeployments         []*QueuedDeployment
		expectedQueuedDeployments []*QueuedDeployment
	}{
		{
			name:                      "no queued deployments",
			queuedDeployments:         []*QueuedDeployment{},
			expectedQueuedDeployments: []*QueuedDeployment{},
		},
		{
			name: "one queued deployment",
			queuedDeployments: []*QueuedDeployment{{
				Manifest: []byte("test-manifest"),
			}},
			expectedQueuedDeployments: []*QueuedDeployment{
				{
					Id:       1,
					Manifest: []byte("test-manifest"),
				},
			},
		},
		{
			name: "multiple queued deployments",
			queuedDeployments: []*QueuedDeployment{
				{
					Manifest: []byte("test-manifest"),
				},
				{
					Manifest: []byte("test-manifest2"),
				},
				{
					Manifest: []byte("test-manifest3"),
				},
			},
			expectedQueuedDeployments: []*QueuedDeployment{
				{
					Id:       1,
					Manifest: []byte("test-manifest"),
				},
				{
					Id:       2,
					Manifest: []byte("test-manifest2"),
				},
				{
					Id:       3,
					Manifest: []byte("test-manifest3"),
				},
			},
		},
	}
	for _, tc := range tcs {
		test := tc
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)
			for _, deploy := range test.queuedDeployments {
				err := WriteQueuedDeployment(ctx, deploy.Manifest, dbHandler)
				if err != nil {
					t.Fatalf("error while writing deployment events: %v", err)
				}
			}
			queuedDeployments, err := GetQueuedDeployments(ctx, dbHandler)
			if err != nil {
				t.Fatalf("expected no error while reading events, but got: %v", err)
			}
			if diff := cmp.Diff(test.expectedQueuedDeployments, queuedDeployments); diff != "" {
				t.Errorf("result mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func setupDB(t *testing.T) *db.DBHandler {
	dir, _ := testutil.CreateMigrationsPath(4)
	tmpDir := t.TempDir()
	cfg := db.DBConfig{
		MigrationsPath: dir,
		DriverName:     "sqlite3",
		DbHost:         tmpDir,
	}

	migErr := db.RunDBMigrations(cfg)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(cfg)
	if err != nil {
		t.Fatal(err)
	}

	return dbHandler
}
