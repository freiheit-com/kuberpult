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
	"fmt"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/cloudrun-service/pkg/cloudrun"
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
		logger.FromContext(ctx).Fatal("Failed to initialize cloud run service")
	}

	setup.Run(ctx, setup.ServerConfig{
		HTTP: []setup.HTTPConfig{},
		GRPC: &setup.GRPCConfig{
			Shutdown: nil,
			Port:     "8443",
			Opts:     nil,
			Register: func(srv *grpc.Server) {
				api.RegisterCloudRunServiceServer(srv, &cloudrun.CloudRunService{})
			},
		},
		Background: []setup.BackgroundTaskConfig{},
		Shutdown:   nil,
	})

	return nil
}
