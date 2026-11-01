package phabricator

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func browseKey(m browseModel, key string) (browseModel, tea.Cmd) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	if key == "enter" {
		msg.Type = tea.KeyEnter
	}
	if key == "esc" {
		msg.Type = tea.KeyEsc
	}
	updated, cmd := m.Update(msg)
	return updated.(browseModel), cmd
}

func fakeBrowseSource() browseSource {
	return browseSource{
		projects: func(context.Context, string, string, string) (browsePage, error) {
			return browsePage{items: []browseItem{{key: "PHID-PROJ-test", title: "A project", url: "https://example.test/project/1"}}}, nil
		},
		tasks: func(context.Context, browseQuery) (browsePage, error) {
			return browsePage{items: []browseItem{{key: "T1", title: "T1 Test task"}}, after: "cursor"}, nil
		},
		columns: func(context.Context, string) ([]browseColumn, error) {
			return []browseColumn{{name: "Backlog", phid: "PHID-PCOL-backlog"}}, nil
		},
		detail: func(context.Context, string) (string, error) { return "Task description", nil }, open: func(string) error { return nil },
	}
}

func completeBrowse(m browseModel, cmd tea.Cmd) browseModel {
	result, _ := m.Update(cmd())
	return result.(browseModel)
}

func loadBrowse(m browseModel) browseModel { cmd := m.load(); return completeBrowse(m, cmd) }

func TestBrowseProjectTaskDetailNavigation(t *testing.T) {
	m := newBrowseModel(context.Background(), fakeBrowseSource(), "")
	m = loadBrowse(m)
	if len(m.page.items) != 1 || m.screen != "projects" {
		t.Fatal(m.View())
	}
	m, cmd := browseKey(m, "enter")
	m = completeBrowse(m, cmd)
	if m.screen != "tasks" || m.query.project != "PHID-PROJ-test" {
		t.Fatal(m.View())
	}
	m, cmd = browseKey(m, "enter")
	m = completeBrowse(m, cmd)
	if m.screen != "detail" || !strings.Contains(m.View(), "Task description") {
		t.Fatal(m.View())
	}
	m, _ = browseKey(m, "esc")
	if m.screen != "tasks" || len(m.page.items) != 1 {
		t.Fatal(m.View())
	}
	m, cmd = browseKey(m, "esc")
	m = completeBrowse(m, cmd)
	if m.screen != "projects" {
		t.Fatal(m.View())
	}
}

func TestBrowseFiltersPagingAndSearch(t *testing.T) {
	source := fakeBrowseSource()
	var queries []browseQuery
	source.tasks = func(_ context.Context, q browseQuery) (browsePage, error) {
		queries = append(queries, q)
		return browsePage{after: "next", before: "previous"}, nil
	}
	m := newBrowseModel(context.Background(), source, "project")
	m = loadBrowse(m)
	m, cmd := browseKey(m, "c")
	m = completeBrowse(m, cmd)
	if queries[len(queries)-1].column != "PHID-PCOL-backlog" {
		t.Fatal(queries)
	}
	m, cmd = browseKey(m, "s")
	m = completeBrowse(m, cmd)
	if !queries[len(queries)-1].allStatuses {
		t.Fatal(queries)
	}
	m, cmd = browseKey(m, "n")
	m = completeBrowse(m, cmd)
	if queries[len(queries)-1].after != "next" {
		t.Fatal(queries)
	}
	m, cmd = browseKey(m, "p")
	m = completeBrowse(m, cmd)
	if q := queries[len(queries)-1]; q.before != "previous" || q.after != "" {
		t.Fatal(q)
	}
	m, _ = browseKey(m, "/")
	m.input.SetValue("docs")
	m, cmd = browseKey(m, "enter")
	m = completeBrowse(m, cmd)
	if q := queries[len(queries)-1]; q.search != "docs" || q.after != "" || q.before != "" {
		t.Fatal(q)
	}
	m, cmd = browseKey(m, "c")
	m = completeBrowse(m, cmd)
	if m.query.column != "" {
		t.Fatal(m.query)
	}
}

func TestBrowseIgnoresStaleResultsAndCancelsRequests(t *testing.T) {
	source := fakeBrowseSource()
	var firstContext context.Context
	source.projects = func(ctx context.Context, _, _, _ string) (browsePage, error) {
		firstContext = ctx
		return browsePage{}, nil
	}
	m := newBrowseModel(context.Background(), source, "")
	first := m.load()
	result := first()
	_ = m.load()
	if !errors.Is(firstContext.Err(), context.Canceled) {
		t.Fatal("old request not canceled")
	}
	updated, _ := m.Update(result)
	m = updated.(browseModel)
	if !m.loading {
		t.Fatal("stale result replaced new request")
	}
	if m.cancel != nil {
		m.cancel()
	}
}

func TestBrowseErrorRetryAndTerminalInput(t *testing.T) {
	source := fakeBrowseSource()
	source.projects = func(context.Context, string, string, string) (browsePage, error) {
		return browsePage{}, errors.New("offline")
	}
	m := newBrowseModel(context.Background(), source, "")
	m = loadBrowse(m)
	if !strings.Contains(m.View(), "offline") {
		t.Fatal(m.View())
	}
	m, cmd := browseKey(m, "r")
	if cmd == nil || !m.loading {
		t.Fatal("retry did not start")
	}
	site := ""
	command := newPhabricatorBrowseCmd(&site)
	command.SetIn(strings.NewReader(""))
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("error: %v", err)
	}
}

func TestBrowseStripsTerminalControlCodes(t *testing.T) {
	if got := browseText("\x1b[31mred\x1b[0m\x00\ntext"); got != "red\ntext" {
		t.Fatalf("%q", got)
	}
}

func TestBrowseProgramLoadsAndQuits(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	projects := make(chan struct{}, 1)
	source := fakeBrowseSource()
	getProjects := source.projects
	source.projects = func(ctx context.Context, q, a, b string) (browsePage, error) {
		page, err := getProjects(ctx, q, a, b)
		projects <- struct{}{}
		return page, err
	}
	m := newBrowseModel(ctx, source, "")
	m.initial = m.load()
	program := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
	finished := make(chan error, 1)
	go func() { _, err := program.Run(); finished <- err }()
	select {
	case <-projects:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// q is available during loading as well as after the response arrives.
	program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestBrowseSourceUsesQueriesAndResolvesComments(t *testing.T) {
	client := queryClient(t, func(method string, params map[string]interface{}) string {
		switch method {
		case "/api/project.search":
			return `{"data":[{"id":5331,"phid":"PHID-PROJ-cli","fields":{"name":"CLI"}}],"cursor":{"after":"next"}}`
		case "/api/maniphest.search":
			if params["constraints"].(map[string]interface{})["ids"] != nil {
				return `{"data":[{"id":1,"phid":"PHID-TASK-one","fields":{"name":"Test","authorPHID":"PHID-USER-one","description":{"raw":"Description"}}}]}`
			}
			statuses := params["constraints"].(map[string]interface{})["statuses"]
			if !reflect.DeepEqual(statuses, []interface{}{"open()"}) {
				t.Fatal(statuses)
			}
			return `{"data":[{"id":1,"fields":{"name":"Test"}}],"cursor":{"after":"more"}}`
		case "/api/transaction.search":
			return `{"data":[{"id":1,"authorPHID":"PHID-USER-one","comments":[{"content":{"raw":"Comment"}}]}]}`
		case "/api/user.search":
			return `{"data":[{"phid":"PHID-USER-one","fields":{"username":"Example"}}]}`
		default:
			t.Fatalf("unexpected method %s", method)
			return ""
		}
	})
	source := newBrowseSource(client)
	projects, err := source.projects(context.Background(), "cli", "", "")
	if err != nil || projects.after != "next" || projects.items[0].url != "https://phab.example/project/profile/5331/" {
		t.Fatalf("%+v %v", projects, err)
	}
	tasks, err := source.tasks(context.Background(), browseQuery{})
	if err != nil || tasks.after != "more" || tasks.items[0].key != "T1" {
		t.Fatalf("%+v %v", tasks, err)
	}
	detail, err := source.detail(context.Background(), "T1")
	if err != nil || !strings.Contains(detail, "Example") || !strings.Contains(detail, "Comment") {
		t.Fatalf("%s %v", detail, err)
	}
}

func TestBrowseFailedTransitionDoesNotOpenStaleItems(t *testing.T) {
	source := fakeBrowseSource()
	source.tasks = func(context.Context, browseQuery) (browsePage, error) { return browsePage{}, errors.New("offline") }
	m := loadBrowse(newBrowseModel(context.Background(), source, ""))
	m, cmd := browseKey(m, "enter")
	m = completeBrowse(m, cmd)
	if m.screen != "tasks" || m.err == nil {
		t.Fatal(m.View())
	}
	for _, key := range []string{"enter", "o", "n", "c", "s"} {
		next, action := browseKey(m, key)
		if action != nil || next.screen != m.screen {
			t.Fatalf("%s acted on stale results", key)
		}
	}
	m, cmd = browseKey(m, "/")
	if !m.editing || cmd == nil {
		t.Fatal("cannot search after error")
	}
}
