package phabricator

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

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

func TestPhabricatorWritesInNoninteractiveMode(t *testing.T) {
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
	t.Setenv("AI_AGENT", "fixture")
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
			if args[0] == "create" || args[0] == "paste" || (args[0] == "task" && (args[1] == "create" || args[1] == "edit")) {
				if err := run(flags...); err == nil || !strings.Contains(err.Error(), "--yes") || writes != before {
					t.Fatalf("unconfirmed operation: writes=%d want=%d error=%v", writes, before, err)
				}
				flags = append(flags, "--yes")
			}
			if args[0] == "mv" || (args[0] == "task" && (args[1] == "move" || args[1] == "mv")) {
				preview, err := executeSafetyCommand(append(append([]string{}, args...), "--profile", "fixture", "--dry-run")...)
				if err != nil || writes != before || !strings.Contains(preview, `Would move T123 (fixture-title-123) to "Backlog" on fixture (PHID-PROJ-fixture), column PHID-PCOL-fixture`) {
					t.Fatalf("move preview: %q, writes=%d, error=%v", preview, writes, err)
				}
			}
			if err := run(flags...); err != nil || writes != before+1 {
				t.Fatalf("writes=%d want=%d error=%v", writes, before+1, err)
			}
		})
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
	if _, err := executeSafetyCommand("auth", "default"); err == nil || !strings.Contains(err.Error(), "--profile") {
		t.Fatalf("implicit default accepted: %v", err)
	}
}
