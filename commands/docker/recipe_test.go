package docker

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd/recipe"
)

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
	if err := waitForSiteURL("client", server.URL, client, 5, 0); err != nil {
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
	err := waitForSiteURL("client", server.URL, client, 3, 0)
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

func TestListLocalRecipeCompletions_WithoutExtension(t *testing.T) {
	dir := t.TempDir()
	recipesDir := filepath.Join(dir, "recipes")
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	files := []string{"foo.yaml", "bar.yml", "baz.txt"}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(recipesDir, file), []byte("x"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", file, err)
		}
	}

	got, err := listLocalRecipeCompletions(dir, "")
	if err != nil {
		t.Fatalf("listLocalRecipeCompletions() error = %v", err)
	}

	want := []string{"bar", "foo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listLocalRecipeCompletions() = %v, want %v", got, want)
	}
}

func TestListLocalRecipeCompletions_WithExtension(t *testing.T) {
	dir := t.TempDir()
	recipesDir := filepath.Join(dir, "recipes")
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	files := []string{"alpha.yaml", "alpha.yml", "beta.yml"}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(recipesDir, file), []byte("x"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", file, err)
		}
	}

	got, err := listLocalRecipeCompletions(dir, "alpha.")
	if err != nil {
		t.Fatalf("listLocalRecipeCompletions() error = %v", err)
	}

	want := []string{"alpha.yaml", "alpha.yml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listLocalRecipeCompletions() = %v, want %v", got, want)
	}
}

func TestListLocalRecipeCompletions_MissingRecipesDirectory(t *testing.T) {
	dir := t.TempDir()

	got, err := listLocalRecipeCompletions(dir, "")
	if err != nil {
		t.Fatalf("listLocalRecipeCompletions() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("listLocalRecipeCompletions() = %v, want empty slice", got)
	}
}
