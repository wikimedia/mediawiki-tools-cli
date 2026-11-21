package gitconfig

import (
	"context"
	osexec "os/exec"
	"strings"
)

// PushRemote follows Git's configured push-remote precedence for the given
// branch, falling back to the sole remote or origin.
func PushRemote(ctx context.Context, branch string) string {
	keys := []string{"remote.pushDefault"}
	if branch != "" {
		keys = []string{"branch." + branch + ".pushRemote", "remote.pushDefault", "branch." + branch + ".remote"}
	}
	for _, key := range keys {
		if value, err := osexec.CommandContext(ctx, "git", "config", "--get", key).Output(); err == nil {
			if remote := strings.TrimSpace(string(value)); remote != "" {
				return remote
			}
		}
	}
	if value, err := osexec.CommandContext(ctx, "git", "remote").Output(); err == nil {
		if remote := strings.TrimSpace(string(value)); remote != "" && !strings.Contains(remote, "\n") {
			return remote
		}
	}
	return "origin"
}
