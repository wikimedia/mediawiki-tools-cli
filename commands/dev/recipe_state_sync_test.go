package dev

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

func TestRecipeJobrunnerFileErrorsAreReturned(t *testing.T) {
	originalContext := devenv.Context
	devenv.Context = "jobrunner-error-test"
	t.Cleanup(func() { devenv.Context = originalContext })
	m := devenv.Environment{BaseDir: t.TempDir()}
	// A directory at the file path fails even when tests run as root.
	if err := os.MkdirAll(filepath.Join(m.Directory(), "mediawiki", "jobrunner-sites"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := applyJobRunnerSites(m, []string{"example"}, false, io.Discard); err == nil || !strings.Contains(err.Error(), "add jobrunner site example") {
		t.Fatalf("expected jobrunner write error, got %v", err)
	}
	if err := saveRecipeRuntimeState(m, recipeRuntimeState{JobRunnerSites: []string{"example"}}); err != nil {
		t.Fatal(err)
	}
	if err := cleanupRecipeRuntimeState(m, false, io.Discard); err == nil || !strings.Contains(err.Error(), "remove jobrunner site example") {
		t.Fatalf("expected jobrunner cleanup error, got %v", err)
	}
	if _, err := os.Stat(recipeRuntimeStatePath(m)); err != nil {
		t.Fatalf("failed cleanup must retain state for retry: %v", err)
	}
}

func TestRecipeRuntimeStateSurvivesSynchronizationAndCleanup(t *testing.T) {
	// No external commands or user config are needed: all operations are scoped
	// to this temporary context, and Docker is deliberately absent from PATH.
	t.Setenv("PATH", t.TempDir())
	originalContext := devenv.Context
	devenv.Context = "recipe-state-test"
	t.Cleanup(func() { devenv.Context = originalContext })
	m := devenv.Environment{BaseDir: t.TempDir()}
	m.EnsureReady()
	m.Env().Set("RECIPE_SETTING", "recipe-value")
	m.Env().Set("UNRELATED_SETTING", "user-value")
	composeFile := filepath.Join(m.Directory(), "custom-recipe.yml")
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sitesFile := filepath.Join(m.Directory(), "mediawiki", "jobrunner-sites")
	if err := os.WriteFile(sitesFile, []byte("recipe-wiki\nother-wiki\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := recipeRuntimeState{
		EnvKeys:        []string{"RECIPE_SETTING"},
		ComposeFiles:   []string{"custom-recipe.yml"},
		JobRunnerSites: []string{"recipe-wiki"},
	}
	if err := saveRecipeRuntimeState(m, state); err != nil {
		t.Fatal(err)
	}
	statePath := recipeRuntimeStatePath(m)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	for cycle := 0; cycle < 3; cycle++ {
		m.EnsureReady()
		after, err := os.ReadFile(statePath)
		if err != nil || !bytes.Equal(after, before) {
			t.Fatalf("sync %d removed or changed recipe state: %q %v", cycle, after, err)
		}
		loaded, err := loadRecipeRuntimeState(m)
		if err != nil || !reflect.DeepEqual(loaded, state) {
			t.Fatalf("sync %d lost recipe records: %#v %v", cycle, loaded, err)
		}
	}
	if err := cleanupRecipeRuntimeState(m, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	if m.Env().Has("RECIPE_SETTING") || m.Env().Get("UNRELATED_SETTING") != "user-value" {
		t.Fatal("recipe cleanup did not remove only its recorded environment settings")
	}
	for _, path := range []string{statePath, composeFile} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("recipe cleanup retained %q: %v", path, err)
		}
	}
	remainingSites, err := os.ReadFile(sitesFile)
	if err != nil || string(remainingSites) != "other-wiki\n" {
		t.Fatalf("recipe cleanup did not preserve unrelated jobrunner sites: %q %v", remainingSites, err)
	}
}
