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

package cloudrun

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var cloudRunServiceClient api.CloudRunServiceClient = nil

func InitCloudRunClient(server string) error {
	if cloudRunServiceClient != nil {
		return fmt.Errorf("failed to initialize cloudrunClient: already initialized")
	}
	systemRoots, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("failed to read CA certificates: %s", err)
	}
	//exhaustruct:ignore
	cred := credentials.NewTLS(&tls.Config{
		RootCAs: systemRoots,
	})

	grpcClientOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(cred),
	}

	con, err := grpc.Dial(server, grpcClientOpts...)
	if err != nil {
		return fmt.Errorf("error dialing %s: %w", server, err)
	}
	cloudRunServiceClient = api.NewCloudRunServiceClient(con)
	return nil
}

func IsInitialized() bool {
	return cloudRunServiceClient != nil
}

func Deploy(ctx context.Context, manifest []byte) error {
	_, err := cloudRunServiceClient.Deploy(ctx, &api.ServiceDeployRequest{Manifest: manifest})
	return err
}
