package cloudvps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/tokens"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestCloudAuthenticationUserAgent(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if !strings.HasPrefix(r.UserAgent(), cli.HTTPUserAgent()+" ") {
			t.Errorf("authentication User-Agent = %q", r.UserAgent())
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	_, err := authenticatedCloudClient(context.Background(), gophercloud.AuthOptions{
		IdentityEndpoint:        server.URL + "/v3/",
		ApplicationCredentialID: "fixture-id", ApplicationCredentialSecret: "fixture-secret",
	})
	if err == nil || requests == 0 {
		t.Fatalf("expected an authentication request rejected by server: requests=%d, err=%v", requests, err)
	}
}

func TestProviderProjectScope(t *testing.T) {
	for _, tc := range []struct {
		id, name string
		valid    bool
	}{
		{"mwcli", "Wikimedia CLI", true},
		{"project-uuid", "mwcli", true},
		{"other-id", "other-name", false},
		{"", "", false},
	} {
		var result tokens.CreateResult
		result.Body = map[string]any{"token": map[string]any{
			"project": map[string]any{"id": tc.id, "name": tc.name},
			"user":    map[string]any{"name": "Addshore"},
		}}
		provider := &gophercloud.ProviderClient{}
		if err := provider.SetTokenAndAuthResult(result); err != nil {
			t.Fatal(err)
		}
		err := checkProviderProject(provider, "mwcli")
		if (err == nil) != tc.valid {
			t.Fatalf("project ID=%q name=%q: %v", tc.id, tc.name, err)
		}
		if tc.id == "other-id" && (!strings.Contains(err.Error(), tc.id) || !strings.Contains(err.Error(), tc.name) || !strings.Contains(err.Error(), "mwcli")) {
			t.Fatalf("project mismatch lacks scope details: %v", err)
		}
	}
}
