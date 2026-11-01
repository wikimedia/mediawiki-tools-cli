package phabricator

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func boardSource() browseSource {
	source := fakeBrowseSource()
	source.columns = func(context.Context, string) ([]browseColumn, error) {
		return []browseColumn{{name: "Backlog", phid: "PHID-PCOL-backlog"}, {name: "In progress", phid: "PHID-PCOL-progress"}, {name: "Review", phid: "PHID-PCOL-review"}}, nil
	}
	source.columnTasks = func(_ context.Context, q browseQuery) (browsePage, error) {
		suffix := "first"
		if q.after != "" {
			suffix = "next"
		}
		return browsePage{items: []browseItem{{key: q.column + "-" + suffix, title: "T123 A task with a meaningful title", subtitle: "Open · High"}, {key: q.column + "-second", title: "T124 Another task", subtitle: "In progress · Normal"}}, after: "next", before: "previous"}, nil
	}
	return source
}

func TestBrowseBoardNavigationAndIndependentPaging(t *testing.T) {
	source := boardSource()
	var mu sync.Mutex
	var queries []browseQuery
	fetch := source.columnTasks
	source.columnTasks = func(ctx context.Context, q browseQuery) (browsePage, error) {
		mu.Lock()
		queries = append(queries, q)
		mu.Unlock()
		return fetch(ctx, q)
	}
	m := newBrowseModel(context.Background(), source, "project")
	m.board = true
	m = loadBrowse(m)
	if len(m.lanes) != 3 {
		t.Fatal(m.lanes)
	}
	for _, lane := range m.lanes {
		if len(lane.page.items) != 2 {
			t.Fatal(lane)
		}
	}
	m, _ = browseKey(m, "right")
	m, _ = browseKey(m, "j")
	item, ok := m.currentItem()
	if !ok || item.key != "PHID-PCOL-progress-second" {
		t.Fatal(item)
	}
	m, cmd := browseKey(m, "enter")
	m = completeBrowse(m, cmd)
	if m.screen != "detail" {
		t.Fatal(m.View())
	}
	m, _ = browseKey(m, "esc")
	if m.screen != "tasks" || m.activeLane != 1 {
		t.Fatal(m.View())
	}
	m, cmd = browseKey(m, "n")
	m = completeBrowse(m, cmd)
	if len(queries) != 4 {
		t.Fatalf("paging refetched the whole board: %d", len(queries))
	}
	if m.lanes[0].page.items[0].key != "PHID-PCOL-backlog-first" || m.lanes[1].page.items[0].key != "PHID-PCOL-progress-next" {
		t.Fatal(m.lanes)
	}
	if q := queries[3]; q.column != "PHID-PCOL-progress" || q.after != "next" || q.before != "" {
		t.Fatal(q)
	}
	m, cmd = browseKey(m, "p")
	m = completeBrowse(m, cmd)
	if q := queries[4]; q.before != "previous" || q.after != "" {
		t.Fatal(q)
	}
	m, cmd = browseKey(m, "s")
	m = completeBrowse(m, cmd)
	for _, q := range queries[5:] {
		if !q.allStatuses {
			t.Fatal(q)
		}
	}
	m, cmd = browseKey(m, "b")
	m = completeBrowse(m, cmd)
	if m.board {
		t.Fatal("did not switch to list")
	}
	m, cmd = browseKey(m, "b")
	m = completeBrowse(m, cmd)
	if !m.board {
		t.Fatal("did not switch back to board")
	}
}

func TestBrowseBoardColumnErrorAndEmptyBoard(t *testing.T) {
	source := boardSource()
	source.columnTasks = func(context.Context, browseQuery) (browsePage, error) {
		return browsePage{}, errors.New("cannot read column")
	}
	m := newBrowseModel(context.Background(), source, "project")
	m.board = true
	m = loadBrowse(m)
	if _, ok := m.currentItem(); ok {
		t.Fatal("selected item from failed column")
	}
	_, cmd := browseKey(m, "enter")
	if cmd != nil {
		t.Fatal("opened failed column")
	}
	if !strings.Contains(ansi.Strip(m.View()), "cannot read column") {
		t.Fatal(m.View())
	}
	source.columns = func(context.Context, string) ([]browseColumn, error) { return nil, nil }
	m.source = source
	m = loadBrowse(m)
	if !strings.Contains(m.View(), "No active workboard columns") {
		t.Fatal(m.View())
	}
}

func TestBrowseBoardRenderingFitsViewportAndUsesColour(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previous)
	m := newBrowseModel(context.Background(), boardSource(), "a_wikimedia_cli")
	m.board = true
	m = loadBrowse(m)
	for _, width := range []int{30, 60, 80, 120} {
		m.width = width
		rendered := m.renderLane(0, width, 12)
		if !strings.Contains(rendered, "\x1b[") {
			t.Fatal("no colour in rendered column")
		}
		for _, line := range strings.Split(rendered, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("column overflow at width %d: %q", width, line)
			}
		}
	}
	m.width = 100
	m.height = 28
	t.Log("\n" + ansi.Strip(m.boardView()))
}

func TestBrowseBoardColumnQueryDoesNotFetchOtherColumns(t *testing.T) {
	client := queryClient(t, func(method string, params map[string]interface{}) string {
		if method != "/api/maniphest.search" {
			t.Fatalf("unexpected lookup per column: %s", method)
		}
		constraints := params["constraints"].(map[string]interface{})
		want := map[string]interface{}{"columnPHIDs": []interface{}{"PHID-PCOL-review"}, "statuses": []interface{}{"open()"}, "query": "docs"}
		if !reflect.DeepEqual(constraints, want) || params["after"] != "cursor" {
			t.Fatal(params)
		}
		return `{"data":[{"id":123}],"cursor":{"after":"more"}}`
	})
	page, err := newBrowseSource(client).columnTasks(context.Background(), browseQuery{column: "PHID-PCOL-review", search: "docs", after: "cursor"})
	if err != nil || len(page.items) != 1 || page.after != "more" {
		t.Fatal(fmt.Sprint(page), err)
	}
}

func TestBrowseBoardFitsCompleteViewport(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 30}, {60, 20}, {30, 20}, {30, 14}, {20, 10}} {
		for _, filtered := range []bool{false, true} {
			for _, editing := range []bool{false, true} {
				m := newBrowseModel(context.Background(), boardSource(), "project")
				m.board = true
				m = loadBrowse(m)
				m.width = size[0]
				m.height = size[1]
				m.editing = editing
				if filtered {
					m.query.search = "documentation"
				}
				view := m.View()
				if lipgloss.Height(view) > m.height {
					t.Fatalf("overflow at %dx%d filtered=%v editing=%v: %d lines\n%s", m.width, m.height, filtered, editing, lipgloss.Height(view), ansi.Strip(view))
				}
			}
		}
	}
}

func TestBrowseProjectChangeClearsPreviousBoard(t *testing.T) {
	m := newBrowseModel(context.Background(), boardSource(), "old")
	m.board = true
	m = loadBrowse(m)
	m, cmd := browseKey(m, "esc")
	m = completeBrowse(m, cmd)
	m, cmd = browseKey(m, "enter")
	if cmd == nil || len(m.lanes) != 0 || m.activeLane != 0 {
		t.Fatal("previous project's board retained during load")
	}
	m = completeBrowse(m, cmd)
}
