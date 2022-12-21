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
Server infrastructure common for all microservices in the project.
It contains the code that start and configures a HTTP and/or GRPC server
correctly.
*/
package setup

/*
Example:

func main() {
	Run(ServerConfig{
		GRPCProxy: &GRPCProxyConfig{
			Port: "8000",
			Register: func(mux *runtime.ServeMux) {
				// Register your GRPC Proxy with gw.RegisterYourServiceHandlerFromEndpoint(grpcSrv, handler)
			},
		},
		GRPC: &GRPCConfig{
			Port: "10080",
			Register: func(grpcSrv *grpc.Server) {
				// Register your GRPC service with pb.RegisterXYServiceServer(grpcSrv, handler)
			},
		},
		HTTP: []HTTPConfig{
			{
				Port: "5000",
				Register: // register prometheus endpoint,
			},
			{
				Port: "8080",
				Register: // register health/live probes,
			},
			{
				Port: "80",
				Register: // register admin endpoints,
				BasicAuth: // use basic auth to secure your admin endpoint
			},
		},
		Background: []BackgroundTaskConfig{
			{
				Run: func(ctx context.Context) error {
					// start pubsub import
					return nil
				},
				Name: "Import Stuff",
				Shutdown: func(ctx context.Context) error {
					// shutdown pub/sub connection
				},
			},
		},
		Shutdown: func(ctx context.Context) error {
			// close overarching connections (e.g. db or other stuff)
		},
	})

}
*/
