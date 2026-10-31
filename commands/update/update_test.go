package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestExecutableNameFromPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "unix path", in: "/usr/local/bin/mw", want: "mw"},
		{name: "windows path", in: `C:\\Users\\adam\\mw.exe`, want: "mw.exe"},
		{name: "plain name", in: "mw", want: "mw"},
		{name: "trailing slash unix", in: "/usr/local/bin/", want: "/usr/local/bin/"},
		{name: "trailing slash windows", in: `C:\\Users\\adam\\`, want: `C:\\Users\\adam\\`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := executableNameFromPath(tc.in)
			if got != tc.want {
				t.Fatalf("executableNameFromPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestReplaceExecutableNonWindows_UsesSameDirStagedReplacement(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "mw")
	newFile := filepath.Join(dir, "mw-new")

	if err := os.WriteFile(dest, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write dest: %v", err)
	}
	if err := os.WriteFile(newFile, []byte("new-binary"), 0o755); err != nil {
		t.Fatalf("write new file: %v", err)
	}

	if err := replaceExecutableNonWindows(newFile, dest); err != nil {
		t.Fatalf("replaceExecutableNonWindows returned error: %v", err)
	}

	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(b) != "new-binary" {
		t.Fatalf("dest content = %q, want %q", string(b), "new-binary")
	}

	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Fatalf("expected new file to be moved away, stat err=%v", err)
	}
}

func TestArchiveBinaryPathSupportsBothNamesAndWindows(t *testing.T) {
	for _, tc := range []struct{ platform, name string }{{"linux", "mw"}, {"linux", "wm"}, {"windows", "mw.exe"}, {"windows", "wm.exe"}} {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(dir, "bin", tc.name)
		if err := os.WriteFile(want, []byte("test-binary"), 0o755); err != nil {
			t.Fatal(err)
		}
		got, err := archiveBinaryPath(dir, tc.platform, "amd64")
		if err != nil || got != want {
			t.Fatalf("%s/%s: %q, %v", tc.platform, tc.name, got, err)
		}
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin", "wm"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := archiveBinaryPath(dir, "linux", "amd64"); err == nil {
		t.Fatal("directory accepted as executable")
	}
}

func TestArchiveBinaryPathSelectsReleasePlatformAndVersion(t *testing.T) {
	for _, tc := range []struct{ version, name, platform, architecture string }{
		{"nightly", "mw", "linux", "amd64"},
		{"v0.34.0", "mw", "darwin", "arm64"},
		{"v2026.10.31.0", "mw", "linux", "amd64"},
		{"v2027.01.02.1", "wm", "windows", "amd64"},
		{"fix/example", "wm", "linux", "arm64"},
		{"dev/example", "mw", "linux", "amd64"},
		{"nightly", "wm", "windows", "amd64"},
	} {
		t.Run(tc.version+"/"+tc.platform+"/"+tc.name, func(t *testing.T) {
			directory := t.TempDir()
			filename := tc.name + "_" + tc.version + "_" + tc.platform + "_" + tc.architecture
			if tc.platform == "windows" {
				filename += ".exe"
			}
			want := filepath.Join(directory, "_release", strings.ReplaceAll(tc.version, "v", ""), filename)
			for _, path := range []string{want, want + ".sha256", want + "_other"} {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("fixture"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			got, err := archiveBinaryPath(directory, tc.platform, tc.architecture)
			if err != nil || got != want {
				t.Fatalf("got %q, %v; want %q", got, err, want)
			}
			if _, err := archiveBinaryPath(directory, tc.platform, "other-architecture"); err == nil {
				t.Fatal("accepted a binary for another architecture")
			}
		})
	}
}

func TestAutomaticUpdateVersionGuard(t *testing.T) {
	for _, tc := range []struct {
		current, target string
		force, want     bool
	}{
		{"0.34.0", "0.35.0", false, true},
		{"0.34.0", "2026.10.31.0", false, true},
		{"2026.10.31.0", "2026.10.31.1", false, true},
		{"2026.11.15.0", "2026.10.31.2", false, false},
		{"2026.11.15.0", "2026.10.31.2", true, false},
		{"2026.10.31.0", "0.99.0", false, false},
		{"2026.10.31.0", "2026.10.31.0", false, false},
		{"2026.10.31.0", "2026.10.31.0", true, true},
		{"2027.01.02.0", "2027.1.2.0", false, false},
		{"latest", "2026.10.31.0", false, true},
		{"nightly", "2026.10.31.0", false, true},
	} {
		if got := automaticUpdateNeeded(cli.Version(tc.current), cli.Version(tc.target), tc.force); got != tc.want {
			t.Errorf("%s -> %s (force=%t): got %t, want %t", tc.current, tc.target, tc.force, got, tc.want)
		}
	}
}

func TestDescribeDatedVersionTransition(t *testing.T) {
	for _, tc := range []struct{ from, to, want string }{
		{"0.34.0", "2026.10.31.0", "Upgraded"},
		{"2026.10.31.9", "2026.10.31.10", "Upgraded"},
		{"2026.11.15.0", "2026.10.31.1", "Downgraded"},
	} {
		if got := describeVersionTransition(cli.Version(tc.from), cli.Version(tc.to)); !strings.HasPrefix(got, tc.want) {
			t.Errorf("%s -> %s: %s", tc.from, tc.to, got)
		}
	}
}

func TestArchiveBinaryPathRejectsAmbiguousVersions(t *testing.T) {
	directory := t.TempDir()
	for _, version := range []string{"nightly", "v0.34.0"} {
		path := filepath.Join(directory, "_release", strings.TrimPrefix(version, "v"), "mw_"+version+"_linux_amd64")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := archiveBinaryPath(directory, "linux", "amd64"); err == nil || !strings.Contains(err.Error(), "multiple releases") {
		t.Fatalf("ambiguous archive accepted: %v", err)
	}
}
