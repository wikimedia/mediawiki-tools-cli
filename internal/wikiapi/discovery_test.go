package wikiapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adrg/xdg"
)

func TestDiscoveryCacheInstallationIsolationAndOfflinePeek(t *testing.T) {
	old := xdg.CacheHome
	xdg.CacheHome = t.TempDir()
	defer func() { xdg.CacheHome = old }()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("public discovery received credentials")
		}
		_ = r.ParseForm()
		if r.Form.Get("action") == "paraminfo" {
			fmt.Fprint(w, `{"paraminfo":{"modules":[{"name":"query","path":"query","parameters":[{"name":"prop","type":["info","revisions"]}]}]}}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "api.php") {
			fmt.Fprint(w, `{"query":{"general":{"generator":"MediaWiki"},"namespaces":{"0":{"name":""}}}}`)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "discovery"):
			fmt.Fprintf(w, `{"modules":{"v1":{"spec":%q},"evil":{"spec":"https://example.invalid/schema"}}}`, serverURL(r)+"/a/rest.php/specs/v0/module/v1")
		case strings.HasSuffix(r.URL.Path, "/v1"):
			fmt.Fprint(w, `{"openapi":"3.0.0","paths":{"/page/{title}/bare":{"get":{"parameters":[{"name":"language","in":"query","schema":{"enum":["en","de"]}}]}}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(server.URL+"/a/api.php", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	d := Discovery{Client: client}
	if _, err = d.RefreshAction(context.Background(), []string{"query"}); err != nil {
		t.Fatal(err)
	}
	if names, e := d.RefreshREST(context.Background()); e != nil || len(names) != 1 {
		t.Fatalf("schemas: %v %v", names, e)
	}
	count := requests
	if m, ok := d.CachedModule("query"); !ok || m.Parameters[0].Values()[0] != "info" {
		t.Fatal("missing cached module")
	}
	if _, ok := d.CachedSite(); !ok {
		t.Fatal("missing site capabilities")
	}
	if _, ok := d.CachedREST("v1"); !ok {
		t.Fatal("missing REST spec")
	}
	second, _ := New(server.URL+"/b/api.php", server.Client())
	if _, ok := (Discovery{Client: second}).CachedModule("query"); ok {
		t.Fatal("sibling installation reused cache")
	}
	if requests != count {
		t.Fatal("cache reads made requests")
	}
}
func serverURL(r *http.Request) string { return "http://" + r.Host }
