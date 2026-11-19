package phabricator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestAnonymousTaskSearchByWorkboardColumn(t *testing.T) {
	client := anonymousPhabricator{http: &http.Client{Transport: authTransport(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Path {
		case "/project/":
			if req.URL.Query().Get("phids") != "PHID-PROJ-team" {
				t.Errorf("unexpected project lookup %s", req.URL)
			}
			body = `<ul class="phui-oi-list-view"><li><a class="phui-oi-link" href="/project/view/12/">Team</a></li></ul>`
		case "/project/board/12/":
			body = `<div class="phabricator-nav-local"><a href="/maniphest/?project=PHID-PROJ-team">Tasks</a></div>
				<div class="phui-workpanel-view"><span class="phui-header-header">Backlog</span>
				<ul data-sigil="project-column" data-meta="0_0"></ul></div>
				<data data-javelin-init-data='{"block":0,"data":[{"columnPHID":"PHID-PCOL-backlog"}]}'></data>`
		case "/maniphest/":
			if req.URL.Query().Get("columnPHIDs") != "PHID-PCOL-backlog" {
				t.Errorf("missing column constraint in %s", req.URL)
			}
			body = `<input name="projectPHIDs[]" value="PHID-PROJ-team"><input name="statuses[]" value="open()">
				<ul class="phui-oi-list-view"><li class="phui-oi"><a class="phui-oi-link" href="/T123">Public task</a></li></ul>`
		default:
			t.Errorf("unexpected request %s", req.URL)
		}
		return anonymousResponse(req, http.StatusOK, body), nil
	})}}
	opts := defaultTaskListOptions()
	opts.Projects = []string{"PHID-PROJ-team"}
	opts.Column = "Backlog"
	page, err := client.searchTasks(context.Background(), opts)
	if err != nil || len(page.Data) != 1 || page.Data[0].ID != 123 {
		t.Fatalf("column search: %+v %v", page, err)
	}
	opts.Column = "Nonexistent"
	if _, err = client.searchTasks(context.Background(), opts); err == nil || !strings.Contains(err.Error(), "did not resolve uniquely") {
		t.Fatalf("unknown column: %v", err)
	}
	opts.Column = "PHID-PCOL-backlog"
	opts.Author = "Unknown"
	if _, err = client.searchTasks(context.Background(), opts); err == nil || !strings.Contains(err.Error(), "could not resolve username") {
		t.Fatalf("search accepted a dropped author filter: %v", err)
	}
}

func TestAnonymousPhabricatorCommands(t *testing.T) {
	isolatedAuthConfig(t)
	if err := mwconfig.PutKeyValueOnDisk("phabricator.profiles.unselected.key", "unused-token"); err != nil {
		t.Fatal(err)
	}
	const task = `
		<meta property="og:title" content="T123 Public task">
		<div class="phui-header-subheader"><span class="phui-tag-core">Open</span></div>
		<div class="phui-property-list-text-content"><div class="phabricator-remarkup">
			<p>Public description</p><p>Another paragraph</p>
		</div></div>
		<div class="phui-timeline-event-view">
			<div class="phui-timeline-title"><a class="phui-link-person">ExampleUser</a></div>
			<div class="transaction-comment"><div class="phabricator-remarkup">Public comment</div></div>
		</div>`
	const project = `
		<div class="phabricator-nav-local">
			<a href="/project/profile/12/">Example Sprint</a>
			<a href="/maniphest/?project=PHID-PROJ-sprint">Open Tasks</a>
		</div>`
	const filters = `
		<input name="assignedPHIDs[]" value="none()">
		<input name="projectPHIDs[]" value="PHID-PROJ-sprint">
		<input name="projectPHIDs[]" value="PHID-PROJ-team">
		<input name="statuses[]" value="open()">
		<input name="priorities[]" value="100"><input name="priorities[]" value="80">`
	const list = `
		<ul class="phui-oi-list-view"><li class="phui-oi">
			<a class="phui-oi-link" href="/T123">Public task</a>
			<div class="phui-oi-status-icon" data-meta="0_0"></div>
		</li></ul>
		<data data-javelin-init-data='{"block":0,"data":[{"tip":"Open, High"}]}'></data>
		<div class="phui-pager-view"><a href="/maniphest/?after=123">Next</a></div>`

	calls := 0
	previous := http.DefaultTransport
	http.DefaultTransport = authTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodGet || req.URL.Scheme != "https" || req.URL.Host != "phabricator.wikimedia.org" ||
			req.Body != nil || req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
			t.Errorf("unexpected anonymous request: %s %s", req.Method, req.URL)
		}
		switch req.URL.Path {
		case "/T123":
			return anonymousResponse(req, http.StatusOK, task), nil
		case "/T999":
			return anonymousResponse(req, http.StatusOK, "<title>Login</title>"), nil
		case "/T456":
			resp := anonymousResponse(req, http.StatusFound, "")
			resp.Header.Set("Location", "https://other.example/T456")
			return resp, nil
		case "/project/":
			if got := req.URL.Query().Get("name"); got != "Example Sprint" {
				t.Errorf("unexpected project name query %q", got)
			}
			return anonymousResponse(req, http.StatusOK, `<ul class="phui-oi-list-view">
				<li><a class="phui-oi-link" href="/project/view/12/">Example Sprint</a></li>
			</ul>`), nil
		case "/project/profile/12/":
			return anonymousResponse(req, http.StatusOK, project), nil
		case "/maniphest/":
			query := req.URL.Query()
			for key, expected := range map[string]string{"projects": "PHID-PROJ-sprint,PHID-PROJ-team", "priorities": "100,80", "assigned": "none()", "statuses": "open()", "limit": "2"} {
				if got := query.Get(key); got != expected {
					t.Errorf("search parameter %s = %q, want %q", key, got, expected)
				}
			}
			if query.Get("after") == "123" {
				return anonymousResponse(req, http.StatusOK, filters+`<div class="phui-info-view-body">No tasks found.</div>`), nil
			}
			return anonymousResponse(req, http.StatusOK, filters+list), nil
		default:
			return nil, fmt.Errorf("unexpected request %s", req.URL.Path)
		}
	})
	t.Cleanup(func() { http.DefaultTransport = previous })

	for _, test := range []struct {
		args []string
		want []string
	}{
		{[]string{"task", "view", defaultPhabricatorURL + "/T123", "--output", "json"}, []string{"Public task", "Public description"}},
		{[]string{"task", "read", "T123"}, []string{"Public task", "Public description\nAnother paragraph", "Public comment"}},
		{[]string{"comments", "T123"}, []string{"ExampleUser", "Public comment"}},
		{[]string{"project", "view", "Example Sprint", "--output", "json"}, []string{"Example Sprint", "PHID-PROJ-sprint", "/project/profile/12/"}},
		{[]string{"task", "list", "--project", "12,PHID-PROJ-team", "--priority", "100,80", "--unassigned", "--limit", "2", "--output", "json"}, []string{"Public task", "High", "\"after\":\"123\""}},
		{[]string{"task", "list", "--project", "12,PHID-PROJ-team", "--priority", "100,80", "--unassigned", "--limit", "2", "--all", "--output", "json"}, []string{"Public task", "\"after\":null"}},
	} {
		got, err := executeSafetyCommand(test.args...)
		if err != nil {
			t.Fatalf("%v: %v", test.args, err)
		}
		for _, want := range test.want {
			if !strings.Contains(got, want) {
				t.Errorf("%v: output %q does not contain %q", test.args, got, want)
			}
		}
	}
	for ref, want := range map[string]string{"T999": "not publicly visible", "T456": "refusing Phabricator redirect"} {
		got, err := executeSafetyCommand("task", "view", ref)
		if err == nil || !strings.Contains(err.Error(), want) || got != "" {
			t.Errorf("%s: output %q, error %v", ref, got, err)
		}
	}

	before := calls
	for _, args := range [][]string{
		{"task", "view", "T123", "--profile", "missing"},
		{"task", "view", "https://other.example/T123"},
		{"task", "view", "T123", "--allow-non-public"},
		{"task", "status", "T123", "resolved"},
		{"paste", "view", "P123"},
		{"api", "conduit.ping"},
	} {
		if _, err := executeSafetyCommand(args...); err == nil {
			t.Errorf("%v unexpectedly succeeded without a profile", args)
		}
	}
	if err := mwconfig.PutKeyValueOnDisk("phabricator.default_profile", "unselected"); err != nil {
		t.Fatal(err)
	}
	if err := mwconfig.PutKeyValueOnDisk("phabricator.profiles.unselected.logged_out", "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeSafetyCommand("task", "view", "T123"); err == nil || !strings.Contains(err.Error(), "logged out") {
		t.Errorf("logged-out default profile should report its error, got %v", err)
	}
	if calls != before {
		t.Fatalf("unsupported or authenticated commands made %d requests", calls-before)
	}
}

func anonymousResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Request:    req,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
