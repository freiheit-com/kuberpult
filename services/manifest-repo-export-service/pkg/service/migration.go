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
	migrations2 "github.com/freiheit-com/kuberpult/pkg/migrations"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/migrations"
)

type MigrationFunc func(ctx context.Context) error

type Migration struct {
	Version   *api.KuberpultVersion
	Migration MigrationFunc
}

type MigrationServer struct {
	KuberpultVersion *api.KuberpultVersion
	DBHandler        *db.DBHandler
	Migrations       []*Migration
}

func (s *MigrationServer) EnsureCustomMigrationApplied(ctx context.Context, in *api.EnsureCustomMigrationAppliedRequest) (*api.EnsureCustomMigrationAppliedResponse, error) {
	if s.KuberpultVersion == nil {
		return nil, fmt.Errorf("configured kuberpult version is nil")
	}
	if in.Version == nil {
		return nil, fmt.Errorf("requested kuberpult version is nil")
	}

	if !migrations2.IsKuberpultVersionEqual(in.Version, s.KuberpultVersion) {
		return nil, fmt.Errorf("different versions of kuberpult are running: %s!=%s",
			migrations2.FormatKuberpultVersion(in.Version),
			migrations2.FormatKuberpultVersion(s.KuberpultVersion),
		)
	}

	err := s.RunMigrations(ctx, in.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &api.EnsureCustomMigrationAppliedResponse{
		MigrationsApplied: true,
	}, nil
}

func (s *MigrationServer) RunMigrations(ctx context.Context, kuberpultVersion *api.KuberpultVersion) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "CustomMigrations")
	defer span.Finish()
	log := logger.FromContext(ctx).Sugar()

	if kuberpultVersion == nil {
		return onErr(fmt.Errorf("RunMigrations: kuberpult version is nil"))
	}

	log.Infof("Starting to run all migrations...")
	for _, m := range s.Migrations {
		err := s.DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
			// 1) Check if we need to run this migration:
			dbVersion, err := migrations.DBReadCustomMigrationCutoff(s.DBHandler, ctx, transaction, m.Version)
			if err != nil {
				return onErr(fmt.Errorf("could not read cutoff: %w", err))
			}
			if migrations2.IsKuberpultVersionEqual(dbVersion, m.Version) {
				log.Infof("migration for version %s already done according to DB", migrations2.FormatKuberpultVersion(m.Version))
				return nil
			}
			log.Infof("running migration for dbVersion %s and migrationVersion %s",
				migrations2.FormatKuberpultVersion(dbVersion),
				migrations2.FormatKuberpultVersion(m.Version),
			)

			// 2) Actually run the migration:
			err = m.Migration(ctx)
			if err != nil {
				return onErr(fmt.Errorf("could not run migration: %w", err))
			}

			// 2) Store that we did run the migration:
			return migrations.DBUpsertCustomMigrationCutoff(s.DBHandler, ctx, transaction, kuberpultVersion)
		})
		if err != nil {
			return onErr(fmt.Errorf("RunMigrations: error for version %s: %w", migrations2.FormatKuberpultVersion(m.Version), err))
		}
	}
	log.Infof("All migrations are applied.")
	return nil
}
