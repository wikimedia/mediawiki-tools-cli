package update

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

// This helper only runs inside a disposable copy of the test executable. Never
// run the self-update command in the parent test runner or an installed mw.
func TestUpdateCommandProcess(t *testing.T) {
	if os.Getenv("MWCLI_UPDATE_COMMAND_HELPER") != "1" {
		return
	}
	cli.Opts.NoInteraction = true
	command := Cmd()
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	command.SetArgs([]string{"--version", os.Getenv("MWCLI_UPDATE_TEST_SOURCE")})
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func TestUpdateCommandDownloads(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	replacement := []byte("replacement executable fixture\n")
	nativeFilename := "bin/mw"
	if runtime.GOOS == "windows" {
		nativeFilename += ".exe"
	}
	archive := updateArchiveFixture(t, map[string][]byte{nativeFilename: replacement})
	filename := "wm_nightly_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	releaseArchive := updateArchiveFixture(t, map[string][]byte{
		"_release/nightly/" + filename:               replacement,
		"_release/nightly/" + filename + ".sha256":   []byte("checksum fixture"),
		"_release/nightly/wm_nightly_other_platform": []byte("wrong platform"),
	})
	wrongArchive := updateArchiveFixture(t, map[string][]byte{"_release/nightly/mw_nightly_other_platform": replacement})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/404":
			http.Error(w, "not an executable: missing release", http.StatusNotFound)
		case "/500":
			http.Error(w, "not an executable: server failure", http.StatusInternalServerError)
		case "/redirect-error":
			http.Redirect(w, r, "/404", http.StatusFound)
		case "/redirect":
			http.Redirect(w, r, "/release", http.StatusFound)
		case "/archive":
			_, _ = w.Write(archive)
		case "/release-archive":
			_, _ = w.Write(releaseArchive)
		case "/wrong-archive":
			_, _ = w.Write(wrongArchive)
		default:
			_, _ = w.Write(replacement)
		}
	}))
	defer server.Close()
	local := filepath.Join(t.TempDir(), "local replacement")
	if err := os.WriteFile(local, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(local)}).String()
	for _, tc := range []struct {
		name   string
		source string
		status string
	}{
		{"404", server.URL + "/404", "404 Not Found"},
		{"500", server.URL + "/500", "500 Internal Server Error"},
		{"redirect-error", server.URL + "/redirect-error", "404 Not Found"},
		{"release", server.URL + "/release", ""},
		{"redirect", server.URL + "/redirect", ""},
		{"archive", server.URL + "/archive", ""},
		{"release-archive", server.URL + "/release-archive", ""},
		{"wrong-archive", server.URL + "/wrong-archive", "no supported executable"},
		{"local", local, ""},
		{"file-url", fileURL, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && tc.name == "file-url" {
				t.Skip("file URLs use POSIX paths; native Windows paths are covered separately")
			}
			name := "disposable mw"
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, original, 0o755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			process := exec.CommandContext(ctx, path, "-test.run=^TestUpdateCommandProcess$")
			process.Env = append(os.Environ(), "MWCLI_UPDATE_COMMAND_HELPER=1", "MWCLI_UPDATE_TEST_SOURCE="+tc.source)
			output, err := process.CombinedOutput()
			if tc.status != "" {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() <= 0 {
					t.Errorf("expected nonzero command exit, got %v; output: %s", err, output)
				}
				if !strings.Contains(string(output), tc.status) || strings.Contains(string(output), "Update successful") {
					t.Errorf("expected HTTP error without success message, got: %s", output)
				}
				actual, readErr := os.ReadFile(path)
				if readErr != nil || !bytes.Equal(actual, original) {
					t.Fatal("failed download changed the original executable")
				}
				return
			}
			if err != nil {
				t.Fatalf("update failed: %v; output: %s", err, output)
			}
			success := "Update successful"
			if runtime.GOOS == "windows" {
				success = "Update staged"
			}
			if !strings.Contains(string(output), success) {
				t.Fatalf("missing success message: %s", output)
			}
			deadline := time.Now().Add(15 * time.Second)
			for {
				actual, readErr := os.ReadFile(path)
				if readErr == nil && bytes.Equal(actual, replacement) {
					break
				}
				if runtime.GOOS != "windows" || time.Now().After(deadline) {
					t.Fatalf("successful update did not install the expected bytes: %v", readErr)
				}
				time.Sleep(100 * time.Millisecond)
			}
		})
	}
}

func updateArchiveFixture(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, content := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o755)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}
