package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDocumentationReadsRetryGatewayResponsesWithoutLeakingBodies(t *testing.T) {
	calls, waits := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, "<html>private diagnostic</html>")
			return
		}
		io.WriteString(w, `{"query":{}}`)
	}))
	defer server.Close()
	client := &http.Client{Transport: documentationTransport{base: http.DefaultTransport, wait: func(_ context.Context, delay time.Duration) error {
		waits++
		if delay < 2*time.Second {
			t.Error("Retry-After ignored")
		}
		return nil
	}}}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if calls != 3 || waits != 2 {
		t.Fatalf("calls=%d, waits=%d", calls, waits)
	}
	calls = 0
	response, err = client.Post(server.URL, "application/json", strings.NewReader(`{}`))
	if response != nil {
		response.Body.Close()
	}
	if err == nil || calls != 1 || !strings.Contains(err.Error(), "HTTP 503") || strings.Contains(err.Error(), "private diagnostic") {
		t.Fatalf("write replayed or error response exposed: calls=%d, err=%v", calls, err)
	}
}

func TestPublicationRecoversOnlyAfterCheckingTheEditOutcome(t *testing.T) {
	oldWait := publicationRetryWait
	publicationRetryWait = func(time.Duration) {}
	t.Cleanup(func() { publicationRetryWait = oldWait })
	for _, scenario := range []string{"rejected by gateway", "applied before gateway failure", "concurrent edit", "permission denied", "persistent failure"} {
		t.Run(scenario, func(t *testing.T) {
			const title = "Cli/ref/Test"
			original := snapshot{Revision: 11, Text: marked("old")}
			desired := marked("new")
			current := original
			edits, reads := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				if r.Form.Get("action") == "query" {
					reads++
					replyRevisions(w, title, []snapshot{current}, "")
					return
				}
				if r.Form.Get("action") != "edit" || r.Form.Get("baserevid") != "11" || r.Form.Get("nocreate") != "1" {
					t.Error("edit lost its revision guard")
				}
				edits++
				if scenario == "permission denied" {
					_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "permissiondenied", "info": "Not allowed"}})
					return
				}
				if edits == 1 || scenario == "persistent failure" {
					if scenario == "applied before gateway failure" {
						current = snapshot{Revision: 12, Text: desired}
					} else if scenario == "concurrent edit" {
						current = snapshot{Revision: 12, Text: "community correction"}
					}
					w.Header().Set("Content-Type", "text/html")
					w.WriteHeader(http.StatusBadGateway)
					io.WriteString(w, "<html>private diagnostic</html>")
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"edit": map[string]any{"result": "Success", "newrevid": 12}})
			}))
			defer server.Close()
			client, err := newWiki(configuration{API: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			client.Tokens["csrf"] = "fixture"
			err = publishDocumentationEdit(client, title, desired, "summary", "2026-10-30T00:00:00Z", original)
			wantEdits := map[string]int{"rejected by gateway": 2, "applied before gateway failure": 1, "concurrent edit": 1, "permission denied": 1, "persistent failure": 3}[scenario]
			wantError := scenario == "concurrent edit" || scenario == "permission denied" || scenario == "persistent failure"
			if edits != wantEdits || (err != nil) != wantError {
				t.Fatalf("edits=%d, reads=%d, err=%v", edits, reads, err)
			}
			if scenario == "permission denied" && reads != 0 {
				t.Fatal("definitive API denial should not retry")
			}
			if err != nil && strings.Contains(err.Error(), "private diagnostic") {
				t.Fatal("gateway response body exposed")
			}
		})
	}
}
