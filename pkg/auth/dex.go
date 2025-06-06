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
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

// Extracted information from JWT/Cookie.
type DexAuthContext struct {
	// The user role extracted from the Cookie.
	Role []string
}

// Dex App Client.
type DexAppClient struct {
	// Dex service internal URL. Used to connect to dex internally in the cluster.
	DexServiceURL string
	// The Dex issuer URL. Needs to be match the dex issuer helm config.
	IssuerURL string
	// The host Kuberpult is running on.
	BaseURL string
	// The Kuberpult client ID. Needs to match the dex staticClients.id helm configuration.
	ClientID string
	// The Kuberpult client secret. Needs to match the dex staticClients.secret helm configuration.
	ClientSecret string
	// The Dex redirect callback. Needs to match the dex staticClients.redirectURIs helm configuration.
	RedirectURI string
	// The available scopes.
	Scopes []string
	// The http client used.
	Client *http.Client
	// Whether dex should be accessed via internal cluster communication.
	UseClusterInternalCommunication bool
}

const (
	// Dex service internal URL. Used to connect to dex internally in the cluster.
	dexServiceURLPattern = "http://%s:5556"
	// Dex issuer path. Needs to be match the dex issuer helm config.
	issuerPATH = "/dex"
	// Dex callback path. Needs to be match the dex staticClients.redirectURIs helm config.
	callbackPATH = "/callback"
	// Kuberpult login path.
	LoginPATH = "/login"
	// Dex OAUTH token name.
	dexOAUTHTokenName = "kuberpult.oauth"
	// Default value for the number of days the token is valid for.
	expirationDays = 1
)

// GetDexServiceURL returns the Dex service URL from the fullNameOverride.
func GetDexServiceURL(fullNameOverride string) string {
	return fmt.Sprintf(dexServiceURLPattern, fullNameOverride)
}

// NewDexAppClient a Dex Client.
func NewDexAppClient(clientID, clientSecret, baseURL, nameOverride string, scopes []string, useClusterInternalCommunication bool) (*DexAppClient, error) {
	a := DexAppClient{
		Client:                          nil,
		ClientID:                        clientID,
		ClientSecret:                    clientSecret,
		Scopes:                          scopes,
		BaseURL:                         baseURL,
		RedirectURI:                     baseURL + callbackPATH,
		IssuerURL:                       baseURL + issuerPATH,
		UseClusterInternalCommunication: useClusterInternalCommunication,
		DexServiceURL:                   GetDexServiceURL(nameOverride),
	}
	//exhaustruct:ignore
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	//exhaustruct:ignore
	a.Client = &http.Client{
		Transport: transport,
	}

	// Creates a transport layer to map all requests to dex internally
	dexURL, _ := url.Parse(fmt.Sprintf(dexServiceURLPattern, nameOverride))
	a.Client.Transport = DexRewriteURLRoundTripper{
		DexURL: dexURL,
		T:      a.Client.Transport,
	}

	// Register Dex handlers.
	a.registerDexHandlers()
	return &a, nil
}

// DexRewriteURLRoundTripper creates a new DexRewriteURLRoundTripper.
// The round tripper is configured to avoid exposing the dex server via a virtual service. Since Kuberpult and dex
// are running on the same cluster, a reverse proxy is configured to redirect all dex calls internally.
type DexRewriteURLRoundTripper struct {
	DexURL *url.URL
	T      http.RoundTripper
}

func (s DexRewriteURLRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL.Host = s.DexURL.Host
	r.URL.Scheme = s.DexURL.Scheme
	r.Host = s.DexURL.Host
	return s.T.RoundTrip(r)
}

// Registers dex handlers for login
func (a *DexAppClient) registerDexHandlers() {
	// Handles calls to the Dex server. Calls are redirected internally using a reverse proxy.
	http.HandleFunc(issuerPATH+"/", NewDexReverseProxy(a.DexServiceURL))
	// Handles the login callback to redirect to dex page.
	http.HandleFunc(LoginPATH, a.handleDexLogin)
	// Call back to the current app once the login is finished
	http.HandleFunc(callbackPATH, a.handleCallback)
}

// NewDexReverseProxy returns a reverse proxy to the Dex server.
func NewDexReverseProxy(serverAddr string) func(writer http.ResponseWriter, request *http.Request) {
	target, err := url.Parse(serverAddr)
	if err != nil {
		logger.FromContext(context.Background()).Error(fmt.Sprintf("Could not parse server URL with error: %s", err))
		return nil
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode == http.StatusInternalServerError {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			err = resp.Body.Close()
			if err != nil {
				return err
			}
			logger.FromContext(context.Background()).Error(fmt.Sprintf("Could not parse server URL with error: %s", string(body)))
			resp.Body = io.NopCloser(bytes.NewReader(make([]byte, 0)))
			return nil
		}
		return nil
	}
	proxy.Director = decorateDirector(proxy.Director, target)
	return func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}
}

func decorateDirector(director func(req *http.Request), target *url.URL) func(req *http.Request) {
	return func(req *http.Request) {
		director(req)
		req.Host = target.Host
	}
}

// Helper function to generate a random string to avoid cashing.
func generateState() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Redirects to the Dex login page with the pre configured connector.
func (a *DexAppClient) handleDexLogin(w http.ResponseWriter, r *http.Request) {
	oauthConfig, err := a.oauth2Config(a.Scopes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Currently a random string is generated but a session state should be built.
	// This is used to avoid caching of the auth URL when making calls to Dex for requesting a token.
	state, err := generateState()
	if err != nil {
		http.Error(w, "could not generate dex state: "+err.Error(), http.StatusInternalServerError)
	}
	authCodeURL := oauthConfig.AuthCodeURL(state)
	http.Redirect(w, r, authCodeURL, http.StatusSeeOther)
}

// HandleCallback is the callback handler for an OAuth2 login flow.
func (a *DexAppClient) handleCallback(w http.ResponseWriter, r *http.Request) {
	oauth2Config, err := a.oauth2Config(nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errMsg := r.FormValue("error"); errMsg != "" {
		errorDesc := r.FormValue("error_description")
		http.Error(w, html.EscapeString(errMsg)+": "+html.EscapeString(errorDesc), http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	ctx := oidc.ClientContext(r.Context(), a.Client)
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get token: %v", err), http.StatusInternalServerError)
		return
	}

	idTokenRAW, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in token response", http.StatusInternalServerError)
		return
	}

	idToken, err := ValidateOIDCToken(ctx, a.IssuerURL, idTokenRAW, a.ClientID, a.DexServiceURL, a.UseClusterInternalCommunication)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to verify the token: %v", err), http.StatusInternalServerError)
		return
	}

	var claims jwt.MapClaims
	err = idToken.Claims(&claims)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Stores the oauth token into the cookie.
	if idTokenRAW != "" {
		expiration := time.Now().Add(time.Duration(expirationDays) * 24 * time.Hour)
		//exhaustruct:ignore
		cookie := http.Cookie{
			Name:    dexOAUTHTokenName,
			Value:   idTokenRAW,
			Expires: expiration,
			Path:    "/",
		}
		http.SetCookie(w, &cookie)
	}
	http.Redirect(w, r, a.BaseURL, http.StatusSeeOther)
}

func ValidateOIDCToken(ctx context.Context, issuerURL, rawToken, allowedAudience, dexServiceURL string, useClusterInternalCommunication bool) (token *oidc.IDToken, err error) {
	var p *oidc.Provider
	// Token must be verified against an allowed audience.
	//exhaustruct:ignore
	config := oidc.Config{ClientID: allowedAudience}
	if useClusterInternalCommunication {
		// When using e.g. GCP IAP, calls to https://kuberpult.com/dex/.well-known/openid-configuration will not return the needed JSON, so we do cluster-internal calls instead to e.g. http://kuberpult-dex:5556/dex
		discoveryBaseURL := dexServiceURL + issuerPATH
		pc := &oidc.ProviderConfig{
			IssuerURL:     discoveryBaseURL,
			AuthURL:       discoveryBaseURL + "/auth",
			TokenURL:      discoveryBaseURL + "/token",
			DeviceAuthURL: discoveryBaseURL + "/device/code",
			UserInfoURL:   discoveryBaseURL + "/userinfo",
			JWKSURL:       discoveryBaseURL + "/keys",
			Algorithms:    []string{"RS256"},
		}
		p = pc.NewProvider(ctx)
		// As the issuer is e.g. https://kuberpult.com/dex and the providerBaseUrl is http://kuberpult-dex:5556/dex (default) we need to SkipIssuerChecks
		config.SkipIssuerCheck = true
	} else {
		p, err = oidc.NewProvider(ctx, issuerURL)
		if err != nil {
			return nil, err
		}
	}

	verifier := p.Verifier(&config)
	idToken, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	return idToken, nil
}

func (a *DexAppClient) oauth2Config(scopes []string) (c *oauth2.Config, err error) {
	ctx := oidc.ClientContext(context.Background(), a.Client)
	p, err := oidc.NewProvider(ctx, a.IssuerURL)
	if err != nil {
		return nil, err
	}

	return &oauth2.Config{
		ClientID:     a.ClientID,
		ClientSecret: a.ClientSecret,
		Endpoint:     p.Endpoint(),
		Scopes:       scopes,
		RedirectURL:  a.RedirectURI,
	}, nil
}

// Verifies if the user is authenticated.
func VerifyToken(ctx context.Context, r *http.Request, clientID, baseURL, dexServiceURL string, useClusterInternalCommunication bool) (jwt.MapClaims, error) {
	// Get the token cookie from the request
	cookie, err := r.Cookie(dexOAUTHTokenName)

	//If token not in cookie verify in header
	var tokenString string
	if err != nil {
		reqToken := r.Header.Get("Authorization")
		if reqToken == "" {
			return nil, fmt.Errorf("%s token not found", dexOAUTHTokenName)
		}
		splitToken := strings.Split(reqToken, "Bearer ")
		if len(splitToken) != 2 {
			return nil, fmt.Errorf("%s token not found", dexOAUTHTokenName)
		}
		tokenString = splitToken[1]
	} else {
		tokenString = cookie.Value
	}

	// Validates token audience and expiring date.
	idToken, err := ValidateOIDCToken(ctx, baseURL+issuerPATH, tokenString, clientID, dexServiceURL, useClusterInternalCommunication)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %s", err)
	}
	// Extract token claims and verify the token is not expired.
	claims := jwt.MapClaims{
		"groups": []string{},
		"email":  "",
		"name":   "",
		"sub":    "",
	}
	err = idToken.Claims(&claims)
	if err != nil {
		return nil, fmt.Errorf("could not parse token claims")
	}

	// check if claims is empty in terms of required fields for identification
	if claims["email"].(string) == "" && len(claims["groups"].([]string)) < 1 {
		return nil, fmt.Errorf("need required fields to determine group of user")
	}

	return claims, nil
}
