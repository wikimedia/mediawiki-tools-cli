package credentialcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

// GitLabIdentity verifies a personal access token without printing server bodies.
func GitLabIdentity(ctx context.Context, endpoint, token string) Result {
	endpoint, err := secrets.Endpoint(endpoint)
	if err != nil || token == "" {
		return Result{Status: "unavailable"}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client := wikiauth.HTTPClient()
	client.Timeout = 15 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/api/v4/user", nil)
	if err != nil {
		return Result{Status: "unavailable"}
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	body, status := send(client, req)
	if status != "" {
		return Result{Status: status}
	}
	var user struct {
		Username string `json:"username"`
	}
	if json.Unmarshal(body, &user) != nil || user.Username == "" {
		return Result{Status: "unavailable"}
	}
	return Result{Status: "valid", Identity: user.Username}
}
