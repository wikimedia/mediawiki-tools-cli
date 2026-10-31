package wikiauth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"cgt.name/pkg/go-mwclient"
)

func TestLoginPasswordWithWarnings(t *testing.T) {
	for _, test := range []struct {
		name, tokenReply, loginReply string
		wantError, wantLogin         bool
	}{
		{"success", `"query":{"tokens":{"logintoken":"fixture-token"}}`, `"login":{"result":"Success"}`, false, true},
		{"wrong password", `"query":{"tokens":{"logintoken":"fixture-token"}}`, `"login":{"result":"Failed","reason":"Incorrect password"}`, true, true},
		{"missing token", `"query":{}`, `"login":{"result":"Success"}`, true, false},
		{"empty token", `"query":{"tokens":{"logintoken":""}}`, `"login":{"result":"Success"}`, true, false},
		{"missing result", `"query":{"tokens":{"logintoken":"fixture-token"}}`, `"login":{}`, true, true},
		{"API error", `"error":{"code":"permissiondenied","info":"Denied"}`, `"login":{"result":"Success"}`, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			loggedIn := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				reply := test.tokenReply
				if r.Form.Get("action") == "login" {
					loggedIn = true
					if r.Method != http.MethodPost || r.Form.Get("lgtoken") != "fixture-token" || r.Form.Get("lgname") != "Admin" || r.Form.Get("lgpassword") != "fixture-password" {
						t.Error("incorrect login request")
					}
					reply = test.loginReply
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{`+reply+`,"warnings":{"main":{"warnings":"API announcement"},"login":{"warnings":"Main-account login deprecated"}}}`)
			}))
			defer server.Close()
			client, err := mwclient.New(server.URL, "test")
			if err != nil {
				t.Fatal(err)
			}
			err = LoginPassword(client, "Admin", "fixture-password")
			if (err != nil) != test.wantError || loggedIn != test.wantLogin {
				t.Fatalf("login error = %v; attempted login = %v", err, loggedIn)
			}
		})
	}
}

func TestLoginPasswordPreservesSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		switch r.Form.Get("action") {
		case "login":
			cookie, err := r.Cookie("login-session")
			if err != nil || cookie.Value != "fixture-session" {
				t.Error("login did not retain token request cookie")
			}
			_, _ = io.WriteString(w, `{"login":{"result":"Success"}}`)
		default:
			http.SetCookie(w, &http.Cookie{Name: "login-session", Value: "fixture-session"})
			_ = json.NewEncoder(w).Encode(map[string]any{"query": map[string]any{"tokens": map[string]string{"logintoken": "fixture-token"}}})
		}
	}))
	defer server.Close()
	client, err := mwclient.New(server.URL, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := LoginPassword(client, "Admin", "fixture-password"); err != nil {
		t.Fatal(err)
	}
}
