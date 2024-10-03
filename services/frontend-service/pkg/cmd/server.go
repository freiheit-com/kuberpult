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
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	grpcerrors "github.com/freiheit-com/kuberpult/pkg/grpc"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/interceptors"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/service"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/handler"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

var c config.ServerConfig
var backendServiceId string = ""

const megaBytes int = 1024 * 1024

func getBackendServiceId(c config.ServerConfig, ctx context.Context) string {
	if c.GKEBackendServiceID == "" && c.GKEBackendServiceName == "" {
		logger.FromContext(ctx).Warn("gke environment variables are not set up correctly! missing backend_service_id or backend_service_name")
		return ""
	}

	if c.GKEBackendServiceID != "" && c.GKEBackendServiceName != "" {
		logger.FromContext(ctx).Warn("gke environment variables are not set up correctly! backend_service_id and backend_service_name cannot be set simultaneously")
		return ""
	}

	if c.GKEBackendServiceID != "" {
		return c.GKEBackendServiceID
	}
	regex, err := regexp.Compile(c.GKEBackendServiceName)
	if err != nil {
		logger.FromContext(ctx).Warn("Error compiling regex for backend_service_name: %v", zap.Error(err))
		return ""
	}
	computeService, err := compute.NewService(ctx)
	if err != nil {
		logger.FromContext(ctx).Warn("Failed to create Compute Service client: %v", zap.Error(err))
		return ""
	}
	backendServices, err := computeService.BackendServices.List(c.GKEProjectNumber).Do()
	if err != nil {
		logger.FromContext(ctx).Warn("Failed to get backend service: %v", zap.Error(err))
		return ""
	}

	serviceId := ""
	for _, backendService := range backendServices.Items {
		if regex.MatchString(backendService.Name) {
			serviceId = fmt.Sprint(backendService.Id)
		}
	}
	if serviceId == "" {
		logger.FromContext(ctx).Warn("No backend services found matching:", zap.String("pattern", c.GKEBackendServiceName))
	}
	return serviceId
}
func readAllAndClose(r io.ReadCloser, maxBytes int64) {
	_, _ = io.ReadAll(io.LimitReader(r, maxBytes))
	_ = r.Close()
}

func readPgpKeyRing() (openpgp.KeyRing, error) {
	if c.PgpKeyRingPath == "" {
		return nil, nil
	}
	file, err := os.Open(c.PgpKeyRingPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return openpgp.ReadArmoredKeyRing(file)
}

func RunServer() {
	err := logger.Wrap(context.Background(), runServer)
	if err != nil {
		fmt.Printf("error: %v %#v", err, err)
	}
}

func runServer(ctx context.Context) error {
	err := envconfig.Process("kuberpult", &c)

	if err != nil {
		logger.FromContext(ctx).Error("config.parse", zap.Error(err))
		return err
	}
	if c.GitAuthorEmail == "" {
		msg := "DefaultGitAuthorEmail must not be empty"
		logger.FromContext(ctx).Error(msg)
		return fmt.Errorf(msg)
	}
	if c.GitAuthorName == "" {
		msg := "DefaultGitAuthorName must not be empty"
		logger.FromContext(ctx).Error(msg)
		return fmt.Errorf(msg)
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
	logger.FromContext(ctx).Info("config.gke_backend_service_name: " + c.GKEBackendServiceName + "\n")
	logger.FromContext(ctx).Info(fmt.Sprintf("config.grpc_max_recv_msg_size: %d", c.GrpcMaxRecvMsgSize*megaBytes))

	if c.GKEProjectNumber != "" {
		backendServiceId = getBackendServiceId(c, ctx)
	}

	grpcServerLogger := logger.FromContext(ctx).Named("grpc_server")

	grpcStreamInterceptors := []grpc.StreamServerInterceptor{
		grpc_zap.StreamServerInterceptor(grpcServerLogger),
	}
	grpcUnaryInterceptors := []grpc.UnaryServerInterceptor{
		grpc_zap.UnaryServerInterceptor(grpcServerLogger),
	}

	var cred credentials.TransportCredentials = insecure.NewCredentials()
	if c.CdServerSecure {
		systemRoots, err := x509.SystemCertPool()
		if err != nil {
			msg := "failed to read CA certificates"
			return fmt.Errorf(msg)
		}
		//exhaustruct:ignore
		cred = credentials.NewTLS(&tls.Config{
			RootCAs: systemRoots,
		})
	}

	grpcClientOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(cred),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(c.GrpcMaxRecvMsgSize * megaBytes)),
	}

	if c.EnableTracing {
		tracer.Start()
		defer tracer.Stop()

		grpcStreamInterceptors = append(grpcStreamInterceptors,
			grpctrace.StreamServerInterceptor(grpctrace.WithServiceName(tracing.ServiceName("kuberpult-frontend-service"))),
		)
		grpcUnaryInterceptors = append(grpcUnaryInterceptors,
			grpctrace.UnaryServerInterceptor(grpctrace.WithServiceName(tracing.ServiceName("kuberpult-frontend-service"))),
		)

		grpcClientOpts = append(grpcClientOpts,
			grpc.WithStreamInterceptor(
				grpctrace.StreamClientInterceptor(grpctrace.WithServiceName(tracing.ServiceName("kuberpult-frontend-service"))),
			),
			grpc.WithUnaryInterceptor(
				grpctrace.UnaryClientInterceptor(grpctrace.WithServiceName(tracing.ServiceName("kuberpult-frontend-service"))),
			),
		)
	}

	var defaultUser = auth.User{
		DexAuthContext: nil,
		Email:          c.GitAuthorEmail,
		Name:           c.GitAuthorName,
	}

	if c.AzureEnableAuth {
		var AzureUnaryInterceptor = func(ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler) (interface{}, error) {
			return interceptors.UnaryAuthInterceptor(ctx, req, info, handler, jwks, c.AzureClientId, c.AzureTenantId)
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

	mux := http.NewServeMux()
	http.DefaultServeMux = mux
	var policy *auth.RBACPolicies
	var dexClient *auth.DexAppClient
	if c.DexEnabled {
		// Registers Dex handlers.
		dexClient, err = auth.NewDexAppClient(c.DexClientId, c.DexClientSecret, c.DexBaseURL, c.DexFullNameOverride, auth.ReadScopes(c.DexScopes), c.DexUseClusterInternalCommunication)
		if err != nil {
			logger.FromContext(ctx).Fatal("error registering dex handlers: ", zap.Error(err))
		}
		policy, err = auth.ReadRbacPolicy(true, c.DexRbacPolicyPath)
		if err != nil {
			logger.FromContext(ctx).Fatal("error getting RBAC policy: ", zap.Error(err))
		}
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
		grpc.MaxRecvMsgSize(c.GrpcMaxRecvMsgSize*megaBytes),
	)
	cdCon, err := grpc.Dial(c.CdServer, grpcClientOpts...)
	if err != nil {
		logger.FromContext(ctx).Fatal("grpc.dial.error", zap.Error(err), zap.String("addr", c.CdServer))
	}
	var rolloutClient api.RolloutServiceClient = nil
	if c.RolloutServer != "" {
		rolloutCon, err := grpc.Dial(c.RolloutServer, grpcClientOpts...)
		if err != nil {
			logger.FromContext(ctx).Fatal("grpc.dial.error", zap.Error(err), zap.String("addr", c.RolloutServer))
		}
		rolloutClient = api.NewRolloutServiceClient(rolloutCon)
	}

	batchClient := &service.BatchServiceWithDefaultTimeout{
		Inner:          api.NewBatchServiceClient(cdCon),
		DefaultTimeout: c.BatchClientTimeout,
	}

	releaseTrainPrognosisClient := api.NewReleaseTrainPrognosisServiceClient(cdCon)
	commitDeploymentsClient := api.NewCommitDeploymentServiceClient(cdCon)
	gproxy := &GrpcProxy{
		OverviewClient:              api.NewOverviewServiceClient(cdCon),
		BatchClient:                 batchClient,
		RolloutServiceClient:        rolloutClient,
		GitClient:                   api.NewGitServiceClient(cdCon),
		EnvironmentServiceClient:    api.NewEnvironmentServiceClient(cdCon),
		ReleaseTrainPrognosisClient: releaseTrainPrognosisClient,
		EslServiceClient:            api.NewEslServiceClient(cdCon),
	}
	api.RegisterOverviewServiceServer(gsrv, gproxy)
	api.RegisterBatchServiceServer(gsrv, gproxy)
	api.RegisterRolloutServiceServer(gsrv, gproxy)
	api.RegisterGitServiceServer(gsrv, gproxy)
	api.RegisterEnvironmentServiceServer(gsrv, gproxy)
	api.RegisterReleaseTrainPrognosisServiceServer(gsrv, gproxy)
	api.RegisterEslServiceServer(gsrv, gproxy)

	frontendConfigService := &service.FrontendConfigServiceServer{
		Config: config.FrontendConfig{
			ArgoCd: &config.ArgoCdConfig{
				BaseUrl:   c.ArgocdBaseUrl,
				Namespace: c.ArgocdNamespace,
			},
			Auth: &config.AuthConfig{
				AzureAuth: &config.AzureAuthConfig{
					Enabled:       c.AzureEnableAuth,
					ClientId:      c.AzureClientId,
					TenantId:      c.AzureTenantId,
					RedirectURL:   c.AzureRedirectUrl,
					CloudInstance: c.AzureCloudInstance,
				},
				DexAuthConfig: &config.DexAuthConfig{
					Enabled: c.DexEnabled,
				},
			},
			ManifestRepoUrl:  c.ManifestRepoUrl,
			SourceRepoUrl:    c.SourceRepoUrl,
			KuberpultVersion: c.Version,
			Branch:           c.GitBranch,
		},
	}

	api.RegisterFrontendConfigServiceServer(gsrv, frontendConfigService)

	grpcWebServer := grpcweb.WrapServer(gsrv)
	httpHandler := handler.Server{
		BatchClient:                 batchClient,
		RolloutClient:               rolloutClient,
		VersionClient:               api.NewVersionServiceClient(cdCon),
		ReleaseTrainPrognosisClient: releaseTrainPrognosisClient,
		CommitDeploymentsClient:     commitDeploymentsClient,
		Config:                      c,
		KeyRing:                     pgpKeyRing,
		AzureAuth:                   c.AzureEnableAuth,
	}
	restHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer readAllAndClose(req.Body, 1024)
		if c.DexEnabled {
			interceptors.DexLoginInterceptor(w, req, httpHandler.Handle, c.DexClientId, c.DexBaseURL, dexClient.DexServiceURL, policy, c.DexUseClusterInternalCommunication)
			return
		}
		httpHandler.Handle(w, req)
	})
	for _, endpoint := range []string{
		"/environments",
		"/environments/",
		"/environment-groups",
		"/environment-groups/",
		"/release",
	} {
		mux.Handle(endpoint, restHandler)
	}

	// api is only accessible via IAP for now unless explicitly disabled
	restApiHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer readAllAndClose(req.Body, 1024)
		if c.ApiEnableDespiteNoAuth {
			httpHandler.HandleAPI(w, req)
			return
		}

		if c.DexEnabled {
			interceptors.DexAPIInterceptor(w, req, httpHandler.HandleAPI, c.DexClientId, c.DexBaseURL, dexClient.DexServiceURL, policy, c.DexUseClusterInternalCommunication)
			return
		}

		if !c.IapEnabled {
			http.Error(w, "IAP not enabled, /api unavailable.", http.StatusUnauthorized)
			return
		}
		interceptors.GoogleIAPInterceptor(w, req, httpHandler.HandleAPI, backendServiceId, c.GKEProjectNumber)
	})
	for _, endpoint := range []string{
		"/api",
		"/api/",
	} {
		mux.Handle(endpoint, restApiHandler)
	}

	dexHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer readAllAndClose(req.Body, 1024)
		if !c.DexEnabled {
			http.Error(w, "Dex not enabled, /token unavailable.", http.StatusUnauthorized)
			return
		}
		httpHandler.HandleDex(w, req, dexClient)
	})
	for _, endpoint := range []string{
		"/token",
		"/token/",
	} {
		mux.Handle(endpoint, dexHandler)
	}

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
				var allowedPaths = []string{"/", "/release", "/health", "/favicon.png"}
				var allowedPrefixes = []string{"/static/js", "/static/css", "/ui"}
				if err := auth.HttpAuthMiddleWare(resp, req, jwks, c.AzureClientId, c.AzureTenantId, allowedPaths, allowedPrefixes); err != nil {
					return
				}
			}
			/**
			When the user requests any path under "/ui", we always return the same index.html (because it's a single page application).
			Anything else may be another valid rest request, like /health or /release.
			*/
			isUi := strings.HasPrefix(req.URL.Path, "/ui")
			isHtml := req.URL.Path == "/" || req.URL.Path == "/index.html"
			doNotCache := isUi || isHtml
			if doNotCache {
				resp.Header().Set("Cache-Control", "no-cache,no-store,must-revalidate,max-age=0")
			} else {
				resp.Header().Set("Cache-Control", "max-age=604800") // 7 days
			}
			if isUi {
				// this is called for example for requests to /ui, /ui/home
				http.ServeFile(resp, req, "build/index.html")
			} else {
				// this is called for example for requests to /, /index.html,css and js
				mux.ServeHTTP(resp, req)
			}
		}
	})
	authHandler := &Auth{
		HttpServer:  splitGrpcHandler,
		DefaultUser: defaultUser,
		KeyRing:     pgpKeyRing,
		Policy:      policy,
	}
	corsHandler := &setup.CORSMiddleware{
		PolicyFor: func(r *http.Request) *setup.CORSPolicy {
			return &setup.CORSPolicy{
				MaxAge:           0,
				AllowMethods:     "POST",
				AllowHeaders:     "content-type,x-grpc-web,authorization",
				AllowOrigin:      c.AllowedOrigins,
				AllowCredentials: true,
			}
		},
		NextHandler: authHandler,
	}

	setup.Run(ctx, setup.ServerConfig{
		GRPC:       nil,
		Background: nil,
		Shutdown:   nil,
		HTTP: []setup.HTTPConfig{
			{
				BasicAuth: nil,
				Shutdown:  nil,
				Port:      "8081",
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
	// KeyRing is as of now required because we do not have technical users yet. So we protect public endpoints by requiring a signature
	KeyRing openpgp.KeyRing
	Policy  *auth.RBACPolicies
}

func getRequestAuthorFromGoogleIAP(ctx context.Context, r *http.Request) *auth.User {
	iapJWT := r.Header.Get("X-Goog-IAP-JWT-Assertion")
	if iapJWT == "" {
		// not using iap (local), default user
		logger.FromContext(ctx).Info("iap.jwt header was not found or doesn't exist")
		return nil
	}

	if backendServiceId == "" {
		logger.FromContext(ctx).Warn("Failed to get backend_service_id! Author information will be lost. Make sure gke environment variables are set up correctly.")
		return nil
	}

	aud := fmt.Sprintf("/projects/%s/global/backendServices/%s", c.GKEProjectNumber, backendServiceId)
	payload, err := idtoken.Validate(ctx, iapJWT, aud)
	if err != nil {
		logger.FromContext(ctx).Warn("iap.idtoken.validate", zap.Error(err))
		return nil
	}

	// here, we can use People api later to get the full name

	// get the authenticated email
	u := &auth.User{
		Name:           "",
		DexAuthContext: nil,
		Email:          payload.Claims["email"].(string),
	}
	return u
}

func getRequestAuthorFromAzure(ctx context.Context, r *http.Request) (*auth.User, error) {
	return auth.ReadUserFromHttpHeader(ctx, r)
}

func (p *Auth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := logger.Wrap(r.Context(), func(ctx context.Context) error {
		span, ctx := tracer.StartSpanFromContext(ctx, "ServeHTTP")
		defer span.Finish()
		var user *auth.User = nil
		var err error
		var source string
		if c.AzureEnableAuth {
			user, err = getRequestAuthorFromAzure(ctx, r)
			if err != nil {
				return err
			}
			source = "azure"
		} else {
			user = getRequestAuthorFromGoogleIAP(ctx, r)
			source = "iap"
		}
		if c.DexEnabled {
			source = "dex"
			dexServiceURL := auth.GetDexServiceURL(c.DexFullNameOverride)
			dexAuthContext := getUserFromDex(w, r, c.DexClientId, c.DexBaseURL, dexServiceURL, p.Policy, c.DexUseClusterInternalCommunication)
			if dexAuthContext == nil {
				logger.FromContext(ctx).Info(fmt.Sprintf("No role assigned from Dex user: %v", user))
			} else {
				if user == nil {
					user = &p.DefaultUser
				}
				user.DexAuthContext = dexAuthContext
				logger.FromContext(ctx).Info(fmt.Sprintf("Dex user: %v - role: %v", user, user.DexAuthContext.Role))
			}
		}
		if user != nil {
			span.SetTag("current-user-name", user.Name)
			span.SetTag("current-user-email", user.Email)
			span.SetTag("current-user-source", source)
		}
		combinedUser := auth.GetUserOrDefault(user, p.DefaultUser)

		auth.WriteUserToHttpHeader(r, combinedUser)
		ctx = auth.WriteUserToContext(ctx, combinedUser)
		ctx = auth.WriteUserToGrpcContext(ctx, combinedUser)
		if user != nil && user.DexAuthContext != nil {
			for _, role := range user.DexAuthContext.Role {
				ctx = auth.WriteUserRoleToGrpcContext(ctx, role)
			}
		}
		p.HttpServer.ServeHTTP(w, r.WithContext(ctx))
		return nil
	})
	if err != nil {
		fmt.Printf("error: %v %#v", err, err)
	}
}

func getUserFromDex(w http.ResponseWriter, req *http.Request, clientID, baseURL, dexServiceURL string, policy *auth.RBACPolicies, useClusterInternalCommunication bool) *auth.DexAuthContext {
	httpCtx, err := interceptors.GetContextFromDex(w, req, clientID, baseURL, dexServiceURL, policy, useClusterInternalCommunication)
	if err != nil {
		return nil
	}
	headerRole64 := req.Header.Get(auth.HeaderUserRole)
	headerRole, err := auth.Decode64(headerRole64)
	if err != nil {
		logger.FromContext(httpCtx).Info("could not decode user role")
		return nil
	}
	return &auth.DexAuthContext{Role: strings.Split(headerRole, ",")}
}

// GrpcProxy passes through gRPC messages to another server.
// This is needed for the UI to communicate with other services via gRPC over web.
// The UI _only_ communicates via gRPC over web (+ static files), while the REST API is only intended for automated processes like build pipelines.
// An alternative to the more generic methods proposed in
// https://github.com/grpc/grpc-go/issues/2297
type GrpcProxy struct {
	OverviewClient              api.OverviewServiceClient
	BatchClient                 api.BatchServiceClient
	RolloutServiceClient        api.RolloutServiceClient
	GitClient                   api.GitServiceClient
	EnvironmentServiceClient    api.EnvironmentServiceClient
	ReleaseTrainPrognosisClient api.ReleaseTrainPrognosisServiceClient
	EslServiceClient            api.EslServiceClient
}

func (p *GrpcProxy) ProcessBatch(
	ctx context.Context,
	in *api.BatchRequest) (*api.BatchResponse, error) {
	for i := range in.Actions {
		batchAction := in.GetActions()[i]
		switch batchAction.Action.(type) {
		case *api.BatchAction_CreateRelease:
			return nil, grpcerrors.PublicError(ctx, fmt.Errorf("action create-release is only supported via http in the frontend-service"))
		}
	}

	return p.BatchClient.ProcessBatch(ctx, in)
}

func (p *GrpcProxy) GetFailedEsls(
	ctx context.Context,
	in *api.GetFailedEslsRequest) (*api.GetFailedEslsResponse, error) {
	return p.EslServiceClient.GetFailedEsls(ctx, in)
}

func (p *GrpcProxy) GetAppDetails(
	ctx context.Context,
	in *api.GetAppDetailsRequest) (*api.GetAppDetailsResponse, error) {
	return p.OverviewClient.GetAppDetails(ctx, in)
}

func (p *GrpcProxy) GetOverview(
	ctx context.Context,
	in *api.GetOverviewRequest) (*api.GetOverviewResponse, error) {
	return p.OverviewClient.GetOverview(ctx, in)
}

func (p *GrpcProxy) GetAppDetails(
	ctx context.Context,
	in *api.GetAppDetailsRequest) (*api.GetAppDetailsResponse, error) {
	return p.OverviewClient.GetAppDetails(ctx, in)
}

func (p *GrpcProxy) GetGitTags(
	ctx context.Context,
	in *api.GetGitTagsRequest) (*api.GetGitTagsResponse, error) {
	return p.GitClient.GetGitTags(ctx, in)
}

func (p *GrpcProxy) GetProductSummary(
	ctx context.Context,
	in *api.GetProductSummaryRequest) (*api.GetProductSummaryResponse, error) {
	return p.GitClient.GetProductSummary(ctx, in)
}

func (p *GrpcProxy) GetCommitInfo(
	ctx context.Context,
	in *api.GetCommitInfoRequest) (*api.GetCommitInfoResponse, error) {
	return p.GitClient.GetCommitInfo(ctx, in)
}

func (p *GrpcProxy) GetEnvironmentConfig(
	ctx context.Context,
	in *api.GetEnvironmentConfigRequest) (*api.GetEnvironmentConfigResponse, error) {
	return p.EnvironmentServiceClient.GetEnvironmentConfig(ctx, in)
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

func (p *GrpcProxy) StreamChangedApps(
	in *api.GetChangedAppsRequest,
	stream api.OverviewService_StreamChangedAppsServer) error {
	if resp, err := p.OverviewClient.StreamChangedApps(stream.Context(), in); err != nil {
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

func (p *GrpcProxy) StreamStatus(in *api.StreamStatusRequest, stream api.RolloutService_StreamStatusServer) error {
	if p.RolloutServiceClient == nil {
		return status.Error(codes.Unimplemented, "rollout service not configured")
	}
	if resp, err := p.RolloutServiceClient.StreamStatus(stream.Context(), in); err != nil {
		return err
	} else {
		for {
			item, err := resp.Recv()
			if err != nil {
				return err
			}
			err = stream.Send(item)
			if err != nil {
				return err
			}
		}
	}
}

func (p *GrpcProxy) GetStatus(ctx context.Context, in *api.GetStatusRequest) (*api.GetStatusResponse, error) {
	if p.RolloutServiceClient == nil {
		return nil, status.Error(codes.Unimplemented, "rollout service not configured")
	}
	return p.RolloutServiceClient.GetStatus(ctx, in)
}

func (p *GrpcProxy) GetReleaseTrainPrognosis(ctx context.Context, in *api.ReleaseTrainRequest) (*api.GetReleaseTrainPrognosisResponse, error) {
	if p.ReleaseTrainPrognosisClient == nil {
		logger.FromContext(ctx).Error("release train prognosis service received a request when it is not configured")
		return nil, status.Error(codes.Internal, "release train prognosis service not configured")
	}
	return p.ReleaseTrainPrognosisClient.GetReleaseTrainPrognosis(ctx, in)
}
