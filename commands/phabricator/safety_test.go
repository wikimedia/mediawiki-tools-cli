package phabricator

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestPhabricatorMutationSafetyCoverage(t *testing.T) {
	previews := 0
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		if cmd.Runnable() {
			hints, err := commandmeta.Read(cmd)
			if err != nil {
				t.Fatal(err)
			}
			if !hints.ReadOnlyHint {
				unsupported := hints.SecretInput || cmd.Name() == "shell" || reflect.DeepEqual(hints.DataAccess, []commandmeta.DataAccess{commandmeta.DataUnknown})
				if unsupported {
					if cmd.Flags().Lookup("dry-run") != nil {
						t.Errorf("%s unexpectedly advertises a safe preview", cmd.CommandPath())
					}
				} else {
					previews++
					for _, flag := range []string{"yes", "dry-run"} {
						if cmd.Flags().Lookup(flag) == nil {
							t.Errorf("%s missing --%s", cmd.CommandPath(), flag)
						}
					}
				}
			}
		}
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(NewPhabricatorCmd())
	if previews != 17 {
		t.Fatalf("preview coverage changed: %d", previews)
	}
}

func forbidPhabricatorHTTP(t *testing.T) {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
		t.Errorf("unexpected network request: %s", r.URL)
		return nil, fmt.Errorf("unexpected network request")
	})
	t.Cleanup(func() { http.DefaultTransport = previous })
}

func executeSafetyCommand(args ...string) (string, error) {
	cmd := NewPhabricatorCmd()
	// The production root supplies this flag.
	cmd.PersistentFlags().Bool("no-interaction", false, "Do not prompt")
	var buffer bytes.Buffer
	cmd.SetOut(&buffer)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buffer.String(), err
}

func phabricatorMutationInvocations() [][]string {
	return [][]string{
		{"assign", "T123", "me"},
		{"task", "assign", "T123", "me"},
		{"mv", "T123", "Backlog", "--project", "fixture"},
		{"task", "move", "T123", "Backlog", "--project", "fixture"},
		{"task", "mv", "T123", "Backlog"},
		{"rmtag", "T123", "fixture"},
		{"task", "rmtag", "T123", "fixture"},
		{"setprio", "T123", "high"},
		{"task", "priority", "T123", "high"},
		{"setstatus", "T123", "resolved"},
		{"task", "status", "T123", "resolved"},
		{"create", "--title", "fixture"},
		{"task", "create", "--title", "fixture"},
		{"task", "edit", "T123", "--title", "fixture"},
		{"paste", "create", "--title", "fixture", "--file", "does-not-exist"},
		{"paste", "edit", "P123", "--title", "fixture"},
		{"auth", "default", "--profile", "fixture"},
		{"auth", "remove", "--profile", "fixture"},
	}
}

func TestAgentBlocksMutationsBeforeInputCredentialsOrNetwork(t *testing.T) {
	isolatedAuthConfig(t)
	forbidPhabricatorHTTP(t)
	t.Setenv("AI_AGENT", "fixture")
	invocations := phabricatorMutationInvocations()
	invocations = append(invocations,
		[]string{"auth", "login", "--method", "phab-cfg", "--config-file", "does-not-exist"},
		[]string{"api", "maniphest.search", "--input", "does-not-exist"},
		[]string{"shell"})
	for _, args := range invocations {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			args = append(append([]string{}, args...), "--agent=false", "--public-only=false")
			output, err := executeSafetyCommand(args...)
			if err == nil || !strings.Contains(err.Error(), "human approval required") || output != "" {
				t.Fatalf("output=%q error=%v", output, err)
			}
		})
	}
}

func TestAgentMutationPreviewsAreOffline(t *testing.T) {
	isolatedAuthConfig(t)
	forbidPhabricatorHTTP(t)
	for key, value := range map[string]string{
		"phabricator.profiles.fixture.username": "fixture",
		"phabricator.profiles.fixture.url":      defaultPhabricatorURL,
	} {
		if err := mwconfig.PutKeyValueOnDisk(key, value); err != nil {
			t.Fatal(err)
		}
	}
	before := mwconfig.State().OnDiskKoanf.Sprint()
	for _, args := range phabricatorMutationInvocations() {
		if args[0] == "paste" && args[1] == "create" {
			// Covered below with supplied stdin instead of a nonexistent file.
			continue
		}
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			args = append(append([]string{}, args...), "--agent", "--dry-run", "--no-interaction")
			output, err := executeSafetyCommand(args...)
			if err != nil || !strings.Contains(output, "Preview only") {
				t.Fatalf("output=%q error=%v", output, err)
			}
		})
	}
	cmd := NewPhabricatorCmd()
	cmd.SetIn(strings.NewReader("supplied paste"))
	cmd.SetOut(io.Discard)
	cmd.SetArgs([]string{"paste", "create", "--title", "fixture", "--agent", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := mwconfig.State().OnDiskKoanf.Sprint(); got != before {
		t.Fatal("preview changed config")
	}
}

func TestHumanMutationDryRunValidates(t *testing.T) {
	isolatedAuthConfig(t)
	forbidPhabricatorHTTP(t)
	for _, args := range [][]string{
		{"assign", "P123", "me"},
		{"assign", "T123", ""},
		{"setprio", "T123", "nonsense"},
		{"setstatus", "T123", "nonsense"},
	} {
		if _, err := executeSafetyCommand(append(args, "--dry-run")...); err == nil {
			t.Fatalf("invalid preview accepted: %v", args)
		}
	}
	for _, flags := range [][]string{{"--agent"}, {"--agent", "--dry-run=false", "--yes=false"}} {
		_, err := executeSafetyCommand(append([]string{"task", "status", "T123", "resolved"}, flags...)...)
		if err == nil || !strings.Contains(err.Error(), "human approval required") {
			t.Fatal(err)
		}
	}
}

func TestHumanTargetedTaskMutationRunsOnceWithoutConfirmation(t *testing.T) {
	isolatedAuthConfig(t)
	ran := 0
	cmd := taskMutationCommand(&cobra.Command{
		Use: "assign <task> <user>", Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			ran++
			if !reflect.DeepEqual(args, []string{"T123", "me"}) {
				t.Fatal(args)
			}
			return nil
		},
	}, nil)
	cmd.SetOut(io.Discard)
	cmd.Flags().Bool("no-interaction", false, "Do not prompt")
	cmd.SetArgs([]string{"T123", "me", "--no-interaction"})
	if err := cmd.Execute(); err != nil || ran != 1 {
		t.Fatalf("ran=%d error=%v", ran, err)
	}
}

func TestHumanWritesConfirmPublishingInNoninteractiveMode(t *testing.T) {
	isolatedAuthConfig(t)
	mockLoginIdentity(t)
	login := NewPhabricatorCmd()
	login.SetOut(io.Discard)
	login.SetIn(strings.NewReader("fixture-secret\n"))
	login.SetArgs([]string{"auth", "login", "--profile", "fixture", "--method", "token", "--token-stdin", "--provider", "keyring"})
	if err := login.Execute(); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"phabricator.profiles.fixture.default_project": "fixture",
		"phabricator.profiles.fixture.cache_path":      t.TempDir(),
	} {
		if err := mwconfig.PutKeyValueOnDisk(key, value); err != nil {
			t.Fatal(err)
		}
	}
	previous := cli.Opts.NoInteraction
	cli.Opts.NoInteraction = true
	t.Cleanup(func() { cli.Opts.NoInteraction = previous })
	writes := 0
	http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
		var result string
		switch r.URL.Path {
		case "/api/maniphest.search":
			result = accessPage(t, accessTask(123, "public", wikimediaPublicSpace))
		case "/api/maniphest.edit", "/api/paste.edit":
			writes++
			result = `{"object":{"id":123}}`
		case "/api/user.whoami":
			result = `{"userName":"fixture"}`
		case "/api/user.search":
			result = `{"data":[{"phid":"PHID-USER-fixture","fields":{"username":"fixture"}}]}`
		case "/api/project.search":
			result = `{"data":[{"id":1,"phid":"PHID-PROJ-fixture","fields":{"name":"fixture"}}]}`
		case "/api/project.column.search":
			result = `{"data":[{"id":1,"phid":"PHID-PCOL-fixture","fields":{"name":"Backlog"}}]}`
		default:
			t.Errorf("unexpected request %s", r.URL)
			return nil, fmt.Errorf("unexpected request %s", r.URL)
		}
		return pasteResponse(`{"result":` + result + `}`), nil
	})
	for _, args := range phabricatorMutationInvocations() {
		if args[0] == "auth" {
			continue
		}
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			args = append([]string{}, args...)
			if args[0] == "paste" && args[1] == "create" {
				args[len(args)-1] = "-"
			}
			before := writes
			run := func(flags ...string) error {
				cmd := NewPhabricatorCmd()
				cmd.PersistentFlags().Bool("no-interaction", false, "Do not prompt")
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
				cmd.SetIn(strings.NewReader("supplied content"))
				cmd.SetArgs(append(append([]string{}, args...), flags...))
				return cmd.Execute()
			}
			flags := []string{"--profile", "fixture", "--no-interaction"}
			if args[0] == "create" || args[0] == "paste" || (args[0] == "task" && args[1] == "create") {
				if err := run(flags...); err == nil || !strings.Contains(err.Error(), "--yes") || writes != before {
					t.Fatalf("unconfirmed operation: writes=%d want=%d error=%v", writes, before, err)
				}
				flags = append(flags, "--yes")
			}
			if err := run(flags...); err != nil || writes != before+1 {
				t.Fatalf("writes=%d want=%d error=%v", writes, before+1, err)
			}
		})
	}
}

func TestAgentReadOnlyMetadataAndParentSetupStillWork(t *testing.T) {
	isolatedAuthConfig(t)
	t.Setenv("AI_AGENT", "fixture")
	parentRuns := 0
	root := &cobra.Command{Use: "mw", PersistentPreRun: func(*cobra.Command, []string) { parentRuns++ }}
	root.AddGroup(&cobra.Group{ID: "service", Title: "Services"})
	root.AddCommand(NewPhabricatorCmd())
	var buffer bytes.Buffer
	root.SetOut(&buffer)
	root.SetArgs([]string{"phab", "auth", "profiles"})
	if err := root.Execute(); err != nil || parentRuns != 1 {
		t.Fatalf("parentRuns=%d error=%v", parentRuns, err)
	}
}

func TestProfileMutationConfirmationAndTarget(t *testing.T) {
	isolatedAuthConfig(t)
	forbidPhabricatorHTTP(t)
	for _, name := range []string{"fixture", "other"} {
		if err := mwconfig.PutKeyValueOnDisk("phabricator.profiles."+name+".username", name); err != nil {
			t.Fatal(err)
		}
	}
	for _, operation := range []string{"remove"} {
		before := mwconfig.State().OnDiskKoanf.Sprint()
		if _, err := executeSafetyCommand("auth", operation, "--profile", "fixture", "--no-interaction"); err == nil || !strings.Contains(err.Error(), "--yes") {
			t.Fatalf("%s error=%v", operation, err)
		}
		if mwconfig.State().OnDiskKoanf.Sprint() != before {
			t.Fatal("unconfirmed operation changed profiles")
		}
	}
	if _, err := executeSafetyCommand("auth", "default", "--profile", "fixture", "--no-interaction"); err != nil {
		t.Fatal(err)
	}
	if got := phabProfiles.Default(); got != "fixture" {
		t.Fatalf("default=%q", got)
	}
	if _, err := executeSafetyCommand("auth", "default", "--dry-run"); err == nil || !strings.Contains(err.Error(), "--profile") {
		t.Fatalf("implicit default accepted: %v", err)
	}
}

func TestExplicitAgentBlocksEscapeHatchesAndDryRunFalse(t *testing.T) {
	isolatedAuthConfig(t)
	forbidPhabricatorHTTP(t)
	for _, args := range [][]string{
		{"auth", "login", "--token", "do-not-read"},
		{"api", "conduit.ping"},
		{"shell"},
		{"task", "status", "T123", "resolved", "--yes=false", "--dry-run=false"},
	} {
		_, err := executeSafetyCommand(append(args, "--agent")...)
		if err == nil || !strings.Contains(err.Error(), "human approval required") {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if _, err := executeSafetyCommand("shell", "--no-interaction"); err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("noninteractive shell accepted: %v", err)
	}
}
