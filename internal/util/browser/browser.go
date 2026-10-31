// Package browser opens web links using the desktop outside WSL when applicable.
package browser

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"

	pkgBrowser "github.com/pkg/browser"
)

func OpenURL(url string) error {
	if !isWSL() {
		return pkgBrowser.OpenURL(url)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, command := range windowsCommands(url) {
		if _, err := exec.LookPath(command[0]); err != nil {
			continue
		}
		// Launch URLs as data and discard launcher diagnostics. In particular,
		// never fall back to a Linux Snap browser inside WSL.
		if exec.CommandContext(ctx, command[0], command[1:]...).Run() == nil { // #nosec G204 -- fixed launcher names; URL is an argument or encoded data.
			return nil
		}
	}
	return errors.New("could not open the Windows browser; open the displayed URL manually or install wslview")
}

func isWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	release, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return strings.Contains(strings.ToLower(string(release)), "microsoft")
}

func windowsCommands(url string) [][]string {
	// EncodedCommand prevents PowerShell from interpreting URL characters as
	// source code, including quotes, ampersands, and OAuth query parameters.
	data := base64.StdEncoding.EncodeToString([]byte(url))
	script := "Start-Process -FilePath ([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + data + "')))"
	units := utf16.Encode([]rune(script))
	encoded := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(encoded[i*2:], unit)
	}
	args := []string{"-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(encoded)}
	return [][]string{
		{"wslview", url},
		append([]string{"powershell.exe"}, args...),
		append([]string{"/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"}, args...),
	}
}
