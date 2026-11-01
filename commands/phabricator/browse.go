package phabricator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"golang.org/x/term"
)

type (
	browseItem struct{ key, title, subtitle, url string }
	browsePage struct {
		items         []browseItem
		after, before string
	}
)

type (
	browseColumn struct{ name, phid string }
	browseQuery  struct {
		project, column, search, after, before string
		allStatuses                            bool
	}
)

type browseSource struct {
	projects func(context.Context, string, string, string) (browsePage, error)
	tasks    func(context.Context, browseQuery) (browsePage, error)
	columns  func(context.Context, string) ([]browseColumn, error)
	detail   func(context.Context, string) (string, error)
	open     func(string) error
}
type browseResult struct {
	generation int
	page       browsePage
	columns    []browseColumn
	detail     string
	err        error
}
type (
	browseOpened struct{ err error }
	browseModel  struct {
		ctx                                         context.Context
		cancel                                      context.CancelFunc
		source                                      browseSource
		screen                                      string
		projectName, projectSearch                  string
		query                                       browseQuery
		page                                        browsePage
		columns                                     []browseColumn
		selected, column, generation, width, height int
		loading, editing                            bool
		err                                         error
		input                                       textinput.Model
		detail                                      viewport.Model
		detailItem                                  browseItem
		initial                                     tea.Cmd
	}
)

func newBrowseModel(ctx context.Context, source browseSource, project string) browseModel {
	input := textinput.New()
	input.Prompt = "Search: "
	input.CharLimit = 500
	m := browseModel{ctx: ctx, source: source, screen: "projects", input: input, width: 80, height: 24, column: -1, detail: viewport.New(80, 15)}
	if project != "" {
		m.screen = "tasks"
		m.query.project = project
		m.projectName = project
	}
	return m
}

func (m *browseModel) load() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.generation++
	generation := m.generation
	m.loading = true
	m.err = nil
	source, screen, query, search, detailItem := m.source, m.screen, m.query, m.projectSearch, m.detailItem
	return func() tea.Msg {
		result := browseResult{generation: generation}
		switch screen {
		case "projects":
			result.page, result.err = source.projects(ctx, search, query.after, query.before)
		case "tasks":
			if query.project != "" {
				result.columns, result.err = source.columns(ctx, query.project)
			}
			if result.err == nil {
				result.page, result.err = source.tasks(ctx, query)
			}
		case "detail":
			result.detail, result.err = source.detail(ctx, detailItem.key)
		}
		return result
	}
}
func (m browseModel) Init() tea.Cmd { return m.initial }
func (m browseModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = max(20, msg.Width)
		m.height = max(10, msg.Height)
		m.input.Width = max(10, m.width-10)
		m.detail.Width = m.width
		m.detail.Height = max(3, m.height-7)
	case browseResult:
		if msg.generation != m.generation {
			return m, nil
		}
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			if m.screen == "detail" {
				m.detail.SetContent(ansi.Hardwrap(browseText(msg.detail), m.width, true))
				m.detail.GotoTop()
			} else {
				m.page = msg.page
				m.columns = msg.columns
				m.selected = 0
			}
		}
	case browseOpened:
		m.err = msg.err
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		if m.editing {
			switch msg.String() {
			case "esc":
				m.editing = false
				m.input.Blur()
				return m, nil
			case "enter":
				m.editing = false
				m.input.Blur()
				m.query.after = ""
				m.query.before = ""
				if m.screen == "projects" {
					m.projectSearch = m.input.Value()
				} else {
					m.query.search = m.input.Value()
				}
				return m, m.load()
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "q":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case "esc", "backspace":
			if m.cancel != nil {
				m.cancel()
			}
			m.generation++
			m.loading = false
			m.err = nil
			if m.screen == "detail" {
				m.screen = "tasks"
				return m, nil
			}
			if m.screen == "tasks" {
				m.screen = "projects"
				m.query = browseQuery{}
				m.column = -1
				return m, m.load()
			}
			return m, tea.Quit
		case "r":
			return m, m.load()
		}
		if m.loading {
			return m, nil
		}
		if m.err != nil && msg.String() != "/" {
			return m, nil
		}
		if m.screen == "detail" {
			if msg.String() == "o" {
				url := m.detailItem.url
				return m, func() tea.Msg { return browseOpened{m.source.open(url)} }
			}
			var cmd tea.Cmd
			m.detail, cmd = m.detail.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "j", "down":
			if m.selected+1 < len(m.page.items) {
				m.selected++
			}
		case "k", "up":
			if m.selected > 0 {
				m.selected--
			}
		case "/":
			m.editing = true
			if m.screen == "projects" {
				m.input.SetValue(m.projectSearch)
			} else {
				m.input.SetValue(m.query.search)
			}
			return m, m.input.Focus()
		case "n", "p":
			cursor := m.page.after
			if msg.String() == "p" {
				cursor = m.page.before
			}
			if cursor != "" {
				m.query.after = ""
				m.query.before = ""
				if msg.String() == "n" {
					m.query.after = cursor
				} else {
					m.query.before = cursor
				}
				return m, m.load()
			}
		case "c":
			if m.screen == "tasks" && len(m.columns) > 0 {
				m.column++
				if m.column >= len(m.columns) {
					m.column = -1
				}
				m.query.column = ""
				if m.column >= 0 {
					m.query.column = m.columns[m.column].phid
				}
				m.query.after = ""
				m.query.before = ""
				return m, m.load()
			}
		case "s":
			if m.screen == "tasks" {
				m.query.allStatuses = !m.query.allStatuses
				m.query.after = ""
				m.query.before = ""
				return m, m.load()
			}
		case "enter":
			if len(m.page.items) == 0 {
				return m, nil
			}
			item := m.page.items[m.selected]
			if m.screen == "projects" {
				m.screen = "tasks"
				m.projectName = item.title
				m.query = browseQuery{project: item.key}
				m.column = -1
			} else {
				m.screen = "detail"
				m.detailItem = item
				m.detail.SetContent("")
			}
			return m, m.load()
		case "o":
			if len(m.page.items) > 0 {
				url := m.page.items[m.selected].url
				return m, func() tea.Msg { return browseOpened{m.source.open(url)} }
			}
		}
	}
	return m, nil
}

func (m browseModel) View() string {
	title := "Phabricator projects"
	if m.screen == "tasks" {
		title = "Tasks — " + m.projectName
		if m.query.project == "" {
			title = "All tasks"
		}
	}
	if m.screen == "detail" {
		title = m.detailItem.title
	}
	var b strings.Builder
	b.WriteString(browseLine(title, m.width) + "\n\n")
	if m.loading {
		b.WriteString("Loading…\n")
	}
	if m.err != nil {
		b.WriteString(browseLine("Error: "+m.err.Error(), m.width) + "\nPress r to retry or Esc to go back.\n")
	}
	if m.screen == "detail" {
		if !m.loading && m.err == nil {
			b.WriteString(m.detail.View() + "\n")
		}
		b.WriteString("\n↑/↓ scroll · o browser · Esc back · q quit\n")
	} else {
		if m.screen == "tasks" {
			column := "All columns"
			if m.column >= 0 && m.column < len(m.columns) {
				column = m.columns[m.column].name
			}
			status := "Open tasks"
			if m.query.allStatuses {
				status = "All statuses"
			}
			b.WriteString(browseLine(column+" · "+status, m.width) + "\n")
		}
		if !m.loading && m.err == nil {
			if len(m.page.items) == 0 {
				b.WriteString("No results. Press / to change the search.\n")
			}
			visible := max(1, (m.height-10)/2)
			start := max(0, m.selected-visible+1)
			for i := start; i < len(m.page.items) && i < start+visible; i++ {
				marker := "  "
				if i == m.selected {
					marker = "> "
				}
				b.WriteString(marker + browseLine(m.page.items[i].title, max(1, m.width-2)) + "\n")
				b.WriteString("  " + browseLine(m.page.items[i].subtitle, max(1, m.width-2)) + "\n")
			}
		}
		search := m.projectSearch
		if m.screen == "tasks" {
			search = m.query.search
		}
		if search != "" {
			b.WriteString("\n" + browseLine("Search: "+search, m.width) + "\n")
		}
		b.WriteString("\n↑/↓ select · Enter open · / search · n/p pages · o browser\n")
		if m.screen == "tasks" {
			b.WriteString("c column · s open/all statuses · ")
		}
		b.WriteString("Esc back · q quit\n")
	}
	if m.editing {
		b.WriteString("\n" + m.input.View() + "\nEnter search · Esc cancel\n")
	}
	return b.String()
}

func browseText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, ansi.Strip(value))
}

func browseLine(value string, width int) string {
	return ansi.Truncate(strings.ReplaceAll(browseText(value), "\n", " "), width, "…")
}

func newPhabricatorBrowseCmd(site *string) *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use: "browse", Short: "Browse projects and tasks interactively",
		Long:    "Browse Phabricator projects, task lists and descriptions/comments in a read-only terminal interface. Search with /, switch workboard columns with c, and press o to open the selected object in your browser. Task changes use the normal task commands.",
		Example: "browse\nbrowse --project a_wikimedia_cli", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			input, inOK := cmd.InOrStdin().(*os.File)
			out, outOK := cmd.OutOrStdout().(*os.File)
			if !cli.CanAskForInput() || !inOK || !outOK || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(out.Fd())) || os.Getenv("TERM") == "dumb" {
				return fmt.Errorf("browse requires an interactive terminal; use task list or project list for scripts")
			}
			client, _, err := clientFromSite(*site)
			if err != nil {
				return err
			}
			if client.cache != nil {
				defer client.cache.flush()
			}
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			model := newBrowseModel(ctx, newBrowseSource(client), project)
			model.initial = model.load()
			_, err = tea.NewProgram(model, tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(out), tea.WithAltScreen()).Run()
			return err
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project slug or PHID to open")
	return cmd
}
