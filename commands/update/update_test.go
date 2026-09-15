package update

import (
	"os"
	"path/filepath"
	"testing"
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
