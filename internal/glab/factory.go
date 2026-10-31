// Package mwcli adapts glab's repository detection to Wikimedia's separate SSH host.
// The nested module lets this adapter use glab's internal types without modifying glab.
package mwcli

import (
	"fmt"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/utils"
)

const canonicalHost = "gitlab.wikimedia.org"

type Factory = cmdutils.Factory

func NewCmdRoot(f Factory) *cobra.Command {
	return commands.NewCmdRoot(f)
}

func ParseURL(rawURL string) (*url.URL, error) {
	return git.ParseURL(rawURL)
}

// Hostname maps Wikimedia's SSH endpoint to the web/API host.
func Hostname(host string) string {
	if strings.EqualFold(host, "gitlab-ssh.wikimedia.org") {
		return canonicalHost
	}
	return host
}

type factory struct {
	*cmdutils.DefaultFactory
	mu        sync.Mutex
	override  string
	userAgent string
}

// NewFactory preserves Git transport URLs while using the canonical API identity.
func NewFactory(version, userAgent string) (Factory, error) {
	if err := os.MkdirAll(config.ConfigDir(), 0o750); err != nil {
		return nil, fmt.Errorf("create GitLab config directory: %w", err)
	}
	cfg, err := config.ParseDefaultConfig()
	if os.IsNotExist(err) {
		err = cfg.WriteAll()
	}
	if err != nil {
		return nil, fmt.Errorf("read GitLab configuration: %w", err)
	}
	host, _ := cfg.Get("", "host")
	if host == "" {
		if err := cfg.Set("", "host", canonicalHost); err != nil {
			return nil, err
		}
	}
	io := iostreams.New(
		iostreams.WithStdin(os.Stdin, iostreams.IsTerminal(os.Stdin)),
		iostreams.WithStdout(iostreams.NewColorable(os.Stdout), iostreams.IsTerminal(os.Stdout)),
		iostreams.WithStderr(iostreams.NewColorable(os.Stderr), iostreams.IsTerminal(os.Stderr)),
		iostreams.WithPagerCommand(iostreams.PagerCommandFromEnv()),
	)
	if pager, _ := cfg.Get("", "glab_pager"); pager != "" {
		io.SetPager(pager)
	}
	if noPrompt, _ := cfg.Get("", "no_prompt"); noPrompt != "" {
		io.SetPrompt(noPrompt)
	}
	if hyperlinks, _ := cfg.Get("", "display_hyperlinks"); hyperlinks == "true" {
		io.SetDisplayHyperlinks("auto")
	}
	if enabled, found := utils.IsEnvVarEnabled("FORCE_HYPERLINKS"); found {
		if enabled {
			io.SetDisplayHyperlinks("always")
		} else {
			io.SetDisplayHyperlinks("never")
		}
	}
	return &factory{
		DefaultFactory: cmdutils.NewFactory(io, false, cfg, api.BuildInfo{
			Version: version, Platform: runtime.GOOS, Architecture: runtime.GOARCH,
		}),
		userAgent: userAgent,
	}, nil
}

func (f *factory) RepoOverride(repo string) error {
	if parsed, err := glrepo.FromFullName(repo, f.DefaultHostname()); err == nil && Hostname(parsed.RepoHost()) != parsed.RepoHost() {
		repo = "https://" + canonicalHost + "/" + parsed.FullName()
	}
	if err := f.DefaultFactory.RepoOverride(repo); err != nil {
		return err
	}
	f.mu.Lock()
	f.override = repo
	f.mu.Unlock()
	return nil
}

func (f *factory) Remotes() (glrepo.Remotes, error) {
	if Hostname(f.DefaultHostname()) != canonicalHost {
		return f.DefaultFactory.Remotes()
	}
	remotes, err := git.Remotes()
	if err != nil {
		return nil, err
	}
	translateSSH := git.ParseSSHConfig().Translator()
	translated := glrepo.TranslateRemotes(remotes, func(u *url.URL) *url.URL {
		u = translateSSH(u)
		if Hostname(u.Hostname()) == canonicalHost {
			copy := *u
			copy.Host = canonicalHost
			return &copy
		}
		return u
	}, canonicalHost)
	matching := glrepo.Remotes{}
	for _, remote := range translated {
		if remote.RepoHost() == canonicalHost {
			matching = append(matching, remote)
		}
	}
	if len(matching) == 0 {
		return f.DefaultFactory.Remotes()
	}
	sort.Sort(matching)
	return matching, nil
}

func (f *factory) BaseRepo() (glrepo.Interface, error) {
	f.mu.Lock()
	overridden := f.override != ""
	f.mu.Unlock()
	if overridden {
		return f.DefaultFactory.BaseRepo()
	}
	remotes, err := f.Remotes()
	if err != nil {
		return nil, err
	}
	return remotes[0], nil
}

func (f *factory) ApiClient(repoHost string) (*api.Client, error) {
	if repoHost == "" {
		repoHost = f.DefaultHostname()
	}
	userAgent := strings.TrimSpace(f.userAgent + " " + f.BuildInfo().UserAgent())
	return api.NewClientFromConfig(Hostname(repoHost), f.Config(), false, userAgent)
}

func (f *factory) GitLabClient() (*gitlab.Client, error) {
	repo, err := f.BaseRepo()
	host := f.DefaultHostname()
	if err == nil {
		host = repo.RepoHost()
	}
	client, err := f.ApiClient(host)
	if err != nil {
		return nil, err
	}
	return client.Lab(), nil
}
