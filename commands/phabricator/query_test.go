package phabricator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func queryClient(t *testing.T, handle func(string, map[string]interface{}) string) *conduitClient {
	t.Helper()
	client := newConduitClient(&PhabConfig{URL: "https://phab.example", Key: "test-token"})
	client.httpClient.Transport = authTransport(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(r.Form.Get("params")), &params); err != nil {
			t.Fatal(err)
		}
		result := handle(r.URL.Path, params)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"result":` + result + `}`)), Header: http.Header{}}, nil
	})
	return client
}

func TestTaskSearchColumnAndPagination(t *testing.T) {
	calls := 0
	client := queryClient(t, func(method string, params map[string]interface{}) string {
		switch method {
		case "/api/project.search":
			constraints := params["constraints"].(map[string]interface{})
			if constraints["slugs"] != nil && !reflect.DeepEqual(constraints["slugs"], []interface{}{"a_wikimedia_cli"}) {
				t.Fatal(constraints)
			}
			return `{"data":[{"id":5331,"phid":"PHID-PROJ-cli","fields":{"name":"a Wikimedia CLI"}}]}`
		case "/api/project.column.search":
			return `{"data":[{"id":1,"phid":"PHID-PCOL-backlog","fields":{"name":"Backlog ✨"}}],"cursor":{"after":null}}`
		case "/api/maniphest.search":
			calls++
			constraints := params["constraints"].(map[string]interface{})
			if !reflect.DeepEqual(constraints["columnPHIDs"], []interface{}{"PHID-PCOL-backlog"}) {
				t.Fatal(constraints)
			}
			if !reflect.DeepEqual(constraints["statuses"], []interface{}{"open()"}) {
				t.Fatal(constraints)
			}
			if constraints["query"] != "documentation" {
				t.Fatal(constraints)
			}
			if calls == 1 {
				return `{"data":[{"id":1}],"cursor":{"after":"opaque-next","before":null,"limit":50}}`
			}
			if params["after"] != "opaque-next" {
				t.Fatal(params)
			}
			return `{"data":[{"id":2}],"cursor":{"after":null,"before":"opaque-prev","limit":50}}`
		default:
			t.Fatalf("unexpected method %s", method)
			return ""
		}
	})
	page, err := client.searchTasks(context.Background(), searchOptions{Project: "a_wikimedia_cli", Column: "Backlog ✨", Search: "documentation", Statuses: []string{"open()"}, Limit: 50, All: true})
	if err != nil || len(page.Data) != 2 || calls != 2 {
		t.Fatalf("page=%+v calls=%d err=%v", page, calls, err)
	}
}

func TestTaskSearchGuardsAndCursors(t *testing.T) {
	for _, opts := range []searchOptions{{Limit: 0}, {Limit: 101}, {Limit: 50, After: "x", Before: "y"}, {Limit: 50, Column: "Backlog"}, {Limit: 50, All: true, After: "x"}} {
		client := queryClient(t, func(string, map[string]interface{}) string { t.Fatal("invalid options contacted API"); return "" })
		if _, err := client.searchTasks(context.Background(), opts); err == nil {
			t.Fatalf("accepted %+v", opts)
		}
	}
	calls := 0
	client := queryClient(t, func(_ string, params map[string]interface{}) string {
		calls++
		if calls > 3 {
			t.Fatal("cursor loop")
		}
		return `{"data":[{"id":1}],"cursor":{"after":"same"}}`
	})
	if _, err := client.searchTasks(context.Background(), searchOptions{Limit: 50, All: true}); err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Fatal(err)
	}
	client = queryClient(t, func(_ string, params map[string]interface{}) string {
		if params["before"] != "previous" {
			t.Fatal(params)
		}
		return `{"data":[],"cursor":{"after":"next","before":null}}`
	})
	page, err := client.searchTasks(context.Background(), searchOptions{Limit: 20, Before: "previous"})
	if err != nil || page.Cursor.After == nil || *page.Cursor.After != "next" {
		t.Fatalf("%+v %v", page, err)
	}
}

func TestProjectResolutionRejectsFuzzyMatch(t *testing.T) {
	client := queryClient(t, func(_ string, params map[string]interface{}) string {
		constraints := params["constraints"].(map[string]interface{})
		if constraints["slugs"] != nil {
			return `{"data":[]}`
		}
		return `{"data":[{"phid":"PHID-PROJ-wrong","fields":{"name":"CLI Other"}}]}`
	})
	if _, err := client.exactProject(context.Background(), "CLI"); err == nil {
		t.Fatal("accepted first fuzzy result")
	}
	client = queryClient(t, func(_ string, params map[string]interface{}) string {
		return `{"data":[{"id":5331,"phid":"PHID-PROJ-cli","fields":{"name":"CLI"}}]}`
	})
	project, err := client.exactProject(context.Background(), "#cli")
	if err != nil || project.ID != 5331 {
		t.Fatalf("%+v %v", project, err)
	}
}

func TestColumnsPreserveNamesAndFetchAllPages(t *testing.T) {
	calls := 0
	client := queryClient(t, func(method string, params map[string]interface{}) string {
		if method == "/api/project.search" {
			return `{"data":[{"phid":"PHID-PROJ-cli"}]}`
		}
		calls++
		if calls == 1 {
			return `{"data":[{"id":1,"phid":"col1","fields":{"name":"To do!"}}],"cursor":{"after":"next"}}`
		}
		if params["after"] != "next" {
			t.Fatal(params)
		}
		return `{"data":[{"id":2,"phid":"col2","fields":{"name":"To do?"}}],"cursor":{"after":null}}`
	})
	columns, err := client.listColumns(context.Background(), "cli")
	if err != nil || len(columns.Data) != 2 || columns.Data[1].Fields.Name != "To do?" {
		t.Fatalf("%+v %v", columns, err)
	}
}

func TestTransactionsFetchAllPages(t *testing.T) {
	calls := 0
	client := queryClient(t, func(method string, params map[string]interface{}) string {
		if method != "/api/transaction.search" {
			t.Fatal(method)
		}
		calls++
		if calls == 1 {
			return `{"data":[{"id":1}],"cursor":{"after":"tx-next"}}`
		}
		if params["after"] != "tx-next" {
			t.Fatal(params)
		}
		return `{"data":[{"id":2}],"cursor":{"after":null}}`
	})
	txs, err := client.getTransactions("PHID-TASK-test")
	if err != nil || len(txs) != 2 {
		t.Fatalf("%+v %v", txs, err)
	}
}

func TestTaskEditPreview(t *testing.T) {
	site := "must-not-read-credentials"
	cmd := newTaskEditCmd(&site)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"T123", "--title", "New title", "--status", "resolved", "--priority", "high", "--assigned-to", "me", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"title: New title", "status: resolved", "priority: high", "Preview only"} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("missing %s: %s", text, out.String())
		}
	}
}

func TestTaskListOutputs(t *testing.T) {
	page := taskPage{}
	if err := json.Unmarshal([]byte(`{"data":[{"id":123,"fields":{"name":"Useful task","status":{"name":"Open"},"priority":{"name":"High"}}}],"cursor":{"after":"next"}}`), &page); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"table", "json", "jq"} {
		t.Run(kind, func(t *testing.T) {
			out := phabOutput([]string{"Task", "Title"}, func(objects interface{}, table *output.Table) {
				for _, task := range objects.(taskPage).Data {
					table.AddRow(fmt.Sprintf("T%d", task.ID), task.Fields.Name)
				}
			})
			cmd := &cobra.Command{Use: "test"}
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			out.Type = kind
			if kind == "jq" {
				out.Format = ".data[].id"
			}
			out.Print(cmd, page)
			want := "Useful task"
			if kind == "jq" {
				want = "123"
			}
			if !strings.Contains(buf.String(), want) {
				t.Fatal(buf.String())
			}
			if kind == "json" && !strings.Contains(buf.String(), "next") {
				t.Fatal("cursor lost")
			}
		})
	}
}

func TestQueryCancellation(t *testing.T) {
	client := newConduitClient(&PhabConfig{URL: "https://phab.example", Key: "test"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client.httpClient.Transport = authTransport(func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })
	if _, err := client.searchTasks(ctx, searchOptions{Limit: 50}); err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatal(err)
	}
}

func TestUserResolutionChunks(t *testing.T) {
	calls := 0
	client := queryClient(t, func(method string, params map[string]interface{}) string {
		if method != "/api/user.search" {
			t.Fatal(method)
		}
		calls++
		phids := params["constraints"].(map[string]interface{})["phids"].([]interface{})
		if len(phids) > 100 {
			t.Fatal("oversized batch")
		}
		users := []map[string]interface{}{}
		for _, phid := range phids {
			users = append(users, map[string]interface{}{"phid": phid, "fields": map[string]string{"username": "Addshore"}})
		}
		data, err := json.Marshal(map[string]interface{}{"data": users})
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	})
	phids := []string{}
	for i := 0; i < 205; i++ {
		phids = append(phids, fmt.Sprintf("PHID-USER-%d", i))
	}
	users, err := client.getUsersContext(context.Background(), phids)
	if err != nil || len(users) != 205 || calls != 3 {
		t.Fatalf("users=%d calls=%d err=%v", len(users), calls, err)
	}
}

func TestSuccessfulTaskEditInvalidatesPersistedComments(t *testing.T) {
	client := queryClient(t, func(method string, _ map[string]interface{}) string {
		switch method {
		case "/api/maniphest.edit":
			return `{"object":{"id":1}}`
		case "/api/transaction.search":
			return `{"data":[{"id":2,"type":"comment"}]}`
		default:
			t.Fatalf("unexpected method %s", method)
			return ""
		}
	})
	dir := t.TempDir()
	client.cache = newPhabCache(dir)
	client.cache.transactions.set("tx:PHID-TASK-one", []phabTransaction{{ID: 1}}, ttlTransactions)
	client.cache.flush()
	if err := client.editTask("PHID-TASK-one", []map[string]interface{}{{"type": "comment", "value": "New comment"}}); err != nil {
		t.Fatal(err)
	}
	client.cache.flush()
	client.cache = newPhabCache(dir)
	txs, err := client.getTransactions("PHID-TASK-one")
	if err != nil || len(txs) != 1 || txs[0].ID != 2 {
		t.Fatalf("stale comments: %+v %v", txs, err)
	}
}

func TestTaskListDefaultOpenAndStatusOverrides(t *testing.T) {
	isolatedAuthConfig(t)
	mockLoginIdentity(t)
	login := NewPhabricatorCmd()
	login.SetOut(io.Discard)
	login.SetIn(strings.NewReader("fixture-secret\n"))
	login.SetArgs([]string{"auth", "login", "--profile", "task-list-test", "--method", "token", "--token-stdin", "--provider", "keyring"})
	if err := login.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		args  []string
		want  []interface{}
		pages int
	}{
		{name: "default", want: []interface{}{"open()"}, pages: 1},
		{name: "all pages stay open", args: []string{"--all"}, want: []interface{}{"open()"}, pages: 2},
		{name: "closed override", args: []string{"--status", "resolved"}, want: []interface{}{"resolved"}, pages: 1},
		{name: "all statuses", args: []string{"--status", "all"}, pages: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/api/maniphest.search" {
					t.Fatalf("unexpected request %s", r.URL)
				}
				params := pasteParams(t, r)
				constraints := params["constraints"].(map[string]interface{})
				got, exists := constraints["statuses"]
				if tc.want == nil {
					if exists {
						t.Fatalf("all statuses still constrained: %v", got)
					}
				} else if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("statuses=%v want=%v", got, tc.want)
				}
				calls++
				cursor := "null"
				if tc.pages == 2 && calls == 1 {
					cursor = `"next"`
				}
				return pasteResponse(`{"result":{"data":[],"cursor":{"after":` + cursor + `}}}`), nil
			})
			cmd := NewPhabricatorCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(append([]string{"task", "list", "--profile", "task-list-test", "--output", "json"}, tc.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if calls != tc.pages {
				t.Fatalf("requests=%d want=%d", calls, tc.pages)
			}
		})
	}
}
