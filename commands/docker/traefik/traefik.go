package traefik

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewCmd() *cobra.Command {
	cmd := mwdd.NewServiceCmd("traefik", mwdd.ServiceTexts{
		Long: "Traefik is mwdd's default web proxy. It listens on the configured PORT and routes the local virtual hosts and /proxy paths.",
	}, []string{})
	cmd.Long = "Traefik is mwdd's default web proxy and replaces nginx. It listens on the configured PORT " +
		"(8080 by default) and routes local virtual hosts and /proxy paths. It discovers services through " +
		"Traefik Compose labels and a read-only Docker API proxy. Authentication is enabled " +
		"by default with a cookie-based gateway login; 'mw docker traefik create' generates a password. " +
		"Sessions expire after 12 hours. HTTP Basic credentials remain supported for scripts. " +
		"Set TRAEFIK_AUTH_MIDDLEWARE=forwarded-https@file " +
		"in the context environment to disable authentication. Configure TRAEFIK_BIND_ADDR to change the bind address."

	var gatewayURL string
	credentialsCmd := &cobra.Command{
		Use:   "credentials",
		Short: "Show or generate the gateway login",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(mwdd.DefaultForUser().Directory(), "compose", "traefik", "auth", "credentials")
			if err := mwdd.EnsureTraefikAuth(); err != nil {
				return err
			}
			credentials, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("gateway credentials are not available yet: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(string(credentials)))
			origin := gatewayURL
			if origin == "" {
				port := mwdd.DefaultForUser().Env().Get("PORT")
				if port == "" {
					port = "8080"
				}
				origin = "http://" + net.JoinHostPort("localhost", port)
			}
			link, err := requestLoginLink(origin, string(credentials))
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Could not create a login link: %v\n", err)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nLogin: %s\nValid for 5 minutes; works once.\n", link)
			return nil
		},
	}
	credentialsCmd.Flags().StringVar(&gatewayURL, "url", "", "Gateway URL for the login link (defaults to localhost; accepts an HTTPS tunnel URL)")
	cmd.AddCommand(credentialsCmd)
	return cmd
}

// StartGateway prepares authentication and starts the default Traefik proxy.
func StartGateway() error {
	instance := mwdd.DefaultForUser()
	if err := instance.PrepareGateway(); err != nil {
		return err
	}
	return instance.DockerCompose().Up([]string{"traefik", "docker-socket-proxy"}, dockercompose.UpOptions{Detached: true})
}

// The gateway issues a short-lived code; the CLI never puts the password in a URL.
func requestLoginLink(origin, credentials string) (string, error) {
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("expected a gateway origin such as http://localhost:8080")
	}
	parsed.Path = ""
	origin = parsed.String()
	var username, password string
	for _, line := range strings.Split(credentials, "\n") {
		if value, ok := strings.CutPrefix(line, "Username: "); ok {
			username = value
		}
		if value, ok := strings.CutPrefix(line, "Password: "); ok {
			password = value
		}
	}
	if username == "" || password == "" {
		return "", fmt.Errorf("gateway credentials are incomplete")
	}
	req, err := http.NewRequest(http.MethodPost, origin+"/_mwcli/login-link", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(username, password)
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gateway unavailable; start it with 'mw docker traefik start'")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gateway returned HTTP %d", response.StatusCode)
	}
	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result); err != nil {
		return "", fmt.Errorf("invalid login-link response: %w", err)
	}
	link, err := url.Parse(result.URL)
	if err != nil || link.Scheme != parsed.Scheme || link.Host != parsed.Host || link.User != nil || link.Path != "/_mwcli/login" || link.RawQuery != "" || !strings.HasPrefix(link.Fragment, "login=") || strings.TrimPrefix(link.Fragment, "login=") == "" {
		return "", fmt.Errorf("gateway returned an invalid login link")
	}
	return result.URL, nil
}
