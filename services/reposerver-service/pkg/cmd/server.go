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
/*
This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com
*/
package cmd

import (
	"context"

	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/reposerver-service/pkg/reposerver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func RunServer() {
	_ = logger.Wrap(context.Background(), func(ctx context.Context) error {
		err := Run(ctx)
		if err != nil {
			logger.FromContext(ctx).Sugar().Errorf("error in startup: %v %#v", err, err)
		}
		return nil
	})
}

func Run(ctx context.Context) error {
	log := logger.FromContext(ctx).Sugar()

	logger.FromContext(ctx).Info("Startup")

	dbLocation, err := valid.ReadEnvVar("KUBERPULT_DB_LOCATION")
	if err != nil {
		return err
	}
	dbName, err := valid.ReadEnvVar("KUBERPULT_DB_NAME")
	if err != nil {
		return err
	}
	dbUserName, err := valid.ReadEnvVar("KUBERPULT_DB_USER_NAME")
	if err != nil {
		return err
	}
	dbPassword, err := valid.ReadEnvVar("KUBERPULT_DB_USER_PASSWORD")
	if err != nil {
		return err
	}
	dbAuthProxyPort, err := valid.ReadEnvVar("KUBERPULT_DB_AUTH_PROXY_PORT")
	if err != nil {
		return err
	}
	dbMaxOpen, err := valid.ReadEnvVarUInt("KUBERPULT_DB_MAX_OPEN_CONNECTIONS")
	if err != nil {
		return err
	}
	dbMaxIdle, err := valid.ReadEnvVarUInt("KUBERPULT_DB_MAX_IDLE_CONNECTIONS")
	if err != nil {
		return err
	}
	sslMode, err := valid.ReadEnvVar("KUBERPULT_DB_SSL_MODE")
	if err != nil {
		return err
	}
	enableTracesString, err := valid.ReadEnvVar("KUBERPULT_ENABLE_TRACING")
	if err != nil {
		log.Info("datadog traces are disabled")
	}
	enableTraces := enableTracesString == "true"
	if enableTraces {
		tracer.Start()
		defer tracer.Stop()
	}
	kuberpultVersionRaw, err := valid.ReadEnvVar("KUBERPULT_VERSION")
	if err != nil {
		return err
	}
	logger.FromContext(ctx).Info("startup", zap.String("kuberpultVersion", kuberpultVersionRaw))

	dbCfg := db.DBConfig{
		DbHost:         dbLocation,
		DbPort:         dbAuthProxyPort,
		DriverName:     "postgres",
		DbName:         dbName,
		DbPassword:     dbPassword,
		DbUser:         dbUserName,
		MigrationsPath: "",
		WriteEslOnly:   false,
		SSLMode:        sslMode,

		MaxIdleConnections: dbMaxIdle,
		MaxOpenConnections: dbMaxOpen,
	}

	dbHandler, err := db.Connect(ctx, dbCfg)
	if err != nil {
		return err
	}

	// Shutdown channel is used to terminate server side streams.
	shutdownCh := make(chan struct{})
	setup.Run(ctx, setup.ServerConfig{
		HTTP: []setup.HTTPConfig{
			{
				BasicAuth: nil,
				Shutdown:  nil,
				Port:      "8080",
				Register:  nil,
			},
		},
		GRPC: &setup.GRPCConfig{
			Shutdown: nil,
			Port:     "8443",
			Opts:     []grpc.ServerOption{},
			Register: func(srv *grpc.Server) {
				reposerver.Register(srv, dbHandler)
				reflection.Register(srv)
			},
		},
		Background: []setup.BackgroundTaskConfig{},
		Shutdown: func(ctx context.Context) error {
			close(shutdownCh)
			return nil
		},
	})

	return nil
}
