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
//
// Setup implementation shared between all microservices.
// If this file is changed it will affect _all_ microservices in the monorepo (and this
// is deliberately so).
//
package setup

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"go.uber.org/zap"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"google.golang.org/grpc"
)

var (
	shutdownChannel = make(chan bool, 1)      // Write here for a manual shutdown
	osSignalChannel = make(chan os.Signal, 1) // System writes here when shutdown
)

func init() {
	signal.Notify(osSignalChannel, syscall.SIGINT, syscall.SIGTERM)
}

type shutdown struct {
	name string
	fn   func(context.Context) error
}

// Setup structure that holds only the shutdown callbacks for all
// grpc and http server for endpoints, metrics, health checks, etc.
type setup struct {
	shutdown []shutdown
}

type BasicAuth struct {
	Username string
	Password string
}

type GRPCConfig struct {
	// required
	Port     string
	Register func(*grpc.Server)
	Opts     []grpc.ServerOption

	// optional
	Shutdown func(context.Context) error
}

type GRPCProxyConfig struct {
	// required
	Port     string
	Register func(*runtime.ServeMux)

	// optional
	BasicAuth *BasicAuth
	Shutdown  func(context.Context) error
}

type HTTPConfig struct {
	// required
	Port     string
	Register func(*http.ServeMux)

	// optional
	BasicAuth *BasicAuth
	Shutdown  func(context.Context) error
}

type BackgroundTaskConfig struct {
	// a function that triggers a graceful shutdown of all other resources after completion
	Run  func(context.Context) error
	Name string
	// optional
	Shutdown func(context.Context) error
}

// Config contains configurations for all servers & tasks will be started.
// A startup order is not guaranteed.
type Config struct {
	GRPCProxy *GRPCProxyConfig
	GRPC      *GRPCConfig
	HTTP      []HTTPConfig
	// BackgroundTasks are tasks that are running forever, like Pub/sub receiver. If they
	// finish, a graceful shutdown will be triggered.
	Background []BackgroundTaskConfig
	Shutdown   func(context.Context) error
}

func Run(ctx context.Context, config Config) {
	s := &setup{}

	ctx, cancel := context.WithCancel(ctx)

	// Start the listening on each protocol
	for _, cfg := range config.HTTP {
		setupHTTP(ctx, s, cfg)
	}
	if config.GRPC != nil {
		setupGRPC(ctx, s, *config.GRPC)
	}
	if config.GRPCProxy != nil {
		setupProxy(ctx, s, *config.GRPCProxy)
	}
	for _, task := range config.Background {
		setupBackgroundTask(ctx, s, task)
	}

	if config.Shutdown != nil {
		s.RegisterShutdown(
			"global shutdown handler",
			config.Shutdown,
		)
	}

	// Listening for shutdown signal
	s.listenToShutdownSignal(ctx, cancel)
}

func (s *setup) RegisterShutdown(name string, shutdownFN func(ctx context.Context) error) {
	s.shutdown = append(s.shutdown, shutdown{name: name, fn: shutdownFN})
}

func (s *setup) listenToShutdownSignal(ctx context.Context, cancelFunc context.CancelFunc) {
	// Wait for a signal to shutdown all servers.
	// This should be a blocking call, because program will exit as soon as
	// the main goroutine returns (so it doesn't wait for other goroutines).
	// Non-blocking call could lead to unfinished cleanup during the shutdown.
	// See also https://golang.org/ref/spec#Program_execution
	select {
	case <-osSignalChannel:
	case <-shutdownChannel:
	}

	// cancel the context
	cancelFunc()

	// call shutdown hooks
	gracefulShutdown(ctx, s, 30*time.Second)
}

func gracefulShutdown(ctx context.Context, s *setup, timeout time.Duration) {
	// Instantiate background context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for i := len(s.shutdown) - 1; i >= 0; i-- {
		sd := s.shutdown[i]
		if err := sd.fn(ctx); err != nil {
			logger.FromContext(ctx).Error("shutdown.failed", zap.Error(err), zap.String("handler", sd.name))
		}
	}
}

func setupGRPC(ctx context.Context, s *setup, config GRPCConfig) {
	// Get service listening port
	addrGRPC := ":" + config.Port

	// Setup a listener for gRPC port
	grpcL, err := net.Listen("tcp", addrGRPC)
	if err != nil {
		logger.FromContext(ctx).Panic("grpc.listen.error", zap.Error(err), zap.String("addr", addrGRPC))
		return
	}
	s.RegisterShutdown("GRPC net listener", func(context.Context) error {
		return grpcL.Close()
	})

	// Instantiate gRPC server
	grpcS := grpc.NewServer(config.Opts...)
	s.RegisterShutdown("GRPC server", func(context.Context) error {
		grpcS.GracefulStop()
		return nil
	})

	config.Register(grpcS)
	if config.Shutdown != nil {
		s.RegisterShutdown("GRPC shutdown handler", config.Shutdown)
	}

	go serveGRPC(ctx, grpcS, grpcL)
}

func serveGRPC(ctx context.Context, grpcS *grpc.Server, grpcL net.Listener) {
	defer func() {
		shutdownChannel <- true
	}()

	if err := grpcS.Serve(grpcL); err != nil {
		logger.FromContext(ctx).Error("grpc.serve.error", zap.Error(err))
	}
}

func setupProxy(ctx context.Context, s *setup, config GRPCProxyConfig) {
	mux := runtime.NewServeMux(runtime.WithIncomingHeaderMatcher(ForwardSessionMatcher))
	config.Register(mux)

	runHTTPHandler(ctx, s, mux, config.Port, config.BasicAuth, config.Shutdown)
}

func setupHTTP(ctx context.Context, s *setup, config HTTPConfig) {
	mux := http.NewServeMux()
	config.Register(mux)

	runHTTPHandler(ctx, s, mux, config.Port, config.BasicAuth, config.Shutdown)
}

func runHTTPHandler(ctx context.Context, s *setup, handler http.Handler, port string, basicAuth *BasicAuth, shutdown func(context.Context) error) {

	if basicAuth != nil {
		handler = NewBasicAuthHandler(basicAuth, handler)
	}

	httpS := &http.Server{
		Handler: handler,
	}
	s.RegisterShutdown(
		fmt.Sprintf("http server on %s", port),
		func(ctx context.Context) error {
			return shutdownHTTP(ctx, httpS)
		},
	)

	if shutdown != nil {
		s.RegisterShutdown(
			fmt.Sprintf("http shutdown handler on %s", port),
			shutdown,
		)
	}

	go serveHTTP(ctx, httpS, port)
}

var shutdownHTTP = func(ctx context.Context, httpS *http.Server) error {
	return httpS.Shutdown(ctx)
}

var serveHTTP = func(ctx context.Context, httpS *http.Server, port string) {
	// if this function returns, the server was stopped, so stop also all the other services
	defer func() {
		shutdownChannel <- true
	}()

	addr := ":" + port

	httpL, err := net.Listen("tcp", addr)
	if err != nil {
		logger.FromContext(ctx).Panic("http.listen.error",zap.Error(err),zap.String("addr", addr))
		return
	}

	if err := httpS.Serve(httpL); err != nil && err != http.ErrServerClosed {
		logger.FromContext(ctx).Error("http.serve.error",zap.Error(err))
	}
}

func setupBackgroundTask(ctx context.Context, s *setup, config BackgroundTaskConfig) {

	if config.Shutdown != nil {
		s.RegisterShutdown(
			fmt.Sprintf("shutdown handler for %s", config.Name),
			config.Shutdown,
		)
	}

	go runBackgroundTask(ctx, config)
}

func runBackgroundTask(ctx context.Context, config BackgroundTaskConfig) {
	defer func() {
		shutdownChannel <- true
	}()
	if err := config.Run(ctx); err != nil {
		logger.FromContext(ctx).Error("background.error", zap.Error(err), zap.String("job",config.Name))
	}
}

// For header mappings to metadata, we want the defaults plus the SessionId (always forwarded lowercase).
func ForwardSessionMatcher(key string) (string, bool) {
	lowerKey := strings.ToLower(key)
	if lowerKey == "x-sessionid" {
		return lowerKey, true
	}
	return runtime.DefaultHeaderMatcher(key)
}

func NewBasicAuthHandler(basicAuth *BasicAuth, chainedHandler http.Handler) http.Handler {
	return &BasicAuthHandler{
		basicAuth:      basicAuth,
		chainedHandler: chainedHandler,
	}
}

type BasicAuthHandler struct {
	basicAuth      *BasicAuth
	chainedHandler http.Handler
}

func (h *BasicAuthHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	reqUser, reqPass, ok := req.BasicAuth()
	if !ok || subtle.ConstantTimeCompare([]byte(reqUser), []byte(h.basicAuth.Username)) != 1 || subtle.ConstantTimeCompare([]byte(reqPass), []byte(h.basicAuth.Password)) != 1 {
		rw.Header().Set("WWW-Authenticate", `Basic realm="Please enter credentials"`)
		rw.WriteHeader(401)
		_, _ = rw.Write([]byte("Unauthorised.\n"))
		return
	}
	h.chainedHandler.ServeHTTP(rw, req)
}
