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
	"net/url"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	argoio "github.com/argoproj/argo-cd/v2/util/io"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	pkgmetrics "github.com/freiheit-com/kuberpult/pkg/metrics"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/metrics"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/notifier"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
)

type Config struct {
	CdServer      string `default:"kuberpult-cd-service:8443"`
	EnableTracing bool   `default:"false" split_words:"true"`

	ArgocdServer             string `split_words:"true"`
	ArgocdInsecure           bool   `default:"false" split_words:"true"`
	ArgocdToken              string `split_words:"true"`
	ArgocdRefreshEnabled     bool   `split_words:"true"`
	ArgocdRefreshConcurrency int    `default:"50" split_words:"true"`
}

func (config *Config) ClientConfig() (apiclient.ClientOptions, error) {
	var opts apiclient.ClientOptions
	opts.ConfigPath = ""
	u, err := url.ParseRequestURI(config.ArgocdServer)
	if err != nil {
		return opts, fmt.Errorf("invalid argocd server url: %w", err)
	}
	opts.ServerAddr = u.Host
	opts.PlainText = u.Scheme == "http"
	opts.UserAgent = "kuberpult"
	opts.Insecure = config.ArgocdInsecure
	opts.AuthToken = config.ArgocdToken
	return opts, nil
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
				grpctrace.StreamClientInterceptor(grpctrace.WithServiceName(tracing.ServiceName("kuberpult-rollout-service"))),
			),
			grpc.WithUnaryInterceptor(
				grpctrace.UnaryClientInterceptor(grpctrace.WithServiceName(tracing.ServiceName("kuberpult-rollout-service"))),
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
	grpcServerLogger := logger.FromContext(ctx).Named("grpc_server")
	grpcStreamInterceptors := []grpc.StreamServerInterceptor{
		grpc_zap.StreamServerInterceptor(grpcServerLogger),
	}
	grpcUnaryInterceptors := []grpc.UnaryServerInterceptor{
		grpc_zap.UnaryServerInterceptor(grpcServerLogger),
	}

	if config.EnableTracing {
		tracer.Start()
		defer tracer.Stop()
		grpcStreamInterceptors = append(grpcStreamInterceptors,
			grpctrace.StreamServerInterceptor(grpctrace.WithServiceName("rollout-service")),
		)
		grpcUnaryInterceptors = append(grpcUnaryInterceptors,
			grpctrace.UnaryServerInterceptor(grpctrace.WithServiceName("rollout-service")),
		)
	}

	opts, err := config.ClientConfig()
	if err != nil {
		return err
	}

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
	versionC := versions.New(overview)

	backgroundTasks := []setup.BackgroundTaskConfig{
		{
			Name: "consume argocd events",
			Run: func(ctx context.Context) error {
				return service.ConsumeEvents(ctx, appClient, versionC, broadcast, func() { ready = true })
			},
		},
		{
			Name: "consume kuberpult events",
			Run: func(ctx context.Context) error {
				return versionC.ConsumeEvents(ctx, broadcast)
			},
		},
	}

	if config.ArgocdRefreshEnabled {

		backgroundTasks = append(backgroundTasks, setup.BackgroundTaskConfig{
			Name: "refresh argocd",
			Run: func(ctx context.Context) error {
				notify := notifier.New(appClient, config.ArgocdRefreshConcurrency)
				return notifier.Subscribe(ctx, notify, broadcast)
			},
		})
	}

	meter, handler, err := pkgmetrics.Init()
	if err != nil {
		return err
	}
	backgroundTasks = append(backgroundTasks, setup.BackgroundTaskConfig{
		Name: "create metrics",
		Run: func(ctx context.Context) error {
			return metrics.Metrics(ctx, broadcast, meter, nil)
		},
	})

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
					mux.Handle("/metrics", handler)
				},
			},
		},
		Background: backgroundTasks,
		GRPC: &setup.GRPCConfig{
			Port: "8443",
			Opts: []grpc.ServerOption{
				grpc.ChainStreamInterceptor(grpcStreamInterceptors...),
				grpc.ChainUnaryInterceptor(grpcUnaryInterceptors...),
			},
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
