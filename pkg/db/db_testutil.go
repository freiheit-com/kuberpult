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
	"context"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"os"
	"strings"
	"testing"
)

// simpleHash is a basic hash function for strings
func simpleHash(s string) string {
	var hash uint64 = 0
	for _, char := range s {
		hash = hash*31 + uint64(char) // Multiply by a prime number and add the character value
	}
	return fmt.Sprintf("%08d", hash)
}

// CreateMigrationsPath detects if it's running withing earthly/CI or locally and adapts the path to the migrations accordingly
func CreateMigrationsPath(numDirs int) (string, error) {
	const subDir = "/database/migrations/postgres"
	_, err := os.Stat("/kp")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			wd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			// this ".." sequence is necessary, because Getwd() returns the path of this go file (when running in an idea like goland):
			return wd + strings.Repeat("/..", numDirs) + subDir, nil
		}
		return "", err
	}
	return "/kp" + subDir, nil
}

func ConnectToPostgresContainer(ctx context.Context, t *testing.T, migrationsPath string, writeEslOnly bool, rawNewDbName string) (*DBConfig, error) {
	dbConfig := &DBConfig{
		// the options here must be the same as provided by docker-compose-unittest.yml
		DbHost:     "localhost",
		DbPort:     "5432",
		DriverName: "postgres",
		DbName:     "kuberpult",
		DbPassword: "mypassword",
		DbUser:     "postgres",
		SSLMode:    "disable",

		MigrationsPath: migrationsPath,
		WriteEslOnly:   writeEslOnly,

		MaxIdleConnections: 0,
		MaxOpenConnections: 0,
	}

	dbHandler, err := Connect(ctx, *dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	var newDbName = fmt.Sprintf("unittest_%s", simpleHash(rawNewDbName))
	logger.FromContext(ctx).Sugar().Infof("Test '%s' will use database '%s'", rawNewDbName, newDbName)
	deleteDBQuery := fmt.Sprintf("DROP DATABASE IF EXISTS %s;", newDbName)
	_, err = dbHandler.DB.ExecContext(
		ctx,
		deleteDBQuery,
	)
	if err != nil {
		// this could mean that the database is in use
		return nil, fmt.Errorf("failed to cleanup database %s: %w", newDbName, err)
	}
	createDBQuery := fmt.Sprintf("CREATE DATABASE %s;", newDbName)
	_, err = dbHandler.DB.ExecContext(
		ctx,
		createDBQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create database %s: %w", newDbName, err)
	}
	t.Logf("Database %s created successfully", newDbName)
	logger.FromContext(ctx).Sugar().Infof("Database %s created successfully", newDbName)

	dbConfig.DbName = newDbName
	err = dbHandler.DB.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close database connection %s: %w", newDbName, err)
	}

	return dbConfig, nil
}
