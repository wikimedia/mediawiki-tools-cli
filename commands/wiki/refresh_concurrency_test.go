package wiki

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/knadh/koanf"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func TestWikiRefreshProcess(t *testing.T) {
	directory := os.Getenv("MWCLI_TEST_WIKI_CONFIG_DIR")
	if directory == "" {
		return
	}
	t.Setenv("CI_PROJECT_DIR", directory)
	a, err := config.LoadWikiProfile("fixture", "personal")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("ready")
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	tokens, err := accountTokens(context.Background(), a, false)
	mode := os.Getenv("MWCLI_TEST_WIKI_REFRESH_MODE")
	if mode == "removed" {
		if err == nil {
			t.Fatal("removed profile was refreshed")
		}
	} else {
		if err != nil {
			t.Fatal(err)
		}
		expected := "refreshed-fixture"
		if mode == "new-login" {
			expected = "new-login-fixture"
		}
		if tokens.AccessToken != expected {
			t.Fatal("stale OAuth bundle used")
		}
	}
}

type wikiRefreshProcess struct {
	command     *exec.Cmd
	input       io.WriteCloser
	output      *bufio.Scanner
	diagnostics bytes.Buffer
}

func startWikiRefreshProcess(t *testing.T, mode string) *wikiRefreshProcess {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	process := &wikiRefreshProcess{command: exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWikiRefreshProcess$")}
	process.command.Env = append(os.Environ(), "MWCLI_TEST_WIKI_CONFIG_DIR="+os.Getenv("CI_PROJECT_DIR"), "MWCLI_TEST_WIKI_REFRESH_MODE="+mode)
	var err error
	process.input, err = process.command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := process.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	process.output = bufio.NewScanner(output)
	process.command.Stderr = &process.diagnostics
	if err := process.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = process.command.Process.Kill() })
	if !process.output.Scan() || process.output.Text() != "ready" {
		t.Fatalf("refresh helper not ready: %s", process.diagnostics.String())
	}
	return process
}

func TestOAuthRefreshAcrossProcesses(t *testing.T) {
	for _, mode := range []string{"parallel", "new-login", "removed"} {
		t.Run(mode, func(t *testing.T) {
			_ = NewWikiCmd() // Reset flag-backed selectors left by other command tests.
			isolateWikiProfiles(t)
			var refreshes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/w/rest.php/oauth2/access_token" {
					t.Errorf("unexpected OAuth destination %s", r.URL.Path)
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Form.Get("refresh_token") != "old-refresh-fixture" {
					t.Error("unexpected refresh token")
				}
				if refreshes.Add(1) != 1 {
					t.Error("refresh token was used more than once")
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"access_token":"refreshed-fixture","refresh_token":"rotated-fixture","token_type":"Bearer","expires_in":3600}`)
			}))
			defer server.Close()
			api := server.URL + "/w/api.php"
			a := config.WikiProfile{Realm: "fixture", Name: "personal", Username: "Fixture", Identity: "Fixture", Method: "oauth", Endpoint: api, Wiki: api, ClientID: "fixture-client", Sites: []string{api}}
			tokens := wikiauth.Tokens{AccessToken: "expired-fixture", RefreshToken: "old-refresh-fixture", TokenType: "Bearer", ExpiresAt: time.Now().Add(-time.Hour)}
			encoded, _ := json.Marshal(tokens)
			metadata := map[string]any{}
			for key, value := range a.Metadata() {
				metadata[key] = value
			}
			metadata["wiki.profiles.fixture.personal.sites"] = a.Sites
			if err := a.Credential().SaveWithProviderMetadata(string(encoded), false, metadata, "plaintext"); err != nil {
				t.Fatal(err)
			}
			processes := []*wikiRefreshProcess{startWikiRefreshProcess(t, mode)}
			switch mode {
			case "parallel":
				processes = append(processes, startWikiRefreshProcess(t, mode))
			case "new-login":
				tokens.AccessToken = "new-login-fixture"
				tokens.RefreshToken = "new-refresh-fixture"
				tokens.ExpiresAt = time.Now().Add(time.Hour)
				encoded, _ = json.Marshal(tokens)
				if err := a.Credential().SaveWithProvider(string(encoded), false, nil, ""); err != nil {
					t.Fatal(err)
				}
			case "removed":
				if _, err := (wikiProfiles{}).Remove("fixture/personal"); err != nil {
					t.Fatal(err)
				}
			}
			for _, process := range processes {
				if _, err := fmt.Fprintln(process.input, "continue"); err != nil {
					t.Fatal(err)
				}
				_ = process.input.Close()
			}
			for _, process := range processes {
				var output strings.Builder
				for process.output.Scan() {
					output.WriteString(process.output.Text())
					output.WriteByte('\n')
				}
				if err := process.command.Wait(); err != nil {
					t.Fatalf("refresh helper failed: %v; %s%s", err, output.String(), process.diagnostics.String())
				}
			}
			expected := int32(0)
			if mode == "parallel" {
				expected = 1
			}
			if refreshes.Load() != expected {
				t.Fatalf("refreshes=%d, want %d", refreshes.Load(), expected)
			}
			if err := config.UpdateDiskConfig(func(_ *koanf.Koanf) error { return nil }); err != nil {
				t.Fatal(err)
			}
			if mode == "removed" {
				if config.HasCredential(a.Credential().Key) || config.State().OnDiskKoanf.Exists("wiki.profiles.fixture.personal") {
					t.Fatal("refresh resurrected removed profile")
				}
			} else {
				value, err := a.Credential().Read()
				if err != nil {
					t.Fatal(err)
				}
				var saved wikiauth.Tokens
				if err := json.Unmarshal([]byte(value), &saved); err != nil {
					t.Fatal(err)
				}
				expected := "refreshed-fixture"
				if mode == "new-login" {
					expected = "new-login-fixture"
				}
				if saved.AccessToken != expected {
					t.Fatal("stored OAuth bundle is stale")
				}
			}
		})
	}
}
