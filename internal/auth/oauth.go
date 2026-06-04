package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// OAuthServer implements a minimal OAuth 2.0 client credentials flow
// sufficient for Claude.ai MCP connector authentication.
type OAuthServer struct {
	clientID     string
	clientSecret string
	// accessToken is the bearer token issued to valid clients.
	// We reuse the existing AUTH_TOKEN so the MCP handler needs no changes.
	accessToken string
}

// New creates an OAuthServer.
func New(clientID, clientSecret, accessToken string) *OAuthServer {
	return &OAuthServer{
		clientID:     clientID,
		clientSecret: clientSecret,
		accessToken:  accessToken,
	}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type oauthMeta struct {
	Issuer                string   `json:"issuer"`
	TokenEndpoint         string   `json:"token_endpoint"`
	GrantTypesSupported   []string `json:"grant_types_supported"`
	TokenEndpointAuthMethods []string `json:"token_endpoint_auth_methods_supported"`
}

// HandleMeta serves the OAuth authorization server metadata document.
func (o *OAuthServer) HandleMeta(w http.ResponseWriter, r *http.Request) {
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	base := scheme + "://" + r.Host

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(oauthMeta{ //nolint:errcheck
		Issuer:                base,
		TokenEndpoint:         base + "/token",
		GrantTypesSupported:   []string{"client_credentials"},
		TokenEndpointAuthMethods: []string{"client_secret_post"},
	})
}

// HandleToken issues an access token for valid client credentials.
func (o *OAuthServer) HandleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")

	// Also support HTTP Basic auth
	if clientID == "" {
		clientID, clientSecret, _ = r.BasicAuth()
	}

	// Also support bearer in Authorization header for secret
	if clientSecret == "" {
		if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			clientSecret = bearer
		}
	}

	if grantType != "client_credentials" {
		http.Error(w, `{"error":"unsupported_grant_type"}`, http.StatusBadRequest)
		return
	}

	if clientID != o.clientID || clientSecret != o.clientSecret {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_client"}`)) //nolint:errcheck
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenResponse{ //nolint:errcheck
		AccessToken: o.accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   315360000, // 10 years — effectively non-expiring
	})
}
