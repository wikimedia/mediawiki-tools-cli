package wiki

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPagePutReturnsAuthenticationAndPermissionErrors(t *testing.T) {
	for _, code := range []string{"mwoauth-invalid-authorization", "permissiondenied"} {
		t.Run(code, func(t *testing.T) {
			isolateWikiProfiles(t)
			var edits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Form.Get("action") == "edit" {
					edits.Add(1)
				}
				if code == "permissiondenied" && r.Form.Get("action") == "query" {
					_, _ = io.WriteString(w, `{"query":{"tokens":{"csrftoken":"csrf-fixture"}}}`)
					return
				}
				_, _ = io.WriteString(w, `{"error":{"code":"`+code+`","info":"Request rejected"}}`)
			}))
			defer server.Close()
			cmd := NewWikiCmd()
			cmd.SetIn(strings.NewReader("test page text"))
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"page", "put", "--wiki", server.URL + "/w/api.php", "--title", "Test"})
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), code) {
				t.Fatalf("expected %s returned without a panic, got %v", code, err)
			}
			if code == "mwoauth-invalid-authorization" && edits.Load() != 0 {
				t.Fatal("edit attempted after CSRF authentication failed")
			}
		})
	}
}

func TestPagePutPasswordLoginWarnings(t *testing.T) {
	for _, result := range []string{"Success", "Failed"} {
		t.Run(result, func(t *testing.T) {
			isolateWikiProfiles(t)
			var edits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				switch r.Form.Get("action") {
				case "login":
					_, _ = io.WriteString(w, `{"login":{"result":"`+result+`"},"warnings":{"login":{"warnings":"Main-account login deprecated"}}}`)
				case "edit":
					edits.Add(1)
					_, _ = io.WriteString(w, `{"edit":{"result":"Success","oldrevid":1,"newrevid":2}}`)
				default:
					if r.Form.Get("type") == "login" {
						_, _ = io.WriteString(w, `{"query":{"tokens":{"logintoken":"fixture-login"}},"warnings":{"main":{"warnings":"API announcement"}}}`)
					} else {
						_, _ = io.WriteString(w, `{"query":{"tokens":{"csrftoken":"fixture-csrf"}}}`)
					}
				}
			}))
			defer server.Close()
			cmd := NewWikiCmd()
			cmd.SetIn(strings.NewReader("Test content"))
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"page", "put", "--wiki", server.URL + "/w/api.php", "--user", "admin", "--password", "fixture-password", "--title", "Testpage1"})
			err := cmd.Execute()
			if result == "Success" {
				if err != nil || edits.Load() != 1 {
					t.Fatalf("successful login with warnings: error = %v, edits = %d", err, edits.Load())
				}
			} else if err == nil || edits.Load() != 0 {
				t.Fatalf("failed login with warnings: error = %v, edits = %d", err, edits.Load())
			}
		})
	}
}
