package wikiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSitelinksPreserveReplaceClearAndRevision(t *testing.T) {
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.Form.Get("action") {
		case "wbgetentities":
			fmt.Fprint(w, `{"entities":{"Q1":{"id":"Q1","type":"item","lastrevid":12,"sitelinks":{"enwiki":{"site":"enwiki","title":"Old","badges":["Q2"],"url":"https://example.org/Old"}}}}}`)
		case "query":
			fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture"}}}`)
		case "wbsetsitelink":
			posts++
			if r.Form.Get("baserevid") != "12" {
				t.Error(r.Form)
			}
			fmt.Fprint(w, `{"success":1}`)
		default:
			t.Error(r.Form)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	ctx := context.Background()
	for _, test := range []struct {
		badges          []string
		replace, remove bool
		want            string
		present         bool
	}{
		{nil, false, false, "", false}, {[]string{"q2", "Q2", "Q3"}, true, false, "Q2|Q3", true}, {nil, true, false, "", true}, {nil, false, true, "", false},
	} {
		plan, err := client.PlanSitelink(ctx, "q1", "enwiki", "New", test.remove, test.badges, test.replace, 12)
		if err != nil {
			t.Fatal(err)
		}
		if plan.fields.Has("badges") != test.present || plan.fields.Get("badges") != test.want {
			t.Fatal(plan.fields)
		}
		if test.remove && plan.fields.Get("linktitle") != "" {
			t.Fatal(plan.fields)
		}
		if _, err = client.ExecuteWikibasePlan(ctx, plan, EditOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		id, site, title string
		remove          bool
		badges          []string
		replace         bool
		revision        int64
	}{
		{"Q1", "enwiki", "New", false, nil, false, 11},
		{"P1", "enwiki", "New", false, nil, false, 12},
		{"Q1", "bad|site", "New", false, nil, false, 12},
		{"Q1", "enwiki", "", false, nil, false, 12},
		{"Q1", "missingwiki", "", true, nil, false, 12},
		{"Q1", "enwiki", "New", false, []string{"P1"}, true, 12},
	} {
		if _, err := client.PlanSitelink(ctx, test.id, test.site, test.title, test.remove, test.badges, test.replace, test.revision); err == nil {
			t.Fatal(test)
		}
	}
	if posts != 4 {
		t.Fatal(posts)
	}
}

func TestRankOnlyPreservesUnsupportedNativeValues(t *testing.T) {
	fixture := `{"entities":{"Q1":{"id":"Q1","lastrevid":12,"claims":{"P1":[{"id":"opaque","type":"statement","rank":"normal","mainsnak":{"property":"P1","snaktype":"value","datatype":"entity-schema","datavalue":{"type":"string","value":"E1"}},"extension-field":{"kept":true},"references":[]}]}}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatal("write during planning")
		}
		fmt.Fprint(w, fixture)
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	entity, _ := client.Entity(context.Background(), "item", "Q1", nil, "claims", false)
	before, _ := FindStatement(entity, "opaque")
	for _, rank := range []string{"normal", "preferred", "deprecated"} {
		plan, err := client.PlanStatement(context.Background(), "item", "Q1", StatementChange{Operation: "update", ID: "opaque", Rank: rank}, 12, false)
		if err != nil {
			t.Fatal(err)
		}
		after := plan.Change.(Statement)
		for key, value := range before {
			var original, updated interface{}
			_ = json.Unmarshal(value, &original)
			_ = json.Unmarshal(after[key], &updated)
			if key != "rank" && !reflect.DeepEqual(original, updated) {
				t.Fatalf("%s changed: %s", key, after[key])
			}
		}
	}
	for _, change := range []StatementChange{{Operation: "update", ID: "opaque", Rank: "invalid"}, {Operation: "update", ID: "foreign", Rank: "normal"}} {
		if _, err := client.PlanStatement(context.Background(), "item", "Q1", change, 12, false); err == nil {
			t.Fatal(change)
		}
	}
}

func TestComposedRecursiveSelectionsPaginateBeforeFinalFilters(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		key := q.Get("cmtitle")
		module := "categorymembers"
		if q.Has("cmnamespace") || q.Has("srnamespace") {
			t.Error("namespace filtered before recursion/composition")
		}
		if q.Get("list") == "search" {
			key = q.Get("srsearch")
			module = "search"
		}
		calls[key]++
		var pages []Page
		more := ""
		switch key {
		case "Category:Root":
			pages = []Page{{ID: 10, Namespace: 14, Title: "Category:Child"}, {ID: 1, Title: "Alpha"}}
		case "Category:Child":
			pages = []Page{{ID: 11, Namespace: 14, Title: "Category:Root"}, {ID: 2, Title: "Beta"}, {ID: 1, Title: "Alpha"}}
		case "late":
			if q.Get("offset") == "" {
				pages = []Page{{ID: 9, Title: "Other"}}
				more = `,"continue":{"continue":"-||","offset":1}`
			} else {
				pages = []Page{{ID: 2, Title: "Beta"}}
			}
		case "empty":
			pages = []Page{}
		default:
			t.Error(q)
		}
		data, _ := json.Marshal(pages)
		fmt.Fprintf(w, `{"query":{%q:%s}%s}`, module, data, more)
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	ns := 0
	options := PageSelection{Selectors: []PageSelector{{Source: "category", Value: "Root"}, {Source: "search", Value: "late"}}, Combine: "intersection", Depth: 2, MaxCategories: 3, Namespace: &ns, Limit: 1}
	pages, err := client.SelectPages(context.Background(), options)
	if err != nil || len(pages) != 1 || pages[0].Title != "Beta" {
		t.Fatalf("%v %v", pages, err)
	}
	if calls["late"] != 2 || calls["Category:Root"] != 1 || calls["Category:Child"] != 1 {
		t.Fatal(calls)
	}
	options.Combine = "union"
	options.Limit = 2
	pages, err = client.SelectPages(context.Background(), options)
	if err != nil || len(pages) != 2 || pages[0].Title != "Alpha" || pages[1].Title != "Beta" {
		t.Fatalf("%v %v", pages, err)
	}
	options.Combine = "intersection"
	options.Selectors[1].Value = "empty"
	pages, err = client.SelectPages(context.Background(), options)
	if err != nil || len(pages) != 0 {
		t.Fatalf("%v %v", pages, err)
	}
	options.MaxCategories = 1
	if pages, err = client.SelectPages(context.Background(), options); err == nil || pages != nil {
		t.Fatalf("bound silently truncated: %v %v", pages, err)
	}
	options.Depth = 21
	if _, err = client.SelectPages(context.Background(), options); err == nil {
		t.Fatal("unbounded depth")
	}
}
