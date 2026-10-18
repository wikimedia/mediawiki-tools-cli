package gitlab

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReleaseMetadataReusesUploadedFiles(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("shell unavailable")
	}
	source, err := os.ReadFile("../../.gitlab/pipelines/commit.yml")
	if err != nil {
		t.Fatal(err)
	}
	var pipeline map[string]interface{}
	if err := yaml.Unmarshal(source, &pipeline); err != nil {
		t.Fatal(err)
	}
	job := pipeline["release-publish"].(map[string]interface{})
	loop := ""
	for _, script := range job["script"].([]interface{}) {
		if text, ok := script.(string); ok && strings.Contains(text, "for release_path") {
			loop = text
		}
	}
	if loop == "" {
		t.Fatal("release asset loop missing")
	}
	for _, prefix := range []string{"mw", "wm"} {
		t.Run(prefix, func(t *testing.T) {
			dir := t.TempDir()
			releaseDir := filepath.Join(dir, "_release", "1.2.3")
			if err := os.MkdirAll(releaseDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, file := range []string{prefix + "_v1.2.3_linux_amd64", prefix + "_v1.2.3_linux_amd64.sha256", "LICENSE"} {
				if err := os.WriteFile(filepath.Join(releaseDir, file), []byte("fixture"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			script := "set -eu\nassets_links=\"\"\n" + loop + "\nprintf '%s\\n' ${assets_links}\n"
			cmd := exec.Command(shell, "-c", script)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "CI_API_V4_URL=https://gitlab.example/api/v4", "CI_PROJECT_ID=16", "CI_PROJECT_URL=https://gitlab.example/group/project", "CI_COMMIT_TAG=v1.2.3")
			data, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("asset loop: %v: %s", err, data)
			}
			links := map[string]struct {
				URL             string
				DirectAssetPath string `json:"direct_asset_path"`
			}{}
			urls := map[string]bool{}
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				if line == "--assets-link" {
					continue
				}
				var link struct {
					Name, URL       string
					DirectAssetPath string `json:"direct_asset_path"`
				}
				if err := json.Unmarshal([]byte(line), &link); err != nil {
					t.Fatalf("bad asset JSON %q: %v", line, err)
				}
				if urls[link.URL] {
					t.Fatal("duplicate URL would be rejected by GitLab")
				}
				urls[link.URL] = true
				links[link.Name] = struct {
					URL             string
					DirectAssetPath string `json:"direct_asset_path"`
				}{link.URL, link.DirectAssetPath}
			}
			want := 3
			if prefix == "wm" {
				want = 5
			}
			if len(links) != want {
				t.Fatalf("got %d links want %d: %s", len(links), want, data)
			}
			legacy := links["mw_v1.2.3_linux_amd64"]
			if legacy.URL == "" {
				t.Fatal("old updaters cannot find mw asset")
			}
			if prefix == "wm" {
				if legacy.URL != "https://gitlab.example/group/project/-/releases/v1.2.3/downloads/binaries/wm_v1.2.3_linux_amd64" {
					t.Fatalf("legacy link does not target canonical download: %s", legacy.URL)
				}
				if links["wm_v1.2.3_linux_amd64"].DirectAssetPath != "/binaries/wm_v1.2.3_linux_amd64" {
					t.Fatal("canonical download path missing")
				}
			}
			files, err := os.ReadDir(releaseDir)
			if err != nil || len(files) != 3 {
				t.Fatal("metadata generation duplicated release files")
			}
		})
	}
}
