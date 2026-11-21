package gerrit

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func runGerritReadTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := NewGerritCmd()
	accesspolicy.AddFlag(cmd)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return out.String(), err
}

func TestGerritListUsesSelectedProfileOnlyForPersonalQueues(t *testing.T) {
	setupAuthTest(t)
	prefix := gerritProfiles.Prefix("reviewer")
	credential := config.Credential{Key: prefix + ".password", Service: "gerrit", Endpoint: gerritProfiles.Endpoint, Account: "fixture"}
	if err := credential.SaveWithProvider("fixture-secret", false, map[string]string{prefix + ".username": "fixture"}, "keyring"); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		args, path, query string
		authenticated     bool
	}{
		{"--mine", "/r/a/changes/", "status:open owner:self", true},
		{"--reviewing", "/r/a/changes/", "status:open reviewer:self", true},
		{"status:merged", "/r/changes/", "status:merged", false},
	} {
		http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
			user, secret, hasAuth := r.BasicAuth()
			if r.URL.Path != tt.path || r.URL.Query().Get("q") != tt.query || hasAuth != tt.authenticated ||
				(hasAuth && (user != "fixture" || secret != "fixture-secret")) {
				t.Errorf("unexpected request path, query or auth: %s %t", r.URL, hasAuth)
			}
			return reviewResponse(r, `[{"_number":42,"project":"test/example","branch":"main","status":"NEW","subject":"Review fixture"}]`)
		})
		out, err := runGerritReadTest(t, "list", "--profile", "reviewer", tt.args)
		if err != nil || !strings.Contains(out, "Review fixture") {
			t.Fatalf("list %s failed: %v\n%s", tt.args, err, out)
		}
	}
	http.DefaultTransport = transportFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("public-only personal query contacted Gerrit")
		return nil, nil
	})
	if _, err := runGerritReadTest(t, "list", "--mine", "--public-only"); err == nil {
		t.Fatal("personal queue should be rejected in public-only mode")
	}
}

func TestGerritViewAndDiffUseSelectedPatchsets(t *testing.T) {
	setupAuthTest(t)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	repository := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		result, err := gitOutput(t.Context(), repository, args...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return result
	}
	git("init", "--quiet")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@example.test")
	git("commit", "--quiet", "--allow-empty", "-m", "base")
	write := func(value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repository, "example.txt"), []byte(value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		git("add", "example.txt")
	}
	write("first patchset")
	git("commit", "--quiet", "-m", "proposed change")
	older := git("rev-parse", "HEAD")
	git("update-ref", "refs/changes/42/42/1", older)
	write("second patchset")
	git("commit", "--quiet", "--amend", "--no-edit")
	latest := git("rev-parse", "HEAD")
	git("update-ref", "refs/changes/42/42/2", latest)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url."+repository+".insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", "https::https://gerrit.wikimedia.org/r/test/example")
	t.Chdir(repository)
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "" {
			t.Error("public review sent credentials")
		}
		switch strings.TrimSuffix(r.URL.Path, "/") {
		case "/r/changes":
			if r.URL.Query().Get("q") != "commit:"+latest {
				t.Errorf("unexpected HEAD query: %s", r.URL)
			}
			return reviewResponse(r, `[{"_number":42}]`)
		case "/r/changes/42/detail":
			body := fmt.Sprintf(`{
				"_number":42,"project":"test/example","branch":"main","status":"NEW","subject":"Review fixture",
				"owner":{"name":"Author"},"current_revision":%q,
				"revisions":{%q:{"_number":1,"ref":"refs/changes/42/42/1"},%q:{"_number":2,"ref":"refs/changes/42/42/2"}},
				"labels":{"Code-Review":{"all":[{"name":"Reviewer","value":1}]}},
				"submit_requirements":[{"name":"Code-Review","status":"UNSATISFIED"}]
			}`, latest, older, latest)
			return reviewResponse(r, body)
		case "/r/changes/42/comments":
			return reviewResponse(r, `{"example.txt":[
				{"id":"a","patch_set":1,"line":1,"message":"Stale question","unresolved":true},
				{"id":"b","patch_set":2,"line":1,"in_reply_to":"a","message":"Resolved question","unresolved":false},
				{"id":"c","patch_set":2,"line":1,"message":"Active question","unresolved":true,"author":{"name":"Reviewer"}}
			]}`)
		default:
			t.Fatalf("unexpected API path: %s", r.URL)
			return nil, nil
		}
	})
	out, err := runGerritReadTest(t, "view", "https://gerrit.wikimedia.org/r/c/test/example/+/42/1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Patchset: 1", "Code-Review: +1 Reviewer", "Code-Review: UNSATISFIED", "Unresolved threads: 1", "Active question"} {
		if !strings.Contains(out, want) {
			t.Errorf("view is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Stale question") {
		t.Fatal("view included a resolved thread")
	}

	out, err = runGerritReadTest(t, "diff", "--base-patchset", "1")
	if err != nil || !strings.Contains(out, "-first patchset\n+second patchset") {
		t.Fatalf("patchset diff failed: %v\n%s", err, out)
	}
	out, err = runGerritReadTest(t, "diff", "42", "--patchset", "1")
	if err != nil || !strings.Contains(out, "+first patchset") || strings.Contains(out, "second patchset") {
		t.Fatalf("diff against parent failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(".git/FETCH_HEAD"); !os.IsNotExist(err) || git("rev-parse", "HEAD") != latest {
		t.Fatal("diff changed the local repository")
	}
}

func reviewResponse(request *http.Request, body string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(")]}'\n" + body)), Request: request}, nil
}
