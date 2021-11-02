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
	"log"
	"net/http"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/kelseyhightower/envconfig"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Config struct {
	CdServer string `default:"kuberpult-cd-service:8443"`
}

func RunServer() {
	var c Config
	err := envconfig.Process("kuberpult", &c)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Printf("config: %+v\n", c)
	ctx := context.Background()

	gsrv := grpc.NewServer()
	con, err := grpc.Dial(c.CdServer, grpc.WithInsecure())
	if err != nil {
		logger.WithError(err).Fatal("error dialing cd server")
	}
	gproxy := GrpcProxy{
		LockClient:     api.NewLockServiceClient(con),
		OverviewClient: api.NewOverviewServiceClient(con),
		DeployClient:   api.NewDeployServiceClient(con),
	}
	api.RegisterLockServiceServer(gsrv, &gproxy)
	api.RegisterOverviewServiceServer(gsrv, &gproxy)
	api.RegisterDeployServiceServer(gsrv, &gproxy)

	grpcProxy := runtime.NewServeMux()
	err = api.RegisterLockServiceHandlerServer(ctx, grpcProxy, &gproxy)
	if err != nil {
		log.Fatal(err.Error())
	}
	err = api.RegisterDeployServiceHandlerServer(ctx, grpcProxy, &gproxy)
	if err != nil {
		log.Fatal(err.Error())
	}

	mux := http.NewServeMux()
	mux.Handle("/environments/", grpcProxy)
	mux.Handle("/", http.FileServer(http.Dir("build")))

	httpSrv := &setup.CORSMiddleware{
		PolicyFor: func(r *http.Request) *setup.CORSPolicy {
			return &setup.CORSPolicy{
				AllowMethods:     "POST",
				AllowHeaders:     "content-type,x-grpc-web",
				AllowOrigin:      "*",
				AllowCredentials: true,
			}
		},
		NextHandler: &SplitGrpc{
			GrpcServer: gsrv,
			HttpServer: mux,
		},
	}

	setup.Run(setup.Config{
		HTTP: []setup.HTTPConfig{
			{
				Port: "8081",
				Register: func(mux *http.ServeMux) {
					mux.Handle("/", httpSrv)
				},
			},
		},
	})
}

// splits of grpc-traffic
type SplitGrpc struct {
	GrpcServer *grpc.Server
	HttpServer http.Handler
}

func (p *SplitGrpc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wrapped := grpcweb.WrapServer(p.GrpcServer)
	if wrapped.IsGrpcWebRequest(r) {
		wrapped.ServeHTTP(w, r)
	} else {
		p.HttpServer.ServeHTTP(w, r)
	}
}

type GrpcProxy struct {
	LockClient     api.LockServiceClient
	OverviewClient api.OverviewServiceClient
	DeployClient   api.DeployServiceClient
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
