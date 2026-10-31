// Package mwcli adapts glab's repository detection to Wikimedia's separate SSH host.
// The nested module lets this adapter use glab's internal types without modifying glab.
package mwcli

import (
	"fmt"
	"net/http"
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
	mu         sync.Mutex
	override   string
	apiHost    string
	userAgent  string
	resolve    CredentialResolver
	httpClient *http.Client
	streams    *IOStreams
}

type IOStreams = iostreams.IOStreams

type FactoryOption func(*factory)

func WithIOStreams(streams *IOStreams) FactoryOption {
	return func(f *factory) { f.streams = streams }
}

func WithHTTPClient(client *http.Client) FactoryOption {
	return func(f *factory) { f.httpClient = client }
}

// NewFactory preserves Git transport URLs while using the canonical API identity.
func NewFactory(version, userAgent string, options ...FactoryOption) (Factory, error) {
	return newFactory(version, userAgent, nil, options...)
}

func newFactory(version, userAgent string, resolve CredentialResolver, options ...FactoryOption) (Factory, error) {
	f := &factory{userAgent: userAgent, resolve: resolve}
	for _, option := range options {
		option(f)
	}

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
	if resolve != nil {
		cfg = &profileConfig{Config: cfg, resolve: resolve}
	}
	streams := f.streams
	if streams == nil {
		streams = iostreams.New(
			iostreams.WithStdin(os.Stdin, iostreams.IsTerminal(os.Stdin)),
			iostreams.WithStdout(iostreams.NewColorable(os.Stdout), iostreams.IsTerminal(os.Stdout)),
			iostreams.WithStderr(iostreams.NewColorable(os.Stderr), iostreams.IsTerminal(os.Stderr)),
			iostreams.WithPagerCommand(iostreams.PagerCommandFromEnv()),
		)
	}
	if pager, _ := cfg.Get("", "glab_pager"); pager != "" {
		streams.SetPager(pager)
	}
	if noPrompt, _ := cfg.Get("", "no_prompt"); noPrompt != "" {
		streams.SetPrompt(noPrompt)
	}
	if hyperlinks, _ := cfg.Get("", "display_hyperlinks"); hyperlinks == "true" {
		streams.SetDisplayHyperlinks("auto")
	}
	if enabled, found := utils.IsEnvVarEnabled("FORCE_HYPERLINKS"); found {
		if enabled {
			streams.SetDisplayHyperlinks("always")
		} else {
			streams.SetDisplayHyperlinks("never")
		}
	}
	f.DefaultFactory = cmdutils.NewFactory(streams, false, cfg, api.BuildInfo{
		Version: version, Platform: runtime.GOOS, Architecture: runtime.GOARCH,
	})
	return f, nil
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
			translatedURL := *u
			translatedURL.Host = canonicalHost
			return &translatedURL
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
	repoHost = Hostname(repoHost)
	var profile *ProfileCredentials
	if f.resolve != nil {
		var err error
		profile, err = f.resolve()
		if err != nil {
			return nil, err
		}
		if profile != nil {
			if !strings.EqualFold(repoHost, profile.Host) {
				return nil, fmt.Errorf("selected GitLab profile belongs to %s, not %s", profile.Host, repoHost)
			}
			if _, err := profile.Token(); err != nil {
				return nil, err
			}
		}
	}
	userAgent := strings.TrimSpace(f.userAgent + " " + f.BuildInfo().UserAgent())
	cfg := f.Config()
	if wrapped, ok := cfg.(*profileConfig); ok {
		cfg = wrapped.Config
	}
	var client *api.Client
	var err error
	if profile == nil {
		client, err = api.NewClientFromConfig(repoHost, cfg, false, userAgent)
	} else {
		client, err = newProfileAPIClient(profile, cfg, userAgent)
	}
	if err != nil {
		return nil, err
	}
	if f.httpClient != nil {
		*client.HTTPClient() = *f.httpClient
	}
	if profile != nil {
		protectProfileClient(client.HTTPClient(), profile.Host)
	}
	return client, nil
}

func (f *factory) GitLabClient() (*gitlab.Client, error) {
	repo, repoErr := f.BaseRepo()
	host := f.DefaultHostname()
	if repoErr == nil {
		host = repo.RepoHost()
	}
	f.mu.Lock()
	if f.apiHost != "" {
		host = f.apiHost
	}
	f.mu.Unlock()
	if f.resolve != nil {
		profile, err := f.resolve()
		if err != nil {
			return nil, err
		}
		if profile != nil {
			f.mu.Lock()
			apiHost := f.apiHost
			f.mu.Unlock()
			if apiHost != "" {
				host = apiHost
			} else {
				host, err = f.profileRepositoryHost(profile.Host)
				if err != nil {
					return nil, err
				}
			}
			if !strings.EqualFold(host, profile.Host) {
				return nil, fmt.Errorf("selected GitLab profile belongs to %s, not %s", profile.Host, host)
			}
			if _, err := profile.Token(); err != nil {
				return nil, err
			}
		}
	}
	client, err := f.ApiClient(host)
	if err != nil {
		return nil, err
	}
	return client.Lab(), nil
}
