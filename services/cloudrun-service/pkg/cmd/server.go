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

Copyright 2023 freiheit.com*/

package cmd

import (
	"context"
	"fmt"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"google.golang.org/grpc"
)

func RunServer() {
	err := logger.Wrap(context.Background(), runServer)
	if err != nil {
		fmt.Printf("error: %v %#v", err, err)
	}
}

func runServer(ctx context.Context) error {
	setup.Run(ctx, setup.ServerConfig{
		HTTP: nil,
		GRPC: &setup.GRPCConfig{
			Shutdown: nil,
			Port:     "8443",
			Opts:     nil,
			Register: func(srv *grpc.Server) {},
		},
		Background: nil,
		Shutdown:   nil,
	})
	return nil
}
