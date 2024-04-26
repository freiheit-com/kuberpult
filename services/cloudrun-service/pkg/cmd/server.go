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
	"os"

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
	projectId, exists := os.LookupEnv("GCP_PROJECT_ID")
	if !exists {
		logger.FromContext(ctx).Fatal("environment variable GCP_PROJECT_ID is missing")
	}
	imageTag, exists := os.LookupEnv("IMAGE_TAG")
	if !exists {
		logger.FromContext(ctx).Fatal("environment variable IMAGE_TAG is missing")
	}
	servicePort := int64(3001)
	svc := &run.Service{
		ApiVersion: "serving.knative.dev/v1",
		Kind:       "Service",
		Metadata: &run.ObjectMeta{
			Name:      "test-service1",
			Namespace: projectId,
			Labels:    map[string]string{"cloud.googleapis.com/location": "europe-west1"},
		},
		Spec: &run.ServiceSpec{
			Template: &run.RevisionTemplate{
				Spec: &run.RevisionSpec{
					Containers: []*run.Container{
						{
							Name:  "test-service",
							Image: imageTag,
							Ports: []*run.ContainerPort{
								{
									ContainerPort: servicePort,
									Name:          "h2c",
								},
							},
							Env: []*run.EnvVar{
								{
									Name:  "SERVERLESS_ECHO_GRPC_PORT",
									Value: "3002",
								},
								{
									Name:  "SERVERLESS_ECHO_HEALTH_PORT",
									Value: "8080",
								},
							},
							StartupProbe: &run.Probe{
								TcpSocket: &run.TCPSocketAction{
									Port: servicePort,
								},
								TimeoutSeconds: 10,
							},
						},
					},
				},
			},
		},
	}

	if err := cloudrun.Init(ctx); err != nil {
		logger.FromContext(ctx).Fatal("Failed to initialize cloud run service")
	}
	if err := cloudrun.Deploy(ctx, svc); err != nil {
		logger.FromContext(ctx).Error("Service deploy failed", zap.String("Error", err.Error()))
	} else {
		logger.FromContext(ctx).Info("Service deployed successfully", zap.String("Service", svc.Metadata.Name))
	}

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
