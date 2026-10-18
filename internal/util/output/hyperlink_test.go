package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

func TestHyperlinksRequireRecognisedTerminalAndTTY(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"unknown", nil, false},
		{"iterm", map[string]string{"TERM_PROGRAM": "iTerm.app"}, true},
		{"vscode", map[string]string{"TERM_PROGRAM": "vscode"}, true},
		{"vte", map[string]string{"VTE_VERSION": "5000"}, true},
		{"old vte", map[string]string{"VTE_VERSION": "4900"}, false},
		{"windows", map[string]string{"WT_SESSION": "session"}, true},
		{"kitty", map[string]string{"TERM": "xterm-kitty"}, true},
		{"dumb", map[string]string{"TERM": "dumb", "TERM_PROGRAM": "iTerm.app"}, false},
		{"ci", map[string]string{"CI": "true", "TERM_PROGRAM": "iTerm.app"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{"TERM", "TERM_PROGRAM", "VTE_VERSION", "WT_SESSION", "KONSOLE_VERSION", "DOMTERM", "CI"} {
				t.Setenv(key, "")
			}
			for key, value := range tc.env {
				t.Setenv(key, value)
			}
			if got := hyperlinkEnvironment(); got != tc.want {
				t.Fatalf("detection: got %v want %v", got, tc.want)
			}
			if supportsHyperlinks(&bytes.Buffer{}) {
				t.Fatal("non-TTY output enabled hyperlinks")
			}
		})
	}
}

func TestHyperlinksRejectUnsafeDestinations(t *testing.T) {
	for _, target := range []string{"", "javascript:alert(1)", "https://user:pass@example.org", "https://example.org/\x1b]bad", "https://example.org/\n", "//example.org"} {
		if got := terminalHyperlink("Page", target, true); got != "Page" {
			t.Fatalf("unsafe destination linked: %q", got)
		}
	}
	target := "https://en.wikipedia.org/wiki/Main_Page"
	linked := terminalHyperlink("Main_Page", target, true)
	if !strings.Contains(linked, "\x1b]8;;"+target+"\x1b\\") || ansi.Strip(linked) != "Main_Page" {
		t.Fatalf("invalid OSC8: %q", linked)
	}
	if terminalHyperlink("Main_Page", target, false) != "Main_Page" {
		t.Fatal("disabled link escaped")
	}
}

func TestGraphLinksPreserveWidthAndFullDestination(t *testing.T) {
	graph := Graph{Points: []GraphPoint{{Label: "日本語の長い記事名", URL: "https://ja.wikipedia.org/wiki/Full_article_title", Value: 100}}}
	var linked, plain strings.Builder
	graph.barGraph(&linked, 24, func(s string) string { return s }, true)
	graph.barGraph(&plain, 24, func(s string) string { return s }, false)
	if !strings.Contains(linked.String(), "https://ja.wikipedia.org/wiki/Full_article_title") {
		t.Fatal("destination truncated with label")
	}
	if ansi.Strip(linked.String()) != plain.String() {
		t.Fatal("hyperlinks changed graph alignment")
	}
	if runewidth.StringWidth(strings.TrimSuffix(plain.String(), "\n")) != 24 {
		t.Fatal("graph exceeds terminal width")
	}
}

func TestPipedLinkedCellsStayPlain(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	table := NewTable([]interface{}{"Page", "Views"}, [][]interface{}{{Hyperlink{Text: "Main_Page", URL: "https://en.wikipedia.org/wiki/Main_Page"}, 42}})
	var buf bytes.Buffer
	table.Print(&buf)
	if strings.Contains(buf.String(), "\x1b]8;") || strings.Contains(buf.String(), "https://") || !strings.Contains(buf.String(), "Main_Page") {
		t.Fatalf("bad redirected table: %q", buf.String())
	}
}
