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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/DataDog/datadog-go/v5/statsd"
	"net/url"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	pkgmetrics "github.com/freiheit-com/kuberpult/pkg/metrics"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/metrics"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/notifier"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/revolution"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/service"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/versions"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
)

type Config struct {
	CdServer       string `default:"kuberpult-cd-service:8443"`
	CdServerSecure bool   `default:"false" split_words:"true"`
	VersionServer  string `default:"kuberpult-manifest-repo-export-service:8443"`
	EnableTracing  bool   `default:"false" split_words:"true"`
	EnableMetrics  bool   `default:"false" split_words:"true"`
	DogstatsdAddr  string `default:"127.0.0.1:8125" split_words:"true"`

	GrpcMaxRecvMsgSize int `default:"4" split_words:"true"`

	ArgocdServer                      string `split_words:"true"`
	ArgocdInsecure                    bool   `default:"false" split_words:"true"`
	ArgocdToken                       string `split_words:"true"`
	ArgocdRefreshEnabled              bool   `split_words:"true"`
	ArgocdRefreshConcurrency          int    `default:"50" split_words:"true"`
	ArgocdRefreshClientTimeoutSeconds int    `default:"30" split_words:"true"`

	RevolutionDoraEnabled     bool          `split_words:"true"`
	RevolutionDoraUrl         string        `split_words:"true" default:""`
	RevolutionDoraToken       string        `split_words:"true" default:""`
	RevolutionDoraConcurrency int           `default:"10" split_words:"true"`
	RevolutionDoraMaxEventAge time.Duration `default:"0" split_words:"true"`

	ManageArgoApplicationsEnabled bool     `split_words:"true" default:"true"`
	ManageArgoApplicationsFilter  []string `split_words:"true" default:"sreteam"`

	ManifestRepoUrl string `default:"" split_words:"true"`
	Branch          string `default:"" split_words:"true"`
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

func (config *Config) RevolutionConfig() (revolution.Config, error) {
	if config.RevolutionDoraUrl == "" {
		return revolution.Config{}, fmt.Errorf("KUBERPULT_REVOLUTION_DORA_URL must be a valid url")
	}
	if config.RevolutionDoraToken == "" {
		return revolution.Config{}, fmt.Errorf("KUBERPULT_REVOLUTION_DORA_TOKEN must not be empty")
	}
	return revolution.Config{
		URL:         config.RevolutionDoraUrl,
		Token:       []byte(config.RevolutionDoraToken),
		Concurrency: config.RevolutionDoraConcurrency,
		MaxEventAge: config.RevolutionDoraMaxEventAge,
	}, nil
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
		fmt.Printf("error: %v %#v\n", err, err)
	}
}

func getGrpcClients(ctx context.Context, config Config) (api.OverviewServiceClient, api.VersionServiceClient, error) {
	const megaBytes int = 1024 * 1024
	var cred credentials.TransportCredentials = insecure.NewCredentials()
	if config.CdServerSecure {
		systemRoots, err := x509.SystemCertPool()
		if err != nil {
			msg := "failed to read CA certificates"
			return nil, nil, fmt.Errorf(msg)
		}
		//exhaustruct:ignore
		cred = credentials.NewTLS(&tls.Config{
			RootCAs: systemRoots,
		})
	}

	grpcClientOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(cred),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(config.GrpcMaxRecvMsgSize * megaBytes)),
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
		return nil, nil, fmt.Errorf("error dialing %s: %w", config.CdServer, err)
	}

	versionServiceCon, err := grpc.Dial(config.VersionServer, grpcClientOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing %s: %w", config.VersionServer, err)
	}

	return api.NewOverviewServiceClient(con), api.NewVersionServiceClient(versionServiceCon), nil
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

	//Datadog metrics
	var ddMetrics statsd.ClientInterface
	var err error
	if config.EnableMetrics {
		ddMetrics, err = statsd.New(config.DogstatsdAddr, statsd.WithNamespace("Kuberpult"))
		if err != nil {
			logger.FromContext(ctx).Fatal("datadog.metrics.error", zap.Error(err))
		}
		ctx = context.WithValue(ctx, "ddMetrics", ddMetrics)

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

	overviewGrpc, versionGrpc, err := getGrpcClients(ctx, config)
	if err != nil {
		return fmt.Errorf("connecting to cd service %q: %w", config.CdServer, err)
	}
	broadcast := service.New()
	shutdownCh := make(chan struct{})
	versionC := versions.New(overviewGrpc, versionGrpc, appClient, config.ManageArgoApplicationsEnabled, config.ManageArgoApplicationsFilter)
	dispatcher := service.NewDispatcher(broadcast, versionC)
	backgroundTasks := []setup.BackgroundTaskConfig{
		{
			Shutdown: nil,
			Name:     "consume argocd events",
			Run: func(ctx context.Context, health *setup.HealthReporter) error {
				return service.ConsumeEvents(ctx, appClient, dispatcher, health, versionC.GetArgoProcessor(), ddMetrics)
			},
		},
		{
			Shutdown: nil,
			Name:     "consume kuberpult events",
			Run: func(ctx context.Context, health *setup.HealthReporter) error {
				return versionC.ConsumeEvents(ctx, broadcast, health)
			},
		},
		{
			Shutdown: nil,
			Name:     "consume self-manage events",
			Run: func(ctx context.Context, health *setup.HealthReporter) error {
				return versionC.GetArgoProcessor().Consume(ctx, health)
			},
		},
		{
			Shutdown: nil,
			Name:     "dispatch argocd events",
			Run:      dispatcher.Work,
		},
	}

	if config.ArgocdRefreshEnabled {

		backgroundTasks = append(backgroundTasks, setup.BackgroundTaskConfig{
			Shutdown: nil,
			Name:     "refresh argocd",
			Run: func(ctx context.Context, health *setup.HealthReporter) error {
				notify := notifier.New(appClient, config.ArgocdRefreshConcurrency, config.ArgocdRefreshClientTimeoutSeconds)
				return notifier.Subscribe(ctx, notify, broadcast, health)
			},
		})
	}

	if config.RevolutionDoraEnabled {
		revolutionConfig, err := config.RevolutionConfig()
		if err != nil {
			return err
		}
		revolutionDora := revolution.New(revolutionConfig)
		backgroundTasks = append(backgroundTasks, setup.BackgroundTaskConfig{
			Shutdown: nil,
			Name:     "revolution dora",
			Run: func(ctx context.Context, health *setup.HealthReporter) error {
				health.ReportReady("pushing")
				return revolutionDora.Subscribe(ctx, broadcast)
			},
		})
	}

	backgroundTasks = append(backgroundTasks, setup.BackgroundTaskConfig{
		Shutdown: nil,
		Name:     "create metrics",
		Run: func(ctx context.Context, health *setup.HealthReporter) error {
			health.ReportReady("reporting")
			return metrics.Metrics(ctx, broadcast, pkgmetrics.FromContext(ctx), nil, func() {})
		},
	})

	setup.Run(ctx, setup.ServerConfig{
		HTTP: []setup.HTTPConfig{
			{
				Register:  nil,
				BasicAuth: nil,
				Shutdown:  nil,
				Port:      "8080",
			},
		},
		Background: backgroundTasks,
		GRPC: &setup.GRPCConfig{
			Shutdown: nil,
			Port:     "8443",
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
