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
	servicePort := int64(3000)
	//exhaustruct:ignore
	metaData := &run.ObjectMeta{
		Name:      "test-service1",
		Namespace: projectId,
		Labels:    map[string]string{"cloud.googleapis.com/location": "europe-west1"},
	}
	//exhaustruct:ignore
	ports := &run.ContainerPort{
		ContainerPort: servicePort,
		Name:          "h2c",
	}
	//exhaustruct:ignore
	envVars := []*run.EnvVar{
		{
			Name:  "SERVERLESS_ECHO_GRPC_PORT",
			Value: fmt.Sprint(servicePort),
		},
		{
			Name:  "SERVERLESS_ECHO_HEALTH_PORT",
			Value: "8080",
		},
	}
	//exhaustruct:ignore
	tcpSocket := &run.TCPSocketAction{Port: servicePort}
	//exhaustruct:ignore
	startupProbe := &run.Probe{
		TcpSocket:      tcpSocket,
		TimeoutSeconds: 10,
	}
	//exhaustruct:ignore
	container := &run.Container{
		Name:         "test-service",
		Image:        imageTag,
		Ports:        []*run.ContainerPort{ports},
		Env:          envVars,
		StartupProbe: startupProbe,
	}
	containers := []*run.Container{container}
	//exhaustruct:ignore
	spec := &run.RevisionSpec{
		Containers: containers,
	}
	//exhaustruct:ignore
	revTemplate := &run.RevisionTemplate{
		Spec: spec,
	}
	//exhaustruct:ignore
	serviceSpec := &run.ServiceSpec{
		Template: revTemplate,
	}
	//exhaustruct:ignore
	service := &run.Service{
		ApiVersion: "serving.knative.dev/v1",
		Kind:       "Service",
		Metadata:   metaData,
		Spec:       serviceSpec,
	}

	if err := cloudrun.Init(ctx); err != nil {
		logger.FromContext(ctx).Fatal("Failed to initialize cloud run service")
	}
	if err := cloudrun.Deploy(ctx, service); err != nil {
		logger.FromContext(ctx).Error("Service deploy failed", zap.String("Error", err.Error()))
	} else {
		logger.FromContext(ctx).Info("Service deployed successfully", zap.String("Service", service.Metadata.Name))
	}
	return nil
}
