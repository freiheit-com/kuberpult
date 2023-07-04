package auth

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	jwt "github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

// Dex App Client.
type DexAppClient struct {
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
}

const (
	// Dex service internal URL. Used to connect to dex internally in the cluster.
	dexServiceURL = "http://kuberpult-dex-service:5556"
	// Dex issuer path. Needs to be match the dex issuer helm config.
	issuerPATH = "/dex"
	// Dex callback path. Needs to be match the dex staticClients.redirectURIs helm config.
	callbackPATH = "/callback"
	// Kuberpult login path.
	loginPATH = "/login"
	// Dex OAUTH token name.
	dexOAUTHTokenName = "kuberpult.oauth"
	// Default value for the number of days the token is valid for.
	expirationDays = 1
)

// NewDexAppClient a Dex Client.
func NewDexAppClient(clientID, clientSecret, baseURL string, scopes []string) (*DexAppClient, error) {
	a := DexAppClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
		BaseURL:      baseURL,
		RedirectURI:  baseURL + callbackPATH,
		IssuerURL:    baseURL + issuerPATH,
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	a.Client = &http.Client{
		Transport: transport,
	}

	// Creates a transport layer to map all requests to dex internally
	dexURL, _ := url.Parse(dexServiceURL)
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
	http.HandleFunc(issuerPATH+"/", NewDexReverseProxy(dexServiceURL))
	// Handles the login callback to redirect to dex page.
	http.HandleFunc(loginPATH, a.handleDexLogin)
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

// Redirects to the Dex login page with the pre configured connector.
func (a *DexAppClient) handleDexLogin(w http.ResponseWriter, r *http.Request) {
	oauthConfig, err := a.oauth2Config(a.Scopes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO(BB) Set an app state to make the connection more secure
	authCodeURL := oauthConfig.AuthCodeURL("APP_STATE")
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

	idToken, err := a.validateToken(ctx, idTokenRAW, a.ClientID)
	if err != nil {
		http.Error(w, "failed to verify the token", http.StatusInternalServerError)
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

func (a *DexAppClient) validateToken(ctx context.Context, rawToken string, allowedAudience string) (token *oidc.IDToken, err error) {
	p, err := oidc.NewProvider(ctx, a.IssuerURL)
	if err != nil {
		return nil, err
	}

	// Token must be verified against an allowed audience.
	config := oidc.Config{ClientID: allowedAudience}
	verifier := p.Verifier(&config)
	idToken, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("the token could not be verified, audience %s is not allowed with err: %s", allowedAudience, err)
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
func (a *DexAppClient) verifyToken(r *http.Request) error {
	// Get the token cookie from the request
	cookie, err := r.Cookie(dexOAUTHTokenName)
	if err != nil {
		return fmt.Errorf("%s token not found", dexOAUTHTokenName)
	}
	tokenString := cookie.Value

	// Validates token audience.
	idToken, err := a.validateToken(context.Background(), tokenString, a.ClientID)
	if err != nil {
		return fmt.Errorf("failed to verify token: %s", err)
	}

	// Extract token claims and verify the token is not expired.
	var claims jwt.MapClaims
	err = idToken.Claims(&claims)
	if err != nil {
		return fmt.Errorf("could not parse token claims")
	}
	expirationTime := claims["exp"].(float64)
	expiration := time.Unix(int64(expirationTime), 0)
	if expiration.Before(time.Now()) {
		return fmt.Errorf("the token has expired")
	}

	// Token is valid and not expired, continue processing.
	return nil
}
