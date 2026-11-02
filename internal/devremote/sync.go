package devremote

import (
	"bytes"
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

var syncExcludes = []string{".git", ".wm", ".env", ".env.*", ".ssh", ".git-credentials", ".netrc", ".npmrc", ".pypirc", "node_modules", "vendor", ".venv", "__pycache__"}

// LocalDirectory validates the source root before enabling continuous replication.
func LocalDirectory(directory string) (string, error) {
	abs, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	home, _ := os.UserHomeDir()
	if !info.IsDir() || abs == filepath.VolumeName(abs)+string(os.PathSeparator) || abs == home {
		return "", fmt.Errorf("choose a project directory, not your home directory or filesystem root")
	}
	if relative, err := filepath.Rel(abs, cli.UserDirectoryPath()); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("the synchronized directory must not contain the CLI's credential/configuration directory")
	}
	if _, err := osexec.LookPath("rsync"); err != nil {
		return "", fmt.Errorf("local-folder mode requires rsync on both machines: %w", err)
	}
	return abs, nil
}

func syncArguments(c Context) ([]string, error) {
	if c.Directory == "" || c.Source == "" || c.Metadata == "" || !strings.HasPrefix(c.Source, c.Workspace+"/") {
		return nil, fmt.Errorf("invalid synchronization root")
	}
	ssh := []string{"ssh", "-T", "-o", "ForwardAgent=no", "-o", "ConnectTimeout=15"}
	if session, err := loadSession(c.Context); err == nil && session.SSHControl != "" {
		ssh = append(ssh, "-S", session.SSHControl)
	}
	ssh = append(ssh, c.SSHArgs[:len(c.SSHArgs)-1]...)
	for i := range ssh {
		ssh[i] = quote(ssh[i])
	}
	args := []string{"-az", "--protect-args", "--no-owner", "--no-group", "--safe-links", "--chmod=u+rwX,go+rX", "--delete-delay", "--backup", "--backup-dir=" + c.Metadata + "/sync-backup", "-e", strings.Join(ssh, " ")}
	for _, pattern := range syncExcludes {
		args = append(args, "--exclude="+pattern)
	}
	host := c.SSHArgs[len(c.SSHArgs)-1]
	if strings.Contains(host, ":") && !strings.Contains(host, "[") {
		host = "[" + host + "]"
	}
	return append(args, "--", filepath.ToSlash(c.Directory)+"/", host+":"+c.Source+"/"), nil
}

func syncDirectory(ctx context.Context, c Context) error {
	args, err := syncArguments(c)
	if err != nil {
		return err
	}
	t := transport{args: c.SSHArgs}
	if _, err := t.output(ctx, "umask 022; command -v rsync >/dev/null; mkdir -p "+quote(c.Source)+" "+quote(c.Metadata+"/sync-backup")); err != nil {
		return err
	}
	var diagnostic bytes.Buffer
	command := osexec.CommandContext(ctx, "rsync", args...)
	command.Stderr = &diagnostic
	if err := command.Run(); err != nil {
		return fmt.Errorf("synchronize source: %w: %s", err, strings.TrimSpace(diagnostic.String()))
	}
	return nil
}
