package browser

import (
	"os/exec"
	"runtime"

	"github.com/pkg/browser"
)

// OpenURL opens a URL in the default browser.
// It has special handling for WSL environments on Linux.
func OpenURL(url string) error {
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("explorer.exe"); err == nil {
			return exec.Command("cmd.exe", "/c", "start", url).Run()
		}
	}
	return browser.OpenURL(url)
}
