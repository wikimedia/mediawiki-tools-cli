// Package enterprise implements the Wikimedia Enterprise HTTP and token contracts.
package enterprise

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

const (
	API      = "https://api.enterprise.wikimedia.com"
	Auth     = "https://auth.enterprise.wikimedia.com"
	Realtime = "https://realtime.enterprise.wikimedia.com"
)

// Tokens is stored as one credential, allowing atomic refreshes in any provider.
// ID tokens are deliberately discarded: the data APIs only need access tokens.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func ParseTokens(value string) (Tokens, error) {
	var tokens Tokens
	if json.Unmarshal([]byte(value), &tokens) != nil || tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.ExpiresAt.IsZero() {
		return Tokens{}, fmt.Errorf("invalid Enterprise credential; use enterprise auth login")
	}
	return tokens, nil
}

type Client struct {
	HTTP                         *http.Client
	APIURL, AuthURL, RealtimeURL string
	// Token returns a current token. rejected is the token rejected by a 401,
	// allowing a credential lock holder to reuse a concurrent refresh.
	Token func(ctx context.Context, rejected string) (string, error)
}

var errConnection = errors.New("Enterprise connection failed")

func NewClient() *Client {
	h := wikiauth.HTTPClient()
	h.Timeout = 0 // Streaming and large downloads use caller cancellation.
	h.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{HTTP: h, APIURL: API, AuthURL: Auth, RealtimeURL: Realtime}
}

// HTTPError never includes response bodies, tokens or signed URLs.
type HTTPError struct{ Status int }

func (e *HTTPError) Error() string {
	switch e.Status {
	case 401:
		return "Enterprise authentication was rejected; use enterprise auth login"
	case 403:
		return "Enterprise access denied; check your account's API entitlement"
	case 429:
		return "Enterprise rate or account quota limit reached; retry later"
	default:
		return fmt.Sprintf("Enterprise returned HTTP %d (%s)", e.Status, http.StatusText(e.Status))
	}
}

func (c *Client) Request(ctx context.Context, method, endpoint string, body any, token, accept string) (*http.Response, error) {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("could not construct Enterprise request")
	}
	req.Header.Set("User-Agent", cli.HTTPUserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errConnection
	}
	return resp, nil
}

func (c *Client) Authorized(ctx context.Context, method, endpoint string, body any, accept string) (*http.Response, error) {
	token, err := c.Token(ctx, "")
	if err != nil {
		return nil, err
	}
	resp, err := c.Request(ctx, method, endpoint, body, token, accept)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	resp.Body.Close()
	token, err = c.Token(ctx, token)
	if err != nil {
		return nil, err
	}
	return c.Request(ctx, method, endpoint, body, token, accept)
}

func (c *Client) JSON(ctx context.Context, path string, body any) (any, error) {
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	return c.JSONMethod(ctx, method, path, body)
}

// JSONMethod issues a data API request. HEAD exposes only public object
// metadata, never a signed Location header from a download redirect.
func (c *Client) JSONMethod(ctx context.Context, method, path string, body any) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	resp, err := c.Authorized(ctx, method, c.APIURL+path, body, "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if method == http.MethodHead && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusTemporaryRedirect) {
		headers := map[string]string{}
		for _, key := range []string{"Content-Length", "Content-Type", "ETag", "Last-Modified", "Accept-Ranges"} {
			if value := resp.Header.Get(key); value != "" {
				headers[key] = value
			}
		}
		return headers, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (64<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("could not read Enterprise response")
	}
	if len(data) > 64<<20 {
		return nil, fmt.Errorf("Enterprise response exceeds 64 MiB; select fewer fields")
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return nil, fmt.Errorf("invalid Enterprise JSON response")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("unexpected data after Enterprise JSON response")
	}
	return value, nil
}

func (c *Client) tokenRequest(ctx context.Context, path string, body any, previous string) (Tokens, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := c.Request(ctx, http.MethodPost, c.AuthURL+path, body, "", "application/json")
	if err != nil {
		return Tokens{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Tokens{}, &HTTPError{resp.StatusCode}
	}
	var reply struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&reply) != nil || reply.AccessToken == "" || reply.ExpiresIn <= 0 || reply.ExpiresIn > 7*24*3600 {
		return Tokens{}, fmt.Errorf("invalid Enterprise login response; complete any account password challenge in the Enterprise dashboard")
	}
	if reply.RefreshToken == "" {
		reply.RefreshToken = previous
	}
	if reply.RefreshToken == "" {
		return Tokens{}, fmt.Errorf("Enterprise login did not return a refresh token")
	}
	return Tokens{reply.AccessToken, reply.RefreshToken, time.Now().Add(time.Duration(reply.ExpiresIn) * time.Second)}, nil
}

func (c *Client) Login(ctx context.Context, username, password string) (Tokens, error) {
	return c.tokenRequest(ctx, "/v1/login", map[string]string{"username": username, "password": password}, "")
}

func (c *Client) Refresh(ctx context.Context, username string, tokens Tokens) (Tokens, error) {
	return c.tokenRequest(ctx, "/v1/token-refresh", map[string]string{"username": username, "refresh_token": tokens.RefreshToken}, tokens.RefreshToken)
}

func (c *Client) Revoke(ctx context.Context, tokens Tokens) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := c.Request(ctx, http.MethodPost, c.AuthURL+"/v1/token-revoke", map[string]string{"refresh_token": tokens.RefreshToken}, "", "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return &HTTPError{resp.StatusCode}
	}
	return nil
}
