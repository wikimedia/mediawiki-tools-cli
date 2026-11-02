package devremote

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func linuxELF(arm bool) []byte {
	// A minimal ELF header is sufficient for validating the asset architecture.
	data := make([]byte, 64)
	copy(data, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	binary.LittleEndian.PutUint16(data[16:], 2)
	machine := uint16(62)
	if arm {
		machine = 183
	}
	binary.LittleEndian.PutUint16(data[18:], machine)
	binary.LittleEndian.PutUint32(data[20:], 1)
	binary.LittleEndian.PutUint16(data[52:], 64)
	return data
}

func TestExactReleaseClassification(t *testing.T) {
	for _, tc := range []struct{ version, summary, state, want string }{
		{"0.35.0", "v0.35.0", "clean", "v0.35.0"},
		{"2026.11.02.0", "v2026.11.02.0", "clean", "v2026.11.02.0"},
		{"0.35.0", "v0.35.0-2-gabcdef", "clean", ""},
		{"0.35.0", "v0.35.0", "dirty", ""},
		{"nightly", "v0.35.0-2-gabcdef", "clean", ""},
		{"latest", "", "", ""},
		{"0.35.0-rc.1", "v0.35.0-rc.1", "clean", ""},
	} {
		if got := exactReleaseTag(cli.VersionAttributes{Version: cli.Version(tc.version), GitSummary: tc.summary, GitState: tc.state}); got != tc.want {
			t.Errorf("%+v: %q", tc, got)
		}
	}
}

func TestDevelopmentBinaryOnlyCopiesMatchingPlatform(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "local-cli")
	if err := os.WriteFile(filename, linuxELF(false), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		platform, arch, machine string
		match                   bool
	}{
		{"linux", "amd64", "x86_64", true}, {"linux", "arm64", "x86_64", false}, {"darwin", "amd64", "x86_64", false}, {"windows", "amd64", "x86_64", false},
	} {
		for _, version := range []cli.Version{"latest", "nightly"} {
			r := binaryResolver{build: cli.VersionAttributes{Version: version}, platform: tc.platform, architecture: tc.arch, executable: filename, lookup: func(string, string, string) (*gitlab.ReleaseLink, error) {
				t.Fatal("dev/nightly must never download")
				return nil, nil
			}}
			file, _, err := r.resolve(context.Background(), tc.machine, io.Discard)
			if file != nil {
				file.Close()
			}
			if (err == nil) != tc.match {
				t.Fatalf("%+v: %v", tc, err)
			}
		}
	}
}

func TestExactReleaseDownloadCacheAndFailures(t *testing.T) {
	for _, failure := range []string{"", "missing release", "bad checksum", "wrong architecture"} {
		t.Run(failure, func(t *testing.T) {
			data := linuxELF(failure == "wrong architecture")
			digest := sha256.Sum256(data)
			var downloads, lookups atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, ".sha256") {
					if failure == "bad checksum" {
						io.WriteString(w, strings.Repeat("0", 64))
					} else {
						io.WriteString(w, hex.EncodeToString(digest[:]))
					}
					return
				}
				downloads.Add(1)
				w.Write(data)
			}))
			defer server.Close()
			r := binaryResolver{build: cli.VersionAttributes{Version: "2026.11.02.0", GitSummary: "v2026.11.02.0", GitState: "clean"}, platform: "windows", architecture: "arm64", cache: t.TempDir(), client: server.Client(), lookup: func(tag, platform, arch string) (*gitlab.ReleaseLink, error) {
				lookups.Add(1)
				if tag != "v2026.11.02.0" || platform != "linux" || arch != "amd64" {
					t.Errorf("wrong selection: %s %s/%s", tag, platform, arch)
				}
				if failure == "missing release" {
					return nil, errors.New("404 release missing")
				}
				return &gitlab.ReleaseLink{URL: server.URL + "/binary"}, nil
			}}
			file, _, err := r.resolve(context.Background(), "x86_64", io.Discard)
			if failure != "" {
				if file != nil {
					file.Close()
				}
				if err == nil {
					t.Fatal("accepted failing download")
				}
				if _, err := os.Stat(filepath.Join(r.cache, "v2026.11.02.0", "linux-amd64", "wm")); !os.IsNotExist(err) {
					t.Fatal("cached a failed download")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			file.Close()
			file, _, err = r.resolve(context.Background(), "x86_64", io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			file.Close()
			if downloads.Load() != 1 || lookups.Load() != 1 {
				t.Fatal("valid cache was downloaded again")
			}
			cache := filepath.Join(r.cache, "v2026.11.02.0", "linux-amd64", "wm")
			if err := os.WriteFile(cache, []byte("damaged"), 0o700); err != nil {
				t.Fatal(err)
			}
			file, _, err = r.resolve(context.Background(), "x86_64", io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			file.Close()
			if downloads.Load() != 2 {
				t.Fatal("corrupted cache was reused")
			}
		})
	}
}
