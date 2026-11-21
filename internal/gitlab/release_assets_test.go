package gitlab

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"gopkg.in/yaml.v3"
)

type releaseJob struct{ Script []string }

type releaseJobs struct {
	Upload  releaseJob `yaml:"release-upload"`
	Publish releaseJob `yaml:"release-publish"`
}

func readReleaseJobs(t *testing.T) releaseJobs {
	t.Helper()
	source, err := os.ReadFile("../../.gitlab/pipelines/commit.yml")
	if err != nil {
		t.Fatal(err)
	}
	var pipeline releaseJobs
	if err := yaml.Unmarshal(source, &pipeline); err != nil {
		t.Fatal(err)
	}
	return pipeline
}

func TestReleaseMetadataReusesUploadedFiles(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("shell unavailable")
	}
	pipeline := readReleaseJobs(t)
	loop := ""
	for _, script := range pipeline.Publish.Script {
		if strings.Contains(script, "for release_path") {
			loop = script
		}
	}
	if loop == "" {
		t.Fatal("release asset loop missing")
	}
	for _, prefix := range []string{"mw", "wm", "mixed"} {
		t.Run(prefix, func(t *testing.T) {
			dir := t.TempDir()
			releaseDir := filepath.Join(dir, "_release", "1.2.3")
			if err := os.MkdirAll(releaseDir, 0o755); err != nil {
				t.Fatal(err)
			}
			filesToUpload := []string{prefix + "_v1.2.3_linux_amd64", prefix + "_v1.2.3_linux_amd64.sha256", "LICENSE"}
			if prefix == "mixed" {
				filesToUpload = []string{"mw_v1.2.3_linux_amd64", "wm_v1.2.3_linux_amd64"}
			}
			for _, file := range filesToUpload {
				if err := os.WriteFile(filepath.Join(releaseDir, file), []byte("fixture"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			upload := "set -eu\ncurl() { echo upload; }\n" + strings.Join(pipeline.Upload.Script, "\n")
			uploadCmd := exec.Command(shell, "-c", upload)
			uploadCmd.Dir = dir
			uploadCmd.Env = append(os.Environ(), "CI_API_V4_URL=https://gitlab.example/api/v4", "CI_PROJECT_ID=16", "CI_COMMIT_TAG=v1.2.3", "CI_JOB_TOKEN=fixture")
			uploaded, err := uploadCmd.CombinedOutput()
			if prefix == "mixed" {
				if err == nil || !strings.Contains(string(uploaded), "both mw_ and wm_ files") || strings.Contains(string(uploaded), "upload\n") {
					t.Fatalf("mixed release must fail before uploads: %v: %s", err, uploaded)
				}
				return
			}
			if err != nil || strings.Count(string(uploaded), "upload\n") != len(filesToUpload) {
				t.Fatalf("release upload: %v: %s", err, uploaded)
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

func TestReleaseUploadRetriesTransientErrors(t *testing.T) {
	for _, tool := range []string{"sh", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s unavailable", tool)
		}
	}
	script := "set -u\n" + strings.Join(readReleaseJobs(t).Upload.Script, "\n")
	for _, tc := range []struct {
		name       string
		first      int
		subsequent int
		requests   int32
		success    bool
	}{
		{"recovers from a transient error", http.StatusServiceUnavailable, http.StatusCreated, 3, true},
		{"permanent error stops uploads", http.StatusForbidden, http.StatusForbidden, 1, false},
		{"persistent transient error stops after three retries", http.StatusServiceUnavailable, http.StatusServiceUnavailable, 4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			releaseDir := filepath.Join(dir, "_release")
			if err := os.Mkdir(releaseDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"wm_v1.2.3_linux_amd64", "wm_v1.2.3_linux_amd64.sha256"} {
				if err := os.WriteFile(filepath.Join(releaseDir, name), []byte(name), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil || r.Method != http.MethodPut || string(body) != filepath.Base(r.URL.Path) {
					t.Errorf("invalid package upload: %s %s, body %q, error %v", r.Method, r.URL.Path, body, err)
				}
				w.Header().Set("Retry-After", "1")
				if requests.Add(1) == 1 {
					w.WriteHeader(tc.first)
				} else {
					w.WriteHeader(tc.subsequent)
				}
			}))
			defer server.Close()
			cmd := exec.CommandContext(t.Context(), "sh", "-c", script)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "CI_API_V4_URL="+server.URL, "CI_PROJECT_ID=16", "CI_COMMIT_TAG=v1.2.3",
				"CI_JOB_TOKEN=fixture", "NO_PROXY=127.0.0.1", "no_proxy=127.0.0.1")
			output, err := cmd.CombinedOutput()
			if (err == nil) != tc.success || requests.Load() != tc.requests {
				t.Fatalf("upload made %d requests (want %d), error: %v, output: %s", requests.Load(), tc.requests, err, output)
			}
			if !tc.success && !strings.Contains(string(output), "Release upload failed for ") {
				t.Fatalf("upload failed without identifying the file: %s", output)
			}
		})
	}
}

func TestReleaseRegeneratesOnlyBinaryChecksums(t *testing.T) {
	for _, tool := range []string{"make", "/bin/bash", "shasum", "true"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s unavailable", tool)
		}
	}
	stub, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".bingo"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Makefile", "LICENSE", ".bingo/Variables.mk"} {
		var data []byte
		if name != ".bingo/Variables.mk" {
			data, err = os.ReadFile(filepath.Join("../..", name))
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, version := range []string{"v1.2.3", "fix/release"} {
		t.Run(version, func(t *testing.T) {
			outputDir := filepath.Join(dir, "release output")
			binary := filepath.Join(outputDir, strings.ReplaceAll(version, "v", ""), "wm_"+version+"_linux_amd64")
			if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
				t.Fatal(err)
			}
			for _, contents := range []string{"first build", "rebuilt binary"} {
				if err := os.WriteFile(binary, []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("make", "-o", "generate", "release", "GOX="+stub, "GOVVV="+stub,
					"RELEASE_DIR="+outputDir, "VERSION="+version)
				cmd.Dir = dir
				cmd.Env = slices.DeleteFunc(os.Environ(), func(value string) bool { return strings.HasPrefix(value, "BINARY_NAME=") })
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("make release: %v: %s", err, output)
				}
				got, err := os.ReadFile(binary + ".sha256")
				if want := fmt.Sprintf("%x\n", sha256.Sum256([]byte(contents))); err != nil || string(got) != want {
					t.Fatalf("checksum: got %q, %v, want %q", got, err, want)
				}
				if _, err := os.Stat(binary + ".sha256.sha256"); !os.IsNotExist(err) {
					t.Fatalf("expected no second checksum file, got %v", err)
				}
			}
		})
	}
}
