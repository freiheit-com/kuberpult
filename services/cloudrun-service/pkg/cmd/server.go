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
	"os"
	"time"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/cloudrun-service/pkg/cloudrun"
	dbx "github.com/freiheit-com/kuberpult/services/cloudrun-service/pkg/db"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func RunServer() {
	err := logger.Wrap(context.Background(), runServer)
	if err != nil {
		fmt.Printf("error: %v %#v", err, err)
	}
}

func runServer(ctx context.Context) error {
	if err := cloudrun.Init(ctx); err != nil {
		logger.FromContext(ctx).Fatal("Failed to initialize cloud run service", zap.Error(err))
	}

	dbLocation, err := readEnvVar("KUBERPULT_DB_LOCATION")
	if err != nil {
		return err
	}
	dbAuthProxyPort, err := readEnvVar("KUBERPULT_DB_AUTH_PROXY_PORT")
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

	var dbCfg db.DBConfig
	if dbOption == "postgreSQL" {
		dbCfg = db.DBConfig{
			DbHost:         dbLocation,
			DbPort:         dbAuthProxyPort,
			DriverName:     "postgres",
			DbName:         dbName,
			DbPassword:     dbPassword,
			DbUser:         dbUserName,
			MigrationsPath: "",
			WriteEslOnly:   false,
			SSLMode:        "verify-full",
		}
	} else {
		logger.FromContext(ctx).Fatal("unsupported value", zap.String("dbOption", dbOption))
	}
	dbHandler, err := db.Connect(dbCfg)
	if err != nil {
		return err
	}

	setup.Run(ctx, setup.ServerConfig{
		HTTP: []setup.HTTPConfig{},
		GRPC: &setup.GRPCConfig{
			Shutdown: nil,
			Port:     "8443",
			Opts:     nil,
			Register: func(srv *grpc.Server) {
				api.RegisterCloudRunServiceServer(srv, &cloudrun.CloudRunService{DBHandler: dbHandler})
			},
		},
		Background: []setup.BackgroundTaskConfig{
			{
				Shutdown: nil,
				Name:     "processDeploymentEvents",
				Run: func(ctx context.Context, hr *setup.HealthReporter) error {
					hr.ReportReady("processing deployment events")
					return processDeploymentEvents(ctx, dbHandler)
				},
			},
		},
		Shutdown: nil,
	})

	return nil
}

func readEnvVar(envName string) (string, error) {
	envValue, ok := os.LookupEnv(envName)
	if !ok {
		return "", fmt.Errorf("could not read environment variable '%s'", envName)
	}
	return envValue, nil
}

func processDeploymentEvents(ctx context.Context, dbHandler *db.DBHandler) error {
	log := logger.FromContext(ctx).Sugar()
	for {
		time.Sleep(5 * time.Second)
		queuedDeployments, err := dbx.GetQueuedDeployments(ctx, dbHandler)
		if err != nil {
			log.Errorf("failed to read queued deployment events: %v", err)
			continue
		}
		if len(queuedDeployments) == 0 {
			log.Info("no queued deployments to process")
			continue
		}
		for _, deploymentEvent := range queuedDeployments {
			err := processEvent(ctx, deploymentEvent, dbHandler)
			if err != nil {
				log.Errorf("failed to process deployment event %+v", deploymentEvent)
			}
		}
	}
}

func processEvent(ctx context.Context, event *dbx.QueuedDeployment, dbHandler *db.DBHandler) error {
	err := cloudrun.DeployService([]byte(event.Manifest))
	if err != nil {
		// We don't return because error during deploying the service means that the service was deployed but not ready to serve traffic
		// which is expected behavior from the cloudrun api
		logger.FromContext(ctx).Sugar().Warnf("service failed to deploy: %v", err)
	}
	err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		updateQuery := dbHandler.AdaptQuery(fmt.Sprintf("UPDATE %s SET processed = ?, processed_at = ? WHERE id = ?", dbx.QueuedDeploymentsTable))
		_, err = transaction.Exec(updateQuery, true, time.Now().UTC(), event.Id)
		if err != nil {
			return fmt.Errorf("failed to update the deployment events table: %v", err)
		}
		return nil
	})
	return nil
}
