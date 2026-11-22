package gerrit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitReviewFieldsUseCommandOutput(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".gitreview"), []byte("[gerrit]\nhost=gerrit.example.org\nport=29418\nproject=test/repo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]string{"project": "test/repo\n", "host": "gerrit.example.org\n", "port": "29418\n"} {
		t.Run(field, func(t *testing.T) {
			cmd := NewGerritDotGitReviewCmd()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{field})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if stdout.String() != want || stderr.Len() != 0 {
				t.Fatalf("stdout %q stderr %q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestGitReviewMissingReturnsError(t *testing.T) {
	t.Chdir(t.TempDir())
	cmd := NewGerritDotGitReviewCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"project"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), ".gitreview") {
		t.Fatalf("expected contextual error, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("error polluted stdout: %q", stdout.String())
	}
}
