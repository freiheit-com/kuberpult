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

package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	jwt "github.com/golang-jwt/jwt/v5"

	xpath "github.com/freiheit-com/kuberpult/pkg/path"
)

func JWKSInitAzure(ctx context.Context) (*keyfunc.JWKS, error) {
	jwksURL := "https://login.microsoftonline.com/common/discovery/v2.0/keys"
	//exhaustruct:ignore
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
		return nil, fmt.Errorf("failed to create JWKS from resource at the given URL: %w", err)
	}
	return jwks, nil
}

func ValidateToken(jwtB64 string, jwks *keyfunc.JWKS, clientId string, tenantId string) (jwt.MapClaims, error) {
	var token *jwt.Token
	if jwks == nil {
		return nil, fmt.Errorf("jwks not initialised")
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(jwtB64, claims, jwks.Keyfunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the JWT: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token provided")
	}
	if val, ok := claims["aud"]; ok {
		if val != clientId {
			return nil, fmt.Errorf("unknown client id provided: %s", val)
		}
	} else {
		return nil, fmt.Errorf("client id not found in token")
	}

	if val, ok := claims["tid"]; ok {
		if val != tenantId {
			return nil, fmt.Errorf("unknown tenant id provided: %s", val)
		}
	} else {
		return nil, fmt.Errorf("tenant id not found in token")
	}

	return claims, nil
}

func HttpAuthMiddleWare(resp http.ResponseWriter, req *http.Request, jwks *keyfunc.JWKS, clientId string, tenantId string, allowedPaths []string, allowedPrefixes []string) error {
	token := req.Header.Get("authorization")
	if AllowBypassingAzureAuth(allowedPaths, req.URL.Path, req.Method, allowedPrefixes) {
		return nil
	}
	claims, err := ValidateToken(token, jwks, clientId, tenantId)
	if _, ok := claims["aud"]; ok && claims["aud"] == clientId {
		req.Header.Set("username", claims["name"].(string))
		req.Header.Set("email", claims["email"].(string))
	}
	if err != nil {
		resp.WriteHeader(http.StatusUnauthorized)
		_, _ = resp.Write([]byte("Invalid authorization header provided"))
	}
	return err
}

func AllowBypassingAzureAuth(allowedPaths []string, requestUrlPath string, requestMethod string, allowedPrefixes []string) bool {
	for _, allowedPath := range allowedPaths {
		if requestUrlPath == allowedPath {
			return true
		}
	}
	for _, allowedPrefix := range allowedPrefixes {
		if strings.HasPrefix(requestUrlPath, allowedPrefix) {
			return true
		}
	}

	// Skip azure authentication with ID for `/` (POST: createEnv), `/release`, `/releasetrain` and `/locks`  endpoints. The requests will be validated with pgp signature
	// usage in requests from outside the cluster (e.g. by GitHub Actions and the publish.sh script).
	group, tail := xpath.Shift(requestUrlPath)

	if group == "environments" || group == "environment-groups" {
		envName, tail := xpath.Shift(tail)
		if envName != "" { // We shouldn't receive an empty env, added just as a second layer of validation
			function, tail := xpath.Shift(tail)
			switch function {
			case "locks":
				return true
			case "releasetrain":
				return true
			case "": // create environment
				if tail == "/" && requestMethod == http.MethodPost {
					return true
				}
			}
		}
	}
	return false
}
