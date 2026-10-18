package mediawiki

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"time"
)

const gerritCommitHookURL = "https://gerrit.wikimedia.org/r/tools/hooks/commit-msg"

func installGerritCommitHook(directory, source string) error {
	// A configured hooksPath may be shared or managed by the user: leave it alone,
	// including an explicitly empty value used to disable hooks.
	if _, err := osexec.Command("git", "-C", directory, "config", "--get", "core.hooksPath").Output(); err == nil {
		return nil
	} else {
		var exitErr *osexec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return fmt.Errorf("read core.hooksPath: %w", err)
		}
	}
	output, err := osexec.Command("git", "-C", directory, "rev-parse", "--git-path", "hooks/commit-msg").Output()
	if err != nil {
		return fmt.Errorf("locate Git hooks: %w", err)
	}
	path := strings.TrimSpace(string(output))
	if !filepath.IsAbs(path) {
		path = filepath.Join(directory, path)
	}
	if _, err := os.Lstat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	request, err := http.NewRequest(http.MethodGet, source, nil)
	if err != nil {
		return fmt.Errorf("create hook download request: %w", err)
	}
	request.Header.Set("User-Agent", mwcliUserAgent())
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download commit-msg hook: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download commit-msg hook: HTTP %d", response.StatusCode)
	}
	const limit = 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if len(data) > limit || !strings.HasPrefix(string(data), "#!/") {
		return fmt.Errorf("invalid commit-msg hook response")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	hook, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_, writeErr := hook.Write(data)
	closeErr := hook.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return errors.Join(writeErr, closeErr)
	}
	return nil
}
