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
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func accessTask(id int, view, space string) phabTask {
	task := phabTask{ID: id, PHID: fmt.Sprintf("PHID-TASK-%d", id)}
	task.Fields.Name = fmt.Sprintf("fixture-title-%d", id)
	task.Fields.Description.Raw = "fixture-description"
	task.Fields.AuthorPHID = "fixture-author"
	task.Fields.OwnerPHID = "fixture-owner"
	task.Fields.DateCreated = 123
	task.Fields.DateModified = 456
	task.Fields.Status.Name = "fixture-status"
	task.Fields.Priority.Name = "fixture-priority"
	task.Fields.SpacePHID = space
	task.Fields.Policy = map[string]string{"view": view}
	task.Attachments.Projects.ProjectPHIDs = []string{"fixture-project"}
	task.Attachments.Columns = json.RawMessage(`{"fixture-column":true}`)
	return task
}

func accessPage(t *testing.T, tasks ...phabTask) string {
	t.Helper()
	data, err := json.Marshal(taskPage{Data: tasks})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func publicTaskAccess() taskAccess {
	return taskAccess{publicOnly: true, endpoint: defaultPhabricatorURL}
}

func TestTaskAccessClassificationAndRedaction(t *testing.T) {
	public := accessTask(1, "public", wikimediaPublicSpace)
	for _, test := range []struct {
		name     string
		view     string
		space    string
		endpoint string
		allowed  bool
	}{
		{name: "public", view: "public", space: wikimediaPublicSpace, endpoint: defaultPhabricatorURL, allowed: true},
		{name: "users", view: "users", space: wikimediaPublicSpace, endpoint: defaultPhabricatorURL},
		{name: "custom", view: "PHID-PLCY-fixture", space: wikimediaPublicSpace, endpoint: defaultPhabricatorURL},
		{name: "missing policy", space: wikimediaPublicSpace, endpoint: defaultPhabricatorURL},
		{name: "missing Space", view: "public", endpoint: defaultPhabricatorURL},
		{name: "other Space", view: "public", space: "PHID-SPCE-other", endpoint: defaultPhabricatorURL},
		{name: "other endpoint", view: "public", space: wikimediaPublicSpace, endpoint: "https://example.test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &conduitClient{access: publicTaskAccess()}
			client.access.endpoint = test.endpoint
			task := accessTask(2, test.view, test.space)
			if got := client.access.permits(task); got != test.allowed {
				t.Fatalf("permits = %t, want %t", got, test.allowed)
			}
			masked := client.taskForList(task)
			if test.allowed {
				if !reflect.DeepEqual(masked, task) {
					t.Fatal("public task changed")
				}
				return
			}
			if masked.ID != task.ID || masked.PHID != "" || masked.Fields.Description.Raw != "" || !masked.Redacted || taskReference(masked) != "T2" {
				t.Fatalf("task was not erased: %+v", masked)
			}
			data, err := json.Marshal(masked)
			var got map[string]interface{}
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			want := map[string]interface{}{
				"id": float64(2), "redacted": true,
				"fields": map[string]interface{}{
					"name":        nonPublicLabel,
					"status":      map[string]interface{}{"value": "", "name": "fixture-status"},
					"priority":    map[string]interface{}{"value": float64(0), "name": "fixture-priority", "color": ""},
					"dateCreated": float64(123), "dateModified": float64(456),
				},
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("JSON = %s, error = %v", data, err)
			}
			if masked.Fields.AuthorPHID != "" || masked.Fields.OwnerPHID != nil || masked.Fields.SpacePHID != "" || masked.Fields.Policy != nil || len(masked.Attachments.Projects.ProjectPHIDs) != 0 || masked.Attachments.Columns != nil {
				t.Fatal("non-allowlisted metadata survived redaction")
			}
			client.access.agent = true
			if agent := client.taskForList(task); !reflect.DeepEqual(agent, masked) {
				t.Fatal("agent mode must retain the same allowlisted metadata")
			}
			client.access.agent = false
			if err := client.requirePublicTask(task); err == nil || !strings.Contains(err.Error(), "--allow-non-public") {
				t.Fatalf("direct access = %v", err)
			}
		})
	}
	client := &conduitClient{}
	if !reflect.DeepEqual(client.taskForList(public), public) || !client.access.permits(accessTask(2, "users", "")) {
		t.Fatal("unrestricted behavior changed")
	}
}

func TestTaskAccessFlagsConfigAndDetection(t *testing.T) {
	isolatedAuthConfig(t)
	for _, test := range []struct {
		name       string
		config     interface{}
		flags      map[string]string
		env        string
		envValue   string
		publicOnly bool
		agent      bool
		wantErr    string
		api        bool
	}{
		{name: "unchanged"},
		{name: "boolean default", config: true, publicOnly: true},
		{name: "string default", config: "true", publicOnly: true},
		{name: "disabled default", config: "false"},
		{name: "invalid default", config: "oops", wantErr: "must be true or false"},
		{name: "explicit public", flags: map[string]string{"public-only": "true"}, publicOnly: true},
		{name: "human override", config: true, flags: map[string]string{"allow-non-public": "true"}},
		{name: "human false flag", config: true, flags: map[string]string{"public-only": "false"}},
		{name: "explicit agent", flags: map[string]string{"agent": "true"}, publicOnly: true, agent: true},
		{name: "agent ignores public false", flags: map[string]string{"agent": "true", "public-only": "false"}, publicOnly: true, agent: true},
		{name: "agent refuses override", flags: map[string]string{"agent": "true", "allow-non-public": "true"}, wantErr: "human approval required"},
		{name: "contradictory flags", flags: map[string]string{"public-only": "true", "allow-non-public": "true"}, wantErr: "cannot be combined"},
		{name: "detected agent cannot be disabled", env: "COPILOT_AGENT_SESSION_ID", envValue: "fixture", flags: map[string]string{"agent": "false", "public-only": "false"}, publicOnly: true, agent: true},
		{name: "detected agent refuses override", env: "CLAUDECODE", envValue: "1", flags: map[string]string{"allow-non-public": "true"}, wantErr: "human approval required"},
		{name: "codex detection", env: "CODEX_THREAD_ID", envValue: "fixture", publicOnly: true, agent: true},
		{name: "gemini detection", env: "GEMINI_CLI", envValue: "true", publicOnly: true, agent: true},
		{name: "false indicator is nonempty upstream", env: "CLAUDECODE", envValue: "false", publicOnly: true, agent: true},
		{name: "zero indicator is nonempty upstream", env: "GEMINI_CLI", envValue: "0", publicOnly: true, agent: true},
		{name: "custom agent", env: "AI_AGENT", envValue: "custom-agent", publicOnly: true, agent: true},
		{name: "cursor cannot disable detection", env: "CURSOR_AGENT", envValue: "1", flags: map[string]string{"agent": "false", "public-only": "false"}, publicOnly: true, agent: true},
		{name: "upstream copilot refuses override", env: "COPILOT_MODEL", envValue: "fixture", flags: map[string]string{"allow-non-public": "true"}, wantErr: "human approval required"},
		{name: "raw API exempt", api: true, config: true, env: "CLAUDECODE", envValue: "1", flags: map[string]string{"agent": "true", "allow-non-public": "true"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := mwconfig.State().EffectiveKoanf
			config.Delete("phabricator.public_only")
			t.Cleanup(func() { config.Delete("phabricator.public_only") })
			if test.config != nil {
				if err := config.Set("phabricator.public_only", test.config); err != nil {
					t.Fatal(err)
				}
			}
			if test.env != "" {
				t.Setenv(test.env, test.envValue)
			}
			root := NewPhabricatorCmd()
			for name, value := range test.flags {
				if err := root.PersistentFlags().Set(name, value); err != nil {
					t.Fatal(err)
				}
			}
			path := []string{"task", "list"}
			if test.api {
				path = []string{"api"}
			}
			cmd, _, err := root.Find(path)
			if err != nil {
				t.Fatal(err)
			}
			access, err := taskAccessFromCommand(cmd, defaultPhabricatorURL+"/")
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil || access.publicOnly != test.publicOnly || access.agent != test.agent {
				t.Fatalf("access = %+v, error = %v", access, err)
			}
		})
	}
}

func TestTaskAccessGuardsDirectOperationsAndCaches(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*conduitClient) error
	}{
		{name: "get task", run: func(c *conduitClient) error { _, err := c.getTask("T2"); return err }},
		{name: "get task context", run: func(c *conduitClient) error { _, err := c.getTaskContext(context.Background(), "T2"); return err }},
		{name: "get by PHID", run: func(c *conduitClient) error { _, err := c.getTaskByPHID("PHID-TASK-2"); return err }},
		{name: "cached PHID", run: func(c *conduitClient) error { _, err := c.lookupTaskPHID("T2"); return err }},
		{name: "cached comments", run: func(c *conduitClient) error { _, err := c.getTransactions("PHID-TASK-2"); return err }},
		{name: "transactions", run: func(c *conduitClient) error {
			_, err := c.taskTransactions(context.Background(), "PHID-TASK-2")
			return err
		}},
		{name: "edit", run: func(c *conduitClient) error { return c.editTaskContext(context.Background(), "PHID-TASK-2", nil) }},
		{name: "move", run: func(c *conduitClient) error { return c.moveTaskToColumn("PHID-TASK-2", "column") }},
		{name: "remove project", run: func(c *conduitClient) error { return c.removeProjectFromTask("PHID-TASK-2", "project") }},
		{name: "priority", run: func(c *conduitClient) error { return c.setTaskPriority("PHID-TASK-2", "high") }},
		{name: "status", run: func(c *conduitClient) error { return c.setTaskStatus("PHID-TASK-2", "resolved") }},
		{name: "assign", run: func(c *conduitClient) error { return c.assignTask("PHID-TASK-2", "user") }},
		{name: "creation", run: func(c *conduitClient) error { _, err := c.createTask("title", "body", nil); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client := queryClient(t, func(method string, _ map[string]interface{}) string {
				requests++
				if method != "/api/maniphest.search" {
					t.Fatalf("blocked operation made request: %s", method)
				}
				return accessPage(t, accessTask(2, "users", wikimediaPublicSpace))
			})
			client.access = publicTaskAccess()
			client.access.agent = true
			client.cache = newPhabCache(t.TempDir())
			client.cache.phids.set("task:T2", "PHID-TASK-2", ttlDefault)
			client.cache.transactions.set("tx:PHID-TASK-2", []phabTransaction{{ID: 2}}, ttlTransactions)
			err := test.run(client)
			if err == nil || !strings.Contains(err.Error(), "human approval required") || strings.Contains(err.Error(), "fixture-") {
				t.Fatalf("error = %v", err)
			}
			if test.name != "creation" && requests != 1 {
				t.Fatalf("expected a fresh visibility check, got %d requests", requests)
			}
			if test.name == "creation" && requests != 0 {
				t.Fatal("creation contacted server")
			}
		})
	}
}

func TestTaskAccessPublicOperationsAndFreshComments(t *testing.T) {
	for _, edit := range []bool{false, true} {
		t.Run(fmt.Sprintf("edit-%t", edit), func(t *testing.T) {
			checks, operations := 0, 0
			client := queryClient(t, func(method string, _ map[string]interface{}) string {
				if method == "/api/maniphest.search" {
					checks++
					return accessPage(t, accessTask(1, "public", wikimediaPublicSpace))
				}
				operations++
				if edit && method == "/api/maniphest.edit" {
					return `{}`
				}
				if !edit && method == "/api/transaction.search" {
					return `{"data":[{"id":123}],"cursor":{"after":null}}`
				}
				t.Fatalf("unexpected method %s", method)
				return ""
			})
			client.access = publicTaskAccess()
			client.cache = newPhabCache(t.TempDir())
			client.cache.transactions.set("tx:PHID-TASK-1", []phabTransaction{{ID: 999}}, ttlTransactions)
			if edit {
				if err := client.setTaskStatus("PHID-TASK-1", "resolved"); err != nil {
					t.Fatal(err)
				}
			} else {
				txs, err := client.getTransactions("PHID-TASK-1")
				if err != nil || len(txs) != 1 || txs[0].ID != 123 {
					t.Fatalf("transactions = %+v, error = %v", txs, err)
				}
			}
			if checks != 1 || operations != 1 {
				t.Fatalf("checks = %d, operations = %d", checks, operations)
			}
		})
	}
}

func TestTaskAccessListsAndBrowsePlaceholders(t *testing.T) {
	client := queryClient(t, func(method string, _ map[string]interface{}) string {
		if method != "/api/maniphest.search" {
			t.Fatal(method)
		}
		return accessPage(t, accessTask(1, "public", wikimediaPublicSpace), accessTask(2, "users", wikimediaPublicSpace))
	})
	client.access = publicTaskAccess()
	page, err := client.searchTasks(context.Background(), searchOptions{Limit: 50})
	if err != nil || len(page.Data) != 2 || !page.Data[1].Redacted {
		t.Fatalf("page = %+v, error = %v", page, err)
	}
	for _, kind := range []string{"json", "jq", "table"} {
		var buffer bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&buffer)
		out := phabOutput([]string{"Task", "Title", "Status", "Priority"}, func(objects interface{}, table *output.Table) {
			for _, task := range objects.(taskPage).Data {
				table.AddRow(taskReference(task), task.Fields.Name, task.Fields.Status.Name, task.Fields.Priority.Name)
			}
		})
		out.Type = kind
		out.Format = "."
		if kind == "table" {
			out.Format = ""
		}
		out.Print(cmd, page)
		if strings.Contains(buffer.String(), "fixture-title-2") || strings.Contains(buffer.String(), "PHID-TASK-2") || strings.Contains(buffer.String(), "T0") || !strings.Contains(buffer.String(), nonPublicLabel) {
			t.Fatalf("%s output leaked task: %s", kind, buffer.String())
		}
	}
	global, err := client.listTasksGlobal(50)
	if err != nil || len(global) != 2 || global[1] != (taskSummary{ID: "T2", Title: nonPublicLabel, Priority: "fixture-priority", DateModified: 456}) {
		t.Fatalf("global = %+v, error = %v", global, err)
	}
	column, err := client.listTasksInColumn("project", "column", 50)
	if err != nil || len(column) != 2 || column[1] != (taskSummary{ID: "T2", Title: nonPublicLabel, Priority: "fixture-priority", DateModified: 456}) {
		t.Fatalf("column = %+v, error = %v", column, err)
	}
	browse := browseTaskPage(page, defaultPhabricatorURL)
	if len(browse.items) != 2 || browse.items[1] != (browseItem{title: "T2  " + nonPublicLabel, subtitle: "fixture-status · fixture-priority"}) {
		t.Fatalf("browse items = %+v", browse.items)
	}
	for _, board := range []bool{false, true} {
		for _, key := range []string{"enter", "o"} {
			model := newBrowseModel(context.Background(), fakeBrowseSource(), "project")
			model.board = board
			placeholder := browsePage{items: []browseItem{browse.items[1]}}
			model.page = placeholder
			model.lanes = []browseLane{{page: placeholder}}
			model, command := browseKey(model, key)
			if command != nil || model.err == nil || model.screen != "tasks" {
				t.Fatalf("placeholder navigation: board=%t key=%s screen=%s err=%v", board, key, model.screen, model.err)
			}
		}
	}
}

func TestTaskAccessPagination(t *testing.T) {
	requests := 0
	client := queryClient(t, func(_ string, params map[string]interface{}) string {
		requests++
		if requests == 1 {
			return `{"data":[],"cursor":{"after":"fixture-after","before":"fixture-before","limit":50}}`
		}
		if params["after"] != "fixture-after" || params["before"] != nil {
			t.Fatalf("decoded pagination = %+v", params)
		}
		return `{"data":[],"cursor":{"after":null,"before":null,"limit":50}}`
	})
	client.access = publicTaskAccess()
	first, err := client.searchTasks(context.Background(), searchOptions{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	for _, cursor := range []*string{first.Cursor.After, first.Cursor.Before} {
		if cursor == nil {
			t.Fatal("pagination cursor missing")
		}
	}
	if *first.Cursor.Before != "fixture-before" {
		t.Fatalf("before cursor = %s", *first.Cursor.Before)
	}
	if _, err := client.searchTasks(context.Background(), searchOptions{Limit: 50, After: *first.Cursor.After}); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestTaskAccessAllPagesAreRedacted(t *testing.T) {
	requests := 0
	client := queryClient(t, func(_ string, params map[string]interface{}) string {
		requests++
		switch requests {
		case 1:
			return `{"data":[{"id":2,"phid":"PHID-TASK-private","fields":{"name":"private-title"}}],"cursor":{"after":"2"}}`
		case 2:
			if params["after"] != "2" {
				t.Fatal("internal paging must use the original server cursor")
			}
			return accessPage(t, accessTask(3, "public", wikimediaPublicSpace))
		default:
			t.Fatal("unexpected page")
			return ""
		}
	})
	client.access = publicTaskAccess()
	page, err := client.searchTasks(context.Background(), searchOptions{Limit: 50, All: true})
	if err != nil || len(page.Data) != 2 || !page.Data[0].Redacted || page.Data[1].ID != 3 || requests != 2 {
		t.Fatalf("page = %+v, requests = %d, error = %v", page, requests, err)
	}
	data, err := json.Marshal(page)
	if err != nil || strings.Contains(string(data), "private") {
		t.Fatalf("private page data leaked: %s, error = %v", data, err)
	}
}

func TestTaskAccessCommandWiringAndCacheIsolation(t *testing.T) {
	isolatedAuthConfig(t)
	mockLoginIdentity(t)
	login := NewPhabricatorCmd()
	login.SetOut(io.Discard)
	login.SetIn(strings.NewReader("fixture-secret\n"))
	login.SetArgs([]string{"auth", "login", "--profile", "access", "--method", "token", "--token-stdin", "--provider", "keyring"})
	if err := login.Execute(); err != nil {
		t.Fatal(err)
	}
	if err := mwconfig.PutKeyValueOnDisk("phabricator.public_only", "true"); err != nil {
		t.Fatal(err)
	}
	private := accessTask(2, "users", wikimediaPublicSpace)
	http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
		var result string
		switch r.URL.Path {
		case "/api/maniphest.search":
			result = accessPage(t, private)
		case "/api/user.search":
			result = `{"data":[{"phid":"PHID-USER-fixture","fields":{"username":"fixture"}}]}`
		case "/api/project.search":
			result = `{"data":[{"id":1,"phid":"PHID-PROJ-fixture","fields":{"name":"fixture"}}]}`
		case "/api/project.column.search":
			result = `{"data":[{"id":1,"phid":"PHID-PCOL-fixture","fields":{"name":"Backlog"}}]}`
		default:
			t.Fatalf("unexpected request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"result":` + result + `}`))}, nil
	})
	for _, args := range [][]string{
		{"view", "T2"},
		{"task", "view", "T2", "--output", "json"},
		{"read", "T2"},
		{"task", "read", "T2"},
		{"comments", "T2"},
		{"task", "comments", "T2"},
		{"task", "edit", "T2", "--title", "new", "--yes"},
		{"assign", "T2", "fixture"},
		{"task", "assign", "T2", "fixture"},
		{"mv", "T2", "Backlog", "--project", "fixture"},
		{"task", "move", "T2", "Backlog", "--project", "fixture"},
		{"rmtag", "T2", "fixture"},
		{"task", "rmtag", "T2", "fixture"},
		{"task", "priority", "T2", "high"},
		{"task", "status", "T2", "resolved"},
		{"setprio", "T2", "high"},
		{"setstatus", "T2", "resolved"},
		{"create", "--title", "new", "--yes"},
		{"task", "create", "--title", "new", "--yes"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := NewPhabricatorCmd()
			var buffer bytes.Buffer
			cmd.SetOut(&buffer)
			cmd.SetErr(io.Discard)
			cmd.SilenceUsage = true
			target, _, err := cmd.Find(args)
			if err != nil {
				t.Fatal(err)
			}
			if target.Flags().Lookup("yes") != nil {
				args = append(append([]string{}, args...), "--yes")
			}
			cmd.SetArgs(append([]string{"--profile", "access"}, args...))
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "public-only access") {
				t.Fatalf("error = %v", err)
			}
			if strings.Contains(buffer.String(), "fixture-title-2") || strings.Contains(buffer.String(), "fixture-description") {
				t.Fatal("command leaked task")
			}
		})
	}
	for _, args := range [][]string{{"task", "list", "--output", "json"}, {"task", "list", "--output", "jq", "--format", ".data[]"}} {
		cmd := NewPhabricatorCmd()
		var buffer bytes.Buffer
		cmd.SetOut(&buffer)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(append([]string{"--profile", "access"}, args...))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(buffer.String(), "fixture-title") || strings.Contains(buffer.String(), "fixture-description") || !strings.Contains(buffer.String(), `"redacted":true`) || !strings.Contains(buffer.String(), nonPublicLabel) || !strings.Contains(buffer.String(), "fixture-status") || !strings.Contains(buffer.String(), "fixture-priority") {
			t.Fatalf("list command leaked task: %s", buffer.String())
		}
	}
	for _, args := range [][]string{{"task", "list"}, {"shell"}, {"browse"}} {
		root := NewPhabricatorCmd()
		cmd, _, err := root.Find(args)
		if err != nil {
			t.Fatal(err)
		}
		client, _, err := clientFromCommand(cmd, "access")
		if err != nil || client.cache != nil || !client.access.publicOnly {
			t.Fatalf("%v: cache was enabled or policy missing: client=%+v err=%v", args, client, err)
		}
	}
	root := NewPhabricatorCmd()
	if err := root.PersistentFlags().Set("allow-non-public", "true"); err != nil {
		t.Fatal(err)
	}
	cmd, _, err := root.Find([]string{"task", "view"})
	if err != nil {
		t.Fatal(err)
	}
	client, _, err := clientFromCommand(cmd, "access")
	if err != nil || client.access.publicOnly || client.cache == nil {
		t.Fatalf("human override: client=%+v err=%v", client, err)
	}
	t.Setenv("COPILOT_AGENT_SESSION_ID", "fixture")
	if _, _, err := clientFromCommand(cmd, "access"); err == nil || !strings.Contains(err.Error(), "human approval required") {
		t.Fatalf("detected agent bypassed human override: %v", err)
	}
	api := NewPhabricatorCmd()
	var raw bytes.Buffer
	api.SetOut(&raw)
	api.SetArgs([]string{"api", "maniphest.search", "--profile", "access", "--agent", "--public-only"})
	if err := api.Execute(); err == nil || !strings.Contains(err.Error(), "human approval required") || strings.Contains(raw.String(), private.Fields.Name) {
		t.Fatalf("agent raw API was not blocked: output=%s error=%v", raw.String(), err)
	}
	t.Setenv("COPILOT_AGENT_SESSION_ID", "")
	api = NewPhabricatorCmd()
	raw.Reset()
	api.SetOut(&raw)
	api.SetArgs([]string{"api", "maniphest.search", "--profile", "access", "--public-only"})
	if err := api.Execute(); err != nil || !strings.Contains(raw.String(), private.Fields.Name) {
		t.Fatalf("raw API behavior changed: output=%s error=%v", raw.String(), err)
	}
}
