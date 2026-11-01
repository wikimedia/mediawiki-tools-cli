package phabricator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestCreateTaskTransactions(t *testing.T) {
	for _, failProject := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing-project-%t", failProject), func(t *testing.T) {
			created := 0
			client := newConduitClient(&PhabConfig{URL: "https://phabricator.example", Key: "test-token"})
			client.httpClient.Transport = authTransport(func(r *http.Request) (*http.Response, error) {
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				var params map[string]interface{}
				if err := json.Unmarshal([]byte(r.Form.Get("params")), &params); err != nil {
					t.Fatal(err)
				}
				var result string
				switch r.URL.Path {
				case "/api/project.search":
					want := map[string]interface{}{"slugs": []interface{}{"a_wikimedia_cli"}}
					if !reflect.DeepEqual(params["constraints"], want) {
						t.Fatalf("project lookup was not exact: %+v", params)
					}
					result = `{"data":[{"phid":"PHID-PROJ-cli"}]}`
					if failProject {
						result = `{"data":[]}`
					}
				case "/api/maniphest.edit":
					created++
					if _, exists := params["objectIdentifier"]; exists {
						t.Fatal("creation must not edit an existing task")
					}
					want := []interface{}{
						map[string]interface{}{"type": "title", "value": "Docs: test"},
						map[string]interface{}{"type": "description", "value": "Quotation\n> original\nIssue: fix it"},
						map[string]interface{}{"type": "projects.add", "value": []interface{}{"PHID-PROJ-cli", "PHID-PROJ-other"}},
					}
					if !reflect.DeepEqual(params["transactions"], want) {
						t.Fatalf("transactions: %+v", params["transactions"])
					}
					result = `{"object":{"id":123}}`
				default:
					t.Fatalf("unexpected request: %s", r.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"result":` + result + `}`)), Header: make(http.Header)}, nil
			})
			id, err := client.createTask("Docs: test", "Quotation\n> original\nIssue: fix it", []string{"a_wikimedia_cli", "PHID-PROJ-other"})
			if failProject {
				if err == nil || created != 0 {
					t.Fatal("missing project must prevent task creation", err)
				}
			} else if err != nil || id != 123 || created != 1 {
				t.Fatalf("creation: id=%d count=%d err=%v", id, created, err)
			}
		})
	}
}

func TestCreateTaskPreviewAndConfirmation(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		input   string
		canAsk  bool
		wantErr string
		wantOut string
		prompts int
	}{
		{name: "preview stdin", args: []string{"--title", "Docs issue", "--description-file", "-", "--project", "#a_wikimedia_cli", "--dry-run"}, wantOut: "Projects: a_wikimedia_cli\n\nIssue body\n\nPreview only"},
		{name: "preview JSON", args: []string{"--input", "-", "--dry-run"}, input: `{"title":"Docs issue","description":"It’s unclear: \"quoted\"","projects":["a_wikimedia_cli"]}`, wantOut: "It’s unclear: \"quoted\""},
		{name: "invalid JSON", args: []string{"--input", "-", "--dry-run"}, input: `{"title":`, wantErr: "parse task JSON"},
		{name: "conflicting JSON and title", args: []string{"--input", "-", "--title", "Docs"}, wantErr: "none of the others can be"},
		{name: "noninteractive", args: []string{"--title", "Docs issue"}, wantErr: "use --yes"},
		{name: "cancel", args: []string{"--title", "Docs issue"}, canAsk: true, prompts: 1, wantOut: "Cancelled"},
		{name: "empty title", args: []string{"--title", " "}, wantErr: "--title must not be empty"},
		{name: "conflicting descriptions", args: []string{"--title", "Docs", "--description", "one", "--description-file", "-"}, wantErr: "none of the others can be"},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile := "must-not-load-real-credentials"
			prompts := 0
			cmd := newPhabricatorCreateCmdWithInteraction(&profile, func() bool { return test.canAsk }, func() (bool, error) {
				prompts++
				return false, nil
			})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			input := test.input
			if input == "" {
				input = "Issue body"
			}
			cmd.SetIn(strings.NewReader(input))
			cmd.SetArgs(test.args)
			err := cmd.Execute()
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if prompts != test.prompts || !strings.Contains(out.String(), test.wantOut) {
				t.Fatalf("prompts=%d output=%s", prompts, out.String())
			}
		})
	}
}

func TestCreateCommandUsesSavedProfile(t *testing.T) {
	isolatedAuthConfig(t)
	mockLoginIdentity(t)
	var out bytes.Buffer
	login := NewPhabricatorCmd()
	login.SetOut(&out)
	login.SetArgs([]string{"auth", "login", "--profile", "reporter", "--method", "token", "--token-stdin", "--provider", "keyring"})
	login.SetIn(strings.NewReader("fixture-secret\n"))
	if err := login.Execute(); err != nil {
		t.Fatal(err)
	}
	created := 0
	http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "phabricator.wikimedia.org" || r.URL.Path != "/api/maniphest.edit" {
			t.Fatalf("unexpected request: %s", r.URL)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(r.Form.Get("params"), "fixture-secret") {
			t.Fatal("saved profile credential was not used")
		}
		created++
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"result":{"object":{"id":456}}}`))}, nil
	})
	out.Reset()
	cmd := NewPhabricatorCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"create", "--profile", "reporter", "--title", "Docs issue", "--description", "Fix the example", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if created != 1 || !strings.Contains(out.String(), "Created T456: https://phabricator.wikimedia.org/T456") || strings.Contains(out.String(), "fixture-secret") {
		t.Fatalf("created=%d output=%s", created, out.String())
	}
}
