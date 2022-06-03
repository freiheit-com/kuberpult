/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package cmd

import (
	"context"
	"net/http"
	"os"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/service"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"golang.org/x/crypto/openpgp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type Config struct {
	// these will be mapped to "KUBERPULT_GIT_URL", etc.
	GitUrl            string `required:"true" split_words:"true"`
	GitBranch         string `default:"master" split_words:"true"`
	GitCommitterEmail string `default:"kuberpult@freiheit.com" split_words:"true"`
	GitCommitterName  string `default:"kuberpult" split_words:"true"`
	GitSshKey         string `default:"/etc/ssh/identity" split_words:"true"`
	GitSshKnownHosts  string `default:"/etc/ssh/ssh_known_hosts" split_words:"true"`
	PgpKeyRing        string `split_words:"true"`
	ArgoCdHost        string `default:"localhost:8080" split_words:"true"`
	ArgoCdUser        string `default:"admin" split_words:"true"`
	ArgoCdPass        string `default:"" split_words:"true"`
	EnableTracing     bool   `default:"false" split_words:"true"`
	EnableMetrics     bool   `default:"false" split_words:"true"`
	DogstatsdAddr     string `default:"127.0.0.1:8125" split_words:"true"`
}

func (c *Config) readPgpKeyRing() (openpgp.KeyRing, error) {
	if c.PgpKeyRing == "" {
		return nil, nil
	}
	file, err := os.Open(c.PgpKeyRing)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return openpgp.ReadArmoredKeyRing(file)
}

func RunServer() {
	logger.Wrap(context.Background(), func(ctx context.Context) error {

		var c Config

		err := envconfig.Process("kuberpult", &c)
		if err != nil {
			logger.FromContext(ctx).Fatal("config.parse.error", zap.Error(err))
		}

		pgpKeyRing, err := c.readPgpKeyRing()
		if err != nil {
			logger.FromContext(ctx).Fatal("pgp.read.error", zap.Error(err))
		}

		if c.ArgoCdPass != "" {
			_, err := service.ArgocdLogin(c.ArgoCdHost, c.ArgoCdUser, c.ArgoCdPass)
			if err != nil {
				logger.FromContext(ctx).Fatal("argocd.login.error", zap.Error(err))
			}
		}

		if c.EnableTracing {
			tracer.Start()
			defer tracer.Stop()
		}

		if c.EnableMetrics {
			ddMetrics, err := statsd.New(c.DogstatsdAddr, statsd.WithNamespace("Kuberpult"))
			if err != nil {
				logger.FromContext(ctx).Fatal("datadog.metrics.error", zap.Error(err))
			}
			ctx = context.WithValue(ctx, "ddMetrics", ddMetrics)
		}

		// If the tracer is not started, calling this function is a no-op.
		span, ctx := tracer.StartSpanFromContext(ctx, "Start server")

		repo, err := repository.New(ctx, repository.Config{
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
			Branch:      c.GitBranch,
			GcFrequency: 20,
		})
		if err != nil {
			logger.FromContext(ctx).Fatal("repository.new.error", zap.Error(err), zap.String("git.url", c.GitUrl), zap.String("git.branch", c.GitBranch))
		}

		repositoryService := &service.Service{
			Repository: repo,
			KeyRing:    pgpKeyRing,
			ArgoCdHost: c.ArgoCdHost,
			ArgoCdUser: c.ArgoCdUser,
			ArgoCdPass: c.ArgoCdPass,
		}

		grpcServerLogger := logger.FromContext(ctx).Named("grpc_server")
		httpServerLogger := logger.FromContext(ctx).Named("http_server")

		span.Finish()

		// Shutdown channel is used to terminate server side streams.
		shutdownCh := make(chan struct{})
		setup.Run(ctx, setup.Config{
			HTTP: []setup.HTTPConfig{
				{
					Port: "8080",
					Register: func(mux *http.ServeMux) {
						handler := logger.WithHttpLogger(httpServerLogger, repositoryService)
						mux.Handle("/", handler)
					},
				},
			},
			GRPC: &setup.GRPCConfig{
				Port: "8443",
				Opts: []grpc.ServerOption{
					grpc.StreamInterceptor(
						grpc_zap.StreamServerInterceptor(grpcServerLogger),
					),
					grpc.UnaryInterceptor(
						grpc_zap.UnaryServerInterceptor(grpcServerLogger),
					),
				},
				Register: func(srv *grpc.Server) {
					api.RegisterLockServiceServer(srv, &service.LockServiceServer{
						Repository: repo,
					})
					api.RegisterDeployServiceServer(srv, &service.DeployServiceServer{
						Repository: repo,
					})
					api.RegisterBatchServiceServer(srv, &service.BatchServer{
						Repository: repo,
					})

					envSrv := &service.EnvironmentServiceServer{
						Repository: repo,
					}
					api.RegisterEnvironmentServiceServer(srv, envSrv)

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
				return nil
			},
		})
		return nil
	})
}
