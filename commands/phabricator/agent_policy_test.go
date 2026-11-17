package phabricator

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestAgentPolicyModesAndCooperativeApproval(t *testing.T) {
	for _, mode := range []string{"permissive", "balanced", "cautious"} {
		t.Run(mode, func(t *testing.T) {
			isolatedAuthConfig(t)
			for key, value := range map[string]string{
				"agent.policy":                            mode,
				"phabricator.profiles.fixture.username":   "fixture",
				"phabricator.profiles.fixture.url":        defaultPhabricatorURL,
				"phabricator.profiles.fixture.key":        "fixture-token",
				"phabricator.profiles.fixture.cache_path": t.TempDir(),
			} {
				if err := mwconfig.PutKeyValueOnDisk(key, value); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("AI_AGENT", "fixture")
			writes, calls := 0, 0
			previous := http.DefaultTransport
			http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				switch r.URL.Path {
				case "/api/maniphest.search":
					return pasteResponse(`{"result":` + accessPage(t, accessTask(123, "public", wikimediaPublicSpace)) + `}`), nil
				case "/api/maniphest.edit":
					writes++
					return pasteResponse(`{"result":{"object":{"id":123}}}`), nil
				case "/api/conduit.ping":
					return pasteResponse(`{"result":"fixture-pong"}`), nil
				default:
					t.Errorf("unexpected request %s", r.URL)
					return nil, fmt.Errorf("unexpected request")
				}
			})
			t.Cleanup(func() { http.DefaultTransport = previous })
			for _, test := range []struct {
				args             []string
				requiresApproval bool
			}{
				{[]string{"auth", "profiles"}, mode == "cautious"},
				{[]string{"auth", "default", "--profile", "fixture"}, mode != "permissive"},
				{[]string{"task", "status", "T123", "resolved", "--profile", "fixture"}, mode != "permissive"},
				{[]string{"api", "conduit.ping", "--profile", "fixture"}, true},
			} {
				beforeCalls, beforeWrites := calls, writes
				output, err := executeSafetyCommand(append(append([]string{}, test.args...), "--no-interaction")...)
				if test.requiresApproval {
					if err == nil || !strings.Contains(err.Error(), "ask the user") || !strings.Contains(err.Error(), "--yes") || output != "" || calls != beforeCalls || writes != beforeWrites {
						t.Fatalf("%v: output=%q calls=%d writes=%d error=%v", test.args, output, calls, writes, err)
					}
				} else if err != nil {
					t.Fatalf("%v: %v", test.args, err)
				}
				output, err = executeSafetyCommand(append(append([]string{}, test.args...), "--yes", "--no-interaction")...)
				if err != nil {
					t.Fatalf("approved %v: output=%q error=%v", test.args, output, err)
				}
				if test.args[0] == "api" && !strings.Contains(output, "fixture-pong") {
					t.Fatalf("approved API did not run: %s", output)
				}
			}
			if writes == 0 {
				t.Fatal("approved public task mutation did not run")
			}
			if _, err := executeSafetyCommand("auth", "remove", "--profile", "fixture", "--yes", "--no-interaction"); err != nil {
				t.Fatal(err)
			}
			if len(phabProfiles.Names()) != 0 {
				t.Fatal("approved profile removal did not run")
			}
		})
	}
}

func TestAgentApprovalPreservesTaskVisibilityChecks(t *testing.T) {
	for _, mode := range []string{"permissive", "balanced", "cautious"} {
		t.Run(mode, func(t *testing.T) {
			isolatedAuthConfig(t)
			for key, value := range map[string]string{
				"agent.policy":                          mode,
				"phabricator.profiles.fixture.username": "fixture",
				"phabricator.profiles.fixture.url":      defaultPhabricatorURL,
				"phabricator.profiles.fixture.key":      "fixture-token",
			} {
				if err := mwconfig.PutKeyValueOnDisk(key, value); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("AI_AGENT", "fixture")
			previous := http.DefaultTransport
			http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/api/maniphest.search" {
					t.Errorf("unexpected request %s", r.URL)
					return nil, fmt.Errorf("unexpected request")
				}
				return pasteResponse(`{"result":` + accessPage(t, accessTask(123, "users", wikimediaPublicSpace)) + `}`), nil
			})
			t.Cleanup(func() { http.DefaultTransport = previous })
			for _, args := range [][]string{
				{"task", "view", "T123"},
				{"task", "status", "T123", "resolved"},
				{"task", "create", "--title", "fixture"},
				{"task", "status", "T123", "resolved", "--allow-non-public"},
			} {
				output, err := executeSafetyCommand(append(args, "--profile", "fixture", "--yes", "--no-interaction")...)
				if err == nil || !strings.Contains(err.Error(), "human approval required") || strings.Contains(output, "fixture-title") {
					t.Fatalf("%v: output=%q error=%v", args, output, err)
				}
			}
		})
	}
}

func TestAgentPolicyInvalidConfigurationFailsExplicitly(t *testing.T) {
	isolatedAuthConfig(t)
	forbidPhabricatorHTTP(t)
	if err := mwconfig.PutKeyValueOnDisk("agent.policy", "balnced"); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{}, {"--agent", "--yes"}, {"--agent", "--dry-run"}} {
		_, err := executeSafetyCommand(append([]string{"task", "status", "T123", "resolved"}, flags...)...)
		if err == nil || !strings.Contains(err.Error(), "agent.policy must be") {
			t.Fatalf("invalid policy silently accepted: %v", err)
		}
	}
}
