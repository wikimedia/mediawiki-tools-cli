package cli

import (
	"os"
	"sync"

	"github.com/charmbracelet/glamour"
	styles "github.com/charmbracelet/glamour/styles"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

/*SkipRenderMarkdown allows markdown rendering to be skipped in certain situations.*/
var SkipRenderMarkdown = false

// termenv.HasDarkBackground queries the terminal and synchronously waits for a
// response every time it is called. RenderMarkdown is called for the Long text
// of many commands on every single CLI invocation, so the result must be
// cached to avoid a terminal round trip (or timeout) per command.
var hasDarkBackground = sync.OnceValue(termenv.HasDarkBackground)

/*RenderMarkdown converts markdown into something nice to be displayed on the terminal.*/
func RenderMarkdown(markdownIn string) string {
	if os.Getenv("MWCLI_SKIP_RENDER_MARKDOWN") != "" {
		return markdownIn
	}

	width, _, _ := term.GetSize(0)

	// Logic copied from glamour.WithAutoStyle
	style := styles.LightStyleConfig
	if hasDarkBackground() {
		style = styles.DarkStyleConfig
	}

	// Styletweak: Avoid a 2 char margin along the "document" on output
	uintPtr := func(u uint) *uint { return &u }
	style.Document.Margin = uintPtr(0)

	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		// wrap output at specific width
		glamour.WithWordWrap(width),
	)

	out, err := r.Render(markdownIn)
	if err != nil {
		panic(err)
	}
	return out
}
