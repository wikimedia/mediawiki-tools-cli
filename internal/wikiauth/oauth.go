package wikiauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

// Temporary, approved basic-access public client supplied for development.
const TestClientID = "50f06966290c605f175ec7ea7a4caabc"
const TestRedirectURI = "http://localhost/auth/login"

type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type Authorization struct{ URL, State, Verifier, RedirectURI string }

func Begin(clientID, redirectURI string) (Authorization, error) {
	u, err := url.Parse(redirectURI)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return Authorization{}, fmt.Errorf("OAuth callback must be an HTTP loopback URL without query or fragment")
	}
	if clientID == "" {
		return Authorization{}, fmt.Errorf("OAuth client ID is required")
	}
	random := make([]byte, 64)
	if _, err := rand.Read(random); err != nil {
		return Authorization{}, err
	}
	a := Authorization{State: base64.RawURLEncoding.EncodeToString(random[:32]), Verifier: base64.RawURLEncoding.EncodeToString(random[32:]), RedirectURI: redirectURI}
	hash := sha256.Sum256([]byte(a.Verifier))
	q := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirectURI}, "state": {a.State}, "code_challenge": {base64.RawURLEncoding.EncodeToString(hash[:])}, "code_challenge_method": {"S256"}}
	a.URL = WikimediaAuthority + "/w/rest.php/oauth2/authorize?" + q.Encode()
	return a, nil
}

func (a Authorization) Code(callback string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(callback))
	expected, _ := url.Parse(a.RedirectURI)
	if err != nil || u.Scheme != expected.Scheme || u.Host != expected.Host || u.Path != expected.Path || u.User != nil || u.Fragment != "" {
		return "", fmt.Errorf("callback does not match the registered redirect URI")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(q["state"]) != 1 || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(a.State)) != 1 {
		return "", fmt.Errorf("OAuth callback state mismatch")
	}
	if q.Get("error") != "" {
		return "", fmt.Errorf("OAuth authorization was declined or failed")
	}
	if len(q["code"]) != 1 || q.Get("code") == "" {
		return "", fmt.Errorf("OAuth callback is missing its authorization code")
	}
	return q.Get("code"), nil
}

func HTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func Exchange(ctx context.Context, client *http.Client, clientID string, a Authorization, code string) (Tokens, error) {
	return tokenRequest(ctx, client, WikimediaAuthority+"/w/rest.php/oauth2/access_token", url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "redirect_uri": {a.RedirectURI}, "code": {code}, "code_verifier": {a.Verifier}})
}

func Refresh(ctx context.Context, client *http.Client, clientID string, old Tokens) (Tokens, error) {
	if old.RefreshToken == "" {
		return Tokens{}, fmt.Errorf("no refresh token; log in again")
	}
	tokens, err := tokenRequest(ctx, client, WikimediaAuthority+"/w/rest.php/oauth2/access_token", url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "refresh_token": {old.RefreshToken}})
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = old.RefreshToken
	}
	return tokens, err
}

func tokenRequest(ctx context.Context, client *http.Client, endpoint string, form url.Values) (Tokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, fmt.Errorf("could not construct OAuth request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", cli.HTTPUserAgent)
	response, err := client.Do(req)
	if err != nil {
		return Tokens{}, fmt.Errorf("OAuth token request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Tokens{}, fmt.Errorf("OAuth token request failed (HTTP %d); log in again if authorization expired", response.StatusCode)
	}
	var tokens Tokens
	if err := json.NewDecoder(io.LimitReader(response.Body, 128<<10)).Decode(&tokens); err != nil {
		return Tokens{}, fmt.Errorf("invalid OAuth token response")
	}
	if tokens.AccessToken == "" || !strings.EqualFold(tokens.TokenType, "Bearer") || tokens.ExpiresIn <= 0 {
		return Tokens{}, fmt.Errorf("incomplete OAuth token response")
	}
	tokens.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	return tokens, nil
}

// BearerTransport attaches a token only to its exact API endpoint. In particular,
// downloaded assets, redirects and other hosts must not receive wiki tokens.
type BearerTransport struct {
	Base            http.RoundTripper
	Endpoint, Token string
}

func (t BearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	copy.Header.Del("Authorization")
	u := *req.URL
	u.RawQuery, u.Fragment = "", ""
	if u.String() == t.Endpoint {
		copy.Header.Set("Authorization", "Bearer "+t.Token)
	}
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(copy)
}

func Identity(ctx context.Context, client *http.Client, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?action=query&meta=userinfo&format=json", nil)
	if err != nil {
		return "", fmt.Errorf("invalid wiki endpoint")
	}
	req.Header.Set("User-Agent", cli.HTTPUserAgent)
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not verify wiki identity")
	}
	defer response.Body.Close()
	var body struct {
		Query struct {
			Userinfo struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"userinfo"`
		} `json:"query"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body) != nil || body.Query.Userinfo.ID == 0 || body.Query.Userinfo.Name == "" {
		return "", fmt.Errorf("wiki authentication failed; log in again")
	}
	return body.Query.Userinfo.Name, nil
}
