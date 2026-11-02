package devremote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	osexec "os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

// ValidateGitRemote accepts user-selected Git destinations, not executable Git
// transports or URLs containing embedded HTTP credentials.
func ValidateGitRemote(raw string) error {
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "-") || strings.ContainsAny(raw, "\r\n\x00") || strings.Contains(raw, "::") {
		return errors.New("unsupported Git destination")
	}
	if filepath.IsAbs(raw) {
		return nil
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "https" && u.Scheme != "ssh" && u.Scheme != "file") {
			return errors.New("use an HTTPS, SSH or local Git destination")
		}
		if u.Scheme == "https" && u.User != nil {
			return errors.New("keep HTTP credentials in local Git configuration, not the destination URL")
		}
		if u.User != nil {
			if _, secret := u.User.Password(); secret {
				return errors.New("Git destinations must not contain passwords")
			}
		}
		if u.RawQuery != "" || u.Fragment != "" {
			return errors.New("Git destinations must not contain query credentials or fragments")
		}
		return nil
	}
	host, _, ok := strings.Cut(raw, ":")
	if !ok || strings.ContainsAny(raw, " \t") || ValidateHost(host) != nil {
		return errors.New("use a complete SSH or HTTPS Git URL")
	}
	return nil
}

func gitMirror(c Context) string {
	dir, _ := contextDirectory(c.Context)
	return filepath.Join(dir, "git.git")
}

func gitEnvironment(c Context) []string {
	env := append(os.Environ(), "MW_REMOTE_LOCAL=1", "GIT_TERMINAL_PROMPT=0")
	ssh := []string{"ssh", "-T", "-o", "ForwardAgent=no", "-o", "BatchMode=yes"}
	if session, err := loadSession(c.Context); err == nil && session.SSHControl != "" {
		ssh = append(ssh, "-S", session.SSHControl)
	}
	ssh = append(ssh, c.SSHArgs[:len(c.SSHArgs)-1]...)
	for i := range ssh {
		ssh[i] = quote(ssh[i])
	}
	return append(env, "GIT_SSH_COMMAND="+strings.Join(ssh, " "))
}

func gitCommand(ctx context.Context, c Context, vm bool, args ...string) *osexec.Cmd {
	// Never run checkout hooks from an upstream repository on the local machine.
	base := []string{"-c", "core.hooksPath=" + os.DevNull}
	if strings.HasPrefix(c.GitRemote, "https://gitlab.wikimedia.org/") {
		if executable, err := os.Executable(); err == nil {
			base = append(base, "-c", "credential.https://gitlab.wikimedia.org.helper=", "-c", "credential.https://gitlab.wikimedia.org.helper=!"+quote(executable)+" gitlab auth git-credential")
		}
	}
	command := osexec.CommandContext(ctx, "git", append(base, args...)...)
	command.Env = append(os.Environ(), "MW_REMOTE_LOCAL=1", "GIT_TERMINAL_PROMPT=0")
	if vm {
		command.Env = gitEnvironment(c)
	}
	return command
}

func gitVMURL(c Context) string {
	host := c.SSHArgs[len(c.SSHArgs)-1]
	u := &url.URL{Scheme: "ssh", Host: host, Path: path.Join(c.Source, ".git")}
	if user, name, ok := strings.Cut(host, "@"); ok {
		u.Host, u.User = name, url.User(user)
	}
	return u.String()
}

func gitLock(ctx context.Context, c Context) (func(), error) {
	dir, err := contextDirectory(c.Context)
	if err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(dir, "git.lock"))
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil || !locked {
		return nil, fmt.Errorf("could not lock the local Git mirror: %w", err)
	}
	return func() { lock.Unlock() }, nil
}

func prepareGit(ctx context.Context, c Context) error {
	if err := ValidateGitRemote(c.GitRemote); err != nil {
		return err
	}
	t := transport{args: c.SSHArgs, stderr: os.Stderr}
	existing, err := t.output(ctx, "if test -d "+quote(c.Source+"/.git")+"; then printf exists; fi")
	if err != nil || existing == "exists" {
		return err
	}
	unlock, err := gitLock(ctx, c)
	if err != nil {
		return err
	}
	defer unlock()
	mirror := gitMirror(c)
	if _, err := os.Stat(filepath.Join(mirror, "HEAD")); os.IsNotExist(err) {
		command := gitCommand(ctx, c, false, "clone", "--mirror", "--", c.GitRemote, mirror)
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("clone using local credentials: %w", err)
		}
	}
	bundle, err := os.CreateTemp("", "mw-git-*.bundle")
	if err != nil {
		return err
	}
	filename := bundle.Name()
	bundle.Close()
	defer os.Remove(filename)
	command := gitCommand(ctx, c, false, "-C", mirror, "bundle", "create", filename, "--all")
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("prepare source bundle: %w", err)
	}
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	remoteBundle := c.Metadata + "/bootstrap.bundle"
	if err := t.run(ctx, "umask 077; cat > "+quote(remoteBundle), f, os.Stdout); err != nil {
		return err
	}
	script := `set -eu
bundle=$1; source=$2; destination=$3
trap 'rm -f -- "$bundle"' EXIT
umask 022
git clone -- "$bundle" "$source"
git -C "$source" remote set-url origin "$destination"`
	return t.script(ctx, script, []string{remoteBundle, c.Source, c.GitRemote}, os.Stdout)
}

func normalizeGit(args []string, cwd string) (operation string, rest []string, directory string, err error) {
	directory = path.Clean(cwd)
	for len(args) > 0 {
		if args[0] == "-C" && len(args) > 1 {
			if path.IsAbs(args[1]) {
				directory = path.Clean(args[1])
			} else {
				directory = path.Join(directory, args[1])
			}
			args = args[2:]
			continue
		}
		break
	}
	if len(args) == 0 {
		return "", nil, directory, errors.New("choose fetch, push or pull")
	}
	operation, rest = args[0], args[1:]
	if operation != "fetch" && operation != "push" && operation != "pull" {
		return "", nil, directory, errors.New("authenticated Git supports fetch, push and pull; clone a private project with remote open --git-remote URL --recipe none")
	}
	return operation, rest, directory, nil
}

func validateGit(c Context, request invocation) error {
	if c.GitRemote == "" {
		return errors.New("no Git destination enabled; set --git-remote locally")
	}
	if err := ValidateGitRemote(c.GitRemote); err != nil {
		return err
	}
	_, _, cwd, err := normalizeGit(request.Args, request.CWD)
	if err != nil {
		return err
	}
	if cwd != c.Source && !strings.HasPrefix(cwd, c.Source+"/") {
		return errors.New("Git delegation is limited to this context's source checkout")
	}
	return nil
}

func runGit(ctx context.Context, c Context, request invocation, stdout, stderr io.Writer) error {
	operation, rest, _, err := normalizeGit(request.Args, request.CWD)
	if err != nil {
		return err
	}
	unlock, err := gitLock(ctx, c)
	if err != nil {
		return err
	}
	defer unlock()
	mirror := gitMirror(c)
	if _, err := os.Stat(filepath.Join(mirror, "HEAD")); os.IsNotExist(err) {
		if out, err := gitCommand(ctx, c, false, "init", "--bare", mirror).CombinedOutput(); err != nil {
			return fmt.Errorf("initialize Git mirror: %w: %s", err, out)
		}
	}
	run := func(vm bool, args ...string) error {
		command := gitCommand(ctx, c, vm, append([]string{"-C", mirror}, args...)...)
		command.Stdout, command.Stderr = stdout, stderr
		return command.Run()
	}
	var flags, refs []string
	remoteSeen := false
	for _, arg := range rest {
		if arg == "origin" && !remoteSeen {
			remoteSeen = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if operation == "push" && (arg == "--dry-run" || arg == "--force-with-lease" || strings.HasPrefix(arg, "--force-with-lease=")) {
				flags = append(flags, arg)
				continue
			}
			return fmt.Errorf("Git option %s is not supported by the session bridge", arg)
		}
		refs = append(refs, arg)
	}
	vmURL := gitVMURL(c)
	if operation == "push" {
		if err := run(true, "fetch", "--update-shallow", "--no-tags", vmURL, "+HEAD:refs/mw/head", "+refs/heads/*:refs/mw/source/*"); err != nil {
			return err
		}
		branch, err := (transport{args: c.SSHArgs, stderr: stderr}).output(ctx, "git -C "+quote(c.Source)+" symbolic-ref --short HEAD")
		if err != nil && len(refs) == 0 {
			return errors.New("specify a destination ref when pushing a detached HEAD")
		}
		branch = strings.TrimSpace(branch)
		if len(refs) == 0 {
			refs = []string{"HEAD:refs/heads/" + branch}
		}
		var specs []string
		for _, ref := range refs {
			source, target, pair := strings.Cut(ref, ":")
			if !pair {
				target = source
				if source == "HEAD" {
					target = branch
				}
			}
			if source == "" || target == "" {
				return errors.New("ref deletion is not supported through the session bridge")
			}
			if !strings.HasPrefix(target, "refs/") {
				target = "refs/heads/" + target
			}
			if out, err := gitCommand(ctx, c, false, "check-ref-format", target).CombinedOutput(); err != nil {
				return fmt.Errorf("invalid destination ref: %s", out)
			}
			if source == "HEAD" {
				source = "refs/mw/head"
			} else {
				source = "refs/mw/source/" + strings.TrimPrefix(source, "refs/heads/")
			}
			specs = append(specs, source+":"+target)
		}
		args := append([]string{"push"}, flags...)
		args = append(args, "--", c.GitRemote)
		return run(false, append(args, specs...)...)
	}
	fetchSpecs := []string{"+refs/heads/*:refs/remotes/origin/*", "refs/tags/*:refs/tags/*"}
	if len(refs) > 0 {
		fetchSpecs = nil
		for _, branch := range refs {
			branch = strings.TrimPrefix(branch, "refs/heads/")
			if _, err := gitCommand(ctx, c, false, "check-ref-format", "refs/heads/"+branch).Output(); err != nil {
				return errors.New("fetch/pull accept branch names, not another destination URL")
			}
			fetchSpecs = append(fetchSpecs, "+refs/heads/"+branch+":refs/remotes/origin/"+branch)
		}
	}
	if err := run(false, append([]string{"fetch", "--", c.GitRemote}, fetchSpecs...)...); err != nil {
		return err
	}
	if err := run(true, "push", vmURL, "+refs/remotes/origin/*:refs/remotes/origin/*", "refs/tags/*:refs/tags/*"); err != nil {
		return err
	}
	t := transport{args: c.SSHArgs, stderr: stderr}
	branch, branchErr := t.output(ctx, "git -C "+quote(c.Source)+" symbolic-ref --short HEAD")
	branch = strings.TrimSpace(branch)
	if len(refs) > 0 {
		branch = strings.TrimPrefix(refs[0], "refs/heads/")
		branchErr = nil
	}
	if branchErr == nil {
		// This is an on-VM fetch from its own tracking ref: it updates FETCH_HEAD
		// without requiring any upstream authentication on the VM.
		if err := t.run(ctx, "git -C "+quote(c.Source)+" fetch --no-tags . "+quote("refs/remotes/origin/"+branch), nil, stdout); err != nil {
			return err
		}
	}
	if operation == "pull" {
		if branchErr != nil {
			return errors.New("pull requires a checked-out branch or an explicit branch name")
		}
		return t.run(ctx, "git -C "+quote(c.Source)+" merge --ff-only "+quote("refs/remotes/origin/"+branch), nil, stdout)
	}
	return nil
}
