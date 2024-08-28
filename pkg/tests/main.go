/*
This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com
*/
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: main <commitid>")
		return
	}
	commitid := os.Args[1]

	systemRoots, err := x509.SystemCertPool()
	if err != nil {
		msg := "failed to read CA certificates"
		panic(msg)
	}
	cred := credentials.NewTLS(&tls.Config{
		RootCAs: systemRoots,
	})
	grcpConn, err := grpc.Dial("cd-service-ojk47oig2q-ew.a.run.app:443", grpc.WithTransportCredentials(cred))
	if err != nil {
		panic(err)
	}
	defer grcpConn.Close()
	ctx := context.Background()
	user := auth.User{
		Name:  "test",
		Email: "test@test.com",
	}
	ctx = auth.WriteUserToContext(ctx, user)
	ctx = auth.WriteUserToGrpcContext(ctx, user)

	client := api.NewCommitDeploymentServiceClient(grcpConn)
	resp, err := client.GetCommitDeploymentInfo(ctx, &api.GetCommitDeploymentInfoRequest{CommitId: commitid})
	if err != nil {
		panic(err)
	}

	fmt.Println("{")
	for app, status := range resp.DeploymentStatus {
		fmt.Printf("  \"%s\": {\n", app)
		for env, deploymetStatus := range status.DeploymentStatus {
			fmt.Printf("    \"%s\": \"%s\",\n", env, deploymetStatus)
		}
		fmt.Println("  },")
	}
}
