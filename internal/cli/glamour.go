package cli

import (
	"os"

	"github.com/charmbracelet/glamour"
	terminal "golang.org/x/term"
)

/*SkipRenderMarkdown allows markdown rendering to be skipped in certain situations.*/
var SkipRenderMarkdown = false

/*RenderMarkdown converts markdown into something nice to be displayed on the terminal.*/
func RenderMarkdown(markdownIn string) string {
	if os.Getenv("MWCLI_SKIP_RENDER_MARKDOWN") != "" {
		return markdownIn
	}

	width, _, _ := terminal.GetSize(0)

	r, _ := glamour.NewTermRenderer(
		// detect background color and pick either the default dark or light theme
		glamour.WithAutoStyle(),
		// wrap output at specific width
		glamour.WithWordWrap(width),
	)

	out, err := r.Render(markdownIn)
	if err != nil {
		panic(err)
	}
	return out
}
