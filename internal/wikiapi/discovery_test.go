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

func TestActionDiscoveryFollowsNestedExtensionModules(t *testing.T) {
	old := xdg.CacheHome
	xdg.CacheHome = t.TempDir()
	defer func() { xdg.CacheHome = old }()
	fetched := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("action") == "query" {
			fmt.Fprint(w, `{"query":{"general":{"generator":"MediaWiki"}}}`)
			return
		}
		modules := []string{}
		for _, name := range strings.Split(query.Get("modules"), "|") {
			if fetched[name] {
				t.Errorf("module fetched twice: %s", name)
			}
			fetched[name] = true
			switch name {
			case "main":
				modules = append(modules, `{"name":"main","path":"main","parameters":[{"name":"action","type":["readinglists"],"submodules":{"readinglists":"readinglists","query":"query"}}]}`)
			case "query":
				modules = append(modules, `{"name":"query","path":"query","parameters":[]}`)
			case "readinglists":
				modules = append(modules, `{"name":"readinglists","path":"readinglists","parameters":[{"name":"command","submodules":{"create":"readinglists+create"}}]}`)
			case "readinglists+create":
				modules = append(modules, `{"name":"create","path":"readinglists+create","parameters":[{"name":"nested","submodules":{"more":"readinglists+create+more","cycle":"readinglists"}}]}`)
			case "readinglists+create+more":
				modules = append(modules, `{"name":"more","path":"readinglists+create+more","parameters":[{"name":"name","type":"string"}]}`)
			default:
				t.Errorf("unexpected module: %s", name)
			}
		}
		fmt.Fprintf(w, `{"paraminfo":{"modules":[%s]}}`, strings.Join(modules, ","))
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	d := Discovery{Client: client}
	modules, err := d.RefreshAction(context.Background(), nil)
	if err != nil || len(modules) != 5 {
		t.Fatalf("recursive modules: %d %v", len(modules), err)
	}
	if _, ok := d.CachedModule("readinglists+create+more"); !ok {
		t.Fatal("deep extension module missing")
	}
	if index, ok := d.CachedActionIndex(); !ok || len(index.Modules) != 5 {
		t.Fatalf("incorrect module index: %v", index)
	}
}

func TestRESTDiscoveryIncludesUnadvertisedWikibaseAndOnlyUsableSpecs(t *testing.T) {
	old := xdg.CacheHome
	xdg.CacheHome = t.TempDir()
	defer func() { xdg.CacheHome = old }()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wiki/rest.php/specs/v0/discovery":
			fmt.Fprint(w, `{"modules":{"v1":{"spec":"/wiki/rest.php/specs/v0/module/v1"},"unavailable":{"spec":"specs/v0/module/unavailable"}}}`)
		case "/wiki/rest.php/specs/v0/module/v1":
			fmt.Fprint(w, `{"openapi":"3.0.0","paths":{"/page/{title}":{"get":{}}}}`)
		case "/wiki/rest.php/wikibase/v1/openapi.json":
			fmt.Fprint(w, `{"openapi":"3.0.0","servers":[{"url":"https://wikibase.example/w/rest.php"}],"paths":{"/wikibase/v1/entities/items/{item_id}":{"get":{}}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/wiki/api.php", server.Client())
	d := Discovery{Client: client}
	names, err := d.RefreshREST(context.Background())
	if err != nil || len(names) != 2 {
		t.Fatalf("schemas: %v %v", names, err)
	}
	index, ok := d.CachedRESTIndex()
	if !ok || len(index.Modules) != 2 {
		t.Fatalf("index differs from successfully cached schemas: %v", index)
	}
	if _, ok := index.Modules["unavailable"]; ok {
		t.Fatal("unusable schema advertised as cached")
	}
	if _, ok := d.CachedREST("wikibase/v1"); !ok {
		t.Fatal("legacy Wikibase schema missing despite generic discovery success")
	}
}
