package wiki

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSitelinkAndRankOnlyCommandsDryRun(t *testing.T) {
	isolateWikiProfiles(t)
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			t.Error("dry-run wrote")
		}
		fmt.Fprint(w, `{"entities":{"Q1":{"id":"Q1","lastrevid":12,"type":"item","sitelinks":{"enwiki":{"site":"enwiki","title":"Old","badges":["Q2"]}},"claims":{"P1":[{"id":"opaque","type":"statement","rank":"normal","mainsnak":{"property":"P1","snaktype":"novalue"}}]}}}}`)
	}))
	defer server.Close()
	base := []string{"--anon", "--wiki", server.URL + "/w/api.php", "ext", "wikibase", "item"}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"sitelink", "list", "Q1"}, `"enwiki"`},
		{[]string{"sitelink", "get", "Q1", "enwiki"}, `"Old"`},
		{[]string{"sitelink", "set", "Q1", "enwiki", "New", "--dry-run"}, `"preserve"`},
		{[]string{"sitelink", "set", "Q1", "enwiki", "New", "--badge", "Q3", "--dry-run"}, `"replace"`},
		{[]string{"sitelink", "set", "Q1", "enwiki", "New", "--clear-badges", "--dry-run"}, `"badges":[]`},
		{[]string{"sitelink", "remove", "Q1", "enwiki", "--dry-run"}, `"remove":true`},
		{[]string{"statement", "update", "Q1", "opaque", "--rank", "deprecated", "--dry-run"}, `"rank":"deprecated"`},
	} {
		out, err := runWikiProfile(t, append(append([]string{}, base...), test.args...), "")
		if err != nil || !strings.Contains(out, test.want) {
			t.Fatalf("%v: %s %v", test.args, out, err)
		}
	}
	if posts != 0 {
		t.Fatal(posts)
	}
}

func TestComposedPageCLIParsesFilesAndRejectsCompatMix(t *testing.T) {
	isolateWikiProfiles(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("list") == "search" {
			fmt.Fprint(w, `{"query":{"search":[{"pageid":1,"ns":0,"title":"Earth"}]}}`)
		} else {
			fmt.Fprint(w, `{"query":{"pages":[{"pageid":1,"ns":0,"title":"Earth"},{"pageid":2,"ns":0,"title":"Mars"}]}}`)
		}
	}))
	defer server.Close()
	base := []string{"--anon", "--wiki", server.URL + "/w/api.php", "page", "list"}
	out, err := runWikiProfile(t, append(append([]string{}, base...), "--select", "search:planet", "--select", "titles:-", "--combine", "intersection", "--limit", "1"), "Earth\nMars\n")
	if err != nil || strings.TrimSpace(out) != "Earth" {
		t.Fatalf("%s %v", out, err)
	}
	for _, args := range [][]string{
		{"--select", "titles:-", "--select", "titles:-"},
		{"--select", "search:planet", "--pywikibot=-ref:Earth"},
		{"--pywikibot=-cat:Physics", "--depth", "1"},
		{"--select", "bad:planet"},
		{"--select", "search:planet", "--prefix", "E"},
	} {
		if out, err = runWikiProfile(t, append(append([]string{}, base...), args...), ""); err == nil {
			t.Fatalf("accepted %v: %s", args, out)
		}
	}
}
