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
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gopkg.in/DataDog/dd-trace-go.v1/profiler"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/interceptors"
	"github.com/freiheit-com/kuberpult/pkg/logging"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/service"
)

const (
	datadogNameCd           = "kuberpult-cd-service"
	minReleaseVersionsLimit = 5
	maxReleaseVersionsLimit = 30

	megaBytes int = 1024 * 1024
)

type Config struct {
	DexMock               bool
	DexEnabled            bool
	DexMockRole           string
	DexRbacPolicyPath     string
	DexRbacTeamPath       string
	DexDefaultRoleEnabled bool

	EnableProfiling       bool
	EnableTracing         bool
	EnableMetrics         bool
	DogstatsdAddr         string
	DatadogApiKeyLocation string

	DbLocation      string
	DbAuthProxyPort string

	DbName               string
	DbUserName           string
	DbUserPassword       string
	DbMigrationsLocation string
	DbSslMode            string

	DbMaxIdleConnections uint
	DbMaxOpenConnections uint

	AllowLongAppNames   bool
	ArgoCdGenerateFiles bool
	AllowedDomains      []string

	GitUrl             string
	GitBranch          string
	GitWriteCommitData bool

	GitNetworkTimeout    time.Duration
	ReleaseVersionsLimit uint

	// the cd-service calls the manifest-export on startup, to run custom migrations:
	MigrationServer       string
	MigrationServerSecure bool

	Version  string
	LockType string

	GrpcMaxRecvMsgSize int
	MaxNumberOfThreads uint

	EnableSqlite bool
	MinorRegexes string
}

func (c *Config) storageBackend() repository.StorageBackend {
	if c.EnableSqlite {
		return repository.SqliteBackend
	} else {
		return repository.GitBackend
	}
}

func parseEnvVars() (_ *Config, err error) {
	var c = Config{}
	c.DexMock = valid.ReadEnvVarBoolWithDefault("KUBERPULT_DEX_MOCK", false)
	c.DexEnabled = valid.ReadEnvVarBoolWithDefault("KUBERPULT_DEX_ENABLED", false)
	c.DexMockRole = valid.ReadEnvVarWithDefault("KUBERPULT_DEX_MOCK_ROLE", "Developer")
	c.DexRbacPolicyPath = valid.ReadEnvVarWithDefault("KUBERPULT_DEX_RBAC_POLICY_PATH", "")
	c.DexRbacTeamPath = valid.ReadEnvVarWithDefault("KUBERPULT_DEX_RBAC_TEAM_PATH", "")
	c.DexDefaultRoleEnabled = valid.ReadEnvVarBoolWithDefault("KUBERPULT_DEX_DEFAULT_ROLE_ENABLED", false)

	c.EnableProfiling = valid.ReadEnvVarBoolWithDefault("KUBERPULT_ENABLE_PROFILING", false)
	c.EnableTracing = valid.ReadEnvVarBoolWithDefault("KUBERPULT_ENABLE_TRACING", false)
	c.EnableMetrics = valid.ReadEnvVarBoolWithDefault("KUBERPULT_ENABLE_METRICS", false)
	c.DogstatsdAddr = valid.ReadEnvVarWithDefault("KUBERPULT_DOGSTATSD_ADDR", "127.0.0.1:8125")
	c.DatadogApiKeyLocation = valid.ReadEnvVarWithDefault("KUBERPULT_DATADOG_API_KEY_LOCATION", "")

	c.DbLocation = valid.ReadEnvVarWithDefault("KUBERPULT_DB_LOCATION", "/kp/database")
	c.DbAuthProxyPort = valid.ReadEnvVarWithDefault("KUBERPULT_DB_AUTH_PROXY_PORT", "5432")
	c.DbName = valid.ReadEnvVarWithDefault("KUBERPULT_DB_NAME", "")
	c.DbUserName = valid.ReadEnvVarWithDefault("KUBERPULT_DB_USER_NAME", "")
	c.DbUserPassword = valid.ReadEnvVarWithDefault("KUBERPULT_DB_USER_PASSWORD", "")
	c.DbMigrationsLocation = valid.ReadEnvVarWithDefault("KUBERPULT_DB_MIGRATIONS_LOCATION", "")
	c.DbSslMode = valid.ReadEnvVarWithDefault("KUBERPULT_DB_SSL_MODE", "verify-full")

	c.DbMaxIdleConnections, err = valid.ReadEnvVarUInt("KUBERPULT_DB_MAX_IDLE_CONNECTIONS")
	if err != nil {
		return nil, err
	}
	c.DbMaxOpenConnections, err = valid.ReadEnvVarUInt("KUBERPULT_DB_MAX_OPEN_CONNECTIONS")
	if err != nil {
		return nil, err
	}

	c.AllowLongAppNames = valid.ReadEnvVarBoolWithDefault("KUBERPULT_ALLOW_LONG_APP_NAMES", false)
	c.ArgoCdGenerateFiles = valid.ReadEnvVarBoolWithDefault("KUBERPULT_ARGO_CD_GENERATE_FILES", true)
	c.AllowedDomains, err = valid.ReadEnvVarAsList("KUBERPULT_ALLOWED_DOMAINS", ",")
	if err != nil {
		return nil, err
	}

	c.MaxNumberOfThreads, err = valid.ReadEnvVarUIntWithDefault("KUBERPULT_MAX_NUMBER_OF_THREADS", 3)
	if err != nil {
		return nil, err
	}

	c.GitUrl = valid.ReadEnvVarWithDefault("KUBERPULT_GIT_URL", "")
	c.GitBranch = valid.ReadEnvVarWithDefault("KUBERPULT_GIT_BRANCH", "master")
	c.GitWriteCommitData = valid.ReadEnvVarBoolWithDefault("KUBERPULT_GIT_WRITE_COMMIT_DATA", false)
	c.GitNetworkTimeout, err = valid.ReadEnvVarDurationWithDefault("KUBERPULT_GIT_NETWORK_TIMEOUT", time.Minute)
	if err != nil {
		return nil, err
	}
	c.ReleaseVersionsLimit, err = valid.ReadEnvVarUIntWithDefault("KUBERPULT_RELEASE_VERSIONS_LIMIT", 20)
	if err != nil {
		return nil, err
	}

	c.MigrationServer, err = valid.ReadEnvVar("KUBERPULT_MIGRATION_SERVER")
	if err != nil {
		return nil, err
	}
	c.MigrationServerSecure, err = valid.ReadEnvVarBool("KUBERPULT_MIGRATION_SERVER_SECURE")
	if err != nil {
		return nil, err
	}

	c.Version, err = valid.ReadEnvVar("KUBERPULT_VERSION")
	if err != nil {
		return nil, err
	}

	c.LockType, err = valid.ReadEnvVar("KUBERPULT_LOCK_TYPE")
	if err != nil {
		return nil, err
	}

	c.GrpcMaxRecvMsgSize, err = valid.ReadEnvVarInt("KUBERPULT_GRPC_MAX_RECV_MSG_SIZE")
	if err != nil {
		return nil, err
	}

	c.EnableSqlite = valid.ReadEnvVarBoolWithDefault("KUBERPULT_ENABLE_SQLITE", true)
	c.MinorRegexes = valid.ReadEnvVarWithDefault("KUBERPULT_MINOR_REGEXES", "")

	return &c, nil
}

func RunServer() {
	logging.Wrap(context.Background(), func(ctx context.Context) error {
		defer logging.HandlePanic(true)

		c, err := parseEnvVars()
		if err != nil {
			logging.Fatal(ctx, "parsing environment variables", zap.Error(err))
		}

		var lockType service.LockType
		lockType, err = service.ParseLockType(c.LockType)
		if err != nil {
			logging.Fatal(ctx, "config.parse.error.lock", zap.Error(err))
		}

		if c.EnableProfiling {
			ddFilename := c.DatadogApiKeyLocation
			if ddFilename == "" {
				logging.Fatal(ctx, "config.profiler.apikey.notfound", zap.Error(err))
			}
			fileContentBytes, err := os.ReadFile(ddFilename)
			if err != nil {
				logging.Fatal(ctx, "config.profiler.apikey.file", zap.Error(err))
			}
			fileContent := string(fileContentBytes)
			err = profiler.Start(profiler.WithAPIKey(fileContent), profiler.WithService(datadogNameCd))
			if err != nil {
				logging.Fatal(ctx, "config.profiler.error", zap.Error(err))
			}
			defer profiler.Stop()
		}

		var reader auth.GrpcContextReader
		if c.DexMock {
			if !c.DexEnabled {
				logging.Fatal(ctx, "dexEnabled must be true if dexMock is true")
			}
			if c.DexMockRole == "" {
				logging.Fatal(ctx, "dexMockRole must be set to a role (e.g 'DEVELOPER' because dexEnabled=true")
			}
			reader = &auth.DummyGrpcContextReader{Role: c.DexMockRole}
		} else {
			reader = &auth.DexGrpcContextReader{DexEnabled: c.DexEnabled, DexDefaultRoleEnabled: c.DexDefaultRoleEnabled}
		}
		dexRbacPolicy, err := auth.ReadRbacPolicy(c.DexEnabled, c.DexRbacPolicyPath)
		if err != nil {
			logging.Fatal(ctx, "dex.read.error", zap.Error(err))
		}
		dexRbacTeam, err := auth.ReadRbacTeam(c.DexEnabled, c.DexRbacTeamPath)
		if err != nil {
			logging.Fatal(ctx, "dex.read.error", zap.Error(err))
		}

		// Unary interceptor. Only parses the Role information if Dex is enabled.
		unaryUserContextInterceptor := func(ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler) (interface{}, error) {
			return interceptors.UnaryUserContextInterceptor(ctx, req, info, handler, reader)
		}

		grpcStreamInterceptors := []grpc.StreamServerInterceptor{
			func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
				defer logging.HandlePanic(true)
				return handler(srv, ss)
			},
		}
		grpcUnaryInterceptors := []grpc.UnaryServerInterceptor{
			unaryUserContextInterceptor,
			func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
				defer logging.HandlePanic(true)
				md, ok := metadata.FromIncomingContext(ctx)
				if ok {
					clientUUIDs := md.Get(auth.HeaderClientUUID)
					if len(clientUUIDs) > 0 && clientUUIDs[0] != "" {
						ctx = context.WithValue(ctx, auth.HeaderClientUUID, clientUUIDs[0])
					}
				}
				return handler(ctx, req)
			},
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
				logging.Fatal(ctx, "datadog.metrics.error", zap.Error(err))
			}
			ctx = context.WithValue(ctx, repository.DdMetricsKey, ddMetrics)
		}
		minorRegexes := []*regexp.Regexp{}
		if c.MinorRegexes != "" {
			for _, minorRegexStr := range strings.Split(c.MinorRegexes, ",") {
				regex, err := regexp.Compile(minorRegexStr)
				if err != nil {
					logging.Error(ctx, "Invalid regex input.", zap.String("minorRegexStr", minorRegexStr))
					continue
				}
				minorRegexes = append(minorRegexes, regex)
			}
		}

		// If the tracer is not started, calling this function is a no-op.
		span, ctx := tracer.StartSpanFromContext(ctx, "Start server")

		if strings.HasPrefix(c.GitUrl, "https") {
			logging.Fatal(ctx, "git.url.protocol.unsupported",
				zap.String("url", c.GitUrl),
				zap.String("details", "https is not supported for git communication, only ssh is supported"))
		}
		if err := checkReleaseVersionLimit(c.ReleaseVersionsLimit); err != nil {
			logging.Fatal(ctx, "cd.config",
				zap.String("details", err.Error()),
			)
		}

		dbCfg := db.DBConfig{
			DbHost:         c.DbLocation,
			DbPort:         c.DbAuthProxyPort,
			DriverName:     "postgres",
			DbName:         c.DbName,
			DbPassword:     c.DbUserPassword,
			DbUser:         c.DbUserName,
			MigrationsPath: c.DbMigrationsLocation,
			SSLMode:        c.DbSslMode,

			MaxIdleConnections: c.DbMaxIdleConnections,
			MaxOpenConnections: c.DbMaxOpenConnections,

			DatadogEnabled:     c.EnableTracing,
			DatadogServiceName: datadogNameCd,
		}
		dbHandler, err := db.Connect(ctx, dbCfg)
		if err != nil {
			logging.Fatal(ctx, "Error establishing DB connection: ", zap.Error(err))
		}
		pErr := dbHandler.DB.Ping()
		if pErr != nil {
			logging.Fatal(ctx, "Error pinging DB: ", zap.Error(pErr))
		}

		migErr := db.RunDBMigrations(ctx, dbCfg)
		if migErr != nil {
			logging.Fatal(ctx, "Error running database migrations: ", zap.Error(migErr))
		}
		logging.Info(ctx, "Finished with basic database migration.")

		cfg := repository.RepositoryConfig{
			URL:                  c.GitUrl,
			MinorRegexes:         minorRegexes,
			MaxNumThreads:        c.MaxNumberOfThreads,
			Branch:               c.GitBranch,
			ReleaseVersionsLimit: c.ReleaseVersionsLimit,
			StorageBackend:       c.storageBackend(),
			NetworkTimeout:       c.GitNetworkTimeout,
			DogstatsdEvents:      c.EnableMetrics,
			WriteCommitData:      c.GitWriteCommitData,
			AllowLongAppNames:    c.AllowLongAppNames,
			ArgoCdGenerateFiles:  c.ArgoCdGenerateFiles,
			DBHandler:            dbHandler,
		}

		repo, err := repository.New(ctx, cfg)
		if err != nil {
			logging.Fatal(ctx, "repository.new.error", zap.Error(err), zap.String("git.url", c.GitUrl), zap.String("git.branch", c.GitBranch))
		}

		repositoryService :=
			&service.Service{
				Repository: repo,
			}
		grpcMsgSizeBytes := c.GrpcMaxRecvMsgSize * megaBytes

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
						if c.EnableTracing {
							wrappedHandler := repositoryService
							handler := httptrace.WrapHandler(wrappedHandler, datadogNameCd, "/")
							mux.Handle("/", handler)
						} else {
							handler := repositoryService
							mux.Handle("/", handler)
						}
					},
				},
			},
			GRPC: &setup.GRPCConfig{
				Shutdown: nil,
				Port:     "8443",
				Opts: []grpc.ServerOption{
					grpc.ChainStreamInterceptor(grpcStreamInterceptors...),
					grpc.ChainUnaryInterceptor(grpcUnaryInterceptors...),
					grpc.MaxRecvMsgSize(grpcMsgSizeBytes),
				},
				Register: func(srv *grpc.Server) {
					api.RegisterBatchServiceServer(srv, &service.BatchServer{
						DBHandler:  dbHandler,
						Repository: repo,
						RBACConfig: auth.RBACConfig{
							DexEnabled: c.DexEnabled,
							Policy:     dexRbacPolicy,
							Team:       dexRbacTeam,
						},
						Config: service.BatchServerConfig{
							WriteCommitData:      c.GitWriteCommitData,
							AllowedCILinkDomains: c.AllowedDomains,
							LockType:             lockType,
						},
					})

					overviewSrv := &service.OverviewServiceServer{
						Repository:       repo,
						RepositoryConfig: cfg,
						Shutdown:         shutdownCh,
						Context:          ctx,
						DBHandler:        dbHandler,
					}
					api.RegisterOverviewServiceServer(srv, overviewSrv)
					api.RegisterProductSummaryServiceServer(srv, &service.ProductSummaryServer{State: repo.State()})
					api.RegisterVersionServiceServer(srv, &service.VersionServiceServer{Repository: repo})
					api.RegisterEnvironmentServiceServer(srv, &service.EnvironmentServiceServer{Repository: repo})
					api.RegisterReleaseTrainPrognosisServiceServer(srv, &service.ReleaseTrainPrognosisServer{
						Repository: repo,
						RBACConfig: auth.RBACConfig{
							DexEnabled: c.DexEnabled,
							Policy:     dexRbacPolicy,
							Team:       dexRbacTeam,
						},
					})
					api.RegisterEslServiceServer(srv, &service.EslServiceServer{Repository: repo})
					reflection.Register(srv)

					if dbHandler != nil {
						api.RegisterCommitDeploymentServiceServer(srv, &service.CommitDeploymentServer{DBHandler: dbHandler})
					}
				},
			},
			Background: []setup.BackgroundTaskConfig{
				{
					Shutdown: nil,
					Name:     "ddmetrics",
					Run: func(ctx context.Context, reporter *setup.HealthReporter) error {
						reporter.ReportReady("sending metrics")
						repository.RegularlySendDatadogMetrics(repo, 300, func(repository2 repository.Repository, even bool) {
							repository.GetRepositoryStateAndUpdateMetrics(ctx, repository2, even)
						})
						return nil
					},
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

func checkReleaseVersionLimit(limit uint) error {
	if limit < minReleaseVersionsLimit || limit > maxReleaseVersionsLimit {
		return releaseVersionsLimitError{limit: limit}
	}
	return nil
}
