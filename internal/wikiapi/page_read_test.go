package wikiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPageInfoPreservesNativeMetadataAndRedirectChoice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method != "GET" || q.Get("prop") != "info" || !strings.Contains(q.Get("inprop"), "protection") {
			t.Error(q)
		}
		switch q.Get("titles") {
		case "Missing":
			fmt.Fprint(w, `{"query":{"pages":[{"title":"Missing","missing":true}]}}`)
		case "Bad[]":
			fmt.Fprint(w, `{"query":{"pages":[{"title":"Bad[]","invalid":true}]}}`)
		default:
			target := "Redirect"
			if q.Get("redirects") == "1" {
				target = "Target"
			}
			fmt.Fprintf(w, `{"query":{"pages":[{"pageid":10,"ns":0,"title":%q,"fullurl":"https://example.invalid/wiki/Target","protection":[{"type":"edit","level":"sysop"}],"extension-field":{"kept":true}}]}}`, target)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	for _, follow := range []bool{false, true} {
		page, err := client.PageInfo(context.Background(), "Redirect", follow)
		if err != nil {
			t.Fatal(err)
		}
		var title string
		_ = json.Unmarshal(page["title"], &title)
		want := "Redirect"
		if follow {
			want = "Target"
		}
		if title != want || !json.Valid(page["extension-field"]) {
			t.Fatalf("metadata lost: %v", page)
		}
	}
	for _, title := range []string{"Missing", "Bad[]", ""} {
		if _, err := client.PageInfo(context.Background(), title, false); err == nil {
			t.Errorf("invalid or missing title accepted: %q", title)
		}
	}
}

func TestPageHistoryPaginationHiddenMetadataAndTimestampNormalization(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if q.Get("prop") != "revisions" || q.Get("rvdir") != "older" || q.Get("rvstart") != "2026-01-01T00:00:00Z" || q.Get("rvuser") != "Fixture" || q.Get("rvlimit") != "3" {
			t.Error(q)
		}
		if strings.Contains(q.Get("rvprop"), "content") {
			t.Error("history downloaded content")
		}
		if calls == 1 {
			fmt.Fprint(w, `{"query":{"pages":[{"pageid":1,"title":"Earth","revisions":[{"revid":3,"parentid":2,"timestamp":"2026-01-01T00:00:00Z","user":"Fixture","size":100,"comment":"Edit","tags":["tag"]},{"revid":2,"parentid":1,"timestamp":"2025-12-31T00:00:00Z","userhidden":true,"commenthidden":true,"size":90}]}]},"continue":{"continue":"||","rvcontinue":"opaque|cursor"}}`)
		} else {
			if q.Get("continue") != "||" || q.Get("rvcontinue") != "opaque|cursor" {
				t.Error(q)
			}
			fmt.Fprint(w, `{"query":{"pages":[{"pageid":1,"title":"Earth","revisions":[{"revid":2,"size":90},{"revid":1,"parentid":0,"timestamp":"2025-12-30T00:00:00Z","user":"Fixture","size":80}]}]},"continue":{"continue":"more","rvcontinue":"next"}}`)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	history, err := client.PageHistory(context.Background(), "Earth", HistoryOptions{Limit: 3, Start: "2026-01-01T01:00:00+01:00", User: "Fixture"})
	if err != nil || len(history) != 3 || calls != 2 || history[2].ID != 1 || !history[1].UserHidden || !history[1].CommentHidden || len(history[0].Tags) != 1 {
		t.Fatalf("history=%+v calls=%d err=%v", history, calls, err)
	}
}

func TestPageCategoriesFilterPaginationAndDistinctLimit(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if q.Get("prop") != "categories" || q.Get("clshow") != "!hidden" || q.Get("cllimit") != "2" {
			t.Error(q)
		}
		if calls == 1 {
			fmt.Fprint(w, `{"query":{"pages":[{"pageid":1,"title":"Earth","categories":[{"ns":14,"title":"Category:A","sortkeyprefix":"Earth"},{"ns":14,"title":"Category:Hidden","hidden":true}]}]},"continue":{"continue":"||","clcontinue":"cursor"}}`)
		} else {
			if q.Get("continue") != "||" || q.Get("clcontinue") != "cursor" {
				t.Error(q)
			}
			fmt.Fprint(w, `{"query":{"pages":[{"pageid":1,"title":"Earth","categories":[{"ns":14,"title":"Category:A"},{"ns":14,"title":"Category:B"}]}]},"continue":{"continue":"more","clcontinue":"next"}}`)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	categories, err := client.PageCategories(context.Background(), "Earth", "exclude", 2)
	if err != nil || len(categories) != 2 || calls != 2 || categories[0].SortKeyPrefix != "Earth" || categories[1].Title != "Category:B" {
		t.Fatalf("%+v calls=%d %v", categories, calls, err)
	}
}

func TestPageReadValidationAndRepeatedContinuation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"query":{"pages":[{"pageid":1,"title":"Earth","revisions":[]}]},"continue":{"continue":"same"}}`)
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	for _, options := range []HistoryOptions{{Limit: -1, Order: "older"}, {Order: "wrong"}, {Order: "older", Start: "yesterday"}, {Order: "older", User: "One", ExcludeUser: "Two"}} {
		if _, err := client.PageHistory(context.Background(), "Earth", options); err == nil {
			t.Errorf("invalid history options accepted: %+v", options)
		}
	}
	if _, err := client.PageCategories(context.Background(), "Earth", "wrong", 1); err == nil {
		t.Fatal("invalid category filter accepted")
	}
	if calls != 0 {
		t.Fatal("invalid parameters contacted wiki")
	}
	if _, err := client.PageHistory(context.Background(), "Earth", HistoryOptions{}); err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Fatalf("repeated history cursor accepted: %v", err)
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}
