
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
