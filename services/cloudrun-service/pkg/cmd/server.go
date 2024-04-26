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
	"github.com/freiheit-com/kuberpult/services/cloudrun-service/pkg/cloudrun"
	"go.uber.org/zap"
	"google.golang.org/api/run/v1"
)

func RunServer() {
	err := logger.Wrap(context.Background(), runServer)
	if err != nil {
		fmt.Printf("error: %v %#v", err, err)
	}
}

func runServer(ctx context.Context) error {
	svc := &run.Service{
		ApiVersion: "serving.knative.dev/v1",
		Kind:       "Service",
		Metadata: &run.ObjectMeta{
			Name:      "test-service2",
			Namespace: "855333057980",
			Labels:    map[string]string{"cloud.googleapis.com/location": "europe-west1"},
		},
		Spec: &run.ServiceSpec{
			Template: &run.RevisionTemplate{
				Spec: &run.RevisionSpec{
					Containers: []*run.Container{
						{
							Name:  "test-service",
							Image: "eu.gcr.io/fdc-standard-setup-dev-env/services/serverless-app-example:rev637cd5fc5a8a99ba555b61fc9e3679715de89ebe",
							Ports: []*run.ContainerPort{
								{
									ContainerPort: 3000,
									Name:          "h2c",
								},
							},
							Env: []*run.EnvVar{
								{
									Name:  "SERVERLESS_ECHO_GRPC_PORT",
									Value: "3000",
								},
								{
									Name:  "SERVERLESS_ECHO_HEALTH_PORT",
									Value: "8080",
								},
							},
							StartupProbe: &run.Probe{
								TcpSocket: &run.TCPSocketAction{
									Port: 3000,
								},
								TimeoutSeconds: 10,
							},
						},
					},
				},
			},
		},
	}
	// req := runService.Projects.Locations.Services.List("projects/855333057980/locations/europe-west1")
	// it, err := req.Do()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// for _, service := range it.Items {
	// 	for _, container := range service.Spec.Template.Spec.Containers {
	// 		fmt.Printf("%s:\t%s\n", service.Metadata.Name, container.Image)
	// 	}
	// }
	if err := cloudrun.Init(ctx); err != nil {
		logger.FromContext(ctx).Fatal("Failed to initialize cloud run service")
	}
	if err := cloudrun.Deploy(ctx, svc); err != nil {
		logger.FromContext(ctx).Error("Service deploy failed", zap.String("Error", err.Error()))
	}
	// if err := cloudrun.Deploy(ctx, svc); err != nil {
	// 	logger.FromContext(ctx).Error("Service deploy failed", zap.String("Error", err.Error()))
	// }

	// setup.Run(ctx, setup.ServerConfig{
	// 	HTTP: nil,
	// 	GRPC: &setup.GRPCConfig{
	// 		Shutdown: nil,
	// 		Port:     "8443",
	// 		Opts:     nil,
	// 		Register: func(srv *grpc.Server) {
	// 			// Just a placeholder for now
	// 		},
	// 	},
	// 	Background: nil,
	// 	Shutdown:   nil,
	// })
	return nil
}
