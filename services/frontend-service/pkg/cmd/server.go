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
/*
This file is part of kuberpult.

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

Copyright 2021 freiheit.com
*/
package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/MicahParks/keyfunc"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/service"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/handler"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type Config struct {
	CdServer            string `default:"kuberpult-cd-service:8443"`
	GKEProjectNumber    string `default:"" split_words:"true"`
	GKEBackendServiceID string `default:"" split_words:"true"`
	EnableTracing       bool   `default:"false" split_words:"true"`
	ArgocdBaseUrl       string `default:"" split_words:"true"`
	AzureEnableAuth     bool   `default:"false" split_words:"true"`
	AzureCloudInstance  string `default:"https://login.microsoftonline.com/" split_words:"true"`
	AzureClientId       string `default:"" split_words:"true"`
	AzureTenantId       string `default:"" split_words:"true"`
	AzureRedirectUrl    string `default:"" split_words:"true"`
	Version             string `default:""`
	GitUrl              string `default:"" split_words:"true"`
}

var c Config

func readAllAndClose(r io.ReadCloser, maxBytes int64) {
	_, _ = io.ReadAll(io.LimitReader(r, maxBytes))
	_ = r.Close()
}

func RunServer() {
	logger.Wrap(context.Background(), func(ctx context.Context) error {
		err := envconfig.Process("kuberpult", &c)
		if err != nil {
			logger.FromContext(ctx).Fatal("config.parse", zap.Error(err))
		}

		var jwks *keyfunc.JWKS = nil
		if c.AzureEnableAuth {
			jwks, err = auth.JWKSInitAzure(ctx)
			if err != nil {
				logger.FromContext(ctx).Fatal("Unable to initialize jwks for azure auth")
			}
		}
		logger.FromContext(ctx).Info("config.gke_project_number: " + c.GKEProjectNumber + "\n")
		logger.FromContext(ctx).Info("config.gke_backend_service_id: " + c.GKEBackendServiceID + "\n")

		grpcServerLogger := logger.FromContext(ctx).Named("grpc_server")

		grpcStreamInterceptors := []grpc.StreamServerInterceptor{
			grpc_zap.StreamServerInterceptor(grpcServerLogger),
		}
		grpcUnaryInterceptors := []grpc.UnaryServerInterceptor{
			grpc_zap.UnaryServerInterceptor(grpcServerLogger),
		}

		grpcClientOpts := []grpc.DialOption{
			grpc.WithInsecure(),
		}

		if c.EnableTracing {
			tracer.Start()
			defer tracer.Stop()

			grpcStreamInterceptors = append(grpcStreamInterceptors,
				grpctrace.StreamServerInterceptor(grpctrace.WithServiceName("frontend-service")),
			)
			grpcUnaryInterceptors = append(grpcUnaryInterceptors,
				grpctrace.UnaryServerInterceptor(grpctrace.WithServiceName("frontend-service")),
			)

			grpcClientOpts = append(grpcClientOpts,
				grpc.WithStreamInterceptor(
					grpctrace.StreamClientInterceptor(grpctrace.WithServiceName("frontend-service")),
				),
				grpc.WithUnaryInterceptor(
					grpctrace.UnaryClientInterceptor(grpctrace.WithServiceName("frontend-service")),
				),
			)
		}

		if c.AzureEnableAuth {
			var AzureUnaryInterceptor = func(ctx context.Context,
				req interface{},
				info *grpc.UnaryServerInfo,
				handler grpc.UnaryHandler) (interface{}, error) {
				return auth.UnaryInterceptor(ctx, req, info, handler, jwks, c.AzureClientId, c.AzureTenantId)
			}
			var AzureStreamInterceptor = func(
				srv interface{},
				stream grpc.ServerStream,
				info *grpc.StreamServerInfo,
				handler grpc.StreamHandler,
			) error {
				return auth.StreamInterceptor(srv, stream, info, handler, jwks, c.AzureClientId, c.AzureTenantId)
			}
			grpcUnaryInterceptors = append(grpcUnaryInterceptors, AzureUnaryInterceptor)
			grpcStreamInterceptors = append(grpcStreamInterceptors, AzureStreamInterceptor)
		}

		gsrv := grpc.NewServer(
			grpc.ChainStreamInterceptor(grpcStreamInterceptors...),
			grpc.ChainUnaryInterceptor(grpcUnaryInterceptors...),
		)
		con, err := grpc.Dial(c.CdServer, grpcClientOpts...)
		if err != nil {
			logger.FromContext(ctx).Fatal("grpc.dial.error", zap.Error(err), zap.String("addr", c.CdServer))
		}

		lockClient := api.NewLockServiceClient(con)
		deployClient := api.NewDeployServiceClient(con)
		gproxy := &GrpcProxy{
			LockClient:     lockClient,
			OverviewClient: api.NewOverviewServiceClient(con),
			DeployClient:   deployClient,
			BatchClient:    api.NewBatchServiceClient(con),
		}
		api.RegisterLockServiceServer(gsrv, gproxy)
		api.RegisterOverviewServiceServer(gsrv, gproxy)
		api.RegisterDeployServiceServer(gsrv, gproxy)
		api.RegisterBatchServiceServer(gsrv, gproxy)

		frontendConfigService := &service.FrontendConfigServiceServer{
			Config: config.FrontendConfig{
				ArgoCd: &config.ArgoCdConfig{BaseUrl: c.ArgocdBaseUrl},
				Git:    &config.GitConfig{Url: c.GitUrl},
				Auth: &config.AuthConfig{
					AzureAuth: &config.AzureAuthConfig{
						Enabled:       c.AzureEnableAuth,
						ClientId:      c.AzureClientId,
						TenantId:      c.AzureTenantId,
						RedirectURL:   c.AzureRedirectUrl,
						CloudInstance: c.AzureCloudInstance,
					},
				},
				KuberpultVersion: c.Version,
			},
		}

		api.RegisterFrontendConfigServiceServer(gsrv, frontendConfigService)

		grpcWebServer := grpcweb.WrapServer(gsrv)
		httpHandler := handler.Server{
			DeployClient: deployClient,
			LockClient:   lockClient,
		}
		mux := http.NewServeMux()
		mux.Handle("/environments/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer readAllAndClose(req.Body, 1024)
			httpHandler.Handle(w, req)
		}))
		mux.Handle("/", http.FileServer(http.Dir("build")))
		// Split HTTP REST from gRPC Web requests, as suggested in the documentation:
		// https://pkg.go.dev/github.com/improbable-eng/grpc-web@v0.15.0/go/grpcweb
		splitGrpcHandler := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if grpcWebServer.IsGrpcWebRequest(req) {
				grpcWebServer.ServeHTTP(resp, req)
			} else {
				/**
				The htst header is a security feature that tells the browser:
				"If someone requests anything on this domain via http, do not do that request, instead make the request with https"
				Docs: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Strict-Transport-Security
				Wiki: https://en.wikipedia.org/wiki/HTTP_Strict_Transport_Security
				Parameter "preload" is not necessary as kuberpult is not on a publicly available domain.
				Parameter "includeSubDomains" is not really necessary for kuberpult right now,
				  but should be set anyway in case we ever have subdomains.
				31536000 seconds = 1 year.
				*/
				resp.Header().Set("strict-Transport-Security", "max-age=31536000; includeSubDomains;")
				if c.AzureEnableAuth {
					if err := auth.HttpAuthMiddleWare(resp, req, jwks, c.AzureClientId, c.AzureTenantId, []string{"/", "/manifest.json", "/favicon.png"}, []string{"/static/js", "/static/css"}); err != nil {
						return
					}
				}
				mux.ServeHTTP(resp, req)
			}
		})
		authHandler := &Auth{
			HttpServer: splitGrpcHandler,
		}
		corsHandler := &setup.CORSMiddleware{
			PolicyFor: func(r *http.Request) *setup.CORSPolicy {
				return &setup.CORSPolicy{
					AllowMethods:     "POST",
					AllowHeaders:     "content-type,x-grpc-web,authorization",
					AllowOrigin:      "*",
					AllowCredentials: true,
				}
			},
			NextHandler: authHandler,
		}

		setup.Run(ctx, setup.Config{
			HTTP: []setup.HTTPConfig{
				{
					Port: "8081",
					Register: func(mux *http.ServeMux) {
						mux.Handle("/", corsHandler)
					},
				},
			},
		})
		return nil
	})
}

type Auth struct {
	HttpServer http.Handler
}

func getRequestAuthor(ctx context.Context, r *http.Request) *auth.User {
	iapJWT := r.Header.Get("X-Goog-IAP-JWT-Assertion")
	if iapJWT == "" {
		// not using iap (local), default user
		logger.FromContext(ctx).Info("iap.jwt header was not found or doesn't exist")
		return auth.DefaultUser
	}

	if c.GKEProjectNumber == "" || c.GKEBackendServiceID == "" {
		// environment variables not set up correctly
		logger.FromContext(ctx).Info("iap.jke environment variables are not set up correctly")
		return auth.DefaultUser
	}

	aud := fmt.Sprintf("/projects/%s/global/backendServices/%s", c.GKEProjectNumber, c.GKEBackendServiceID)
	payload, err := idtoken.Validate(ctx, iapJWT, aud)
	if err != nil {
		logger.FromContext(ctx).Warn("iap.idtoken.validate", zap.Error(err))
		return auth.DefaultUser
	}

	// here, we can use People api later to get the full name

	// get the authenticated email
	u := &auth.User{
		Email: payload.Claims["email"].(string),
	}
	return u
}

func (p *Auth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.Wrap(r.Context(), func(ctx context.Context) error {
		u := getRequestAuthor(ctx, r)
		p.HttpServer.ServeHTTP(w, r.WithContext(auth.ToContext(ctx, u)))
		return nil
	})
}

// GrpcProxy passes through gRPC messages to another server.
// An alternative to the more generic methods proposed in
// https://github.com/grpc/grpc-go/issues/2297
type GrpcProxy struct {
	LockClient     api.LockServiceClient
	OverviewClient api.OverviewServiceClient
	DeployClient   api.DeployServiceClient
	BatchClient    api.BatchServiceClient
}

func (p *GrpcProxy) ProcessBatch(
	ctx context.Context,
	in *api.BatchRequest) (*emptypb.Empty, error) {
	return p.BatchClient.ProcessBatch(ctx, in)
}

func (p *GrpcProxy) CreateEnvironmentLock(
	ctx context.Context,
	in *api.CreateEnvironmentLockRequest) (*emptypb.Empty, error) {
	return p.LockClient.CreateEnvironmentLock(ctx, in)
}

func (p *GrpcProxy) DeleteEnvironmentLock(
	ctx context.Context,
	in *api.DeleteEnvironmentLockRequest) (*emptypb.Empty, error) {
	return p.LockClient.DeleteEnvironmentLock(ctx, in)
}

func (p *GrpcProxy) CreateEnvironmentApplicationLock(
	ctx context.Context,
	in *api.CreateEnvironmentApplicationLockRequest) (*emptypb.Empty, error) {
	return p.LockClient.CreateEnvironmentApplicationLock(ctx, in)
}

func (p *GrpcProxy) DeleteEnvironmentApplicationLock(
	ctx context.Context,
	in *api.DeleteEnvironmentApplicationLockRequest) (*emptypb.Empty, error) {
	return p.LockClient.DeleteEnvironmentApplicationLock(ctx, in)
}

func (p *GrpcProxy) GetOverview(
	ctx context.Context,
	in *api.GetOverviewRequest) (*api.GetOverviewResponse, error) {
	return p.OverviewClient.GetOverview(ctx, in)
}

func (p *GrpcProxy) StreamOverview(
	in *api.GetOverviewRequest,
	stream api.OverviewService_StreamOverviewServer) error {
	if resp, err := p.OverviewClient.StreamOverview(stream.Context(), in); err != nil {
		return err
	} else {
		for {
			if item, err := resp.Recv(); err != nil {
				return err
			} else {
				if err := stream.Send(item); err != nil {
					return err
				}
			}
		}
	}
}

func (p *GrpcProxy) Deploy(
	ctx context.Context,
	in *api.DeployRequest) (*emptypb.Empty, error) {
	return p.DeployClient.Deploy(ctx, in)
}

func (p *GrpcProxy) ReleaseTrain(
	ctx context.Context,
	in *api.ReleaseTrainRequest) (*emptypb.Empty, error) {
	return p.DeployClient.ReleaseTrain(ctx, in)
}
