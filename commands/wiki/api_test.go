package wiki

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func seedAPIOAuth(t *testing.T, endpoint string) {
	t.Helper()
	a := config.WikiProfile{Realm: "fixture", Name: "personal", Username: "Fixture", Method: "oauth", Endpoint: endpoint, Wiki: endpoint, ClientID: "fixture-client", Sites: []string{endpoint}}
	metadata := a.Metadata()
	metadata["wiki.realms.fixture.endpoint"] = endpoint
	metadata["wiki.default_profile"] = "fixture/personal"
	metadata["wiki.realms.fixture.default_profile"] = "personal"
	tokens, _ := json.Marshal(wikiauth.Tokens{AccessToken: "api-fixture", TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour)})
	if err := a.Credential().SaveWithProvider(string(tokens), false, metadata, "keyring"); err != nil {
		t.Fatal(err)
	}
}

func TestWikiAPIProfilesAnonymousAndREST(t *testing.T) {
	isolateWikiProfiles(t)
	authorized := 0
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") == "Bearer api-fixture" {
			authorized++
		}
		if strings.HasSuffix(r.URL.Path, "/bare") {
			fmt.Fprint(w, `{"title":"Earth"}`)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("action") == "fail" {
			fmt.Fprint(w, `{"error":{"code":"badvalue","info":"fixture failure"}}`)
			return
		}
		if r.Form.Get("action") != "query" || r.Form.Get("titles") != "Earth" || r.Form.Get("prop") != "info" {
			t.Errorf("incorrect parameters: %v", r.Form)
		}
		fmt.Fprint(w, `{"query":{"pages":[{"title":"Earth"}]}}`)
	}))
	defer server.Close()
	endpoint := server.URL + "/custom/api.php"
	seedAPIOAuth(t, endpoint)
	for _, args := range [][]string{{"api", "query", "-f", "titles=Earth", "-f", "prop=info"}, {"action", "query", "-f", "titles=Earth", "-f", "prop=info"}, {"rest", "get", "/v1/page/Earth/bare"}} {
		out, err := runWikiProfile(t, args, "")
		if err != nil || !strings.Contains(out, "Earth") {
			t.Fatalf("%v: %s %v", args, out, err)
		}
	}
	if authorized != 3 {
		t.Fatalf("only %d requests authenticated", authorized)
	}
	keyring.MockInitWithError(errors.New("locked"))
	// Default site's metadata is independent of default credentials.
	out, err := runWikiProfile(t, []string{"rest", "get", "/v1/page/Earth/bare", "--anon"}, "")
	if err != nil || !strings.Contains(out, "Earth") || authorized != 3 {
		t.Fatalf("anonymous default-site request: %s %v", out, err)
	}
	// Explicit site selection must not enforce the unrelated profile's auth scope.
	_, err = runWikiProfile(t, []string{"rest", "get", "/v1/page/Earth/bare", "--anon", "--profile", "fixture/personal", "--wiki", server.URL + "/other/api.php"}, "")
	if err != nil || authorized != 3 {
		t.Fatalf("anonymous site selection: %v", err)
	}
	_, err = runWikiProfile(t, []string{"api", "fail", "--anon"}, "")
	if err == nil || !strings.Contains(err.Error(), "badvalue") {
		t.Fatalf("HTTP 200 API error accepted: %v", err)
	}
	if requests != 6 {
		t.Fatalf("unexpected extra login requests: %d", requests)
	}
}

func TestWikiDiscoveryAndCompletionNeverUseCredentials(t *testing.T) {
	isolateWikiProfiles(t)
	oldCache := xdg.CacheHome
	xdg.CacheHome = t.TempDir()
	defer func() { xdg.CacheHome = oldCache }()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("discovery authenticated")
		}
		_ = r.ParseForm()
		switch r.Form.Get("action") {
		case "paraminfo":
			fmt.Fprint(w, `{"paraminfo":{"modules":[{"name":"main","path":"main","parameters":[{"name":"action","type":["query","wbgetentities"]}]},{"name":"query","path":"query","parameters":[{"name":"prop","type":["info","revisions"]}]},{"name":"revisions","path":"query+revisions","prefix":"rv","parameters":[{"name":"prop","description":"Revision fields","type":["ids","content"]}]},{"name":"readinglists","path":"readinglists","parameters":[{"name":"command","type":["create"],"submodules":{"create":"readinglists+create"}}]},{"name":"create","path":"readinglists+create","prefix":"rl","parameters":[{"name":"name","type":"string"}]}]}}`)
		case "query":
			fmt.Fprint(w, `{"query":{"general":{"generator":"MediaWiki"}}}`)
		default:
			switch {
			case strings.HasSuffix(r.URL.Path, "/discovery"):
				fmt.Fprint(w, `{"modules":{"wikibase/v1":{"spec":"wikibase/v1/openapi.json"}}}`)
			case strings.HasSuffix(r.URL.Path, "/openapi.json"):
				fmt.Fprint(w, `{"openapi":"3.0.0","paths":{"/entities/items/{item_id}":{"get":{"parameters":[{"$ref":"#/components/parameters/language"}]}}},"components":{"parameters":{"language":{"name":"language","in":"query","schema":{"$ref":"#/components/schemas/language"}}},"schemas":{"language":{"enum":["en","de"]}}}}`)
			default:
				http.NotFound(w, r)
			}
		}
	}))
	defer server.Close()
	seedAPIOAuth(t, server.URL+"/custom/api.php")
	keyring.MockInitWithError(errors.New("locked"))
	if out, err := runWikiProfile(t, []string{"api", "refresh", "main", "query", "query+revisions"}, ""); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	if out, err := runWikiProfile(t, []string{"rest", "refresh"}, ""); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	before := calls
	cmd := newWikiAPICmd()
	_ = cmd.Flags().Set("field", "prop=revisions")
	values, _ := completeAPIFields(cmd, []string{"query"}, "rvprop=c")
	if len(values) != 1 || values[0] != "rvprop=content" {
		t.Fatalf("wrong parameter completion: %v", values)
	}
	values, _ = completeAPIActions(cmd, nil, "wb")
	if len(values) != 1 || values[0] != "wbgetentities" {
		t.Fatalf("wrong action completion: %v", values)
	}
	nested := newWikiAPICmd()
	_ = nested.Flags().Set("field", "command=create")
	values, _ = completeAPIFields(nested, []string{"readinglists"}, "rlna")
	if len(values) != 1 || !strings.HasPrefix(values[0], "rlname=") {
		t.Fatalf("nested extension parameter missing: %v", values)
	}
	values, _ = completeAPIModules(nested, nil, "readinglists+")
	if len(values) != 1 || values[0] != "readinglists+create" {
		t.Fatalf("nested module absent from describe completion: %v", values)
	}
	values, _ = completeAPIFields(cmd, []string{"query"}, "action")
	if len(values) != 0 {
		t.Fatalf("invalid action= parameter suggested: %v", values)
	}
	if _, err := runWikiProfile(t, []string{"api", "describe", "query+revisions"}, ""); err != nil {
		t.Fatal(err)
	}
	rest := newWikiRESTCmd()
	values, _ = completeRESTPaths(rest, []string{"get"}, "/wikibase")
	if len(values) != 1 || values[0] != "/wikibase/v1/entities/items/{item_id}" {
		t.Fatalf("wrong REST path completion: %v", values)
	}
	values, _ = completeRESTFields(rest, []string{"get", "/wikibase/v1/entities/items/Q42"}, "language=e")
	if len(values) != 1 || values[0] != "language=en" {
		t.Fatalf("wrong REST parameter completion: %v", values)
	}
	if calls != before {
		t.Fatal("completion/describe contacted wiki")
	}
}

func TestWikiRESTBodyTypesAndActionCSRF(t *testing.T) {
	isolateWikiProfiles(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.Contains(r.URL.Path, "rest.php") {
			var input struct {
				Value int `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Value != 42 || r.Method != "PATCH" {
				t.Errorf("REST JSON types lost: %v", err)
			}
			fmt.Fprint(w, `{"ok":true}`)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("meta") == "tokens" {
			fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture-csrf"}}}`)
			return
		}
		if r.Method != "POST" || r.Form.Get("token") != "fixture-csrf" {
			t.Errorf("incorrect write: %s %v", r.Method, r.Form)
		}
		fmt.Fprint(w, `{"edit":{"result":"Success"}}`)
	}))
	defer server.Close()
	endpoint := server.URL + "/w/api.php"
	_, err := runWikiProfile(t, []string{"rest", "patch", "/wikibase/v1/entities/items/Q42", "--anon", "--wiki", endpoint, "--input", "-"}, `{"value":42}`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runWikiProfile(t, []string{"api", "edit", "--anon", "--wiki", endpoint, "--method", "POST", "--token", "csrf", "-f", "title=Sandbox", "-f", "text=example"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatal(calls)
	}
}

func TestWikiPasswordRESTSessionDoesNotCarryIntoAnonymousRequests(t *testing.T) {
	isolateWikiProfiles(t)
	logins := 0
	restRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.Form.Get("action") {
		case "query":
			fmt.Fprint(w, `{"query":{"tokens":{"logintoken":"fixture-login"}}}`)
		case "login":
			logins++
			if r.Form.Get("lgpassword") != "fixture-password" {
				t.Error("password login lost")
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "fixture", Path: "/"})
			fmt.Fprint(w, `{"login":{"result":"Success"}}`)
		default:
			restRequests++
			cookie, err := r.Cookie("session")
			if restRequests == 1 && (err != nil || cookie.Value != "fixture") {
				t.Error("REST lost logged-in cookie session")
			}
			if restRequests == 2 && err == nil {
				t.Error("anonymous request carried cookie")
			}
			fmt.Fprint(w, `{"title":"Earth"}`)
		}
	}))
	defer server.Close()
	endpoint := server.URL + "/w/api.php"
	a := config.WikiProfile{Realm: "fixture", Name: "bot", Username: "Fixture@bot", Method: "password", Endpoint: endpoint, Wiki: endpoint}
	metadata := a.Metadata()
	metadata["wiki.default_profile"] = "fixture/bot"
	metadata["wiki.realms.fixture.default_profile"] = "bot"
	metadata["wiki.realms.fixture.endpoint"] = endpoint
	if err := a.Credential().SaveWithProvider("fixture-password", false, metadata, "keyring"); err != nil {
		t.Fatal(err)
	}
	if _, err := runWikiProfile(t, []string{"rest", "get", "/v1/page/Earth/bare"}, ""); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(errors.New("locked"))
	if _, err := runWikiProfile(t, []string{"rest", "get", "/v1/page/Earth/bare", "--anon"}, ""); err != nil {
		t.Fatal(err)
	}
	if logins != 1 || restRequests != 2 {
		t.Fatalf("unexpected session activity: %d logins, %d REST calls", logins, restRequests)
	}
}

func TestWikiAnonymousPageRequestsDoNotRetainCookies(t *testing.T) {
	isolateWikiProfiles(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("anonymous page request carried credentials")
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "unexpected", Path: "/"})
		if r.URL.Query().Get("meta") == "siteinfo" {
			fmt.Fprint(w, `{"query":{"general":{"mainpage":"Earth"}}}`)
			return
		}
		fmt.Fprint(w, `{"query":{"pages":[{"title":"Earth","revisions":[{"slots":{"main":{"content":"Earth source"}}}]}]}}`)
	}))
	defer server.Close()
	if out, err := runWikiProfile(t, []string{"page", "get", "--anon", "--wiki", server.URL + "/w/api.php"}, ""); err != nil || out != "Earth source" {
		t.Fatalf("%q %v", out, err)
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestRepeatedRESTQueryParametersAndActionDuplicateError(t *testing.T) {
	isolateWikiProfiles(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		values := r.URL.Query()["tag"]
		if len(values) != 2 || values[0] != "first" || values[1] != "second" {
			t.Errorf("repeated query values lost: %v", values)
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()
	endpoint := server.URL + "/w/api.php"
	if _, err := runWikiProfile(t, []string{"rest", "get", "/v1/example", "--anon", "--wiki", endpoint, "-f", "tag=first", "-f", "tag=second"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := runWikiProfile(t, []string{"api", "query", "--anon", "--wiki", endpoint, "-f", "prop=info", "-f", "prop=revisions"}, ""); err == nil || !strings.Contains(err.Error(), "duplicate field") {
		t.Fatalf("Action duplicate was not diagnosed: %v", err)
	}
	if requests != 1 {
		t.Fatal("invalid Action parameters reached server")
	}
}

func TestRESTSchemaBasePaths(t *testing.T) {
	for _, tc := range []struct{ name, module, server, path, want string }{
		{"modern module", "wikibase/v1", "https://test.wikidata.org/w/rest.php/wikibase/v1", "/entities/items/{item_id}", "/wikibase/v1/entities/items/{item_id}"},
		{"legacy example origin", "wikibase/v1", "https://wikibase.example/w/rest.php", "/wikibase/v1/entities/items/{item_id}", "/wikibase/v1/entities/items/{item_id}"},
		{"legacy no server", "wikibase/v1", "", "/wikibase/v1/entities/items/{item_id}", "/wikibase/v1/entities/items/{item_id}"},
		{"custom script path", "extension/v2", "/custom/rest.php/extension/v2", "/thing", "/extension/v2/thing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var spec wikiapi.RESTSpec
			if err := json.Unmarshal([]byte(fmt.Sprintf(`{"servers":[{"url":%q}]}`, tc.server)), &spec); err != nil {
				t.Fatal(err)
			}
			if got := restRoute(wikiapi.Discovery{}, tc.module, spec, tc.path); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
