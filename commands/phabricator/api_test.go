package phabricator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAPICommand(t *testing.T) {
	isolatedAuthConfig(t)
	mockLoginIdentity(t)
	login := NewPhabricatorCmd()
	login.SetOut(io.Discard)
	login.SetIn(strings.NewReader("fixture-secret\n"))
	login.SetArgs([]string{"auth", "login", "--profile", "api-test", "--method", "token", "--token-stdin", "--provider", "keyring"})
	if err := login.Execute(); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(t.TempDir(), "params.json")
	if err := os.WriteFile(inputPath, []byte(`{"limit":1,"constraints":{"query":"example"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name         string
		args         []string
		input        string
		params       map[string]interface{}
		response     string
		status       int
		wantOut      string
		wantErr      string
		transportErr bool
	}{
		{name: "ping", args: []string{"conduit.ping"}, response: `{"result":"pong"}`, wantOut: "\"pong\"\n"},
		{name: "string fields", args: []string{"user.query", "-f", "limit=1", "-f", "offset=0"}, params: map[string]interface{}{"limit": "1", "offset": "0"}, response: `{"result":[]}`, wantOut: "[]\n"},
		{name: "equals and commas in strings", args: []string{"conduit.ping", "-f", "value=a=b,c", "-f", "empty="}, params: map[string]interface{}{"value": "a=b,c", "empty": ""}, response: `{"result":null}`, wantOut: "null\n"},
		{name: "typed JSON stdin", args: []string{"project.search", "--input", "-"}, input: `{"limit":1,"constraints":{"ids":[1]},"attachments":{"members":true},"after":null,"large":9007199254740993}`, params: map[string]interface{}{"limit": json.Number("1"), "constraints": map[string]interface{}{"ids": []interface{}{json.Number("1")}}, "attachments": map[string]interface{}{"members": true}, "after": nil, "large": json.Number("9007199254740993")}, response: `{"result":{"large":9007199254740993}}`, wantOut: "{\"large\":9007199254740993}\n"},
		{name: "JSON file", args: []string{"project.search", "--input", inputPath}, params: map[string]interface{}{"limit": json.Number("1"), "constraints": map[string]interface{}{"query": "example"}}, response: `{"result":{"data":[]}}`, wantOut: "{\"data\":[]}\n"},
		{name: "jq", args: []string{"conduit.ping", "--output", "jq", "--format", "."}, response: `{"result":"pong"}`, wantOut: "pong\n"},
		{name: "JSON format", args: []string{"conduit.ping", "--format", "."}, response: `{"result":"pong"}`, wantOut: "\"pong\"\n"},
		{name: "Conduit failure", args: []string{"conduit.ping"}, response: `{"result":null,"error_code":"ERR-EXAMPLE","error_info":"Example failure"}`, wantErr: "ERR-EXAMPLE (info: Example failure)"},
		{name: "HTTP failure", args: []string{"conduit.ping"}, status: http.StatusForbidden, wantErr: "Phabricator returned HTTP 403"},
		{name: "invalid response", args: []string{"conduit.ping"}, response: `not JSON`, wantErr: "parsing response from conduit.ping"},
		{name: "transport failure", args: []string{"conduit.ping"}, transportErr: true, wantErr: "example transport failure"},
		{name: "jq runtime failure", args: []string{"conduit.ping", "--format", `., error("example")`}, response: `{"result":"pong"}`, wantErr: "jq:"},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			ctx := context.WithValue(context.Background(), apiContextKey{}, "example")
			http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.Method != http.MethodPost || r.URL.Host != "phabricator.wikimedia.org" || r.URL.Path != "/api/"+test.args[0] {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
				}
				if r.Context().Value(apiContextKey{}) != "example" {
					t.Fatal("command context was not forwarded")
				}
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				var params map[string]interface{}
				decoder := json.NewDecoder(strings.NewReader(r.Form.Get("params")))
				decoder.UseNumber()
				if err := decoder.Decode(&params); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(params["__conduit__"], map[string]interface{}{"token": "fixture-secret"}) {
					t.Fatal("saved profile token was not used")
				}
				delete(params, "__conduit__")
				want := test.params
				if want == nil {
					want = map[string]interface{}{}
				}
				if !reflect.DeepEqual(params, want) {
					t.Fatalf("params = %#v, want %#v", params, want)
				}
				if test.transportErr {
					return nil, fmt.Errorf("example transport failure")
				}
				status := test.status
				if status == 0 {
					status = http.StatusOK
				}
				return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(test.response))}, nil
			})
			var stdout, stderr bytes.Buffer
			cmd := NewPhabricatorCmd()
			cmd.SilenceUsage = true
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetIn(strings.NewReader(test.input))
			cmd.SetArgs(append([]string{"api", "--profile", "api-test"}, test.args...))
			err := cmd.ExecuteContext(ctx)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if requests != 1 || stdout.String() != test.wantOut {
				t.Fatalf("requests = %d, stdout = %q, want %q", requests, stdout.String(), test.wantOut)
			}
			if strings.Contains(stdout.String()+stderr.String(), "fixture-secret") {
				t.Fatal("token leaked to output")
			}
		})
	}
}

type apiContextKey struct{}

func TestAPIValidationBeforeCredentials(t *testing.T) {
	for _, test := range []struct {
		name, input, wantErr string
		args                 []string
	}{
		{name: "missing method", wantErr: "accepts 1 arg"},
		{name: "extra method", args: []string{"conduit.ping", "extra"}, wantErr: "accepts 1 arg"},
		{name: "URL", args: []string{"https://example.org/api/conduit.ping"}, wantErr: "invalid Conduit method"},
		{name: "path", args: []string{"../conduit.ping"}, wantErr: "invalid Conduit method"},
		{name: "query", args: []string{"conduit.ping?x=y"}, wantErr: "invalid Conduit method"},
		{name: "field missing equals", args: []string{"conduit.ping", "-f", "limit"}, wantErr: "fields must use name=value"},
		{name: "empty field name", args: []string{"conduit.ping", "-f", "=value"}, wantErr: "fields must use name=value"},
		{name: "duplicate field", args: []string{"conduit.ping", "-f", "limit=1", "-f", "limit=2"}, wantErr: "duplicate field"},
		{name: "field auth", args: []string{"conduit.ping", "-f", "__conduit__=other"}, wantErr: "__conduit__ is reserved"},
		{name: "JSON auth", args: []string{"conduit.ping", "--input", "-"}, input: `{"__conduit__":{}}`, wantErr: "__conduit__ is reserved"},
		{name: "conflicting input", args: []string{"conduit.ping", "--input", "-", "-f", "limit=1"}, wantErr: "none of the others can be"},
		{name: "malformed JSON", args: []string{"conduit.ping", "--input", "-"}, input: `{`, wantErr: "parse Conduit input JSON object"},
		{name: "array input", args: []string{"conduit.ping", "--input", "-"}, input: `[]`, wantErr: "parse Conduit input JSON object"},
		{name: "null input", args: []string{"conduit.ping", "--input", "-"}, input: `null`, wantErr: "must be a JSON object"},
		{name: "multiple objects", args: []string{"conduit.ping", "--input", "-"}, input: `{} {}`, wantErr: "parse Conduit input JSON object"},
		{name: "missing file", args: []string{"conduit.ping", "--input", filepath.Join(t.TempDir(), "missing")}, wantErr: "read Conduit input"},
		{name: "table output", args: []string{"conduit.ping", "--output", "table"}, wantErr: "--output must be json or jq"},
		{name: "jq without format", args: []string{"conduit.ping", "--output", "jq"}, wantErr: "--output jq requires --format"},
		{name: "invalid jq", args: []string{"conduit.ping", "--format", "["}, wantErr: "jq:"},
		{name: "undefined jq function", args: []string{"conduit.ping", "--format", "nosuchfunction"}, wantErr: "jq:"},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile := "must-not-load-credentials"
			cmd := newPhabricatorAPICmd(&profile)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetIn(strings.NewReader(test.input))
			cmd.SetArgs(test.args)
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}
