package cmd

import (
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func TestCommandTelemetry(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	previousEvents, previousOptions := events, cli.Opts
	effective := config.State().Effective
	previousTelemetry := effective.Telemetry
	t.Cleanup(func() {
		events, cli.Opts = previousEvents, previousOptions
		effective.Telemetry = previousTelemetry
	})

	for _, tc := range []struct {
		name, preference, testContext string
		args                          []string
		want                          string
	}{
		{"nested hooks", "yes", "", []string{"probe", "nested", "run"}, "probe nested run"},
		{"no nested hooks", "yes", "", []string{"probe", "plain"}, "probe plain"},
		{"arguments excluded", "yes", "", []string{"probe", "plain", "private-value"}, "probe plain"},
		{"command opted out", "no", "", []string{"probe", "nested", "run"}, ""},
		{"help opted in", "yes", "", []string{"debug", "events", "--help"}, "debug events --help"},
		{"help opted out", "no", "", []string{"debug", "events", "--help"}, ""},
		{"help unset", "", "", []string{"debug", "events", "--help"}, ""},
		{"gitlab help opted in", "yes", "", []string{"gitlab", "--help"}, "gitlab --help"},
		{"gitlab help opted out", "no", "", []string{"gitlab", "--help"}, ""},
		{"test execution", "yes", "1", []string{"probe", "nested", "run"}, ""},
		{"test help", "yes", "1", []string{"debug", "events", "--help"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("CI_PROJECT_DIR", dir)
			t.Setenv("MWCLI_CONTEXT_TEST", tc.testContext)
			effective.Telemetry = tc.preference
			// Help must read the preference instead of relying on stale global state.
			cli.Opts.Telemetry = true
			events = cli.NewEvents(filepath.Join(dir, ".events"))
			root := NewMwCliCmd()
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			probe := &cobra.Command{Use: "probe", PersistentPreRun: cobrautil.CallAllPersistentPreRun}
			nested := &cobra.Command{Use: "nested", PersistentPreRun: func(c *cobra.Command, args []string) {
				cobrautil.CallAllPersistentPreRun(c, args)
				// Parent setup hooks must still receive their own command.
				if c.Name() != "run" && c.Name() != "nested" {
					t.Errorf("unexpected hook command %q", c.Name())
				}
			}}
			nested.AddCommand(&cobra.Command{Use: "run", Run: func(*cobra.Command, []string) {}})
			probe.AddCommand(nested, &cobra.Command{Use: "plain", Run: func(*cobra.Command, []string) {}})
			root.AddCommand(probe)
			cli.Opts.NoInteraction = true
			root.SetArgs(tc.args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			records := events.RawEvents()
			if tc.want == "" {
				if len(records) != 0 {
					t.Fatalf("unexpected telemetry: %v", records)
				}
				return
			}
			if len(records) != 1 {
				t.Fatalf("got %d events, want one: %v", len(records), records)
			}
			var event struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal([]byte(records[0]), &event); err != nil {
				t.Fatal(err)
			}
			if event.Command != tc.want {
				t.Errorf("command = %q, want %q", event.Command, tc.want)
			}
			if cobrautil.ExecutingCommand(root) != root {
				t.Fatal("executing command leaked beyond setup hooks")
			}
		})
	}
}
