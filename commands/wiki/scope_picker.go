package wiki

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

var (
	oauthPresetLabels = []string{"Basic only", "Low", "Low + vandalism", "Low + vandalism + security"}
	oauthGroupOrder   = []string{"page-interaction", "file-interaction", "watchlist-interaction", "email", "customization", "administration", "high-volume", "private-information", "other", "unknown"}
)

// Selection is independent of terminal rendering so bulk permission changes can
// be checked without a real login, a password manager or a terminal session.
type oauthSelection struct {
	grants  []oauthGrant
	checked map[string]bool
	level   int
	cursor  int
	filter  string
}

func newOAuthSelection(defaults []string) *oauthSelection {
	state := &oauthSelection{checked: map[string]bool{"basic": true}}
	known := map[string]bool{"basic": true}
	for _, grant := range defaultOAuthGrants {
		known[grant.Scope] = true
		if grant.Scope != "basic" {
			state.grants = append(state.grants, grant)
		}
	}
	for _, scope := range defaults {
		if scope == "" {
			continue
		}
		state.checked[scope] = true
		if !known[scope] {
			// Saved custom scopes survive reauthorization without assuming a risk tier.
			state.grants = append(state.grants, oauthGrant{Scope: scope, Label: scope, Group: "unknown", Risk: "unknown"})
			known[scope] = true
		}
	}
	order := map[string]int{}
	for i, group := range oauthGroupOrder {
		order[group] = i
	}
	sort.SliceStable(state.grants, func(i, j int) bool {
		a, b := state.grants[i], state.grants[j]
		if order[a.Group] != order[b.Group] {
			return order[a.Group] < order[b.Group]
		}
		return a.Scope < b.Scope
	})
	for level := 0; level <= 3; level++ {
		if state.matchesPreset(level) {
			state.level = level
		}
	}
	return state
}

func oauthRiskLevel(risk string) int {
	switch risk {
	case "low":
		return 1
	case "vandalism":
		return 2
	case "security":
		return 3
	default:
		return -1
	}
}

func (s *oauthSelection) matchesPreset(level int) bool {
	for _, grant := range s.grants {
		risk := oauthRiskLevel(grant.Risk)
		expected := risk > 0 && risk <= level
		if s.checked[grant.Scope] != expected {
			return false
		}
	}
	return true
}

func (s *oauthSelection) label() string {
	for level, label := range oauthPresetLabels {
		if s.matchesPreset(level) {
			return label
		}
	}
	return "Custom"
}

func (s *oauthSelection) preset(delta int) {
	s.level += delta
	if s.level < 0 {
		s.level = 0
	}
	if s.level > 3 {
		s.level = 3
	}
	for _, grant := range s.grants {
		if risk := oauthRiskLevel(grant.Risk); risk > 0 {
			s.checked[grant.Scope] = risk <= s.level
		}
	}
	// Internal and unknown grants are never added or removed by risk presets.
}

func (s *oauthSelection) visible() []int {
	var indices []int
	filter := strings.ToLower(s.filter)
	for i, grant := range s.grants {
		text := grant.Scope + " " + grant.Label + " " + oauthGroupLabels[grant.Group] + " " + grant.Risk
		if strings.Contains(strings.ToLower(text), filter) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (s *oauthSelection) change(key rune) {
	visible := s.visible()
	switch key {
	case terminal.KeyArrowRight:
		s.preset(1)
	case terminal.KeyArrowLeft:
		s.preset(-1)
	case terminal.KeyArrowDown, terminal.KeyTab:
		if len(visible) > 0 {
			s.cursor = (s.cursor + 1) % len(visible)
		}
	case terminal.KeyArrowUp:
		if len(visible) > 0 {
			s.cursor = (s.cursor + len(visible) - 1) % len(visible)
		}
	case terminal.KeySpace:
		if len(visible) > 0 {
			scope := s.grants[visible[s.cursor]].Scope
			s.checked[scope] = !s.checked[scope]
		}
	case terminal.KeyBackspace, terminal.KeyDelete:
		chars := []rune(s.filter)
		if len(chars) > 0 {
			s.filter = string(chars[:len(chars)-1])
			s.cursor = 0
		}
	case terminal.KeyEscape, terminal.KeyDeleteLine, terminal.KeyDeleteWord:
		s.filter = ""
		s.cursor = 0
	default:
		if unicode.IsPrint(key) {
			s.filter += string(key)
			s.cursor = 0
		}
	}
}

func (s *oauthSelection) scopes() []string {
	result := []string{"basic"}
	for _, grant := range s.grants {
		if s.checked[grant.Scope] {
			result = append(result, grant.Scope)
		}
	}
	return result
}

// Escape stored scope names before displaying them; explicit custom scopes are
// validated again by the OAuth request builder before any authorization request.
func scopeDisplay(value string) string {
	quoted := strconv.QuoteToASCII(value)
	return quoted[1 : len(quoted)-1]
}

type oauthScopeRow struct {
	Group, Label, Scope, Risk, Color string
	Checked, Focused                 bool
}

type oauthScopeView struct {
	Version                  string
	Label, Filter            string
	Count, Start, End, Total int
	Rows                     []oauthScopeRow
}

func (s *oauthSelection) view(pageSize int) oauthScopeView {
	visible := s.visible()
	start := (s.cursor / pageSize) * pageSize
	end := start + pageSize
	if end > len(visible) {
		end = len(visible)
	}
	view := oauthScopeView{Version: oauthCatalogueVersion, Label: s.label(), Filter: scopeDisplay(s.filter), Count: len(s.scopes()) - 1, Total: len(visible)}
	if len(visible) == 0 {
		return view
	}
	view.Start, view.End = start+1, end
	previous := ""
	for position := start; position < end; position++ {
		grant := s.grants[visible[position]]
		row := oauthScopeRow{Label: scopeDisplay(grant.Label), Scope: scopeDisplay(grant.Scope), Risk: grant.Risk, Checked: s.checked[grant.Scope], Focused: position == s.cursor}
		if grant.Group != previous {
			row.Group = oauthGroupLabels[grant.Group]
			previous = grant.Group
		}
		switch grant.Risk {
		case "low":
			row.Color = "green"
		case "vandalism":
			row.Color = "yellow"
		case "security":
			row.Color = "red"
		case "internal":
			row.Color = "magenta"
		default:
			row.Color = "cyan"
		}
		view.Rows = append(view.Rows, row)
	}
	return view
}

type oauthScopePrompt struct {
	survey.Renderer
	selection *oauthSelection
}

const oauthScopeTemplate = `{{color "cyan"}}?{{color "reset"}} Wiki access: {{.Label}} ({{.Count}} additional grants; basic always included)
  Up/Down: move | Space: toggle | Enter: confirm | Type: filter | Esc: clear filter
  Left/Right: global presets Basic > Low > +Vandalism > +Security
  Internal/unknown grants are manual; presets preserve their current selections.
  Shared catalogue: {{.Version}}; the wiki validates consumer availability.
{{if .Filter}}  Filter: {{.Filter}}
{{end}}{{range .Rows}}{{if .Group}}{{color "default+hb"}}  {{.Group}}{{color "reset"}}
{{end}}  {{if .Focused}}{{color "cyan"}}>{{color "reset"}}{{else}} {{end}} [{{if .Checked}}x{{else}} {{end}}] {{color .Color}}[{{.Risk}}]{{color "reset"}} {{.Label}} ({{.Scope}})
{{end}}{{if .Total}}  {{.Start}}-{{.End}} of {{.Total}} matching grants
{{else}}  No matching grants
{{end}}`

func (p *oauthScopePrompt) Prompt(_ *survey.PromptConfig) (interface{}, error) {
	if p.selection == nil {
		p.selection = newOAuthSelection(nil)
	}
	reader := p.NewRuneReader()
	if err := reader.SetTermMode(); err != nil {
		return nil, fmt.Errorf("could not open the permission picker terminal: %w", err)
	}
	defer func() { _ = reader.RestoreTermMode() }()
	cursor := p.NewCursor()
	cursor.Hide()
	defer cursor.Show()
	for {
		if err := p.Render(oauthScopeTemplate, p.selection.view(8)); err != nil {
			return nil, err
		}
		key, _, err := reader.ReadRune()
		if err != nil {
			return nil, err
		}
		switch key {
		case '\r', '\n':
			return p.selection.scopes(), nil
		case terminal.KeyInterrupt, terminal.KeyEndTransmission:
			return nil, terminal.InterruptErr
		default:
			p.selection.change(key)
		}
	}
}

func (p *oauthScopePrompt) Cleanup(_ *survey.PromptConfig, value interface{}) error {
	scopes, ok := value.([]string)
	if !ok {
		return fmt.Errorf("invalid permission selection")
	}
	return p.Render("Selected wiki access: basic + {{.}} additional grants\n", len(scopes)-1)
}
