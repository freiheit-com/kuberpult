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

// Middleware for handling CORS (cross origin requests)
// Warning: CORS requests needs to be whitelisted on ist.io ingress configuration
// Technically this middleware shouldn't be necessary but currently the ingress
// proxy forwards the CORS preflight request to the pod
package setup

import (
	"fmt"
	"net/http"
)

type CORSPolicy struct {
	AllowMethods     string
	AllowHeaders     string
	AllowOrigin      string
	AllowCredentials bool
	MaxAge           int
}

type CORSMiddleware struct {
	PolicyFor   func(req *http.Request) *CORSPolicy
	NextHandler http.Handler
}

func (check *CORSMiddleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	origin := req.Header.Get("Origin")
	if origin == "" {
		check.NextHandler.ServeHTTP(rw, req)
		return
	}

	requestMethod := req.Header.Get("Access-Control-Request-Method")
	policy := check.PolicyFor(req)
	if policy == nil {
		rw.WriteHeader(403)
		req.Body.Close()
		return
	}

	if req.Method == "OPTIONS" && requestMethod != "" {

		rw.Header().Add("Access-Control-Allow-Methods", policy.AllowMethods)
		rw.Header().Add("Access-Control-Allow-Headers", policy.AllowHeaders)
		if policy.AllowCredentials {
			rw.Header().Add("Access-Control-Allow-Credentials", "true")
		}
		rw.Header().Add("Access-Control-Max-Age", fmt.Sprintf("%d", policy.MaxAge))
		rw.Header().Add("Access-Control-Allow-Origin", policy.AllowOrigin)

		rw.WriteHeader(204)
		req.Body.Close()
		return
	}
	rw.Header().Add("Access-Control-Allow-Origin", policy.AllowOrigin)
	if policy.AllowCredentials {
		rw.Header().Add("Access-Control-Allow-Credentials", "true")
	}
	check.NextHandler.ServeHTTP(rw, req)

}
