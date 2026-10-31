package mwcli

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
)

// ProfileCredentials supplies a host-bound identity without persisting its token
// in glab configuration. Token should cache a successful read for the invocation.
type ProfileCredentials struct {
	Host, Username string
	Token          func() (string, error)
}

type CredentialResolver func() (*ProfileCredentials, error)

func NewProfileFactory(version, userAgent string, resolve CredentialResolver, options ...FactoryOption) (Factory, error) {
	return newFactory(version, userAgent, resolve, options...)
}

type profileConfig struct {
	config.Config
	resolve CredentialResolver
}

func (c *profileConfig) Get(host, key string) (string, error) {
	value, _, err := c.GetWithSource(host, key, true)
	return value, err
}

func (c *profileConfig) GetWithSource(host, key string, env bool) (string, string, error) {
	profile, err := c.resolve()
	if err != nil {
		return "", "", err
	}
	if profile == nil {
		return c.Config.GetWithSource(host, key, env)
	}
	key = config.ConfigKeyEquivalence(key)
	switch key {
	case "token":
		if !strings.EqualFold(Hostname(host), profile.Host) {
			return "", "", fmt.Errorf("selected GitLab profile belongs to %s, not %s", profile.Host, host)
		}
		token, err := profile.Token()
		return token, "a-wikimedia-cli profile", err
	case "job_token", "oauth2_refresh_token", "oauth2_expiry_date", "oauth2_code_verifier":
		return "", "a-wikimedia-cli profile", nil
	case "is_oauth2", "skip_tls_verify":
		return "false", "a-wikimedia-cli profile", nil
	case "api_host":
		// An upstream override must not redirect a profile token to another server.
		return Hostname(host), "a-wikimedia-cli profile", nil
	case "api_protocol":
		return "https", "a-wikimedia-cli profile", nil
	case "user":
		if strings.EqualFold(Hostname(host), profile.Host) {
			return profile.Username, "a-wikimedia-cli profile", nil
		}
	}
	return c.Config.GetWithSource(host, key, env)
}

func (c *profileConfig) Set(host, key, value string) error {
	profile, err := c.resolve()
	if err != nil {
		return err
	}
	if profile != nil && IsCredentialKey(key) {
		return fmt.Errorf("manage profile credentials with gitlab auth login")
	}
	return c.Config.Set(host, key, value)
}

func (c *profileConfig) Hosts() ([]string, error) {
	hosts, err := c.Config.Hosts()
	if err != nil {
		return nil, err
	}
	profile, err := c.resolve()
	if err != nil {
		return nil, err
	}
	if profile != nil {
		for _, host := range hosts {
			if host == profile.Host {
				return hosts, nil
			}
		}
		hosts = append(hosts, profile.Host)
		sort.Strings(hosts)
	}
	return hosts, nil
}

func IsCredentialKey(key string) bool {
	key = config.ConfigKeyEquivalence(key)
	return key == "token" || key == "job_token" || key == "is_oauth2" || strings.HasPrefix(key, "oauth2_")
}

// LegacyToken is an explicit import operation. Environment tokens and expiring
// OAuth credentials are not imported as permanent personal access tokens.
func LegacyToken(host string) (string, error) {
	cfg, err := config.ParseDefaultConfig()
	if err != nil {
		return "", fmt.Errorf("could not read glab configuration")
	}
	oauth, _, err := cfg.GetWithSource(host, "is_oauth2", false)
	if err != nil {
		return "", fmt.Errorf("could not read glab authentication type")
	}
	if oauth == "true" {
		return "", fmt.Errorf("glab OAuth sessions cannot be imported yet; use a personal access token")
	}
	token, _, err := cfg.GetWithSource(host, "token", false)
	if err != nil || token == "" {
		return "", fmt.Errorf("no saved glab token for %s", host)
	}
	return token, nil
}

// LegacyGitToken preserves the pre-profile Git helper's environment/config lookup.
func LegacyGitToken(host string) (string, error) {
	cfg, err := config.ParseDefaultConfig()
	if err != nil {
		return "", fmt.Errorf("could not read glab configuration")
	}
	token, err := cfg.Get(host, "token")
	if err != nil || token == "" {
		return "", fmt.Errorf("no GitLab credential for this host")
	}
	return token, nil
}

// OverrideAPIHost lets `api --hostname` choose its client independently of the
// current checkout. Repository metadata is still available for placeholders.
func OverrideAPIHost(f cmdutils.Factory, host string) (func(), error) {
	adapted, ok := f.(*factory)
	if !ok {
		return nil, fmt.Errorf("GitLab API host override requires the profile adapter")
	}
	adapted.mu.Lock()
	previous := adapted.apiHost
	adapted.apiHost = host
	adapted.mu.Unlock()
	return func() { adapted.mu.Lock(); adapted.apiHost = previous; adapted.mu.Unlock() }, nil
}

// ValidateRepositoryHost checks the repository target without reading a token.
func ValidateRepositoryHost(f cmdutils.Factory, host string) error {
	adapted, ok := f.(*factory)
	if !ok {
		return fmt.Errorf("GitLab repository validation requires the profile adapter")
	}
	_, err := adapted.profileRepositoryHost(host)
	return err
}

func (f *factory) profileRepositoryHost(expected string) (string, error) {
	f.mu.Lock()
	override := f.override
	f.mu.Unlock()
	repo, err := f.BaseRepo()
	if err != nil {
		if override != "" {
			return "", err
		}
		return expected, nil
	}
	host := repo.RepoHost()
	explicitHTTP := false
	if parsed, err := url.Parse(override); err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") {
		// glrepo.RepoHost drops ports. Preserve an explicitly supplied web URL.
		host = parsed.Host
		explicitHTTP = true
		if parsed.Scheme != "https" {
			return "", fmt.Errorf("GitLab profile repository URL must use HTTPS")
		}
	}
	profileURL, _ := url.Parse("https://" + expected)
	if !explicitHTTP && profileURL.Port() != "" && strings.EqualFold(host, profileURL.Hostname()) {
		host = expected
	}
	if !strings.EqualFold(host, expected) {
		return "", fmt.Errorf("selected GitLab profile belongs to %s, not %s", expected, host)
	}
	return expected, nil
}
