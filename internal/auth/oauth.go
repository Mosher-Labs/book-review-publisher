package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// OAuthServer implements OAuth 2.0 Authorization Code + PKCE flow
// sufficient for Claude.ai MCP connector authentication.
type OAuthServer struct {
	clientID    string
	accessToken string

	mu    sync.Mutex
	codes map[string]authCode
}

type authCode struct {
	challenge string
	expiresAt time.Time
}

// New creates an OAuthServer.
func New(clientID, _ /* clientSecret */, accessToken string) *OAuthServer {
	return &OAuthServer{
		clientID:    clientID,
		accessToken: accessToken,
		codes:       make(map[string]authCode),
	}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type oauthMeta struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethods          []string `json:"token_endpoint_auth_methods_supported"`
}

func (o *OAuthServer) base(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

// HandleMeta serves the OAuth authorization server metadata document.
func (o *OAuthServer) HandleMeta(w http.ResponseWriter, r *http.Request) {
	base := o.base(r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(oauthMeta{ //nolint:errcheck
		Issuer:                        base,
		AuthorizationEndpoint:         base + "/authorize",
		TokenEndpoint:                 base + "/token",
		ResponseTypesSupported:        []string{"code"},
		GrantTypesSupported:           []string{"authorization_code"},
		CodeChallengeMethodsSupported: []string{"S256"},
		TokenEndpointAuthMethods:      []string{"none"},
	})
}

// HandleAuthorize handles GET /authorize — auto-approves and redirects back
// with an authorization code. Since this is a single-user personal server
// there is no login UI needed.
func (o *OAuthServer) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	challenge := q.Get("code_challenge")
	method := q.Get("code_challenge_method")
	state := q.Get("state")

	if clientID != o.clientID {
		http.Error(w, "unknown client", http.StatusBadRequest)
		return
	}
	if method != "S256" {
		http.Error(w, "only S256 code_challenge_method supported", http.StatusBadRequest)
		return
	}
	if redirectURI == "" || challenge == "" {
		http.Error(w, "missing required parameters", http.StatusBadRequest)
		return
	}

	code := generateCode()
	o.mu.Lock()
	o.codes[code] = authCode{
		challenge: challenge,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	o.mu.Unlock()

	location := redirectURI + "?code=" + code
	if state != "" {
		location += "&state=" + state
	}
	http.Redirect(w, r, location, http.StatusFound)
}

// HandleToken exchanges an authorization code for an access token.
func (o *OAuthServer) HandleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if r.FormValue("grant_type") != "authorization_code" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"unsupported_grant_type"}`)) //nolint:errcheck
		return
	}

	code := r.FormValue("code")
	verifier := r.FormValue("code_verifier")

	o.mu.Lock()
	stored, ok := o.codes[code]
	if ok {
		delete(o.codes, code)
	}
	o.mu.Unlock()

	if !ok || time.Now().After(stored.expiresAt) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant"}`)) //nolint:errcheck
		return
	}

	if !verifyS256(verifier, stored.challenge) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant"}`)) //nolint:errcheck
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenResponse{ //nolint:errcheck
		AccessToken: o.accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   315360000,
	})
}

func verifyS256(verifier, challenge string) bool {
	h := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])
	return computed == challenge
}

func generateCode() string {
	b := make([]byte, 32)
	rand.Read(b) //nolint:errcheck
	return base64.RawURLEncoding.EncodeToString(b)
}
