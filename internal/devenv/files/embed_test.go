package files

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
	"gitlab.wikimedia.org/repos/releng/cli/mount"
)

func TestSyncerIgnoreFiles_KeepCustomComposeFiles(t *testing.T) {
	ignore := syncer("/tmp").IgnoreFiles

	cases := []struct {
		name     string
		path     string
		expected bool
	}{
		{name: "custom yml", path: "custom.yml", expected: true},
		{name: "custom yaml", path: "custom.yaml", expected: true},
		{name: "custom dashed", path: "custom-two.yml", expected: true},
		{name: "custom dashed multi", path: "custom-two-extra.yaml", expected: true},
		{name: "custom dotted", path: "custom.local.yml", expected: true},
		{name: "custom mixed", path: "custom.local-dev_1.yaml", expected: true},
		{name: "custom dockerfile override", path: "custom-dockerfile-mediawiki.yml", expected: true},
		{name: "Dockerfile.service", path: "Dockerfile.mediawiki", expected: true},
		{name: "Dockerfile.dashed-service", path: "Dockerfile.shellbox-media", expected: true},
		{name: "bare Dockerfile not preserved", path: "Dockerfile", expected: false},
		{name: "non custom", path: "compose/base/compose.yml", expected: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stringsutil.StringInRegexSlice(tc.path, ignore)
			if got != tc.expected {
				t.Fatalf("StringInRegexSlice(%q, IgnoreFiles) = %v, want %v", tc.path, got, tc.expected)
			}
		})
	}
}

func TestEnsureReadyCreatesJobRunnerSitesFile(t *testing.T) {
	projectDir := t.TempDir()

	EnsureReady(projectDir)

	jobRunnerSitesPath := filepath.Join(projectDir, "mediawiki", "jobrunner-sites")
	info, err := os.Stat(jobRunnerSitesPath)
	if err != nil {
		t.Fatalf("Stat(%q): %v", jobRunnerSitesPath, err)
	}
	if info.IsDir() {
		t.Fatalf("%q is a directory, want file", jobRunnerSitesPath)
	}
}

func TestEnsureReadyPreservesLegacyProxyMounts(t *testing.T) {
	projectDir := t.TempDir()
	path := filepath.Join(projectDir, "compose", "base", "mount", "nginx", "timeouts.conf")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	contents := "proxy_read_timeout 120s;\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	EnsureReady(projectDir)
	got, err := os.ReadFile(path)
	if err != nil || string(got) != contents {
		t.Fatalf("legacy bind mount must remain available for rollback: %q %v", got, err)
	}
}

func TestEnsureReadyPreservesRecipeRuntimeState(t *testing.T) {
	for _, initialized := range []bool{false, true} {
		name := "new context"
		if initialized {
			name = "existing context"
		}
		t.Run(name, func(t *testing.T) {
			projectDir := t.TempDir()
			if initialized {
				EnsureReady(projectDir)
			}
			path := filepath.Join(projectDir, ".mwcli-recipe-state.json")
			state := []byte("{\n  \"envKeys\": [\"RECIPE_SETTING\"],\n  \"composeFiles\": [\"custom-recipe.yml\"],\n  \"jobRunnerSites\": [\"recipe-wiki\"]\n}\n")
			if err := os.WriteFile(path, state, 0o600); err != nil {
				t.Fatal(err)
			}
			modified := time.Unix(1234567890, 0)
			if err := os.Chtimes(path, modified, modified); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			for cycle := 0; cycle < 3; cycle++ {
				EnsureReady(projectDir)
				contents, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(contents, state) {
					t.Fatalf("sync %d changed recipe state: contents=%q err=%v", cycle, contents, err)
				}
				after, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if after.Mode() != before.Mode() || !after.ModTime().Equal(before.ModTime()) {
					t.Fatalf("sync %d changed recipe state permissions or modification time", cycle)
				}
			}
		})
	}
}

func TestEnsureReadyStillRemovesStaleAssets(t *testing.T) {
	projectDir := t.TempDir()
	EnsureReady(projectDir)
	relativePaths := []string{
		"obsolete-asset.txt",
		"compose/retired/obsolete.conf",
		".mwcli-recipe-state.json.bak",
		"prefix.mwcli-recipe-state.json",
		"compose/retired/.mwcli-recipe-state.json",
	}
	asset := filepath.Join(projectDir, "compose", "base", "compose.yml")
	expected, err := mount.DevContent.ReadFile("dev/compose/base/compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	for cycle := 0; cycle < 3; cycle++ {
		for _, name := range relativePaths {
			path := filepath.Join(projectDir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("obsolete"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(asset, []byte("outdated embedded asset"), 0o600); err != nil {
			t.Fatal(err)
		}
		EnsureReady(projectDir)
		for _, name := range relativePaths {
			if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(name))); !os.IsNotExist(err) {
				t.Errorf("sync %d retained stale asset %q: %v", cycle, name, err)
			}
		}
		actual, err := os.ReadFile(asset)
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatalf("sync %d did not restore the embedded asset: %v", cycle, err)
		}
	}
}
