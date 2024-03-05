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
	"net/http"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/handler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authorize returns an error when the authentication failed
// Note that it may return (nil,nil) if the authentication was ok, but had no userdata.
// Never returns the default user
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
	// here, everything is valid, but we way still have empty strings, so we use the defaultUser here
	var u *auth.User = nil
	if _, ok := claims["aud"]; ok && claims["aud"] == clientId {
		u = &auth.User{
			DexAuthContext: nil,
			Email:          claims["email"].(string),
			Name:           claims["name"].(string),
		}
	}

	return u, nil
}

func UnaryAuthInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
	jwks *keyfunc.JWKS,
	clientId string,
	tenantId string) (interface{}, error) {
	if info.FullMethod != "/api.v1.FrontendConfigService/GetConfig" {
		_, err := authorize(ctx, jwks, clientId, tenantId)
		if err != nil {
			return nil, err
		}
	}
	h, err := handler(ctx, req)
	return h, err
}

func StreamAuthInterceptor(
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

// DexLoginInterceptor intercepts HTTP calls to the frontend service.
// DexLoginInterceptor must only be used if dex is enabled.
// If the user us not logged in, it redirected the calls to the Dex login page.
// If the user is already logged in, proceeds with the request.
func DexLoginInterceptor(
	w http.ResponseWriter,
	req *http.Request,
	httpHandler handler.Server,
	clientID, baseURL string,
) {
	role, err := auth.VerifyToken(req.Context(), req, clientID, baseURL)
	if err != nil {
		// If user is not authenticated redirect to the login page.
		http.Redirect(w, req, auth.LoginPATH, http.StatusFound)
	}
	auth.WriteUserRoleToHttpHeader(req, role)
	httpCtx := auth.WriteUserRoleToGrpcContext(req.Context(), role)
	req = req.WithContext(httpCtx)
	httpHandler.Handle(w, req)
}
