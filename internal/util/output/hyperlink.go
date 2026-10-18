package output

import (
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// Hyperlink keeps a cell's label separate from its destination so truncation
// and alignment happen before terminal escape sequences are added.
type Hyperlink struct {
	Text string
	URL  string
}

func (h Hyperlink) String() string { return graphLabel(h.Text) }

func supportsHyperlinks(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	return ok && isTerminal(file) && hyperlinkEnvironment()
}

func hyperlinkEnvironment() bool {
	if os.Getenv("TERM") == "dumb" || os.Getenv("CI") != "" {
		return false
	}
	// These environment hints identify terminals with OSC 8 support. Unknown
	// terminals retain plain labels. VTE introduced hyperlinks in version 0.50.
	if version, _ := strconv.Atoi(os.Getenv("VTE_VERSION")); version >= 5000 {
		return true
	}
	if os.Getenv("WT_SESSION") != "" || os.Getenv("KONSOLE_VERSION") != "" || os.Getenv("DOMTERM") != "" {
		return true
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "iTerm.app", "WezTerm", "vscode", "ghostty":
		return true
	}
	switch os.Getenv("TERM") {
	case "xterm-kitty", "xterm-ghostty", "foot", "foot-extra":
		return true
	}
	return false
}

func terminalHyperlink(text, target string, enabled bool) string {
	if !enabled || target == "" || strings.ContainsFunc(target, unicode.IsControl) {
		return text
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return text
	}
	return "\x1b]8;;" + target + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}
