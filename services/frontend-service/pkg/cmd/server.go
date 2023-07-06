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
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/interceptors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/MicahParks/keyfunc/v2"
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

var c config.ServerConfig

func readAllAndClose(r io.ReadCloser, maxBytes int64) {
	_, _ = io.ReadAll(io.LimitReader(r, maxBytes))
	_ = r.Close()
}

func readPgpKeyRing() (openpgp.KeyRing, error) {
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
	logger.Wrap(context.Background(), runServer)
}

func runServer(ctx context.Context) error {
	err := envconfig.Process("kuberpult", &c)
	if err != nil {
		logger.FromContext(ctx).Fatal("config.parse", zap.Error(err))
		return err
	}
	logger.FromContext(ctx).Warn(fmt.Sprintf("config: \n%v", c))
	if c.GitAuthorEmail == "" {
		logger.FromContext(ctx).Fatal("DefaultGitAuthorEmail must not be empty")
	}
	if c.GitAuthorName == "" {
		logger.FromContext(ctx).Fatal("DefaultGitAuthorName must not be empty")
	}

	var jwks *keyfunc.JWKS = nil
	if c.AzureEnableAuth {
		jwks, err = auth.JWKSInitAzure(ctx)
		if err != nil {
			logger.FromContext(ctx).Fatal("Unable to initialize jwks for azure auth")
			return err
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

	var defaultUser = auth.User{
		Email: c.GitAuthorEmail,
		Name:  c.GitAuthorName,
	}

	if c.AzureEnableAuth {
		var AzureUnaryInterceptor = func(ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler) (interface{}, error) {
			return interceptors.UnaryAuthInterceptor(ctx, req, info, handler, jwks, c.AzureClientId, c.AzureTenantId, defaultUser)
		}
		var AzureStreamInterceptor = func(
			srv interface{},
			stream grpc.ServerStream,
			info *grpc.StreamServerInfo,
			handler grpc.StreamHandler,
		) error {
			return interceptors.StreamAuthInterceptor(srv, stream, info, handler, jwks, c.AzureClientId, c.AzureTenantId)
		}
		grpcUnaryInterceptors = append(grpcUnaryInterceptors, AzureUnaryInterceptor)
		grpcStreamInterceptors = append(grpcStreamInterceptors, AzureStreamInterceptor)
	}

	pgpKeyRing, err := readPgpKeyRing()
	if err != nil {
		logger.FromContext(ctx).Fatal("pgp.read.error", zap.Error(err))
		return err
	}
	if c.AzureEnableAuth && pgpKeyRing == nil {
		logger.FromContext(ctx).Fatal("azure.auth.error: pgpKeyRing is required to authenticate manifests when \"KUBERPULT_AZURE_ENABLE_AUTH\" is true")
		return err
	}

	gsrv := grpc.NewServer(
		grpc.ChainStreamInterceptor(grpcStreamInterceptors...),
		grpc.ChainUnaryInterceptor(grpcUnaryInterceptors...),
	)
	con, err := grpc.Dial(c.CdServer, grpcClientOpts...)
	if err != nil {
		logger.FromContext(ctx).Fatal("grpc.dial.error", zap.Error(err), zap.String("addr", c.CdServer))
	}

	batchClient := api.NewBatchServiceClient(con)
	deployClient := api.NewDeployServiceClient(con)
	environmentClient := api.NewEnvironmentServiceClient(con)
	gproxy := &GrpcProxy{
		OverviewClient:    api.NewOverviewServiceClient(con),
		DeployClient:      deployClient,
		BatchClient:       batchClient,
		EnvironmentClient: environmentClient,
	}
	api.RegisterOverviewServiceServer(gsrv, gproxy)
	api.RegisterDeployServiceServer(gsrv, gproxy)
	api.RegisterBatchServiceServer(gsrv, gproxy)
	api.RegisterEnvironmentServiceServer(gsrv, gproxy)

	frontendConfigService := &service.FrontendConfigServiceServer{
		Config: config.FrontendConfig{
			ArgoCd: &config.ArgoCdConfig{BaseUrl: c.ArgocdBaseUrl},
			Auth: &config.AuthConfig{
				AzureAuth: &config.AzureAuthConfig{
					Enabled:       c.AzureEnableAuth,
					ClientId:      c.AzureClientId,
					TenantId:      c.AzureTenantId,
					RedirectURL:   c.AzureRedirectUrl,
					CloudInstance: c.AzureCloudInstance,
				},
			},
			SourceRepoUrl:    c.SourceRepoUrl,
			KuberpultVersion: c.Version,
		},
	}

	api.RegisterFrontendConfigServiceServer(gsrv, frontendConfigService)

	grpcWebServer := grpcweb.WrapServer(gsrv)
	httpHandler := handler.Server{
		DeployClient:      deployClient,
		BatchClient:       batchClient,
		EnvironmentClient: environmentClient,
		Config:            c,
		KeyRing:           pgpKeyRing,
		AzureAuth:         c.AzureEnableAuth,
	}
	mux := http.NewServeMux()
	mux.Handle("/environments/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer readAllAndClose(req.Body, 1024)
		httpHandler.Handle(w, req)
	}))
	mux.Handle("/release", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer readAllAndClose(req.Body, 1024)
		httpHandler.Handle(w, req)
	}))
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintf(w, "ok\n")
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
			/**
			- self is generally sufficient for most sources
			- fonts.googleapis.com is used for font hosting
			- unsafe-inline is needed at the moment to make emotionjs work
			- fonts.gstatic.con is used for font hosting
			- login.microsoftonline.com is used for azure login
			*/
			resp.Header().Set("Content-Security-Policy", "default-src 'self'; style-src-elem 'self' fonts.googleapis.com 'unsafe-inline'; font-src fonts.gstatic.com; connect-src 'self' login.microsoftonline.com; child-src 'none'")
			// We are not using referrer headers.
			resp.Header().Set("Referrer-Policy", "no-referrer")
			// We don't want to be displayed in frames
			resp.Header().Set("X-Frame-Options", "DENY")
			// Don't sniff content-type
			resp.Header().Set("X-Content-Type-Options", "nosniff")
			// We don't need any special browser features.
			// This policy was generated using https://www.permissionspolicy.com/
			// with "Disable all" for all implemented and proposed features as of may 2023.
			resp.Header().Add("Permission-Policy", "accelerometer=(), ambient-light-sensor=(), autoplay=(), battery=(), camera=(), cross-origin-isolated=(), display-capture=(), document-domain=(), encrypted-media=(), execution-while-not-rendered=(), execution-while-out-of-viewport=(), fullscreen=(), geolocation=(), gyroscope=(), keyboard-map=(), magnetometer=(), microphone=(), midi=(), navigation-override=(), payment=(), picture-in-picture=(), publickey-credentials-get=(), screen-wake-lock=(), sync-xhr=(), usb=(), web-share=(), xr-spatial-tracking=(), clipboard-read=(), clipboard-write=(), gamepad=(), speaker-selection=()")

			if c.AzureEnableAuth {
				// these are the paths and prefixes that must not have azure authentication, in order to bootstrap the html, js, etc:
				var allowedPaths = []string{"/", "/release", "/health", "/manifest.json", "/favicon.png"}
				var allowedPrefixes = []string{"/static/js", "/static/css", "/ui"}
				if err := auth.HttpAuthMiddleWare(resp, req, jwks, c.AzureClientId, c.AzureTenantId, allowedPaths, allowedPrefixes); err != nil {
					return
				}
			}
			/**
			When the user requests any path under "/ui", we always return the same index.html (because it's a single page application).
			Anything else may be another valid rest request, like /health or /release.
			*/
			if strings.HasPrefix(req.URL.Path, "/ui") {
				http.ServeFile(resp, req, "build/index.html")
			} else {
				mux.ServeHTTP(resp, req)
			}
		}
	})
	authHandler := &Auth{
		HttpServer:  splitGrpcHandler,
		DefaultUser: defaultUser,
	}
	corsHandler := &setup.CORSMiddleware{
		PolicyFor: func(r *http.Request) *setup.CORSPolicy {
			return &setup.CORSPolicy{
				AllowMethods:     "POST",
				AllowHeaders:     "content-type,x-grpc-web,authorization",
				AllowOrigin:      c.AllowedOrigins,
				AllowCredentials: true,
			}
		},
		NextHandler: authHandler,
	}

	setup.Run(ctx, setup.ServerConfig{
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
}

type Auth struct {
	HttpServer  http.Handler
	DefaultUser auth.User
}

func getRequestAuthorFromGoogleIAP(ctx context.Context, r *http.Request) *auth.User {
	iapJWT := r.Header.Get("X-Goog-IAP-JWT-Assertion")

	if iapJWT == "" {
		// not using iap (local), default user
		logger.FromContext(ctx).Info("iap.jwt header was not found or doesn't exist")
		return nil
	}

	if c.GKEProjectNumber == "" || c.GKEBackendServiceID == "" {
		// environment variables not set up correctly
		logger.FromContext(ctx).Info("iap.jke environment variables are not set up correctly")
		return nil
	}

	aud := fmt.Sprintf("/projects/%s/global/backendServices/%s", c.GKEProjectNumber, c.GKEBackendServiceID)
	payload, err := idtoken.Validate(ctx, iapJWT, aud)
	if err != nil {
		logger.FromContext(ctx).Warn("iap.idtoken.validate", zap.Error(err))
		return nil
	}

	// here, we can use People api later to get the full name

	// get the authenticated email
	u := &auth.User{
		Email: payload.Claims["email"].(string),
	}
	return u
}

func getRequestAuthorFromAzure(r *http.Request) *auth.User {
	username := r.Header.Get("username")
	email := r.Header.Get("email")
	if username == "" || email == "" {
		return nil
	}

	u := &auth.User{
		Name:  username,
		Email: email,
	}
	return u

}

func (p *Auth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.Wrap(r.Context(), func(ctx context.Context) error {
		span, ctx := tracer.StartSpanFromContext(ctx, "ServeHTTP")
		defer span.Finish()
		var u *auth.User = nil
		var source = ""
		if c.AzureEnableAuth {
			u = getRequestAuthorFromAzure(r)
			source = "azure"
		} else {
			u = getRequestAuthorFromGoogleIAP(ctx, r)
			source = "iap"
		}
		if u != nil {
			span.SetTag("current-user-name", u.Name)
			span.SetTag("current-user-email", u.Email)
			span.SetTag("current-user-source", source)
		}
		combinedUser := auth.GetUserOrDefault(u, p.DefaultUser)

		auth.WriteUserToHttpHeader(r, combinedUser)
		ctx = auth.WriteUserToContext(ctx, combinedUser)
		ctx = auth.WriteUserToGrpcContext(ctx, combinedUser)
		p.HttpServer.ServeHTTP(w, r.WithContext(ctx))
		return nil
	})
}

// GrpcProxy passes through gRPC messages to another server.
// An alternative to the more generic methods proposed in
// https://github.com/grpc/grpc-go/issues/2297
type GrpcProxy struct {
	OverviewClient    api.OverviewServiceClient
	DeployClient      api.DeployServiceClient
	BatchClient       api.BatchServiceClient
	EnvironmentClient api.EnvironmentServiceClient
}

func (p *GrpcProxy) ProcessBatch(
	ctx context.Context,
	in *api.BatchRequest) (*emptypb.Empty, error) {
	return p.BatchClient.ProcessBatch(ctx, in)
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
	in *api.ReleaseTrainRequest) (*api.ReleaseTrainResponse, error) {
	return p.DeployClient.ReleaseTrain(ctx, in)
}

func (p *GrpcProxy) CreateEnvironment(
	ctx context.Context,
	in *api.CreateEnvironmentRequest) (*emptypb.Empty, error) {
	return p.EnvironmentClient.CreateEnvironment(ctx, in)
}
