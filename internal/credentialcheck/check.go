// Package credentialcheck performs bounded, authenticated service checks.
// Results contain public identity and status only, never server error bodies.
package credentialcheck

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mwclient "cgt.name/pkg/go-mwclient"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

type Request struct {
	Service, Endpoint, Account, Value string
	WikiMethod, WikiEndpoint          string
}

type Result struct {
	Status   string
	Identity string
}

// Check proves service acceptance, not permission to perform every operation.
// It never refreshes tokens or changes any persisted credential.
func Check(ctx context.Context, input Request) Result {
	if input.Value == "" {
		return Result{Status: "unavailable"}
	}
	endpoint, err := secrets.Endpoint(input.Endpoint)
	if err != nil || input.Account == "" {
		return Result{Status: "unavailable"}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client := wikiauth.HTTPClient()
	client.Timeout = 15 * time.Second
	switch input.Service {
	case "phabricator":
		params, _ := json.Marshal(map[string]any{"__conduit__": map[string]string{"token": input.Value}})
		form := url.Values{"params": {string(params)}, "output": {"json"}}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/api/user.whoami", strings.NewReader(form.Encode()))
		if err != nil {
			return Result{Status: "unavailable"}
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		body, status := send(client, req)
		if status != "" {
			return Result{Status: status}
		}
		var reply struct {
			Result struct {
				Username string `json:"userName"`
			} `json:"result"`
			ErrorCode json.RawMessage `json:"error_code"`
		}
		if json.Unmarshal(body, &reply) != nil {
			return Result{Status: "unavailable"}
		}
		if code := string(reply.ErrorCode); code != "" && code != "null" && code != `""` && code != "0" {
			if code == `"ERR-INVALID-AUTH"` || code == `"ERR-NO-TOKEN"` {
				return Result{Status: "invalid"}
			}
			return Result{Status: "unavailable"}
		}
		expected := input.Account
		if strings.EqualFold(expected, reply.Result.Username) {
			expected = reply.Result.Username
		}
		return identityResult(expected, reply.Result.Username)
	case "gerrit":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/a/accounts/self/detail", nil)
		if err != nil {
			return Result{Status: "unavailable"}
		}
		req.SetBasicAuth(input.Account, input.Value)
		body, status := send(client, req)
		if status != "" {
			return Result{Status: status}
		}
		var reply struct {
			Username string `json:"username"`
		}
		if json.Unmarshal([]byte(strings.TrimPrefix(string(body), ")]}'\n")), &reply) != nil {
			return Result{Status: "unavailable"}
		}
		return identityResult(input.Account, reply.Username)
	case "cloudvps":
		project, id, ok := strings.Cut(input.Account, ":")
		if !ok || id == "" {
			return Result{Status: "unavailable"}
		}
		payload, _ := json.Marshal(map[string]any{"auth": map[string]any{"identity": map[string]any{"methods": []string{"application_credential"}, "application_credential": map[string]string{"id": id, "secret": input.Value}}}})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/auth/tokens", strings.NewReader(string(payload)))
		if err != nil {
			return Result{Status: "unavailable"}
		}
		req.Header.Set("Content-Type", "application/json")
		body, status := send(client, req)
		if status != "" {
			return Result{Status: status}
		}
		var reply struct {
			Token struct {
				User struct {
					Name string `json:"name"`
				} `json:"user"`
				Project struct {
					Name string `json:"name"`
				} `json:"project"`
			} `json:"token"`
		}
		if json.Unmarshal(body, &reply) != nil || reply.Token.Project.Name == "" {
			return Result{Status: "unavailable"}
		}
		if reply.Token.Project.Name != project {
			return Result{Status: "wrong-account", Identity: reply.Token.User.Name}
		}
		return Result{Status: "valid", Identity: reply.Token.User.Name}
	case "wiki":
		return checkWiki(ctx, client, input)
	default:
		return Result{Status: "unsupported"}
	}
}

func identityResult(expected, actual string) Result {
	if actual == "" {
		return Result{Status: "unavailable"}
	}
	if actual != expected {
		return Result{Status: "wrong-account", Identity: actual}
	}
	return Result{Status: "valid", Identity: actual}
}

func send(client *http.Client, req *http.Request) ([]byte, string) {
	req.Header.Set("User-Agent", cli.HTTPUserAgent)
	response, err := client.Do(req)
	if err != nil {
		return nil, "unavailable"
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		return nil, "invalid"
	}
	if response.StatusCode == http.StatusForbidden {
		return nil, "denied"
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "unavailable"
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return nil, "unavailable"
	}
	return body, ""
}

type contextTransport struct{ ctx context.Context }

func (t contextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return http.DefaultTransport.RoundTrip(req.Clone(t.ctx))
}

func checkWiki(ctx context.Context, client *http.Client, input Request) Result {
	endpoint, err := wikiauth.Endpoint(input.WikiEndpoint)
	if err != nil {
		return Result{Status: "unavailable"}
	}
	if input.WikiMethod == "oauth" {
		var tokens wikiauth.Tokens
		if json.Unmarshal([]byte(input.Value), &tokens) != nil || tokens.AccessToken == "" || !strings.EqualFold(tokens.TokenType, "Bearer") {
			return Result{Status: "unavailable"}
		}
		if !tokens.ExpiresAt.IsZero() && !time.Now().Before(tokens.ExpiresAt) {
			return Result{Status: "expired"}
		}
		client.Transport = wikiauth.BearerTransport{Endpoint: endpoint, Token: tokens.AccessToken}
	} else if input.WikiMethod == "bot-password" {
		w, err := mwclient.New(endpoint, cli.HTTPUserAgent)
		if err != nil {
			return Result{Status: "unavailable"}
		}
		client.Transport = contextTransport{ctx: ctx}
		w.SetHTTPClient(client)
		if err := w.Login(input.Account, input.Value); err != nil {
			var apiError mwclient.APIError
			if errors.As(err, &apiError) && (apiError.Code == "Failed" || apiError.Code == "WrongPass" || apiError.Code == "NotExists") {
				return Result{Status: "invalid"}
			}
			return Result{Status: "unavailable"}
		}
	} else {
		return Result{Status: "unsupported"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?action=query&meta=userinfo&format=json", nil)
	if err != nil {
		return Result{Status: "unavailable"}
	}
	body, status := send(client, req)
	if status != "" {
		return Result{Status: status}
	}
	var reply struct {
		Query struct {
			Userinfo struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"userinfo"`
		} `json:"query"`
	}
	if json.Unmarshal(body, &reply) != nil {
		return Result{Status: "unavailable"}
	}
	if reply.Query.Userinfo.ID == 0 {
		return Result{Status: "invalid"}
	}
	expected := input.Account
	if input.WikiMethod == "bot-password" {
		expected = strings.SplitN(expected, "@", 2)[0]
	}
	return identityResult(expected, reply.Query.Userinfo.Name)
}
