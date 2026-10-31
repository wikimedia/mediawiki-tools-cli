package mwcli

import (
	"fmt"
	"net/http"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/config"
)

// Construct profile authentication explicitly: upstream's config-based constructor
// requires its concrete file configuration and cannot use our profile wrapper.
func newProfileAPIClient(profile *ProfileCredentials, cfg config.Config, userAgent string) (*api.Client, error) {
	token, err := profile.Token()
	if err != nil {
		return nil, err
	}
	proxy, err := api.ProxyFromConfig(cfg, profile.Host)
	if err != nil {
		return nil, err
	}
	options := []api.ClientOption{
		api.WithBaseURL("https://" + profile.Host + "/api/v4/"),
		api.WithUserAgent(userAgent), api.WithProxy(proxy),
	}
	if ca, _ := cfg.Get(profile.Host, "ca_cert"); ca != "" {
		options = append(options, api.WithCustomCA(ca))
	}
	cert, _ := cfg.Get(profile.Host, "client_cert")
	key, _ := cfg.Get(profile.Host, "client_key")
	if cert != "" && key != "" {
		options = append(options, api.WithClientCertificate(cert, key))
	}
	return api.NewClient(func(*http.Client) (gitlab.AuthSource, error) {
		return gitlab.AccessTokenAuthSource{Token: token}, nil
	}, options...)
}

// protectProfileClient covers REST, GraphQL and raw API requests, including
// pagination and redirects, on each newly constructed profile client.
func protectProfileClient(client *http.Client, host string) {
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = profileTransport{host: host, next: transport}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many GitLab redirects")
		}
		if req.URL.Scheme != "https" || !strings.EqualFold(req.URL.Host, host) {
			return fmt.Errorf("GitLab profile request cannot leave its HTTPS host")
		}
		return nil
	}
}

type profileTransport struct {
	host string
	next http.RoundTripper
}

func (t profileTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || !strings.EqualFold(req.URL.Host, t.host) {
		return nil, fmt.Errorf("GitLab profile request cannot leave its HTTPS host")
	}
	return t.next.RoundTrip(req)
}
