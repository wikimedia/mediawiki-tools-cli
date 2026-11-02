package dev

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv/recipe"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
)

func TestRecipeNamedCheckoutUsesProgressWriter(t *testing.T) {
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	spec := recipe.Spec{}
	spec.Source.GerritInteractionType = "http"
	spec.Code.Core = true
	var progress bytes.Buffer
	if err := applyCodeCheckout(spec, mediawiki.MediaWiki(t.TempDir()), true, &progress); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Cloning repositories", "Would clone", "Repositories cloned"} {
		if !strings.Contains(progress.String(), want) {
			t.Fatalf("missing %q from recipe progress: %q", want, progress.String())
		}
	}
}

func TestApplyContentWritesProgressToProvidedWriter(t *testing.T) {
	previousContext := devenv.Context
	devenv.Context = "recipe-stream-test"
	t.Cleanup(func() { devenv.Context = previousContext })
	m := devenv.Environment{BaseDir: t.TempDir()}
	var stderr bytes.Buffer
	content := recipe.Content{Pages: []recipe.PageContent{{Title: "Example", Wiki: "default", Text: "test"}}}
	if err := applyContent(m, nil, content, true, &stderr); err != nil {
		t.Fatal(err)
	}
	if got, want := stderr.String(), "[dry-run] create page Example on default: test\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestExpandRecipeTemplateVars_ReplacesPortAcrossRelevantSections(t *testing.T) {
	spec := recipe.Spec{
		Name: "wikibase-repoclient",
		LocalSettings: recipe.LocalSettings{
			Files: recipe.LocalSettingsFiles{
				Shared: []recipe.LocalSettingsFile{
					{Content: "$wgWBClientSettings['repoUrl'] = 'http://default.mediawiki.local.wmftest.net:${PORT}';"},
				},
			},
		},
		Maintenance: []recipe.ContainerCommandStep{
			{
				Name:    "add-site",
				Command: []string{"php", "maintenance/run.php", "addSite.php", "--pagepath", "http://default.mediawiki.local.wmftest.net:${PORT}/w/index.php?title=$1"},
			},
		},
	}

	expanded := expandRecipeTemplateVars(spec, map[string]string{"PORT": "19090"})

	shared := expanded.LocalSettings.Files.Shared[0].Content
	if shared != "$wgWBClientSettings['repoUrl'] = 'http://default.mediawiki.local.wmftest.net:19090';" {
		t.Fatalf("unexpected expanded shared content: %q", shared)
	}

	cmdArg := expanded.Maintenance[0].Command[4]
	if cmdArg != "http://default.mediawiki.local.wmftest.net:19090/w/index.php?title=$1" {
		t.Fatalf("unexpected expanded maintenance arg: %q", cmdArg)
	}
}

func TestExpandRecipeEnvVars_UnknownVariableIsPreserved(t *testing.T) {
	got := expandRecipeEnvVars("http://example:${UNKNOWN}/x", map[string]string{"PORT": "1234"})
	want := "http://example:${UNKNOWN}/x"
	if got != want {
		t.Fatalf("expandRecipeEnvVars() = %q, want %q", got, want)
	}
}

func TestWaitForSiteURL_EventuallyReady(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	if err := waitForSiteURL(io.Discard, "client", server.URL, client, 5, 0); err != nil {
		t.Fatalf("waitForSiteURL() unexpected error: %v", err)
	}

	if attempts != 3 {
		t.Fatalf("waitForSiteURL() attempts = %d, want 3", attempts)
	}
}

func TestWaitForSiteURL_TimeoutIncludesLastStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := server.Client()
	err := waitForSiteURL(io.Discard, "client", server.URL, client, 3, 0)
	if err == nil {
		t.Fatal("waitForSiteURL() error = nil, want timeout error")
	}

	if !strings.Contains(err.Error(), "timed out waiting for client site to respond after 3 attempts") {
		t.Fatalf("waitForSiteURL() error = %q, want attempts detail", err.Error())
	}
	if !strings.Contains(err.Error(), "last HTTP status: 503") {
		t.Fatalf("waitForSiteURL() error = %q, want last status detail", err.Error())
	}
}
