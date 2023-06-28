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
package interceptors

import (
	"context"
	"github.com/MicahParks/keyfunc/v2"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func authorize(ctx context.Context, jwks *keyfunc.JWKS, clientId string, tenantId string) (*auth.User, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "Retrieving metadata failed")
	}

	authHeader, ok := md["authorization"]
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "Authorization token not supplied")
	}

	token := authHeader[0]
	claims, err := auth.ValidateToken(token, jwks, clientId, tenantId)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "Invalid authorization token provided")
	}
	u := auth.MakeDefaultUser()
	if _, ok := claims["aud"]; ok && claims["aud"] == clientId {
		u = &auth.User{
			Email: claims["email"].(string),
			Name:  claims["name"].(string),
		}
	}

	return u, nil
}

func UnaryInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
	jwks *keyfunc.JWKS,
	clientId string,
	tenantId string) (interface{}, error) {
	if info.FullMethod != "/api.v1.FrontendConfigService/GetConfig" {
		userData, err := authorize(ctx, jwks, clientId, tenantId)
		if err != nil {
			return nil, err
		}
		ctx = auth.ToContext(ctx, userData)
	}
	h, err := handler(ctx, req)
	return h, err

}

func StreamInterceptor(
	srv interface{},
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
	jwks *keyfunc.JWKS,
	clientId string,
	tenantId string,

) error {
	_, err := authorize(stream.Context(), jwks, clientId, tenantId)
	if err != nil {
		return err
	}
	return handler(srv, stream)
}
