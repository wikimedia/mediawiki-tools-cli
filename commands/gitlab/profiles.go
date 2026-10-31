package gitlab

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"

	"gitlab.com/gitlab-org/cli/mwcli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

const (
	gitlabEndpoint     = "https://gitlab.wikimedia.org"
	profileEnvironment = "MWCLI_GITLAB_PROFILE"
)

var gitlabProfiles = authprofile.Manager{Service: "gitlab", Label: "GitLab", SecretField: "token", Endpoint: gitlabEndpoint}

type profileSession struct {
	selected *string
	mu       sync.Mutex
	cached   map[string]*mwcli.ProfileCredentials
}

func (s *profileSession) name() (string, error) {
	name := strings.TrimSpace(*s.selected)
	if name == "" {
		name = os.Getenv(profileEnvironment)
	}
	if name == "" && gitlabProfiles.Default() == "" {
		return "", nil
	}
	return gitlabProfiles.Selected(name)
}

func profileEndpoint(name string) (string, error) {
	endpoint := config.State().EffectiveKoanf.String(gitlabProfiles.Prefix(name) + ".url")
	if endpoint == "" {
		endpoint = gitlabEndpoint
	}
	return normalizeGitlabEndpoint(endpoint)
}

func normalizeGitlabEndpoint(raw string) (string, error) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	endpoint, err := secrets.Endpoint(raw)
	if err != nil {
		return "", fmt.Errorf("provide an HTTPS GitLab hostname")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Path != "" {
		return "", fmt.Errorf("provide an HTTPS GitLab hostname without a path")
	}
	u.Host = mwcli.Hostname(u.Host)
	return u.String(), nil
}

func gitlabCredential(name string) (config.Credential, error) {
	endpoint, err := profileEndpoint(name)
	if err != nil {
		return config.Credential{}, err
	}
	return config.Credential{Key: gitlabProfiles.Prefix(name) + ".token", Service: "gitlab", Endpoint: endpoint, Account: config.State().EffectiveKoanf.String(gitlabProfiles.Prefix(name) + ".username")}, nil
}

func (s *profileSession) credentials() (*mwcli.ProfileCredentials, error) {
	name, err := s.name()
	if err != nil || name == "" {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached := s.cached[name]; cached != nil {
		return cached, nil
	}
	credential, err := gitlabCredential(name)
	if err != nil {
		return nil, err
	}
	if credential.Account == "" {
		return nil, fmt.Errorf("GitLab profile %q has no username; use auth login", name)
	}
	endpoint, _ := url.Parse(credential.Endpoint)
	var once sync.Once
	var token string
	var readErr error
	profile := &mwcli.ProfileCredentials{Host: endpoint.Host, Username: credential.Account, Token: func() (string, error) {
		once.Do(func() {
			token, readErr = credential.Read()
			if readErr != nil {
				readErr = fmt.Errorf("could not read GitLab profile %q; unlock its credential provider or use auth login", name)
			}
			if readErr == nil && token == "" {
				readErr = fmt.Errorf("GitLab profile %q has no saved token", name)
			}
		})
		return token, readErr
	}}
	if s.cached == nil {
		s.cached = map[string]*mwcli.ProfileCredentials{}
	}
	s.cached[name] = profile
	return profile, nil
}
