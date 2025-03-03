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
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
)

func TestCalculateProcessDelay(t *testing.T) {
	exampleTime, err := time.Parse("2006-01-02 15:04:05", "2024-06-18 16:14:07")
	if err != nil {
		t.Fatal(err)
	}
	exampleTime10SecondsBefore := exampleTime.Add(-10 * time.Second)
	tcs := []struct {
		Name          string
		eslEvent      *db.EslEventRow
		currentTime   time.Time
		ExpectedDelay float64
	}{
		{
			Name:          "Should return 0 if there are no events",
			eslEvent:      nil,
			currentTime:   time.Now(),
			ExpectedDelay: 0,
		},
		{
			Name:          "Should return 0 if time created is not set",
			eslEvent:      &db.EslEventRow{},
			currentTime:   time.Now(),
			ExpectedDelay: 0,
		},
		{
			Name: "With one Event",
			eslEvent: &db.EslEventRow{
				EslVersion: 1,
				Created:    exampleTime10SecondsBefore,
				EventType:  "CreateApplicationVersion",
				EventJson:  "{}",
			},
			currentTime:   exampleTime,
			ExpectedDelay: 10,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := testutil.MakeTestContext()
			delay, err := calculateProcessDelay(ctx, tc.eslEvent, tc.currentTime)
			if err != nil {
				t.Fatal(err)
			}
			if delay != tc.ExpectedDelay {
				t.Errorf("expected %f, got %f", tc.ExpectedDelay, delay)
			}
		})
	}
}

func SetupRepositoryTestWithDB(t *testing.T) repository.Repository {
	r, _ := SetupRepositoryTestWithDBOptions(t, false)
	return r
}

func SetupRepositoryTestWithDBOptions(t *testing.T, writeEslOnly bool) (repository.Repository, *db.DBHandler) {
	ctx := context.Background()
	migrationsPath, err := testutil.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dbConfig := &db.DBConfig{
		DriverName:     "sqlite3",
		MigrationsPath: migrationsPath,
		WriteEslOnly:   writeEslOnly,
	}

	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	err = cmd.Start()
	if err != nil {
		t.Fatalf("error starting %v", err)
		return nil, nil
	}
	err = cmd.Wait()
	if err != nil {
		t.Fatalf("error waiting %v", err)
		return nil, nil
	}
	t.Logf("test created dir: %s", localDir)

	repoCfg := repository.RepositoryConfig{
		URL:                 remoteDir,
		Path:                localDir,
		CommitterEmail:      "kuberpult@freiheit.com",
		CommitterName:       "kuberpult",
		ArgoCdGenerateFiles: true,
	}
	dbConfig.DbHost = dir

	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	repoCfg.DBHandler = dbHandler

	repo, err := repository.New(
		testutil.MakeTestContext(),
		repoCfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, dbHandler
}
