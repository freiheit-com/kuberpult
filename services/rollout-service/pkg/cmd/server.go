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

	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/services/frontend-service/api"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
)

type Config struct {
	CdServer      string `default:"kuberpult-cd-service:8443"`
	EnableTracing bool   `default:"false" split_words:"true"`

	ArgocdServer   string `split_words:"true"`
	ArgocdInsecure bool   `default:"false" split_words:"true"`
	ArgocdToken    string `split_words:"true"`
}

func RunServer() {
	var config Config
	err := logger.Wrap(context.Background(), func(ctx context.Context) error {
		err := envconfig.Process("kuberpult", &config)
		if err != nil {
			logger.FromContext(ctx).Fatal("config.parse", zap.Error(err))
		}
		return runServer(ctx, config)
	})
	if err != nil {
		panic(err)
	}
}

func getGrpcClient(ctx context.Context, config Config ) (api.DeployedVersionServiceClient, error) {
	grpcClientOpts := []grpc.DialOption{
		grpc.WithInsecure(),
	}
	if config.EnableTracing {
		grpcClientOpts = append(grpcClientOpts,
			grpc.WithStreamInterceptor(
				grpctrace.StreamClientInterceptor(grpctrace.WithServiceName("rollout-service")),
			),
			grpc.WithUnaryInterceptor(
				grpctrace.UnaryClientInterceptor(grpctrace.WithServiceName("rollout-service")),
			),
		)
	}

	con, err := grpc.Dial(config.CdServer, grpcClientOpts...)
	if err != nil {
	    return nil, fmt.Errorf("error dialing %s: %w", config.CdServer, err)
	}
		
	return api.NewDeployedVersionServiceClient(con), nil
}

func runServer(ctx context.Context, config Config) error {
	if config.EnableTracing {
		tracer.Start()
		defer tracer.Stop()
	}

	var opts apiclient.ClientOptions
	opts.ServerAddr = config.ArgocdServer
	opts.UserAgent = "kuberpult"
	opts.Insecure = config.ArgocdInsecure
	opts.AuthToken = config.ArgocdToken

	client, err := apiclient.NewClient(&opts)
	if err != nil {
		return fmt.Errorf("connecting to argocd(%s): %w", opts.ServerAddr, err)
	}
	closer, versionClient, err := client.NewVersionClient()
	if err != nil {
		return fmt.Errorf("connecting to argocd version: %w", err)
	}
	defer argoio.Close(closer)
	version, err := versionClient.Version(ctx, &emptypb.Empty{})
	if err != nil {
		return fmt.Errorf("retrieving argocd version: %w", err)
	}
	logger.FromContext(ctx).Info("argocd.connected", zap.String("argocd.version", version.Version))
	closer, appClient, err := client.NewApplicationClient()
	if err != nil {
		return fmt.Errorf("connecting to argocd app: %w", err)
	}
	defer argoio.Close(closer)
	return consumeEvents(ctx, appClient)
}

func consumeEvents(ctx context.Context, appClient application.ApplicationServiceClient) error {
	for {
		watch, err := appClient.Watch(ctx, &application.ApplicationQuery{})
		if err != nil {
			return err
		}
		for {
			ev, err := watch.Recv()
			if err != nil {
				logger.FromContext(ctx).Warn("argocd.application.recv", zap.Error(err))
				break
			}

			fmt.Printf("%s %s %s %s %s %#v\n", ev.Type, ev.Application.Name,ev.Application.Spec.Project, ev.Application.Status.Sync.Status, ev.Application.Status.Sync.Revision, ev.Application.Status.Health.Status)
		}
	}
}

type argoEvent struct {
  Application string
  SyncStatusCode v1alpha1.SyncStatusCode
  Revision string
  HealthStatusCode health.HealthStatusCode
}


type argoEventProcessor struct {

}

func (ep *argoEventProcessor) Process(ev *argoEvent) {

}
