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
)

var jwks *keyfunc.JWKS = nil

func JWKSInitAzure(ctx context.Context) error {
	jwksURL := "https://login.microsoftonline.com/common/discovery/v2.0/keys"
	options := keyfunc.Options{
		Ctx: ctx,
		RefreshErrorHandler: func(err error) {
			log.Printf("There was an error with the jwt.Keyfunc\nError: %s", err.Error())
		},
		RefreshInterval:   time.Hour,
		RefreshRateLimit:  time.Minute * 5,
		RefreshTimeout:    time.Second * 10,
		RefreshUnknownKID: true,
	}
	var err error
	jwks, err = keyfunc.Get(jwksURL, options)
	if err != nil {
		return fmt.Errorf("Failed to create JWKS from resource at the given URL.\nError: %s", err.Error())
	}
	return nil
}

func ValidateToken(jwtB64 string, clientId string, tenantId string) (*User, error) {
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

	email, _ := claims["preferred_username"]
	name, _ := claims["name"]

	return &User{
		Email: email.(string),
		Name:  name.(string),
	}, nil
}
