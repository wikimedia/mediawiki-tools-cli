package files

import (
	"os"
	"path/filepath"
	"testing"

	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
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
