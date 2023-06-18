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
	"net/http"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	argoio "github.com/argoproj/argo-cd/v2/util/io"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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
		fmt.Printf("error: %v %#v", err, err)
	}
}

func getGrpcClient(ctx context.Context, config Config) (api.OverviewServiceClient, error) {
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

	return api.NewOverviewServiceClient(con), nil
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

	logger.FromContext(ctx).Info("argocd.connecting", zap.String("argocd.addr", opts.ServerAddr))
	client, err := apiclient.NewClient(&opts)
	if err != nil {
		return fmt.Errorf("connecting to argocd %s: %w", opts.ServerAddr, err)
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

	overview, err := getGrpcClient(ctx, config)
	if err != nil {
		return fmt.Errorf("connecting to cd service %q: %w", config.CdServer, err)
	}
	broadcast := service.New()
	shutdownCh := make(chan struct{})
	ready := false
	setup.Run(ctx, setup.ServerConfig{
		HTTP: []setup.HTTPConfig{
			{
				Port: "8080",
				Register: func(mux *http.ServeMux) {
					mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if ready {
							w.WriteHeader(200)
						} else {
							w.WriteHeader(500)
						}
					}))
				},
			},
		},
		Background: []setup.BackgroundTaskConfig{
			{
				Name: "consume events",
				Run: func(ctx context.Context) error {
					return service.ConsumeEvents(ctx, appClient, versions.New(overview), broadcast, func() { ready = true })
				},
			}},
		GRPC: &setup.GRPCConfig{
			Port: "8443",
			/*			Opts: []grpc.ServerOption{
						grpc.ChainStreamInterceptor(grpcStreamInterceptors...),
						grpc.ChainUnaryInterceptor(grpcUnaryInterceptors...),
					},*/
			Register: func(srv *grpc.Server) {
				api.RegisterRolloutServiceServer(srv, broadcast)
				reflection.Register(srv)
			},
		},
		Shutdown: func(ctx context.Context) error {
			close(shutdownCh)
			return nil
		},
	})
	return nil
}
