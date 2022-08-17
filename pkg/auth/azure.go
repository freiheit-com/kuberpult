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
package auth

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/MicahParks/keyfunc"
	jwt "github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var Jwks *keyfunc.JWKS = nil
var clientId string
var tenantId string

func JWKSInitAzure(ctx context.Context, _clientId string, _tenantId string) error {
	clientId = _clientId
	tenantId = _tenantId
	jwksURL := "https://login.microsoftonline.com/common/discovery/v2.0/keys"
	options := keyfunc.Options{
		Ctx: ctx,
		RefreshErrorHandler: func(err error) {
			log.Printf("There was an error with the jwt.Keyfunc. Error: %s", err.Error())
		},
		RefreshInterval:   time.Hour,
		RefreshRateLimit:  time.Minute * 5,
		RefreshTimeout:    time.Second * 10,
		RefreshUnknownKID: true,
	}
	var err error
	Jwks, err = keyfunc.Get(jwksURL, options)
	if err != nil {
		return fmt.Errorf("Failed to create JWKS from resource at the given URL. Error: %s", err.Error())
	}
	return nil
}

func ValidateToken(jwtB64 string) error {
	var token *jwt.Token
	if Jwks == nil {
		return fmt.Errorf("JWKS not initialized.")
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(jwtB64, claims, Jwks.Keyfunc)
	if err != nil {
		return fmt.Errorf("Failed to parse the JWT.\nError: %s", err.Error())
	}
	if !token.Valid {
		return fmt.Errorf("Invalid token provided.")
	}
	if val, ok := claims["aud"]; ok {
		if val != clientId {
			return fmt.Errorf("Unknown client id provided: %s", val)
		}
	} else {
		return fmt.Errorf("Client id not found in token.")
	}

	if val, ok := claims["tid"]; ok {
		if val != tenantId {
			return fmt.Errorf("Unknown tenant id provided: %s", val)
		}
	} else {
		return fmt.Errorf("Tenant id not found in token.")
	}

	return nil
}

func authorize(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.InvalidArgument, "Retrieving metadata failed")
	}

	authHeader, ok := md["authorization"]
	if !ok {
		return status.Errorf(codes.Unauthenticated, "Authorization token not supplied")
	}

	token := authHeader[0]
	err := ValidateToken(token)

	if err != nil {
		return status.Errorf(codes.Unauthenticated, err.Error())
	}
	return nil
}

func UnaryInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {

	if info.FullMethod != "/api.v1.FrontendConfigService/GetConfig" {
		err := authorize(ctx)
		if err != nil {
			return nil, err
		}
	}

	h, err := handler(ctx, req)
	return h, err
}

func StreamInterceptor(
	srv interface{},
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	err := authorize(stream.Context())
	if err != nil {
		return err
	}
	return handler(srv, stream)
}
