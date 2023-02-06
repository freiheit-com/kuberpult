
//
// Middleware for handling CORS (cross origin requests)
// Warning: CORS requests needs to be whitelisted on ist.io ingress configuration
// Technically this middleware shouldn't be necessary but currently the ingress
// proxy forwards the CORS preflight request to the pod
//
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
