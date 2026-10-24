package cloudvps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestCloudAuthenticationUserAgent(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if !strings.HasPrefix(r.UserAgent(), cli.HTTPUserAgent+" ") {
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
