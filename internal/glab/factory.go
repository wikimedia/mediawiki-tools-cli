// Package mwcli adapts glab's repository detection to Wikimedia's separate SSH host.
// The nested module lets this adapter use glab's internal types without modifying glab.
package mwcli

import (
	"net/url"
	"sort"
	"strings"
	"sync"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/pkg/git"
	"gitlab.com/gitlab-org/cli/pkg/glinstance"
	"gitlab.com/gitlab-org/cli/pkg/iostreams"
)

const canonicalHost = "gitlab.wikimedia.org"

// Hostname maps Wikimedia's SSH endpoint to the web/API host.
func Hostname(host string) string {
	if strings.EqualFold(host, "gitlab-ssh.wikimedia.org") {
		return canonicalHost
	}
	return host
}

type factory struct {
	*cmdutils.DefaultFactory
	mu       sync.Mutex
	override string
}

// NewFactory preserves Git transport URLs while using the canonical API identity.
func NewFactory(io *iostreams.IOStreams) cmdutils.Factory {
	return &factory{DefaultFactory: cmdutils.NewFactory(io, false)}
}

func (f *factory) RepoOverride(repo string) {
	if parsed, err := glrepo.FromFullName(repo); err == nil && Hostname(parsed.RepoHost()) != parsed.RepoHost() {
		repo = "https://" + canonicalHost + "/" + parsed.FullName()
	}
	f.mu.Lock()
	f.override = repo
	f.mu.Unlock()
	f.DefaultFactory.RepoOverride(repo)
}

func (f *factory) Remotes() (glrepo.Remotes, error) {
	if Hostname(glinstance.OverridableDefault()) != canonicalHost {
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
	})
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

func (f *factory) HttpClient() (*gitlab.Client, error) {
	repo, err := f.BaseRepo()
	if err != nil {
		return f.DefaultFactory.HttpClient()
	}
	cfg, err := f.Config()
	if err != nil {
		return nil, err
	}
	return cmdutils.LabClientFunc(repo.RepoHost(), cfg, false)
}
