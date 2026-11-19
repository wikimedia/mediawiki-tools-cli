package cmd

import (
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/timers"
)

func TestCLIProcess(t *testing.T) {
	// Child entry point with the same process startup as the installed command.
	if len(os.Args) > 3 && os.Args[2] == "--" {
		os.Args = append([]string{"mw"}, os.Args[3:]...)
		Execute("", "", "", "", "", "latest")
		os.Exit(0)
	}
}

func cliTestProcess(t *testing.T, ctx context.Context, configDir string, args ...string) *osexec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	process := osexec.CommandContext(ctx, executable, append([]string{"-test.run=^TestCLIProcess$", "--"}, args...)...)
	process.Env = append(os.Environ(), "GITLAB_CI=true", "CI_PROJECT_DIR="+configDir)
	return process
}

func TestMCPProcessAnonymousAndDeferred(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Even invalid saved config must be irrelevant to the anonymous server.
	if err := os.WriteFile(path, []byte("invalid JSON"), 0o600); err != nil {
		t.Fatal(err)
	}

	process := cliTestProcess(t, ctx, dir, "--public-only", "mcp", "--services", "wiki", "--deferred-services", "phab")
	host := mcp.NewClient(&mcp.Implementation{Name: "startup-test", Version: "1"}, nil)
	session, err := host.Connect(ctx, &mcp.CommandTransport{Command: process}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	list := func() []string {
		t.Helper()
		result, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, tool := range result.Tools {
			names = append(names, tool.Name)
		}
		return names
	}
	if tools := list(); !slices.Contains(tools, "wiki_page_info") || slices.Contains(tools, "phabricator_get_task") {
		t.Fatalf("unexpected initial tools: %v", tools)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "mw_enable_service", Arguments: map[string]any{"service": "phab"}})
	if err != nil || result.IsError {
		t.Fatalf("activation failed: %v, %v", result, err)
	}
	if tools := list(); !slices.Contains(tools, "phabricator_get_task") || slices.Contains(tools, "phabricator_get_paste") || slices.Contains(tools, "gitlab_get_diff") {
		t.Fatalf("unexpected tools after activation: %v", tools)
	}
}

func TestMCPProcessRequiresExplicitPasteProfile(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"mcp", "--services", "phab", "--allow-paste"},
		{"phab", "mcp", "--allow-paste"},
		{"mcp", "--services", "phab", "--profile", "phab=missing"},
	} {
		output, err := cliTestProcess(t, ctx, dir, args...).CombinedOutput()
		if err == nil || !strings.Contains(string(output), "profile") {
			t.Fatalf("%v: expected profile error, got %s (%v)", args, output, err)
		}
	}
}

func TestPhabricatorPreviewProcessSkipsStartup(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	before := `{"telemetry":"yes"}`
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := cliTestProcess(t, ctx, dir, "phab", "task", "status", "T123", "resolved", "--dry-run=true").CombinedOutput()
	if err != nil || !strings.Contains(string(output), "Preview only") {
		t.Fatalf("preview failed: %s (%v)", output, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != before {
		t.Fatalf("preview modified configuration: %q (%v)", after, err)
	}
}

func TestGlobalPublicOnlyRejectsUnsupportedProcessCommands(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	for _, test := range []struct {
		name     string
		settings string
		args     []string
	}{
		{"flag before service", "", []string{"--public-only", "wiki", "api", "query"}},
		{"flag after command", "", []string{"phab", "api", "maniphest.search", "--public-only"}},
		{"config default", `,"access":{"public_only":true}`, []string{"wiki", "api", "query"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			settings := `{"telemetry":"no","timer_last_update_checked":"` + timers.String(timers.NowUTC()) + `"` + test.settings + `}`
			if err := os.WriteFile(path, []byte(settings), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := cliTestProcess(t, ctx, dir, test.args...).CombinedOutput()
			if err == nil || !strings.Contains(string(output), "does not support --public-only") {
				t.Fatalf("%v: expected public-only error, got %s (%v)", test.args, output, err)
			}
		})
	}
}
