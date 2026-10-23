package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderedHeredocCanBePasted(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("requires bash")
	}
	t.Setenv("MWCLI_SKIP_RENDER_MARKDOWN", "")
	originalBackground := hasDarkBackground
	t.Cleanup(func() { hasDarkBackground = originalBackground })
	const script = `cat > example.yaml <<'YAML'
metadata:
  name: hello
YAML
printf 'done\n'
`
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			hasDarkBackground = func() bool { return dark }
			pasted := ansi.Strip(RenderMarkdown("```sh\n" + script + "```\n"))
			dir := t.TempDir()
			cmd := exec.Command("bash", "-e", "-c", pasted)
			cmd.Dir = dir
			output, err := cmd.CombinedOutput()
			if err != nil || strings.TrimSpace(string(output)) != "done" {
				t.Fatalf("rendered shell block is not pasteable: %v\n%s\nRendered: %q", err, output, pasted)
			}
			actual, err := os.ReadFile(filepath.Join(dir, "example.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if string(actual) != "metadata:\n  name: hello\n" {
				t.Fatalf("rendering changed YAML whitespace: %q", actual)
			}
		})
	}
}
