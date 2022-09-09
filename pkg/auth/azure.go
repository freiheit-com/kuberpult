package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc"
	jwt "github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func JWKSInitAzure(ctx context.Context) (*keyfunc.JWKS, error) {
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
	jwks, err := keyfunc.Get(jwksURL, options)
	if err != nil {
		return nil, fmt.Errorf("Failed to create JWKS from resource at the given URL. Error: %s", err.Error())
	}
	return jwks, nil
}

func ValidateToken(jwtB64 string, jwks *keyfunc.JWKS, clientId string, tenantId string) (jwt.MapClaims, error) {
	var token *jwt.Token
	if jwks == nil {
		return nil, fmt.Errorf("JWKS not initialized.")
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(jwtB64, claims, jwks.Keyfunc)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse the JWT.\nError: %s", err.Error())
	}
	if !token.Valid {
		return nil, fmt.Errorf("Invalid token provided.")
	}
	if val, ok := claims["aud"]; ok {
		if val != clientId {
			return nil, fmt.Errorf("Unknown client id provided: %s", val)
		}
	} else {
		return nil, fmt.Errorf("Client id not found in token.")
	}

	if val, ok := claims["tid"]; ok {
		if val != tenantId {
			return nil, fmt.Errorf("Unknown tenant id provided: %s", val)
		}
	} else {
		return nil, fmt.Errorf("Tenant id not found in token.")
	}

	return claims, nil
}

func authorize(ctx context.Context, jwks *keyfunc.JWKS, clientId string, tenantId string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.InvalidArgument, "Retrieving metadata failed")
	}

	authHeader, ok := md["authorization"]
	if !ok {
		return status.Errorf(codes.Unauthenticated, "Authorization token not supplied")
	}

	token := authHeader[0]
	_, err := ValidateToken(token, jwks, clientId, tenantId)

	if err != nil {
		return status.Errorf(codes.Unauthenticated, "Invalid authorization token provided")
	}
	return nil
}

func UnaryInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
	jwks *keyfunc.JWKS,
	clientId string,
	tenantId string) (interface{}, error) {
	if info.FullMethod != "/api.v1.FrontendConfigService/GetConfig" {
		err := authorize(ctx, jwks, clientId, tenantId)
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
	jwks *keyfunc.JWKS,
	clientId string,
	tenantId string,

) error {
	err := authorize(stream.Context(), jwks, clientId, tenantId)
	if err != nil {
		return err
	}
	return handler(srv, stream)
}

func HttpAuthMiddleWare(resp http.ResponseWriter, req *http.Request, jwks *keyfunc.JWKS, clientId string, tenantId string, allowedPaths []string, allowedPrefixes []string) error {
	token := req.Header.Get("authorization")
	for _, allowedPath := range allowedPaths {
		if req.URL.Path == allowedPath {
			return nil
		}
	}
	for _, allowedPrefix := range allowedPrefixes {
		if strings.HasPrefix(req.URL.Path, allowedPrefix) {
			return nil
		}
	}

	claims, err := ValidateToken(token, jwks, clientId, tenantId)

	if _, ok := claims["tid"]; ok {
		req.Header.Set("username", claims["name"].(string))
		req.Header.Set("email", claims["email"].(string))

	}
	if err != nil {
		resp.WriteHeader(http.StatusUnauthorized)
		resp.Write([]byte("Invalid authorization header provided"))
	}
	return err
}
