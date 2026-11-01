package phabricator

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func pasteResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func pasteParams(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(r.Form.Get("params")), &params); err != nil {
		t.Fatal(err)
	}
	return params
}

func TestPasteSearchRequestsContentAndExactID(t *testing.T) {
	client := newConduitClient(&PhabConfig{URL: "https://phabricator.example", Key: "fixture"})
	client.httpClient.Transport = authTransport(func(r *http.Request) (*http.Response, error) {
		params := pasteParams(t, r)
		if r.URL.Path != "/api/paste.search" || !reflect.DeepEqual(params["constraints"], map[string]interface{}{"ids": []interface{}{float64(123)}}) || !reflect.DeepEqual(params["attachments"], map[string]interface{}{"content": true}) {
			t.Fatalf("unexpected query: %s %+v", r.URL, params)
		}
		return pasteResponse(`{"result":{"data":[{"id":123,"phid":"PHID-PSTE-test","fields":{"title":"Fixture","language":"text"},"attachments":{"content":{"content":"raw\ntext"}}}]}}`), nil
	})
	paste, err := client.getPaste("P123")
	if err != nil || paste.Fields.Title != "Fixture" || paste.Attachments.Content.Content != "raw\ntext" {
		t.Fatalf("paste=%+v err=%v", paste, err)
	}
}

func TestPasteSearchErrors(t *testing.T) {
	for _, body := range []string{
		`{"error_code":"ERR-PERMISSIONS","error_info":"denied"}`,
		`{"result":{"data":[]}}`,
		`{"result":{"data":[{"id":124,"phid":"PHID-PSTE-other"}]}}`,
		`{"result":{"data":[{"id":123,"phid":"PHID-PSTE-test"}]}}`,
		`{"result":{"data":[{"id":123,"phid":"PHID-PSTE-test","attachments":{"content":{"content":null}}}]}}`,
		`{"result":"not a search result"}`,
	} {
		client := newConduitClient(&PhabConfig{URL: "https://phabricator.example", Key: "fixture"})
		client.httpClient.Transport = authTransport(func(*http.Request) (*http.Response, error) { return pasteResponse(body), nil })
		if _, err := client.getPaste("P123"); err == nil {
			t.Fatalf("accepted invalid paste response: %s", body)
		}
	}
}

func TestPasteEditTransactions(t *testing.T) {
	for _, ref := range []string{"", "P123"} {
		client := newConduitClient(&PhabConfig{URL: "https://phabricator.example", Key: "fixture"})
		transactions := []map[string]interface{}{{"type": "title", "value": "Title"}, {"type": "text", "value": "bytes\n"}, {"type": "language", "value": "text"}}
		client.httpClient.Transport = authTransport(func(r *http.Request) (*http.Response, error) {
			params := pasteParams(t, r)
			if r.URL.Path != "/api/paste.edit" {
				t.Fatal(r.URL)
			}
			if ref == "" {
				if _, ok := params["objectIdentifier"]; ok {
					t.Fatal("create supplied an existing object")
				}
			} else if params["objectIdentifier"] != ref {
				t.Fatal(params)
			}
			want, _ := json.Marshal(transactions)
			got, _ := json.Marshal(params["transactions"])
			if string(got) != string(want) {
				t.Fatalf("transactions=%s", got)
			}
			return pasteResponse(`{"result":{"object":{"id":123}}}`), nil
		})
		if id, err := client.editPaste(ref, transactions); err != nil || id != 123 {
			t.Fatalf("id=%d err=%v", id, err)
		}
	}
}

func TestPasteWritePreviewAndConfirmation(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		args              []string
		editing, canAsk   bool
		wantErr           bool
	}{
		{name: "stdin", args: []string{"--title", "Title", "--dry-run"}, input: "paste\ntext", want: "text: paste\ntext"},
		{name: "edit title", editing: true, args: []string{"P123", "--title", "New", "--dry-run"}, want: "title: New"},
		{name: "language", editing: true, args: []string{"P123", "--language", "text", "--dry-run"}, want: "language: text"},
		{name: "empty", args: []string{"--title", "Title", "--dry-run"}, want: "nonempty UTF-8", wantErr: true},
		{name: "binary", args: []string{"--title", "Title", "--dry-run"}, input: "\x00", want: "nonempty UTF-8", wantErr: true},
		{name: "missing edit", editing: true, args: []string{"P123", "--dry-run"}, want: "at least one field", wantErr: true},
		{name: "status", editing: true, args: []string{"P123", "--status", "closed", "--dry-run"}, want: "active or archived", wantErr: true},
		{name: "task", editing: true, args: []string{"T123", "--title", "Title", "--dry-run"}, want: "expected a paste", wantErr: true},
		{name: "no confirmation", args: []string{"--title", "Title"}, input: "content", want: "use --yes", wantErr: true},
		{name: "cancel", args: []string{"--title", "Title"}, input: "content", canAsk: true, want: "Cancelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A nonexistent profile ensures preview/cancellation never reads credentials.
			profile := "must-not-access-credentials"
			cmd := newPhabricatorPasteWriteCmd(&profile, tc.editing, func() bool { return tc.canAsk }, func() (bool, error) { return false, nil })
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetIn(strings.NewReader(tc.input))
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if tc.wantErr != (err != nil) {
				t.Fatalf("error=%v output=%s", err, out.String())
			}
			if err != nil {
				out.WriteString(err.Error())
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("want %q in %s", tc.want, out.String())
			}
		})
	}
}

func TestPasteCommandsUseProfileAndDispatch(t *testing.T) {
	isolatedAuthConfig(t)
	mockLoginIdentity(t)
	login := NewPhabricatorCmd()
	login.SetOut(io.Discard)
	login.SetIn(strings.NewReader("fixture-secret\n"))
	login.SetArgs([]string{"auth", "login", "--profile", "pastes", "--method", "token", "--token-stdin", "--provider", "keyring"})
	if err := login.Execute(); err != nil {
		t.Fatal(err)
	}
	writes := 0
	http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
		params := pasteParams(t, r)
		if r.URL.Host != "phabricator.wikimedia.org" || !strings.Contains(r.Form.Get("params"), "fixture-secret") {
			t.Fatalf("wrong profile: %s", r.URL)
		}
		switch r.URL.Path {
		case "/api/paste.edit":
			writes++
			return pasteResponse(`{"result":{"object":{"id":123}}}`), nil
		case "/api/paste.search":
			return pasteResponse(`{"result":{"data":[{"id":123,"phid":"PHID-PSTE-test","fields":{"title":"Fixture","uri":"https://phabricator.wikimedia.org/P123"},"attachments":{"content":{"content":"content without trailing newline"}}}]}}`), nil
		default:
			t.Fatalf("unexpected endpoint: %s params=%+v", r.URL, params)
			return nil, nil
		}
	})
	for _, args := range [][]string{
		{"paste", "send", "--title", "Title", "--yes"},
		{"paste", "edit", "P123", "--title", "New", "--yes"},
		{"paste", "view", "P123", "--raw"},
		{"v", "https://phabricator.wikimedia.org/P123", "--raw"},
	} {
		cmd := NewPhabricatorCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetIn(strings.NewReader("content"))
		cmd.SetArgs(append(args, "--profile", "pastes"))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if args[len(args)-1] == "--raw" && out.String() != "content without trailing newline" {
			t.Fatal(out.String())
		}
	}
	if writes != 2 {
		t.Fatalf("writes=%d", writes)
	}
}
