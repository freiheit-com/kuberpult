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
	"database/sql"
	"errors"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/file"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/profiler"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/argocd/reposerver"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/interceptors"

	"github.com/DataDog/datadog-go/v5/statsd"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/service"
	sqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const datadogNameCd = "kuberpult-cd-service"

func getDBConnection(dbFolderLocation string) (*sql.DB, error) {
	return sql.Open("sqlite3", path.Join(dbFolderLocation, "db.sqlite")) //not clear on what is needed for the user and password
}

func runDBMigrations(ctx context.Context, dbFolderLocation string) {
	db, err := getDBConnection(dbFolderLocation)

	if err != nil {
		logger.FromContext(ctx).Fatal("DB Error opening DB connection. Error: ", zap.Error(err))
		return
	}
	defer db.Close()

	driver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		logger.FromContext(ctx).Fatal("Error creating DB driver. Error: ", zap.Error(err))
		return
	}

	migrationsSrc, err := (&file.File{}).Open(path.Join(dbFolderLocation, "migrations"))
	if err != nil {
		logger.FromContext(ctx).Fatal("Error opening DB migrations. Error: ", zap.Error(err))
		return
	}
	m, err := migrate.NewWithInstance("file", migrationsSrc, "sqlite3", driver)
	if err != nil {
		logger.FromContext(ctx).Fatal("Error creating migration instance. Error: ", zap.Error(err))
		return
	}

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			logger.FromContext(ctx).Fatal("Error running DB migrations. Error: ", zap.Error(err))
		}
	}
}

func retrieveDatabaseInformation(ctx context.Context, databaseLocation string) *sql.Rows {
	db, err := getDBConnection(databaseLocation)
	if err != nil {
		logger.FromContext(ctx).Fatal("Error creating DB connection. Error: ", zap.Error(err))
		return nil
	}
	result, err := db.Query("SELECT * FROM dummy_table;")

	if err != nil {
		logger.FromContext(ctx).Fatal("Error cquerying the database. Error: ", zap.Error(err))
		return nil
	}
	return result
}
func insertDatabaseInformation(ctx context.Context, databaseLocation string) (sql.Result, error) {
	db, err := getDBConnection(databaseLocation)
	if err != nil {
		logger.FromContext(ctx).Fatal("Error creating DB connection. Error: ", zap.Error(err))
		return nil, err
	}

	message := "Hello DB!"
	result, err := db.Exec("INSERT INTO dummy_table (id , created , data)  VALUES (?, ?, ?);", rand.Intn(9999), time.Now(), message)
	if err != nil {
		logger.FromContext(ctx).Warn("Error inserting information into DB. Error: ", zap.Error(err))
		return nil, err
	}

	if err != nil {
		logger.FromContext(ctx).Warn("Error querying the database. Error: ", zap.Error(err))
		return nil, err
	}
	return result, nil
}

func printQuery(ctx context.Context, res *sql.Rows) {
	var (
		id   int
		date []byte
		name string
	)

	for res.Next() {
		err := res.Scan(&id, &date, &name)
		if err != nil {
			logger.FromContext(ctx).Warn("Error retrieving information from query. Error: ", zap.Error(err))
			return
		}
		fmt.Println(id, string(date), name)
	}

}

type Config struct {
	// these will be mapped to "KUBERPULT_GIT_URL", etc.
	GitUrl                   string        `required:"true" split_words:"true"`
	GitBranch                string        `default:"master" split_words:"true"`
	BootstrapMode            bool          `default:"false" split_words:"true"`
	GitCommitterEmail        string        `default:"kuberpult@freiheit.com" split_words:"true"`
	GitCommitterName         string        `default:"kuberpult" split_words:"true"`
	GitSshKey                string        `default:"/etc/ssh/identity" split_words:"true"`
	GitSshKnownHosts         string        `default:"/etc/ssh/ssh_known_hosts" split_words:"true"`
	GitNetworkTimeout        time.Duration `default:"1m" split_words:"true"`
	GitWriteCommitData       bool          `default:"false" split_words:"true"`
	PgpKeyRingPath           string        `split_words:"true"`
	AzureEnableAuth          bool          `default:"false" split_words:"true"`
	DexEnabled               bool          `default:"false" split_words:"true"`
	DexRbacPolicyPath        string        `split_words:"true"`
	EnableTracing            bool          `default:"false" split_words:"true"`
	EnableMetrics            bool          `default:"false" split_words:"true"`
	EnableEvents             bool          `default:"false" split_words:"true"`
	DogstatsdAddr            string        `default:"127.0.0.1:8125" split_words:"true"`
	EnableProfiling          bool          `default:"false" split_words:"true"`
	DatadogApiKeyLocation    string        `default:"" split_words:"true"`
	EnableSqlite             bool          `default:"true" split_words:"true"`
	DexMock                  bool          `default:"false" split_words:"true"`
	DexMockRole              string        `default:"Developer" split_words:"true"`
	ArgoCdServer             string        `default:"" split_words:"true"`
	ArgoCdInsecure           bool          `default:"false" split_words:"true"`
	GitWebUrl                string        `default:"" split_words:"true"`
	GitMaximumCommitsPerPush uint          `default:"1" split_words:"true"`
	MaximumQueueSize         uint          `default:"5" split_words:"true"`
	ArgoCdGenerateFiles      bool          `default:"true" split_words:"true"`
	DbEnabled                bool          `default:"false" split_words:"true"`
	DbLocation               string        `default:"/kp/database" split_words:"true"`
}

func (c *Config) storageBackend() repository.StorageBackend {
	if c.EnableSqlite {
		return repository.SqliteBackend
	} else {
		return repository.GitBackend
	}
}

func RunServer() {
	err := logger.Wrap(context.Background(), func(ctx context.Context) error {

		var c Config

		err := envconfig.Process("kuberpult", &c)

		if c.EnableProfiling {
			ddFilename := c.DatadogApiKeyLocation
			if ddFilename == "" {
				logger.FromContext(ctx).Fatal("config.profiler.apikey.notfound", zap.Error(err))
			}
			fileContentBytes, err := os.ReadFile(ddFilename)
			if err != nil {
				logger.FromContext(ctx).Fatal("config.profiler.apikey.file", zap.Error(err))
			}
			fileContent := string(fileContentBytes)
			err = profiler.Start(profiler.WithAPIKey(fileContent), profiler.WithService(datadogNameCd))
			if err != nil {
				logger.FromContext(ctx).Fatal("config.profiler.error", zap.Error(err))
			}
			defer profiler.Stop()
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
		dexRbacPolicy, err := auth.ReadRbacPolicy(c.DexEnabled, c.DexRbacPolicyPath)
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

		grpcStreamInterceptors := []grpc.StreamServerInterceptor{
			grpc_zap.StreamServerInterceptor(grpcServerLogger),
		}
		grpcUnaryInterceptors := []grpc.UnaryServerInterceptor{
			grpc_zap.UnaryServerInterceptor(grpcServerLogger),
			unaryUserContextInterceptor,
		}

		if c.EnableTracing {
			tracer.Start()
			defer tracer.Stop()

			grpcStreamInterceptors = append(grpcStreamInterceptors,
				grpctrace.StreamServerInterceptor(grpctrace.WithServiceName(tracing.ServiceName(datadogNameCd))),
			)
			grpcUnaryInterceptors = append(grpcUnaryInterceptors,
				grpctrace.UnaryServerInterceptor(grpctrace.WithServiceName(tracing.ServiceName(datadogNameCd))),
			)
		}

		if c.EnableMetrics {
			ddMetrics, err := statsd.New(c.DogstatsdAddr, statsd.WithNamespace("Kuberpult"))
			if err != nil {
				logger.FromContext(ctx).Fatal("datadog.metrics.error", zap.Error(err))
			}
			ctx = context.WithValue(ctx, repository.DdMetricsKey, ddMetrics)
		}

		// If the tracer is not started, calling this function is a no-op.
		span, ctx := tracer.StartSpanFromContext(ctx, "Start server")

		if strings.HasPrefix(c.GitUrl, "https") {
			logger.FromContext(ctx).Fatal("git.url.protocol.unsupported",
				zap.String("url", c.GitUrl),
				zap.String("details", "https is not supported for git communication, only ssh is supported"))
		}
		if c.GitMaximumCommitsPerPush == 0 {
			logger.FromContext(ctx).Fatal("git.config",
				zap.String("details", "the maximum number of commits per push must be at least 1"),
			)
		}
		if c.MaximumQueueSize < 2 || c.MaximumQueueSize > 100 {
			logger.FromContext(ctx).Fatal("cd.config",
				zap.String("details", "the size of the queue must be between 2 and 100"),
			)
		}

		if c.DbEnabled {
			runDBMigrations(ctx, c.DbLocation)
			_, err := insertDatabaseInformation(ctx, c.DbLocation)
			if err != nil {
				logger.FromContext(ctx).Warn("Error inserting into the database. Error: ", zap.Error(err))
			} else {
				printQuery(ctx, retrieveDatabaseInformation(ctx, c.DbLocation))
			}
		}
		cfg := repository.RepositoryConfig{
			WebhookResolver: nil,
			URL:             c.GitUrl,
			Path:            "./repository",
			CommitterEmail:  c.GitCommitterEmail,
			CommitterName:   c.GitCommitterName,
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
			DogstatsdEvents:        c.EnableMetrics,
			WriteCommitData:        c.GitWriteCommitData,
			MaximumCommitsPerPush:  c.GitMaximumCommitsPerPush,
			MaximumQueueSize:       c.MaximumQueueSize,
			ArgoCdGenerateFiles:    c.ArgoCdGenerateFiles,
		}
		repo, repoQueue, err := repository.New2(ctx, cfg)
		if err != nil {
			logger.FromContext(ctx).Fatal("repository.new.error", zap.Error(err), zap.String("git.url", c.GitUrl), zap.String("git.branch", c.GitBranch))
		}

		repositoryService := &service.Service{
			Repository: repo,
		}

		span.Finish()

		// Shutdown channel is used to terminate server side streams.
		shutdownCh := make(chan struct{})
		setup.Run(ctx, setup.ServerConfig{
			HTTP: []setup.HTTPConfig{
				{
					BasicAuth: nil,
					Shutdown:  nil,
					Port:      "8080",
					Register: func(mux *http.ServeMux) {
						handler := logger.WithHttpLogger(httpServerLogger, repositoryService)
						if c.EnableTracing {
							handler = httptrace.WrapHandler(handler, datadogNameCd, "/")
						}
						mux.Handle("/", handler)
					},
				},
			},
			GRPC: &setup.GRPCConfig{
				Shutdown: nil,
				Port:     "8443",
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
						Config: service.BatchServerConfig{
							WriteCommitData: c.GitWriteCommitData,
						},
					})

					overviewSrv := &service.OverviewServiceServer{
						Repository:       repo,
						RepositoryConfig: cfg,
						Shutdown:         shutdownCh,
					}
					api.RegisterOverviewServiceServer(srv, overviewSrv)
					api.RegisterGitServiceServer(srv, &service.GitServer{Config: cfg, OverviewService: overviewSrv})
					api.RegisterVersionServiceServer(srv, &service.VersionServiceServer{Repository: repo})
					api.RegisterEnvironmentServiceServer(srv, &service.EnvironmentServiceServer{Repository: repo})
					api.RegisterReleaseTrainPrognosisServiceServer(srv, &service.ReleaseTrainPrognosisServer{
						Repository: repo,
						RBACConfig: auth.RBACConfig{
							DexEnabled: c.DexEnabled,
							Policy:     dexRbacPolicy,
						},
					})
					reflection.Register(srv)
					reposerver.Register(srv, repo, cfg)

				},
			},
			Background: []setup.BackgroundTaskConfig{
				{
					Shutdown: nil,
					Name:     "ddmetrics",
					Run: func(ctx context.Context, reporter *setup.HealthReporter) error {
						reporter.ReportReady("sending metrics")
						repository.RegularlySendDatadogMetrics(repo, 300, func(repository2 repository.Repository) {
							repository.GetRepositoryStateAndUpdateMetrics(ctx, repository2)
						})
						return nil
					},
				},
				{
					Shutdown: nil,
					Name:     "push queue",
					Run:      repoQueue,
				},
			},
			Shutdown: func(ctx context.Context) error {
				close(shutdownCh)
				return nil
			},
		})

		return nil
	})
	if err != nil {
		fmt.Printf("error in logger.wrap: %v %#v", err, err)
	}
}
