package wikiapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiffSelectorsUnifiedFallbackAndReadableHTML(t *testing.T) {
	for _, selector := range []string{"", "rev:0", "rev:wrong", "title:"} {
		if _, err := ParseDiffSelector(selector); err == nil {
			t.Errorf("invalid selector accepted: %q", selector)
		}
	}
	numeric, err := ParseDiffSelector("title:123")
	if err != nil || numeric.Title != "123" {
		t.Fatal(numeric, err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if r.Method != "GET" || q.Get("fromrev") != "3" || q.Get("totitle") != "Earth" {
			t.Error(q)
		}
		if q.Get("difftype") == "unified" {
			fmt.Fprint(w, `{"error":{"code":"badvalue","info":"Unsupported difftype unified"}}`)
			return
		}
		fmt.Fprint(w, `{"compare":{"fromrevid":3,"torevid":4,"body":"<tr><td class='diff-deletedline'><div>old &lt;tag&gt;</div></td><td class='diff-addedline'><div>new &amp; value</div></td></tr><tr><td class='diff-context'><div>same</div></td><td class='diff-context'><div>same</div></td></tr>"}}`)
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	diff, err := client.PageDiff(context.Background(), "rev:3", "title:Earth", true)
	if err != nil || calls != 2 {
		t.Fatal(calls, err)
	}
	text, err := DiffText(rawString(diff["body"]))
	if err != nil || text != "-old <tag>\n+new & value\n same\n" {
		t.Fatalf("%q %v", text, err)
	}
	text, err = DiffText("<tr><td><pre>@@ -1 +1 @@\n-old &lt;text&gt;\n+new\n</pre></td></tr>")
	if err != nil || !strings.Contains(text, "-old <text>\n+new") {
		t.Fatalf("unified text: %q %v", text, err)
	}
}

func TestSingleDiffDirectionForTitlesAndRevisionIDs(t *testing.T) {
	for _, selector := range []string{"Earth", "rev:20", "20"} {
		t.Run(selector, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				q := r.URL.Query()
				if q.Get("action") == "query" {
					if q.Get("rvprop") != "ids" || q.Get("prop") != "revisions" {
						t.Error(q)
					}
					if selector == "Earth" {
						if q.Get("titles") != "Earth" || q.Get("rvlimit") != "1" {
							t.Error(q)
						}
					} else if q.Get("revids") != "20" || q.Has("rvlimit") {
						t.Error(q)
					}
					fmt.Fprint(w, `{"query":{"pages":[{"title":"Earth","revisions":[{"revid":20,"parentid":19}]}]}}`)
					return
				}
				if q.Get("fromrev") != "19" || q.Get("torev") != "20" || q.Has("torelative") || q.Has("fromtitle") {
					t.Error(q)
				}
				fmt.Fprint(w, `{"compare":{"fromrevid":19,"torevid":20,"body":"<tr><td class='diff-deletedline'><div>removed</div></td><td class='diff-addedline'><div>added</div></td></tr>"}}`)
			}))
			defer server.Close()
			client, _ := New(server.URL+"/w/api.php", server.Client())
			diff, err := client.PageDiff(context.Background(), selector, "", false)
			if err != nil {
				t.Fatal(err)
			}
			text, err := DiffText(rawString(diff["body"]))
			if err != nil || text != "-removed\n+added\n" || requests != 2 {
				t.Fatalf("%q requests=%d %v", text, requests, err)
			}
		})
	}
}

func TestSingleDiffUnavailableAndFirstRevision(t *testing.T) {
	for _, response := range []string{
		`{"query":{"pages":[{"revisions":[{"revid":1,"parentid":0}]}]}}`,
		`{"query":{"pages":[{"title":"Missing","missing":true}]}}`,
		`{"query":{"badrevids":{"20":{"revid":20,"missing":true}}}}`,
		`{"query":{"pages":[{"revisions":[{"revid":20}]}]}}`,
	} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, response) }))
		client, _ := New(server.URL+"/w/api.php", server.Client())
		_, err := client.PageDiff(context.Background(), "Earth", "", false)
		server.Close()
		if err == nil || calls != 1 {
			t.Fatalf("%s calls=%d %v", response, calls, err)
		}
	}
}

func TestFileSharedMetadataAndPaginatedLocalUsage(t *testing.T) {
	assetRequests, metadataCalls, usageCalls := 0, 0, 0
	asset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assetRequests++
		t.Error("metadata client fetched an asset")
	}))
	defer asset.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method != "GET" || q.Get("titles") != "File:Example.jpg" {
			t.Error(q)
		}
		if q.Get("prop") == "imageinfo" {
			metadataCalls++
			fmt.Fprintf(w, `{"query":{"pages":[{"ns":6,"title":"File:Example.jpg","missing":true,"imagerepository":"shared","imageinfo":[{"url":%q,"mime":"image/jpeg","width":20}]}]},"continue":{"continue":"||","iicontinue":"old-version"}}`, asset.URL+"/asset.jpg")
			return
		}
		usageCalls++
		if q.Get("prop") != "fileusage" || q.Has("iicontinue") || q.Get("funamespace") != "0" {
			t.Error("usage accidentally queried upload history", q)
		}
		if usageCalls == 1 {
			fmt.Fprint(w, `{"query":{"pages":[{"ns":6,"title":"File:Example.jpg","missing":true,"fileusage":[{"pageid":1,"ns":0,"title":"Earth","redirect":true},{"pageid":2,"ns":1,"title":"Talk:Earth"}]}]},"continue":{"continue":"||","fucontinue":"next"}}`)
		} else {
			if q.Get("fucontinue") != "next" || q.Get("continue") != "||" {
				t.Error(q)
			}
			fmt.Fprint(w, `{"query":{"pages":[{"ns":6,"title":"File:Example.jpg","missing":true,"fileusage":[{"pageid":1,"ns":0,"title":"Earth"},{"pageid":3,"ns":0,"title":"Moon"}]}]},"continue":{"continue":"more","fucontinue":"later"}}`)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	info, err := client.FileInfo(context.Background(), "Example.jpg", true)
	if err != nil || rawString(info["imagerepository"]) != "shared" {
		t.Fatal(info, err)
	}
	ns := 0
	pages, err := client.FileUsage(context.Background(), "Example.jpg", &ns, 2)
	if err != nil || len(pages) != 2 || pages[1].Title != "Moon" || !pages[0].Redirect || usageCalls != 2 || metadataCalls != 2 || assetRequests != 0 {
		t.Fatalf("%+v metadata=%d usage=%d assets=%d err=%v", pages, metadataCalls, usageCalls, assetRequests, err)
	}
	if _, err = FileTitle(asset.URL); err == nil {
		t.Fatal("asset URL accepted as title")
	}
	if title, err := FileTitle("Datei:Example.jpg"); err != nil || title != "Datei:Example.jpg" {
		t.Fatal(title, err)
	}
}

func TestSingleDiffHiddenPredecessorReturnsAPIError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("action") == "query" {
			fmt.Fprint(w, `{"query":{"pages":[{"revisions":[{"revid":20,"parentid":19}]}]}}`)
		} else {
			fmt.Fprint(w, `{"error":{"code":"missingcontent","info":"Predecessor content is not available"}}`)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	_, err := client.PageDiff(context.Background(), "20", "", false)
	if err == nil || !strings.Contains(err.Error(), "missingcontent") || calls != 2 {
		t.Fatal(calls, err)
	}
}
