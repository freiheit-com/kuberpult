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
	"net/http"
	"sync"
	"time"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/interceptors"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/service"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Config struct {
	// these will be mapped to "KUBERPULT_GIT_URL", etc.
	GitUrl            string        `required:"true" split_words:"true"`
	GitBranch         string        `default:"master" split_words:"true"`
	BootstrapMode     bool          `default:"false" split_words:"true"`
	GitCommitterEmail string        `default:"kuberpult@freiheit.com" split_words:"true"`
	GitCommitterName  string        `default:"kuberpult" split_words:"true"`
	GitSshKey         string        `default:"/etc/ssh/identity" split_words:"true"`
	GitSshKnownHosts  string        `default:"/etc/ssh/ssh_known_hosts" split_words:"true"`
	GitNetworkTimeout time.Duration `default:"1m" split_words:"true"`
	PgpKeyRing        string        `split_words:"true"`
	AzureEnableAuth   bool          `default:"false" split_words:"true"`
	DexEnabled        bool          `default:"false" split_words:"true"`
	DexRbacPolicy     string        `split_words:"true"`
	EnableTracing     bool          `default:"false" split_words:"true"`
	EnableMetrics     bool          `default:"false" split_words:"true"`
	DogstatsdAddr     string        `default:"127.0.0.1:8125" split_words:"true"`
	EnableSqlite      bool          `default:"true" split_words:"true"`
	DexMock           bool          `default:"false" split_words:"true"`
	DexMockRole       string        `default:"Developer" split_words:"true"`
	ArgoCdServer      string        `default:"" split_words:"true"`
	ArgoCdInsecure    bool          `default:"false" split_words:"true"`
	GitWebUrl         string        `default:"" split_words:"true"`
}

func (c *Config) storageBackend() repository.StorageBackend {
	if c.EnableSqlite {
		return repository.SqliteBackend
	} else {
		return repository.GitBackend
	}
}

func RunServer() {
	logger.Wrap(context.Background(), func(ctx context.Context) error {

		var c Config

		var wg sync.WaitGroup

		err := envconfig.Process("kuberpult", &c)
		if err != nil {
			logger.FromContext(ctx).Fatal("config.parse.error", zap.Error(err))
		}

		var reader auth.GrpcContextReader
		if c.DexMock {
			if !c.DexEnabled {
				logger.FromContext(ctx).Fatal("dexEnabled must be true if dexMock is true")
			}
			//if c.DexMockRole = nil {
			//	logger.FromContext(ctx).Fatal("dexMockRole must be set to a role (e.g 'DEVELOPER'")
			//}
			reader = &auth.DummyGrpcContextReader{Role: c.DexMockRole}
		} else {
			reader = &auth.DexGrpcContextReader{DexEnabled: c.DexEnabled}
		}
		dexRbacPolicy, err := auth.ReadRbacPolicy(c.DexEnabled)
		if err != nil {
			logger.FromContext(ctx).Fatal("dex.read.error", zap.Error(err))
		}

		grpcServerLogger := logger.FromContext(ctx).Named("grpc_server")
		httpServerLogger := logger.FromContext(ctx).Named("http_server")

		// Unary interceptor. Only parses the Role information if Dex is enabled.
		unaryUserContextInterceptor := func(ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler) (interface{}, error) {
			return interceptors.UnaryUserContextInterceptor(ctx, req, info, handler, reader)
		}

		ctx, cancel, err := tracing.WithTracerProvider(ctx, "kuberpult-cd-service")
		defer cancel()

		grpcStreamInterceptors := []grpc.StreamServerInterceptor{
			grpc_zap.StreamServerInterceptor(grpcServerLogger),
			tracing.OTELStreamServerInterceptor(ctx),
		}
		grpcUnaryInterceptors := []grpc.UnaryServerInterceptor{
			grpc_zap.UnaryServerInterceptor(grpcServerLogger),
			tracing.OTELUnaryServerInterceptor(ctx),
			unaryUserContextInterceptor,
		}

		if c.EnableMetrics {
			ddMetrics, err := statsd.New(c.DogstatsdAddr, statsd.WithNamespace("Kuberpult"))
			if err != nil {
				logger.FromContext(ctx).Fatal("datadog.metrics.error", zap.Error(err))
			}
			ctx = context.WithValue(ctx, "ddMetrics", ddMetrics)
		}

		// If the tracer is not started, calling this function is a no-op.
		ctx, span := tracing.Start(ctx, "Start server")

		repo, err := repository.New(ctx, repository.RepositoryConfig{
			URL:            c.GitUrl,
			Path:           "./repository",
			CommitterEmail: c.GitCommitterEmail,
			CommitterName:  c.GitCommitterName,
			Credentials: repository.Credentials{
				SshKey: c.GitSshKey,
			},
			Certificates: repository.Certificates{
				KnownHostsFile: c.GitSshKnownHosts,
			},
			Branch:                 c.GitBranch,
			GcFrequency:            20,
			BootstrapMode:          c.BootstrapMode,
			EnvironmentConfigsPath: "./environment_configs.json",
			StorageBackend:         c.storageBackend(),
			ArgoInsecure:           c.ArgoCdInsecure,
			ArgoWebhookUrl:         c.ArgoCdServer,
			WebURL:                 c.GitWebUrl,
			NetworkTimeout:         c.GitNetworkTimeout,
		})
		if err != nil {
			logger.FromContext(ctx).Fatal("repository.new.error", zap.Error(err), zap.String("git.url", c.GitUrl), zap.String("git.branch", c.GitBranch))
			span.RecordError(err)
		}
		span.End()

		repositoryService := &service.Service{
			Repository: repo,
		}

		wg.Add(1)
		go repository.RegularlySendDatadogMetrics(repo, 300, repository.GetRepositoryStateAndUpdateMetrics)

		// Shutdown channel is used to terminate server side streams.
		shutdownCh := make(chan struct{})
		setup.Run(ctx, setup.ServerConfig{
			HTTP: []setup.HTTPConfig{
				{
					Port: "8080",
					Register: func(mux *http.ServeMux) {
						handler := logger.WithHttpLogger(httpServerLogger, repositoryService)
						handler = tracing.WrapHttp(ctx, handler)
						mux.Handle("/", handler)
					},
				},
			},
			GRPC: &setup.GRPCConfig{
				Port: "8443",
				Opts: []grpc.ServerOption{
					grpc.ChainStreamInterceptor(grpcStreamInterceptors...),
					grpc.ChainUnaryInterceptor(grpcUnaryInterceptors...),
				},
				Register: func(srv *grpc.Server) {
					api.RegisterBatchServiceServer(srv, &service.BatchServer{
						Repository: repo,
						RBACConfig: auth.RBACConfig{
							DexEnabled: c.DexEnabled,
							Policy:     dexRbacPolicy,
						},
					})

					overviewSrv := &service.OverviewServiceServer{
						Repository: repo,
						Shutdown:   shutdownCh,
					}
					api.RegisterOverviewServiceServer(srv, overviewSrv)
					reflection.Register(srv)
				},
			},
			Shutdown: func(ctx context.Context) error {
				close(shutdownCh)
				wg.Done()
				return nil
			},
		})

		return nil
	})
}
