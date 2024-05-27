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

package interceptors

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"google.golang.org/api/idtoken"
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
		// this happens if the caller does not pass the "authHeader".
		// correct example: api.overviewService().StreamOverview({}, authHeader)
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

// GoogleIAPInterceptor intercepts HTTP calls to the frontend service.
// If the user us not logged in or no JWT is found, an Unauthenticated error is returned.
func GoogleIAPInterceptor(
	w http.ResponseWriter,
	req *http.Request,
	httpHandler http.HandlerFunc,
	backendServiceId, gkeProjectNumber string,
) {
	iapJWT := req.Header.Get("X-Goog-IAP-JWT-Assertion")
	if iapJWT == "" {
		http.Error(w, "iap.jwt header was not found or doesn't exist", http.StatusUnauthorized)
		return
	}

	aud := fmt.Sprintf("/projects/%s/global/backendServices/%s", gkeProjectNumber, backendServiceId)
	// NOTE: currently we just validate that the token exists, but no handlers are using data from the payload.
	// This might change in the future.
	_, err := idtoken.Validate(req.Context(), iapJWT, aud)
	if err != nil {
		http.Error(w, "iap.jwt could not be validated", http.StatusUnauthorized)
		return
	}
	httpHandler(w, req)
}

func AddRoleToContext(httpCtx context.Context, w http.ResponseWriter, req *http.Request, userGroup string, policy *auth.RBACPolicies) context.Context {
	for _, policyGroup := range policy.Groups {
		if policyGroup.Group == userGroup {
			auth.WriteUserRoleToHttpHeader(req, policyGroup.Role)
			httpCtx = auth.WriteUserRoleToGrpcContext(req.Context(), policyGroup.Role)
		}
	}
	return httpCtx
}

// DexLoginInterceptor intercepts HTTP calls to the frontend service.
// DexLoginInterceptor must only be used if dex is enabled.
// If the user us not logged in, it redirected the calls to the Dex login page.
// If the user is already logged in, proceeds with the request.
func DexLoginInterceptor(
	w http.ResponseWriter,
	req *http.Request,
	httpHandler http.HandlerFunc,
	clientID, baseURL string, DexRbacPolicy *auth.RBACPolicies, useClusterInternalCommunication bool,
) {
	httpCtx, err := GetContextFromDex(w, req, clientID, baseURL, DexRbacPolicy, useClusterInternalCommunication)
	if err != nil {
		logger.FromContext(req.Context()).Debug(fmt.Sprintf("Error verifying token for Dex: %s", err))
		// If user is not authenticated redirect to the login page.
		http.Redirect(w, req, auth.LoginPATH, http.StatusFound)
		return
	}
	req = req.WithContext(httpCtx)
	httpHandler(w, req)
}

func GetContextFromDex(w http.ResponseWriter, req *http.Request, clientID, baseURL string, DexRbacPolicy *auth.RBACPolicies, useClusterInternalCommunication bool) (context.Context, error) {
	claims, err := auth.VerifyToken(req.Context(), req, clientID, baseURL, useClusterInternalCommunication)
	if err != nil {
		logger.FromContext(req.Context()).Info(fmt.Sprintf("Error verifying token for Dex: %s", err))
		return req.Context(), err
	}
	httpCtx := req.Context()
	// switch case to handle multiple types of claims that can be extracted from the Dex Response
	switch val := claims["groups"].(type) {
	case []interface{}:
		for _, group := range val {
			groupName := strings.Trim(group.(string), "\"")
			httpCtx = AddRoleToContext(httpCtx, w, req, groupName, DexRbacPolicy)
		}
	case []string:
		httpCtx = AddRoleToContext(httpCtx, w, req, strings.Join(val, ","), DexRbacPolicy)
	case string:
		httpCtx = AddRoleToContext(httpCtx, w, req, val, DexRbacPolicy)
	}

	if claims["email"].(string) != "" {
		httpCtx = AddRoleToContext(httpCtx, w, req, claims["email"].(string), DexRbacPolicy)
	} else if claims["groups"] == nil {
		return nil, fmt.Errorf("unable to parse token with expected fields for DEX login")
	}
	test := "-----BEGIN RSA PRIVATE KEY-----\nthisisnotarealsecretMIIEpgIBAAKCAQEApo/dIQcfxH2jQFyP7/cIIKnylAOehtl9fSs+a/nBmFXtdr+h\nHhKVqWJqG2niVMrMwgvpNTwKxrelrVGYmEHbVmkHUvTshr94KQn2NQmYE9+APoYn\nkbdd7VHssJLYzMkaSdOsPpdTO55xgKOhUE1e7Emxy3OFeDLIXi4p4hHnXNJZVgaC\nVMYvNNXt28WnE0Wlk6REifYbkd/ZirdomcsPSuXL/mWLrcS79IVBVtrhbma6Dxpo\nn0FbkQtXSgeLdCCj/fGuVju4qJ1jEHJubMuVPOB8C3svFJGznCTftBxIYeDxdKM4\n/gI+wCGa4wcQE8UbtscAmHKowl5e/n3H282A/QIDAQABAoIBAQCWcZ0/Jssn1H9v\nM+wCyDN8JWTpEnfOv4WcWEHyj02wxfRN7PqfShQKQc0rY7E9+0uE/fMv1UK6iMhJ\ny0i0Rc/Std0argUyFKF5F/ldoIPPd3HDh7MX5/Mb14KdXpYaKo7nHh0XD+HrGMrW\ncXXULX5OmKXR6U8l6WCXyMSl4JwEdrcb2pwTsBU19fsLNhX8tP6eZC1gDYUOoGNd\nBVCnS8WYxLfxESzETQbgt65ozh+wuRUzUIhnawmDaWJCuCZ48+nCBJ5ujIPSfh+2\n+ZZIwDQCL2bn8dAR/YTYLHMdeuxCvva1K2+7T1OcCx+ZKS/jGEneI5pkkyZANsHj\nWmY2Ldh1AoGBAOfDzikBNi9/pNXD3slwuqXrhtMv2XGIdCJh4L6xoPhzPBeaIJ+V\n1ilYgyT6WtwNhq9p7CK4wO2k2ZwHiggH+Ql1KJ+jkCnj/kVSuB7j2E2GixSNKo5m\n2updwrroc8N+elcFQHfXzikN0ChE0h9JbDmeHUDJ4KEKwr+pXuBSrZtHAoGBALf6\nnozgHCHYwLv9V+UeFz0AMUkUWdG04B+IvWrl2U8OhkllPMnInjXl11xVmHdh2WUV\n89foDWWzD60MYMQuED9L8I++9g2dkMUxU071VEnMpJWpjNKsPtHnLdb1x9S6UdEc\nkCEqxj8jcc1iI0YmYVIhQROUZ8QFcq0AmzygMBubAoGBAJLtO+Yc4YgNKKdn6/XS\nZFFJVgjOHdBuzAj7+emKXF0FWMQxrprc947wkPtBR5aXcJoF0XBVpeFCD75tvSDI\nRSWsw1so6vzTj9/Mx/K1SOwk7kjSEXeDVyca15d8Q99ccBx4tN0Ez6qRGjRdJMQ2\n3MhOJ4dqM+CEHOA6dG5Lm8mDAoGBAK5cgC0lLKRLR4YiuU10cjOm3g7Tkbh0gsCA\nGHyaL5SEQHKI1s6qKn8MQEnK+X+TJbRu1LW3wBK1XFL12zOyMEW809V39ru6q/yn\nHbxEN8jlgMoycTsscTD/tur17phGqMnVFyfH4TDvh6hNrP6L20o6J/HFgX4+Z4tc\nesM/UbinAoGBAKdtl8TV9kkVWo923njpmDc1QUkTS9OLs/bEeEGn4wtrZXpN42Pf\nBC/+9+wOwKKyqJySHbd7M7tmRxKBGeRabgKfhRN0FrvPaZzRoLVc89y2aaws7EQ8\njRe6DZTpZ9K9rIDlzK1qwkYE7yvtUy52TZ+kBekRv+1pLOE3rwTloRyT\n-----END RSA PRIVATE KEY-----"
	fmt.Println(test)
	return httpCtx, nil
}
