package mediawiki

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/exec"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

/*CloneOpts for use with GithubCloneMediaWiki.*/
type CloneOpts struct {
	GetMediaWiki          bool
	GetVector             bool
	GetGerritSkins        []string
	GetGerritExtensions   []string
	UseGithub             bool
	UseShallow            bool
	GerritInteractionType string
	GerritUsername        string
	DryRun                bool
	SkipCommitHook        bool
	Jobs                  int
	In                    io.Reader
	Out                   io.Writer
	Err                   io.Writer
}

type synchronizedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (w synchronizedWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(data)
}

func commandWriters(in io.Reader, out, errOut io.Writer) (io.Reader, io.Writer, io.Writer) {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	if errOut == nil {
		errOut = os.Stderr
	}
	mu := &sync.Mutex{}
	return in, synchronizedWriter{mu, out}, synchronizedWriter{mu, errOut}
}

func (cp CloneOpts) AreThereThingsToClone() bool {
	return cp.GetMediaWiki || cp.GetVector || len(cp.GetGerritSkins) > 0 || len(cp.GetGerritExtensions) > 0
}

/*CloneSetup provides a packages initial setup method for MediaWiki etc with some speedy features.*/
func (m MediaWiki) CloneSetup(options CloneOpts) error {
	if _, err := osexec.LookPath("git"); err != nil {
		return fmt.Errorf("git is required: %w", err)
	}
	if options.Jobs < 0 {
		return fmt.Errorf("jobs must be positive")
	}
	jobs := options.Jobs
	if jobs == 0 {
		jobs = 1
	}
	if options.GerritInteractionType != "http" && options.GerritInteractionType != "ssh" {
		return fmt.Errorf("unknown Gerrit interaction type %q", options.GerritInteractionType)
	}
	in, resultOut, progressOut := commandWriters(options.In, options.Out, options.Err)
	if options.GerritInteractionType == "ssh" && options.GerritUsername == "" {
		return fmt.Errorf("Gerrit username required for SSH")
	}
	for _, name := range append(append([]string{}, options.GetGerritSkins...), options.GetGerritExtensions...) {
		if !filepath.IsLocal(name) {
			return fmt.Errorf("repository name must be a local relative path: %q", name)
		}
	}
	if options.GetVector && !stringsutil.StringInSlice("Vector", options.GetGerritSkins) {
		options.GetGerritSkins = append(options.GetGerritSkins, "Vector")
	}
	fmt.Fprintf(progressOut, "Cloning repositories (jobs: %d)...\n", jobs)
	if options.DryRun {
		fmt.Fprintln(resultOut, "...DRY RUN - no actual operations will be performed...")
	}
	clone := func(directory, startRemote, endRemote string, interactive bool) error {
		return cloneAndSetRemoteWithIO(directory, startRemote, endRemote, options.UseShallow, options.DryRun, !options.SkipCommitHook, interactive, in, resultOut, progressOut)
	}
	// Always finish core before starting children: cloning core into a directory
	// already populated by a skin/extension is not supported by Git.
	if options.GetMediaWiki {
		start := "https://gerrit.wikimedia.org/r/mediawiki/core"
		if options.UseGithub {
			start = "https://github.com/wikimedia/mediawiki.git"
		}
		end := "https://gerrit.wikimedia.org/r/mediawiki/core"
		if options.GerritInteractionType == "ssh" {
			end = "ssh://" + options.GerritUsername + "@gerrit.wikimedia.org:29418/mediawiki/core"
		}
		if err := clone(m.Path(""), start, end, true); err != nil {
			return err
		}
	}
	var work []func() error
	seen := map[string]bool{}
	var destinations []string
	var overlapErr error
	add := func(directory, start, end string) {
		if seen[directory] {
			return
		}
		directory = filepath.Clean(directory)
		for _, other := range destinations {
			if strings.HasPrefix(directory, other+string(filepath.Separator)) || strings.HasPrefix(other, directory+string(filepath.Separator)) {
				overlapErr = fmt.Errorf("overlapping checkout destinations %q and %q", other, directory)
			}
		}
		destinations = append(destinations, directory)
		seen[directory] = true
		work = append(work, func() error { return clone(directory, start, end, jobs == 1) })
	}
	for _, name := range options.GetGerritSkins {
		start, end := gerritHTTPRemoteForSkin(name), gerritHTTPRemoteForSkin(name)
		if options.UseGithub {
			start = githubRemoteForSkin(name)
		}
		if options.GerritInteractionType == "ssh" {
			end = gerritSSHRemoteForSkin(name, options.GerritUsername)
		}
		add(m.Path("skins/"+name), start, end)
	}
	for _, name := range options.GetGerritExtensions {
		start, end := gerritHTTPRemoteForExtension(name), gerritHTTPRemoteForExtension(name)
		if options.UseGithub {
			start = githubRemoteForExtension(name)
		}
		if options.GerritInteractionType == "ssh" {
			end = gerritSSHRemoteForExtension(name, options.GerritUsername)
		}
		add(m.Path("extensions/"+name), start, end)
	}
	if overlapErr != nil {
		return overlapErr
	}
	if err := runCloneJobs(work, jobs); err != nil {
		return err
	}
	fmt.Fprintln(progressOut, "Repositories cloned.")
	return nil
}

func runCloneJobs(work []func() error, parallelism int) error {
	if parallelism < 1 {
		return fmt.Errorf("jobs must be positive")
	}
	if parallelism > len(work) {
		parallelism = len(work)
	}
	results := make([]error, len(work))
	pending := make(chan int)
	var workers sync.WaitGroup
	for i := 0; i < parallelism; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range pending {
				results[index] = work[index]()
			}
		}()
	}
	for index := range work {
		pending <- index
	}
	close(pending)
	workers.Wait()
	// Preserve request order when reporting multiple failures.
	return errors.Join(results...)
}

func gerritHTTPRemoteForSkin(skin string) string {
	return "https://gerrit.wikimedia.org/r/mediawiki/skins/" + skin
}

func gerritSSHRemoteForSkin(skin string, username string) string {
	return "ssh://" + username + "@gerrit.wikimedia.org:29418/mediawiki/skins/" + skin
}

func githubRemoteForSkin(skin string) string {
	return "https://github.com/wikimedia/mediawiki-skins-" + skin + ".git"
}

func gerritHTTPRemoteForExtension(extension string) string {
	return "https://gerrit.wikimedia.org/r/mediawiki/extensions/" + extension
}

func gerritSSHRemoteForExtension(extension string, username string) string {
	return "ssh://" + username + "@gerrit.wikimedia.org:29418/mediawiki/extensions/" + extension
}

func githubRemoteForExtension(extension string) string {
	return "https://github.com/wikimedia/mediawiki-extensions-" + extension + ".git"
}

func cloneAndSetRemote(directory string, startRemote string, endRemote string, useShallow bool, dryRun bool, installHook bool, interactive bool) error {
	return cloneAndSetRemoteWithIO(directory, startRemote, endRemote, useShallow, dryRun, installHook, interactive, os.Stdin, os.Stdout, os.Stderr)
}

func cloneAndSetRemoteWithIO(directory string, startRemote string, endRemote string, useShallow bool, dryRun bool, installHook bool, interactive bool, in io.Reader, out, errOut io.Writer) error {
	slog.Debug(fmt.Sprint("Cloning " + startRemote + " to " + directory + " and setting remote to " + endRemote + " (shallow: " + strconv.FormatBool(useShallow) + ")"))
	if dryRun {
		fmt.Fprintf(out, "Would clone %s to %s\n", startRemote, directory)
		if startRemote != endRemote {
			fmt.Fprintf(out, "Would set remote origin to %s\n", endRemote)
		}
		if installHook {
			fmt.Fprintf(out, "Would install Gerrit commit-msg hook in %s (preserving existing hooks)\n", directory)
		}
		return nil
	}
	if err := runCloneCommandWithIO(gitCloneCommand(directory, startRemote, useShallow), interactive, in, out, errOut); err != nil {
		return fmt.Errorf("clone %s: %w", startRemote, err)
	}
	if startRemote != endRemote {
		if err := runCloneCommandWithIO(osexec.Command("git", gitRemoteSetURLArguments(directory, endRemote)...), interactive, in, out, errOut); err != nil {
			return fmt.Errorf("set remote for %s: %w", directory, err)
		}
	}
	if installHook {
		if err := installGerritCommitHook(directory, gerritCommitHookURL); err != nil {
			fmt.Fprintf(errOut, "Repository cloned, but commit-msg hook setup failed: %v. See https://gerrit.wikimedia.org/r/Documentation/cmd-hook-commit-msg.html\n", err)
		}
	}
	return nil
}

func runCloneCommandWithIO(cmd *osexec.Cmd, interactive bool, in io.Reader, out, errOut io.Writer) error {
	cmd.Stdout = out
	cmd.Stderr = errOut
	if interactive {
		cmd.Stdin = in
	}
	return cmd.Run()
}

func gitCloneCommand(directory, remote string, useShallow bool) *osexec.Cmd {
	return identifyGitCommand(exec.Command("git", gitCloneArguments(directory, remote, useShallow)...))
}

func identifyGitCommand(cmd *osexec.Cmd) *osexec.Cmd {
	// Identify the CLI to HTTP servers, including in recursive submodule clones.
	// Keep the override local to this subprocess and respect user configuration.
	if _, overridden := os.LookupEnv("GIT_HTTP_USER_AGENT"); !overridden {
		cmd.Env = append(cmd.Environ(), "GIT_HTTP_USER_AGENT="+mwcliUserAgent())
	}
	return cmd
}

func mwcliUserAgent() string {
	version := cli.VersionDetails.Version.String()
	if version == "" {
		version = "dev"
	}
	return cli.AgentName + "/" + version + " (" + cli.ProjectURL + ")"
}

func gitCloneArguments(directory string, remote string, useShallow bool) []string {
	args := []string{"clone"}
	if useShallow {
		args = append(args, "--depth=1")
	}
	args = append(args, "--recurse-submodules")
	args = append(args, remote)
	args = append(args, directory)
	return args
}

func gitRemoteSetURLArguments(directory string, newRemote string) []string {
	return []string{"-C", directory, "remote", "set-url", "origin", newRemote}
}
