package browser

import (
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestWSLOpensWindowsBrowserWithoutForwardingDiagnostics(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("WSL runs on Linux")
	}
	t.Setenv("WSL_DISTRO_NAME", "test")
	dir := t.TempDir()
	log := filepath.Join(dir, "url")
	t.Setenv("MWCLI_TEST_BROWSER_LOG", log)
	t.Setenv("PATH", dir)
	if err := os.WriteFile(filepath.Join(dir, "wslview"), []byte("#!/bin/sh\nprintf '%s' \"$1\" > \"$MWCLI_TEST_BROWSER_LOG\"\necho 'launcher diagnostic' >&2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	const url = "https://example.test/oauth?state=example&code_challenge=example"
	if err := OpenURL(url); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil || string(data) != url {
		t.Fatalf("launcher did not receive exact URL: %q, %v", data, err)
	}
}

func TestPowerShellURLIsEncodedAsData(t *testing.T) {
	const url = "https://example.test/?x=';Write-Output oops;&state=ä"
	commands := windowsCommands(url)
	for _, command := range commands[1:] {
		if command[3] != "-EncodedCommand" {
			t.Fatalf("unexpected Windows command: %v", command[:4])
		}
		data, err := base64.StdEncoding.DecodeString(command[4])
		if err != nil {
			t.Fatal(err)
		}
		units := make([]uint16, len(data)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(data[i*2:])
		}
		script := string(utf16.Decode(units))
		if strings.Contains(script, url) || !strings.Contains(script, "'"+base64.StdEncoding.EncodeToString([]byte(url))+"'") {
			t.Fatal("URL was not encoded separately from PowerShell source")
		}
	}
}
