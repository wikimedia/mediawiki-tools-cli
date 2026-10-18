package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRESTRequestMethodsFieldsBodiesAndRendering(t *testing.T) {
	for _, tc := range []struct {
		name, method, response, want string
		args                         []string
		stdin                        string
		check                        func(*testing.T, *http.Request)
	}{
		{name: "query fields", method: "GET", response: `{"ok":true}`, want: "{\"ok\":true}\n", args: []string{"items?existing=1", "-X", "GET", "-f", "q=a&b"}, check: func(t *testing.T, r *http.Request) {
			if r.URL.Query().Get("q") != "a&b" || r.URL.Query().Get("existing") != "1" {
				t.Errorf("incorrect query: %s", r.URL.RawQuery)
			}
		}},
		{name: "implicit POST", method: "POST", response: `{"ok":true}`, want: "{\"ok\":true}\n", args: []string{"items", "-f", "title=quoted \"value\""}, check: func(t *testing.T, r *http.Request) {
			var data map[string]string
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil || data["title"] != "quoted \"value\"" || r.Header.Get("Content-Type") != "application/json" {
				t.Error("incorrect JSON body")
			}
		}},
		{name: "stdin body", method: "PUT", response: `{"ok":true}`, want: "{\"ok\":true}\n", stdin: `{"enabled":true}`, args: []string{"items", "-X", "PUT", "--input", "-"}, check: func(t *testing.T, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if string(body) != `{"enabled":true}` {
				t.Error("stdin body changed")
			}
		}},
		{name: "raw", method: "GET", response: "{\n  \"ok\": true\n}\n", want: "{\n  \"ok\": true\n}\n", args: []string{"items", "--raw"}},
		{name: "jq", method: "GET", response: `{"title":"Earth"}`, want: "Earth\n", args: []string{"items", "--output", "jq", "--format", ".title"}},
		{name: "non JSON", method: "GET", response: "plain response\n", want: "plain response\n", args: []string{"items"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Method != tc.method || r.URL.Path != "/api/items" || r.Header.Get("User-Agent") == "" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if tc.check != nil {
					tc.check(t, r)
				}
				_, _ = io.WriteString(w, tc.response)
			}))
			defer server.Close()
			cmd := NewCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetIn(strings.NewReader(tc.stdin))
			cmd.SetArgs(append(append([]string{}, tc.args...), "--base-url", server.URL+"/api/"))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !called || out.String() != tc.want {
				t.Fatalf("unexpected output: %q", out.String())
			}
		})
	}
}

func TestAbsoluteURLHeadersEmptyResponsesAndHTTPFailures(t *testing.T) {
	status := http.StatusNoContent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" || r.Method != "DELETE" {
			t.Error("method or header not preserved")
		}
		w.Header().Set("X-Result", "test")
		w.WriteHeader(status)
		if status >= 400 {
			_, _ = io.WriteString(w, `{"error":"bad request"}`)
		}
	}))
	defer server.Close()
	for _, code := range []int{http.StatusNoContent, http.StatusBadRequest} {
		status = code
		cmd := NewCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{server.URL + "/resource", "-X", "DELETE", "-H", "Authorization: Bearer test", "--include"})
		err := cmd.Execute()
		if code == http.StatusNoContent && err != nil {
			t.Fatal(err)
		}
		if code == http.StatusBadRequest && (err == nil || !strings.Contains(err.Error(), "bad request")) {
			t.Fatalf("HTTP failure lost: %v", err)
		}
		if !strings.Contains(out.String(), "X-Result: test") {
			t.Fatal("missing response headers")
		}
	}
}

func TestInvalidRESTOptionsDoNotSendRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("invalid options made a request") }))
	defer server.Close()
	for _, args := range [][]string{
		{"items", "-f", "invalid"},
		{"items", "-H", "invalid"},
		{"items", "-f", "x=1", "--input", "-"},
		{"items", "--output", "table"},
		{"items", "--output", "jq"},
		{"items", "--raw", "--format", "."},
		{"items", "-f", "x=1", "-f", "x=2"},
	} {
		cmd := NewCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(append(args, "--base-url", server.URL))
		if err := cmd.Execute(); err == nil {
			t.Errorf("invalid options accepted: %v", args)
		}
	}
}
