package wikiapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPageSelectionSourcesPaginationDedupAndLimit(t *testing.T) {
	for _, tc := range []struct{ source, module, prefix, key, value string }{
		{"search", "search", "sr", "srsearch", "earth"}, {"category", "categorymembers", "cm", "cmtitle", "Category:Physics"}, {"backlinks", "backlinks", "bl", "bltitle", "Earth"}, {"transclusions", "embeddedin", "ei", "eititle", "Template:Example"}, {"allpages", "allpages", "ap", "", ""},
	} {
		t.Run(tc.source, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				q := r.URL.Query()
				if q.Get("list") != tc.module || q.Get(tc.prefix+"namespace") != "0" || (tc.key != "" && q.Get(tc.key) != tc.value) {
					t.Errorf("incorrect selector: %v", q)
				}
				switch calls {
				case 1:
					fmt.Fprintf(w, `{"query":{%q:[{"pageid":1,"ns":0,"title":"Earth"},{"pageid":2,"ns":1,"title":"Talk:Earth"}]},"continue":{"continue":"-||","offset":4}}`, tc.module)
				case 2:
					if q.Get("continue") != "-||" || q.Get("offset") != "4" {
						t.Errorf("continuation map lost: %v", q)
					}
					fmt.Fprintf(w, `{"query":{%q:[{"pageid":1,"ns":0,"title":"Earth"},{"pageid":3,"ns":0,"title":"Mars"}]},"continue":{"continue":"next","nextkey":"cursor"}}`, tc.module)
				case 3:
					if q.Has("offset") || q.Get("nextkey") != "cursor" {
						t.Errorf("old continuation fields retained: %v", q)
					}
					fmt.Fprintf(w, `{"query":{%q:[{"pageid":4,"ns":0,"title":"Venus"},{"pageid":5,"ns":0,"title":"Saturn"}]},"continue":{"continue":"more"}}`, tc.module)
				default:
					t.Error("fetched beyond limit")
				}
			}))
			defer server.Close()
			client, _ := New(server.URL+"/w/api.php", server.Client())
			namespace := 0
			value := tc.value
			if tc.source == "category" {
				value = "Physics"
			}
			pages, err := client.SelectPages(context.Background(), PageSelection{Source: tc.source, Value: value, Namespace: &namespace, Limit: 3})
			if err != nil || len(pages) != 3 || pages[0].Title != "Earth" || pages[2].Title != "Venus" || calls != 3 {
				t.Fatalf("pages=%v calls=%d error=%v", pages, calls, err)
			}
		})
	}
}

func TestTitleFileSelectionNormalizesAndPreservesInputOrder(t *testing.T) {
	titles, err := ReadTitles(strings.NewReader("mars\nEarth\nmars\nMissing\nBad[]\n\n"))
	if err != nil || len(titles) != 4 {
		t.Fatalf("%v %v", titles, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("titles") != "mars|Earth|Missing|Bad[]" {
			t.Error(r.URL.Query())
		}
		fmt.Fprint(w, `{"query":{"normalized":[{"from":"mars","to":"Mars"}],"pages":[{"pageid":1,"ns":0,"title":"Earth"},{"pageid":2,"ns":0,"title":"Mars"},{"ns":0,"title":"Missing","missing":true},{"ns":0,"title":"Bad[]","invalid":true}]}}`)
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	pages, err := client.SelectPages(context.Background(), PageSelection{Source: "titles", Titles: titles})
	if err != nil || len(pages) != 2 || pages[0].Title != "Mars" || pages[1].Title != "Earth" {
		t.Fatalf("%v %v", pages, err)
	}
}

func TestPageSelectionRejectsRepeatedCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"query":{"allpages":[]},"continue":{"continue":"same"}}`)
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	if _, err := client.SelectPages(context.Background(), PageSelection{}); err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Fatalf("repeated cursor accepted: %v", err)
	}
}
