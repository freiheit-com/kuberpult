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
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"os"
)

func RunServer() {
	err := logger.Wrap(context.Background(), func(ctx context.Context) error {
		logger.FromContext(ctx).Sugar().Warnf("hello world from the manifest-repo-export-service!")

		dbLocation, err := readEnvVar("KUBERPULT_DB_LOCATION")
		if err != nil {
			return err
		}
		dbName, err := readEnvVar("KUBERPULT_DB_NAME")
		if err != nil {
			return err
		}
		dbOption, err := readEnvVar("KUBERPULT_DB_OPTION")
		if err != nil {
			return err
		}
		dbUserName, err := readEnvVar("KUBERPULT_DB_USER_NAME")
		if err != nil {
			return err
		}
		dbPassword, err := readEnvVar("KUBERPULT_DB_USER_PASSWORD")
		if err != nil {
			return err
		}
		dbAuthProxyPort, err := readEnvVar("KUBERPULT_DB_AUTH_PROXY_PORT")
		if err != nil {
			return err
		}

		var dbCfg db.DBConfig
		if dbOption == "cloudsql" {
			dbCfg = db.DBConfig{
				DbHost:         dbLocation,
				DbPort:         dbAuthProxyPort,
				DriverName:     "postgres",
				DbName:         dbName,
				DbPassword:     dbPassword,
				DbUser:         dbUserName,
				MigrationsPath: "",
				WriteEslOnly:   false,
			}
		} else if dbOption == "sqlite" {
			dbCfg = db.DBConfig{
				DbHost:         dbLocation,
				DbPort:         dbAuthProxyPort,
				DriverName:     "sqlite3",
				DbName:         dbName,
				DbPassword:     dbPassword,
				DbUser:         dbUserName,
				MigrationsPath: "",
				WriteEslOnly:   false,
			}
		} else {
			logger.FromContext(ctx).Fatal("Database was enabled but no valid DB option was provided.")
		}
		dbHandler, err := db.Connect(dbCfg)
		if err != nil {
			return err
		}

		err = dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
			esl, err := dbHandler.DBReadEslEventInternal(ctx, transaction)
			if err != nil {
				return err
			}
			logger.FromContext(ctx).Sugar().Warnf("esl event: %v", esl)

			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		fmt.Printf("error in startup: %v %#v", err, err)
	}
}

func readEnvVar(envName string) (string, error) {
	envValue, ok := os.LookupEnv(envName)
	if !ok {
		return "", fmt.Errorf("could not read environment variable '%s'", envName)
	}
	return envValue, nil
}
