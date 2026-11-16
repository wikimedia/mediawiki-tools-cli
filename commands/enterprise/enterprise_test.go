package enterprise

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/knadh/koanf"
	"github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	api "gitlab.wikimedia.org/repos/releng/cli/internal/enterprise"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func isolate(t *testing.T, handler func(*http.Request) (*http.Response, error)) {
	t.Helper()
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	config.State()
	if err := config.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	keyring.MockInit()
	oldTransport, oldNoInteraction := http.DefaultTransport, cli.Opts.NoInteraction
	http.DefaultTransport = testTransport(handler)
	cli.Opts.NoInteraction = true
	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
		cli.Opts.NoInteraction = oldNoInteraction
		config.ReApplyKoanfConf(koanf.New("."))
	})
}

func reply(req *http.Request, body string) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func run(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()
	cmd := NewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func login(t *testing.T) {
	t.Helper()
	if _, err := run(t, "password-secret\n", "auth", "login", "--profile", "bot", "--username", "Fixture", "--password-stdin", "--provider", "keyring"); err != nil {
		t.Fatal(err)
	}
}

func TestLoginInventoryRefreshAndProfileLifecycle(t *testing.T) {
	logins, refreshes, requests := 0, 0, 0
	isolate(t, func(r *http.Request) (*http.Response, error) {
		requests++
		switch r.URL.Host + r.URL.Path {
		case "auth.enterprise.wikimedia.com/v1/login":
			logins++
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["username"] != "fixture" || body["password"] != "password-secret" {
				t.Fatal("bad login data")
			}
			return reply(r, `{"access_token":"initial-secret","refresh_token":"refresh-secret","expires_in":86400}`)
		case "auth.enterprise.wikimedia.com/v1/token-refresh":
			refreshes++
			return reply(r, `{"access_token":"updated-secret","expires_in":86400}`)
		case "api.enterprise.wikimedia.com/v2/projects":
			if r.Header.Get("Authorization") != "Bearer updated-secret" {
				t.Fatal("did not use refreshed token")
			}
			return reply(r, `[{"identifier":"enwiki"}]`)
		case "auth.enterprise.wikimedia.com/v1/token-revoke":
			resp, _ := reply(r, "")
			resp.StatusCode = 204
			return resp, nil
		default:
			t.Fatal("unexpected request", r.URL)
			return nil, nil
		}
	})
	login(t)
	cred, err := credential("bot")
	if err != nil {
		t.Fatal(err)
	}
	value, err := cred.Read()
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := api.ParseTokens(value)
	if err != nil {
		t.Fatal(err)
	}
	tokens.ExpiresAt = time.Now().Add(-time.Minute)
	encoded, _ := json.Marshal(tokens)
	if err := cred.SaveWithProvider(string(encoded), false, nil, "keyring"); err != nil {
		t.Fatal(err)
	}
	before := requests
	out, err := run(t, "", "auth", "status")
	if err != nil || requests != before || !strings.Contains(out, "fixture") {
		t.Fatal("status accessed provider/network or failed", out, err)
	}
	if _, err := run(t, "", "projects", "list", "--output", "json"); err != nil {
		t.Fatal(err)
	}
	if logins != 1 || refreshes != 1 {
		t.Fatal("wrong token lifecycle", logins, refreshes)
	}
	value, _ = cred.Read()
	tokens, err = api.ParseTokens(value)
	if err != nil || tokens.RefreshToken != "refresh-secret" || tokens.AccessToken != "updated-secret" {
		t.Fatal("refreshed credential not persisted")
	}
	entries := config.SecretInventory()
	if len(entries) != 1 || entries[0].Service != "enterprise" || entries[0].Provider != "keyring" {
		t.Fatalf("missing inventory entry: %+v", entries)
	}
	config.InspectSecretInventory(t.Context(), entries, false, true)
	if entries[0].Validation != "valid" {
		t.Fatalf("verify unsupported: %+v", entries[0])
	}
	disk, _ := os.ReadFile(config.Path())
	if strings.Contains(string(disk)+out, "secret") {
		t.Fatal("tokens/password leaked into metadata or output")
	}
	if _, err := run(t, "", "auth", "default", "--profile", "bot"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "", "auth", "revoke"); err != nil {
		t.Fatal(err)
	}
	if config.HasCredential(cred.Key) || profiles.Default() != "" {
		t.Fatal("revoked profile was retained")
	}
}

func TestCommandEndpointMappingsAndFilters(t *testing.T) {
	var path string
	var payload map[string]any
	isolate(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "auth.enterprise.wikimedia.com" {
			return reply(r, `{"access_token":"access-secret","refresh_token":"refresh-secret","expires_in":86400}`)
		}
		path = r.URL.EscapedPath()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		return reply(r, `[{"identifier":"test","name":"Test"}]`)
	})
	login(t)
	tests := []struct {
		args []string
		path string
	}{
		{[]string{"articles", "get", "Albert Einstein", "--wiki", "enwiki", "--fields", "name,url"}, "/v2/articles/Albert_Einstein"},
		{[]string{"articles", "get", "A/B", "--structured"}, "/v2/structured-contents/A%2FB"},
		{[]string{"snapshots", "list"}, "/v2/snapshots"},
		{[]string{"snapshots", "info", "enwiki_namespace_0", "--structured"}, "/v2/snapshots/structured-contents/enwiki_namespace_0"},
		{[]string{"snapshots", "info", "wikidata_items", "--wikidata"}, "/v2/snapshots/wikidata/items"},
		{[]string{"snapshots", "chunks", "list", "items", "--wikidata"}, "/v2/snapshots/wikidata/items/chunks"},
		{[]string{"snapshots", "chunks", "info", "enwiki_namespace_0", "chunk1"}, "/v2/snapshots/enwiki_namespace_0/chunks/chunk1"},
		{[]string{"wikidata", "items", "get", "Q42"}, "/v2/wikidata/items/Q42"},
		{[]string{"wikidata", "properties", "get", "P31"}, "/v2/wikidata/properties/P31"},
		{[]string{"wikidata", "labels", "get", "P31"}, "/v2/wikidata/labels/P31"},
		{[]string{"batches", "list", "--date", "2026-11-04", "--hour", "0"}, "/v2/batches/2026-11-04/00"},
		{[]string{"batches", "info", "properties", "--wikidata", "--date", "2026-11-04", "--hour", "12"}, "/v2/batches/2026-11-04/12/wikidata/properties"},
		{[]string{"projects", "info", "enwiki"}, "/v2/projects/enwiki"},
		{[]string{"projects", "list", "--filter", "code=wikidata"}, "/v2/projects"},
		{[]string{"languages", "info", "en"}, "/v2/languages/en"},
		{[]string{"codes", "list"}, "/v2/codes"},
		{[]string{"namespaces", "list", "--filter", "identifier=0"}, "/v2/namespaces"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			out, err := run(t, "", append(tt.args, "--output", "json")...)
			if err != nil || path != tt.path || !json.Valid([]byte(out)) {
				t.Fatalf("%s: path %s, error %v, output %s", tt.path, path, err, out)
			}
			if tt.args[0] == "articles" && tt.args[2] == "Albert Einstein" {
				filters := payload["filters"].([]any)
				if filters[0].(map[string]any)["value"] != "enwiki" || len(payload["fields"].([]any)) != 2 {
					t.Fatal("query options lost")
				}
			}
		})
	}
}

func TestValidationDoesNotReadCredentialsOrMakeRequests(t *testing.T) {
	isolate(t, func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected request to %s", r.URL.Host)
	})
	for _, args := range [][]string{
		{"articles", "get", "Squirrel", "--limit", "11"},
		{"snapshots", "list", "--structured", "--wikidata"},
		{"snapshots", "chunks", "list", "enwiki_namespace_0", "--structured"},
		{"snapshots", "info", "../escape"},
		{"batches", "list", "--date", "2026-02-30", "--hour", "12"},
		{"batches", "list", "--date", "2026-11-04", "--hour", "24"},
		{"wikidata", "items", "get", "P31"},
		{"projects", "list", "--filter", "invalid"},
		{"projects", "list", "--filter", "identifier={}"},
		{"projects", "list", "--wiki", "enwiki", "--filter", "is_part_of.identifier=dewiki"},
	} {
		_, err := run(t, "", args...)
		if err == nil || strings.Contains(err.Error(), "saved") || strings.Contains(err.Error(), "unexpected request") {
			t.Fatal("validation deferred until after credential access", args, err)
		}
	}
}

func TestConcurrentRefreshReusesUpdatedCredential(t *testing.T) {
	refreshes := 0
	isolate(t, func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "token-refresh") {
			refreshes++
			return reply(r, `{"access_token":"new-secret","expires_in":86400}`)
		}
		return reply(r, `{"access_token":"initial-secret","refresh_token":"refresh-secret","expires_in":86400}`)
	})
	login(t)
	c, err := client("bot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Token(context.Background(), "initial-secret"); err != nil {
		t.Fatal(err)
	}
	// Another request that was rejected with the old token reuses the refreshed
	// credential instead of consuming another refresh from the account's quota.
	if token, err := c.Token(context.Background(), "initial-secret"); err != nil || token != "new-secret" || refreshes != 1 {
		t.Fatal("redundant refresh", token, err, refreshes)
	}
}

func TestAPIHelperUsesProfileAndTypedJSONInput(t *testing.T) {
	var receivedMethod, receivedPath string
	var receivedBody map[string]any
	isolate(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "auth.enterprise.wikimedia.com" {
			return reply(r, `{"access_token":"access-secret","refresh_token":"refresh-secret","expires_in":86400}`)
		}
		if r.Header.Get("Authorization") != "Bearer access-secret" {
			t.Fatal("raw API helper bypassed stored authentication")
		}
		receivedMethod, receivedPath = r.Method, r.URL.RequestURI()
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		}
		if r.Method == "HEAD" {
			resp, _ := reply(r, "")
			resp.StatusCode = 307
			resp.Header.Set("Location", "https://storage.example/file?secret=signed-secret")
			resp.Header.Set("Content-Length", "1234")
			return resp, nil
		}
		return reply(r, `[{"name":"Squirrel"}]`)
	})
	login(t)
	out, err := run(t, `{"filters":[{"field":"namespace.identifier","value":0}]}`, "api", "articles/Squirrel", "--input", "-")
	if err != nil || receivedMethod != "POST" || receivedPath != "/v2/articles/Squirrel" || !strings.Contains(out, "Squirrel") {
		t.Fatal("raw API request failed", out, err, receivedMethod, receivedPath)
	}
	if receivedBody["filters"].([]any)[0].(map[string]any)["value"] != float64(0) {
		t.Fatal("typed filter lost")
	}
	out, err = run(t, "", "api", "/v2/snapshots/enwiki_namespace_0/download", "--method", "HEAD")
	if err != nil || !strings.Contains(out, "1234") || strings.Contains(out, "secret") {
		t.Fatal("HEAD leaked a signed URL or failed", out, err)
	}
	if _, err := run(t, "", "api", "projects?fields=identifier"); err != nil || receivedPath != "/v2/projects?fields=identifier" || receivedMethod != "GET" {
		t.Fatal("query parameters lost", err, receivedPath)
	}
	for _, path := range []string{"https://evil.example/v2/projects", "//evil.example/v2/projects", "/v2/../v1/login", "/v2/%2e%2e/v1/login", "/v1/login"} {
		if _, err := apiPath(path); err == nil {
			t.Fatal("unsafe raw API path accepted", path)
		}
	}
	for _, input := range []string{`null`, `[]`, `{} {}`, `not-json`} {
		if _, err := run(t, input, "api", "projects", "--input", "-"); err == nil {
			t.Fatal("invalid raw API body accepted", input)
		}
	}
}

func TestRepeatLoginReportsCredentialStoreFailure(t *testing.T) {
	isolate(t, func(r *http.Request) (*http.Response, error) {
		return reply(r, `{"access_token":"access-secret","refresh_token":"refresh-secret","expires_in":86400}`)
	})
	login(t)
	keyring.MockInitWithError(errors.New("locked credential store"))
	out, err := run(t, "password-secret\n", "auth", "login", "--profile", "bot", "--username", "fixture", "--password-stdin", "--provider", "keyring")
	if err == nil || strings.Contains(out, "Saved Enterprise profile") {
		t.Fatal("repeat login hid storage failure", out, err)
	}
}
