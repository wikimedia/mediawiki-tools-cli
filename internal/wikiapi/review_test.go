package wikiapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/adrg/xdg"
)

func TestMalformedReadEnvelopesAreNotEmptyResults(t *testing.T) {
	for _, response := range []string{`{}`, `{"query":{"search":null}}`, `{"query":{"pages":[]}}`, `{"query":{"pages":[{}]}}`} {
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, response) }))
			defer server.Close()
			client, _ := New(server.URL+"/w/api.php", server.Client())
			ctx := context.Background()
			if _, err := client.SelectPages(ctx, PageSelection{Source: "search", Value: "Earth"}); err == nil {
				t.Fatal("malformed selection accepted")
			}
			if _, err := client.SelectPages(ctx, PageSelection{Source: "titles", Titles: []string{"Earth"}}); err == nil {
				t.Fatal("malformed title response accepted")
			}
			if _, err := client.SearchEntities(ctx, "item", "Earth", "en", 1); err == nil {
				t.Fatal("malformed entity search accepted")
			}
			if _, err := client.PageInfo(ctx, "Earth", false); err == nil {
				t.Fatal("malformed page response accepted")
			}
			if _, err := client.FileInfo(ctx, "Earth.jpg", false); err == nil {
				t.Fatal("malformed file response accepted")
			}
		})
	}
}

func TestContinuationCannotOverwriteFixedParameters(t *testing.T) {
	fields := url.Values{"action": {"query"}, "titles": {"Earth"}, "list": {"allpages"}}
	state := queryContinuation{}
	for _, key := range []string{"action", "titles", "list"} {
		if _, err := state.advance(fields, map[string]json.RawMessage{key: json.RawMessage(`"changed"`)}); err == nil {
			t.Fatal(key)
		}
		if fields.Get("action") != "query" || fields.Get("titles") != "Earth" {
			t.Fatal(fields)
		}
	}
	for _, value := range []string{`true`, `null`, `{}`, `[]`} {
		if _, err := state.advance(fields, map[string]json.RawMessage{"cursor": json.RawMessage(value)}); err == nil {
			t.Fatal(value)
		}
	}
	if _, err := state.advance(fields, map[string]json.RawMessage{"cursor": json.RawMessage(`"first"`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.advance(fields, map[string]json.RawMessage{"cursor": json.RawMessage(`"second"`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.advance(fields, map[string]json.RawMessage{"cursor": json.RawMessage(`"first"`)}); err == nil {
		t.Fatal("cursor cycle accepted")
	}
}

func TestBulkMutationResponseFailuresAndNochange(t *testing.T) {
	for _, test := range []struct {
		name, operation, response, code string
		ambiguous, success              bool
	}{
		{"truncated-wb", "entity-create", `{"success":1`, "operation_failed", true, false},
		{"html-wb", "entity-create", `<html>Gateway failure</html>`, "operation_failed", true, false},
		{"missing-wb-success", "entity-create", `{}`, "operation_failed", true, false},
		{"wb-failed", "entity-create", `{"success":0}`, "edit-failed", false, false},
		{"permission", "entity-create", `{"error":{"code":"permissiondenied","info":"Denied"}}`, "permissiondenied", false, false},
		{"readonly", "entity-create", `{"errors":[{"code":"readonly","text":"Read only"}]}`, "readonly", false, false},
		{"truncated-page", "page-put", `{"edit":{"result":"Success"}`, "operation_failed", true, false},
		{"page-failed", "page-put", `{"edit":{"result":"Failure","code":"abusefilter-disallowed"}}`, "edit-failed", false, false},
		{"page-nochange", "page-put", `{"edit":{"result":"Success","nochange":true}}`, "", false, true},
		{"wb-nochange", "entity-create", `{"success":1,"nochange":true}`, "", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				if r.Method == http.MethodPost {
					posts++
					fmt.Fprint(w, test.response)
					return
				}
				if r.Form.Get("meta") == "tokens" {
					fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture"}}}`)
				} else {
					fmt.Fprint(w, `{"query":{"pages":[{"title":"Sandbox","lastrevid":5}]}}`)
				}
			}))
			defer server.Close()
			client, _ := New(server.URL+"/w/api.php", server.Client())
			record := `{"operation":"entity-create","kind":"item","data":{"labels":{"en":{"language":"en","value":"Example"}}},"summary":"Fixture"}`
			if test.operation == "page-put" {
				record = `{"operation":"page-put","title":"Sandbox","text":"Example","baserevid":5,"summary":"Fixture"}`
			}
			var output bytes.Buffer
			_, err := client.RunBulk(context.Background(), strings.NewReader(record), &output, BulkOptions{})
			rows := bulkResults(t, output.String())
			if posts != 1 || len(rows) != 1 {
				t.Fatalf("posts=%d rows=%v", posts, rows)
			}
			if test.success {
				if err != nil || rows[0].Status != "success" {
					t.Fatalf("%v %v", rows, err)
				}
			} else {
				if err == nil || rows[0].Error == nil || rows[0].Error.Code != test.code || rows[0].Error.Ambiguous != test.ambiguous {
					t.Fatalf("%s %v", output.String(), err)
				}
			}
		})
	}
}

func TestCanceledRESTRefreshRetainsPublishedIndex(t *testing.T) {
	old := xdg.CacheHome
	xdg.CacheHome = t.TempDir()
	defer func() { xdg.CacheHome = old }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	specs := 0
	transport := reviewRoundTripper(func(r *http.Request) (*http.Response, error) {
		body := `{"modules":{"one":{"spec":"specs/one"},"two":{"spec":"specs/two"}}}`
		if !strings.HasSuffix(r.URL.Path, "discovery") {
			specs++
			if specs == 2 {
				cancel()
				return nil, context.Canceled
			}
			body = `{"openapi":"3.0.0","paths":{}}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	client, _ := New("https://example.org/w/api.php", &http.Client{Transport: transport})
	d := Discovery{Client: client}
	original := RESTIndex{Modules: map[string]RESTModule{"old": {ModuleID: "old", Spec: "specs/old"}}}
	_, _ = schemaCache[RESTIndex](client.Endpoint, "rest-index", "").Refresh(context.Background(), func(context.Context) (RESTIndex, error) { return original, nil })
	if _, err := d.RefreshREST(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	cached, ok := d.CachedRESTIndex()
	if !ok || len(cached.Modules) != 1 || cached.Modules["old"].ModuleID != "old" {
		t.Fatal(cached)
	}
}

type reviewRoundTripper func(*http.Request) (*http.Response, error)

func (f reviewRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
