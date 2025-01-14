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

package service

/**
The basic idea is to store kuberpult version numbers alongside of custom sql migrations.
("custom" means here that pure SQL is not enough, we need so go-code)
Each migration gets assigned a number, and all finished migrations are stored in the database.
*/

import (
	"context"
	"database/sql"
	"fmt"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/migrations"
)

type MigrationFunc func(ctx context.Context) error

type Migration struct {
	Version   *api.KuberpultVersion
	Migration MigrationFunc
}

type MigrationServer struct {
	DBHandler *db.DBHandler
}

// GetAllMigrations returns an array of ALL migrations (already applied or not)
func GetAllMigrations() []*Migration {
	return []*Migration{
		// This is where we list all required custom migrations
		// Will be filled in Ref SRX-V6RVYF
	}
}

func (s *MigrationServer) EnsureCustomMigrationApplied(ctx context.Context, in *api.EnsureCustomMigrationAppliedRequest) (*api.EnsureCustomMigrationAppliedResponse, error) {
	log := logger.FromContext(ctx).Sugar()

	if in.Version == nil {
		return nil, fmt.Errorf("kuberpult version is nil")
	}

	// 1) Check if migrations are done:
	dbDone, err := s.CustomMigrationsDone(ctx, in.Version)
	if err != nil {
		return nil, fmt.Errorf("could not check if migrations are done: %w", err)
	}
	if dbDone {
		log.Info("no migrations need to run")
		return &api.EnsureCustomMigrationAppliedResponse{
			MigrationsApplied: true,
		}, nil
	}

	err = s.RunMigrations(ctx, in.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &api.EnsureCustomMigrationAppliedResponse{
		MigrationsApplied: false,
	}, nil
}

func (s *MigrationServer) CustomMigrationsDone(ctx context.Context, version *api.KuberpultVersion) (bool, error) {
	dbVersion, err := db.WithTransactionT(s.DBHandler, ctx, 0, true, func(ctx context.Context, transaction *sql.Tx) (*api.KuberpultVersion, error) {
		dbVersion, tErr := migrations.DBReadCustomMigrationCutoff(s.DBHandler, ctx, transaction, version)
		if tErr != nil {
			return nil, tErr
		}
		return dbVersion, nil
	})
	if err != nil {
		return false, fmt.Errorf("could not check if migrations are done: %w", err)
	}
	if dbVersion == nil {
		return false, nil
	}
	if migrations.FormatKuberpultVersion(dbVersion) == migrations.FormatKuberpultVersion(version) {
		return true, nil
	}
	log := logger.FromContext(ctx).Sugar()
	log.Infof("CustomMigrationsDone diff: %s!=%s", dbVersion, version)
	return false, nil
}

func (s *MigrationServer) RunMigrations(ctx context.Context, kuberpultVersion *api.KuberpultVersion) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "CustomMigrations")
	defer span.Finish()
	log := logger.FromContext(ctx).Sugar()

	if kuberpultVersion == nil {
		return onErr(fmt.Errorf("RunMigrations: kuberpult version is nil"))
	}

	log.Infof("Starting to run all migrations...")
	all := GetAllMigrations()
	for _, m := range all {
		err := m.Migration(ctx)
		if err != nil {
			return onErr(fmt.Errorf("error during migration for version %s: %w", migrations.FormatKuberpultVersion(m.Version), err))
		}
	}
	log.Infof("All migrations are applied.")

	return onErr(s.DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		return migrations.DBWriteCustomMigrationCutoff(s.DBHandler, ctx, transaction, kuberpultVersion)
	}))
}
