package phabricator

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	browseHeading  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#24292F", Dark: "#E6EDF3"})
	browseAccent   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0969DA", Dark: "#79C0FF"})
	browseMuted    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#57606A", Dark: "#8B949E"})
	browseError    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#CF222E", Dark: "#FF7B72"})
	browseSelected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#164C88"))
)

func browsePriority(label string) lipgloss.Style {
	value := strings.ToLower(label)
	switch {
	case strings.Contains(value, "unbreak"), strings.Contains(value, "high"):
		return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#BC4C00", Dark: "#FFA657"})
	case strings.Contains(value, "resolved"), strings.Contains(value, "low"):
		return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1A7F37", Dark: "#7EE787"})
	case strings.Contains(value, "stalled"), strings.Contains(value, "triage"):
		return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#E3B341"})
	default:
		return browseAccent
	}
}

type browseLane struct {
	column   browseColumn
	page     browsePage
	selected int
	err      error
}

// A board requests each column independently: a busy column cannot consume the
// entire board's page limit. Limit concurrency to avoid flooding Conduit.
func loadBrowseBoard(ctx context.Context, source browseSource, query browseQuery, columns []browseColumn) []browseLane {
	lanes := make([]browseLane, len(columns))
	slots := make(chan struct{}, 4)
	var work sync.WaitGroup
	for i, column := range columns {
		work.Add(1)
		go func(index int, column browseColumn) {
			defer work.Done()
			lane := browseLane{column: column}
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				lane.err = ctx.Err()
				lanes[index] = lane
				return
			}
			q := query
			q.column = column.phid
			q.after = ""
			q.before = ""
			lane.page, lane.err = source.columnTasks(ctx, q)
			lanes[index] = lane
		}(i, column)
	}
	work.Wait()
	return lanes
}

func (m *browseModel) loadLane(after, before string) tea.Cmd {
	ctx, generation := m.beginRequest()
	index, source, query := m.activeLane, m.source, m.query
	lane := m.lanes[index]
	query.column = lane.column.phid
	query.after = after
	query.before = before
	return func() tea.Msg {
		lane.page, lane.err = source.columnTasks(ctx, query)
		lane.selected = 0
		return browseResult{generation: generation, lane: &lane, laneIndex: index}
	}
}

func (m browseModel) currentItem() (browseItem, bool) {
	if m.screen == "tasks" && m.board {
		if len(m.lanes) == 0 {
			return browseItem{}, false
		}
		lane := m.lanes[m.activeLane]
		if lane.err != nil || lane.selected >= len(lane.page.items) {
			return browseItem{}, false
		}
		return lane.page.items[lane.selected], true
	}
	if m.selected >= len(m.page.items) {
		return browseItem{}, false
	}
	return m.page.items[m.selected], true
}

func (m *browseModel) updateBoardKey(key string) (bool, tea.Cmd) {
	if len(m.lanes) == 0 {
		return false, nil
	}
	lane := &m.lanes[m.activeLane]
	switch key {
	case "left", "h":
		m.activeLane = max(0, m.activeLane-1)
	case "right", "l":
		m.activeLane = min(len(m.lanes)-1, m.activeLane+1)
	case "c":
		m.activeLane = (m.activeLane + 1) % len(m.lanes)
	case "j", "down":
		if lane.selected+1 < len(lane.page.items) {
			lane.selected++
		}
	case "k", "up":
		if lane.selected > 0 {
			lane.selected--
		}
	case "n":
		if lane.page.after != "" {
			return true, m.loadLane(lane.page.after, "")
		}
	case "p":
		if lane.page.before != "" {
			return true, m.loadLane("", lane.page.before)
		}
	default:
		return false, nil
	}
	return true, nil
}

func (m browseModel) boardView() string {
	status := "Open tasks"
	if m.query.allStatuses {
		status = "All statuses"
	}
	var b strings.Builder
	b.WriteString(browseAccent.Render(browseLine(status+" · board view", m.width)) + "\n")
	if m.height < 14 {
		b.WriteString(browseLine("Enlarge terminal or use list", m.width) + "\n" + browseLine("b list · q quit", m.width) + "\n")
		return b.String()
	}
	help := ansi.Wrap("←/→ columns · ↑/↓ tasks · Enter open · n/p column pages\nb list · / search · s open/all · o browser · r refresh\nEsc back · q quit", m.width, "")
	overhead := 6 + lipgloss.Height(help)
	if m.query.search != "" {
		overhead++
	}
	if m.editing {
		overhead += 3
	}
	if m.loading {
		overhead++
	}
	if m.height-overhead < 5 {
		b.WriteString(browseLine("Enlarge terminal or use list", m.width) + "\n" + browseLine("b list · q quit", m.width) + "\n")
		return b.String()
	}
	if m.err == nil && len(m.lanes) > 0 {
		visible := max(1, min(len(m.lanes), m.width/30))
		start := max(0, min(m.activeLane-visible+1, len(m.lanes)-visible))
		width := (m.width - (visible - 1)) / visible
		height := max(3, m.height-overhead-2)
		rendered := make([]string, 0, visible*2)
		for i := start; i < start+visible; i++ {
			if len(rendered) > 0 {
				rendered = append(rendered, " ")
			}
			rendered = append(rendered, m.renderLane(i, width, height))
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, rendered...) + "\n")
		b.WriteString(browseMuted.Render(browseLine(fmt.Sprintf("Columns %d–%d of %d · tasks ordered by update time", start+1, start+visible, len(m.lanes)), m.width)) + "\n")
	} else if !m.loading && m.err == nil {
		b.WriteString("No active workboard columns. Press b for the task list.\n")
	}
	if m.query.search != "" {
		b.WriteString(browseAccent.Render(browseLine("Search: "+m.query.search, m.width)) + "\n")
	}
	b.WriteString("\n" + browseMuted.Render(help) + "\n")
	return b.String()
}

func (m browseModel) renderLane(index, width, height int) string {
	lane := m.lanes[index]
	inner := max(1, width-4)
	header := browseHeading
	border := lipgloss.AdaptiveColor{Light: "#D0D7DE", Dark: "#30363D"}
	if index == m.activeLane {
		header = browseAccent.Bold(true)
		border = lipgloss.AdaptiveColor{Light: "#0969DA", Dark: "#58A6FF"}
	}
	var b strings.Builder
	name := lane.column.name
	if index == m.activeLane {
		name = "▸ " + name
	}
	b.WriteString(header.Render(browseLine(name, inner)) + "\n")
	count := fmt.Sprintf("%d loaded", len(lane.page.items))
	if lane.page.after != "" {
		count += " · more"
	}
	b.WriteString(browseMuted.Render(browseLine(count, inner)) + "\n")
	if height >= 4 {
		b.WriteString("\n")
	}
	if lane.err != nil && height >= 5 {
		b.WriteString(browseError.Render(browseLine(lane.err.Error(), inner)) + "\n")
	}
	if lane.err == nil && len(lane.page.items) == 0 && height >= 5 {
		b.WriteString(browseMuted.Render("No matching tasks") + "\n")
	}
	capacity := max(0, (height-4)/4)
	compact := capacity == 0
	if compact {
		capacity = max(0, height-4)
	}
	start := max(0, lane.selected-capacity+1)
	for i := start; lane.err == nil && i < len(lane.page.items) && i < start+capacity; i++ {
		item := lane.page.items[i]
		if compact {
			line := browseLine(item.title, inner)
			if index == m.activeLane && i == lane.selected {
				line = browseSelected.Width(inner).Render(line)
			}
			b.WriteString(line + "\n")
			continue
		}
		titleText := browseText(item.title)
		if index == m.activeLane && i == lane.selected {
			titleText = "› " + titleText
		}
		lines := strings.Split(ansi.Wrap(titleText, inner, ""), "\n")
		if len(lines) > 2 {
			lines = lines[:2]
			lines[1] = ansi.Truncate(lines[1], max(1, inner-1), "") + "…"
		}
		for len(lines) < 2 {
			lines = append(lines, "")
		}
		title := strings.Join(lines, "\n")
		subtitle := browseLine(item.subtitle, inner)
		if index == m.activeLane && i == lane.selected {
			b.WriteString(browseSelected.Width(inner).Render(title+"\n"+subtitle) + "\n\n")
		} else {
			b.WriteString(browseHeading.Render(title) + "\n" + browsePriority(item.subtitle).Render(subtitle) + "\n\n")
		}
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1).Width(width - 2).Height(height).Render(b.String())
}
