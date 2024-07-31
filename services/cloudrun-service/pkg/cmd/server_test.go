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

package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/services/cloudrun-service/pkg/cloudrun"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestReadQueuedDeploymentEvents(t *testing.T) {
	tcs := []struct {
		name                      string
		queuedDeployments         []*cloudrun.QueuedDeploymentEvent
		expectedQueuedDeployments []*cloudrun.QueuedDeploymentEvent
	}{
		{
			name:                      "no queued deployments",
			queuedDeployments:         []*cloudrun.QueuedDeploymentEvent{},
			expectedQueuedDeployments: []*cloudrun.QueuedDeploymentEvent{},
		},
		{
			name: "queued deployments",
			queuedDeployments: []*cloudrun.QueuedDeploymentEvent{{
				Id:       1,
				Manifest: []byte("test-manifest"),
			}},
			expectedQueuedDeployments: []*cloudrun.QueuedDeploymentEvent{
				{
					Id:       1,
					Manifest: []byte("test-manifest"),
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
			err := writeDeploymentEvents(ctx, dbHandler, test.queuedDeployments)
			if err != nil {
				t.Fatalf("error while writing deployment events: %v", err)
			}
			queuedDeployments, err := readQueuedDeploymentEvents(ctx, dbHandler)
			if err != nil {
				t.Fatalf("expected no error while reading events, but got: %v", err)
			}
			// if !compareArrays(test.expectedQueuedDeployments, queuedDeployments) {
			// 	t.Error("output mismatch")
			// }
			if diff := cmp.Diff(test.expectedQueuedDeployments, queuedDeployments, cmpopts.IgnoreFields(cloudrun.QueuedDeploymentEvent{}, "Manifest")); diff != "" {
				t.Errorf("result mismatch (-want, +got):\n%s", diff)
			}
			// if !reflect.DeepEqual(test.expectedQueuedDeployments, queuedDeployments) {
			// 	t.Errorf("result mismatch (-want, +got):\n")
			// }
		})
	}
}

func writeDeploymentEvents(ctx context.Context, dbHandler *db.DBHandler, events []*cloudrun.QueuedDeploymentEvent) error {
	for _, event := range events {
		err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
			insertQuery := dbHandler.AdaptQuery(fmt.Sprintf("INSERT INTO %s (created_at, manifest, processed) VALUES (?, ?, ?);", cloudrun.QueuedDeploymentsTable))
			_, err := transaction.Exec(insertQuery, time.Now().UTC(), event.Manifest, false)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("could not write deployment to %s table: %v", cloudrun.QueuedDeploymentsTable, err)
		}
	}
	return nil
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
