package traefik

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewCmd() *cobra.Command {
	cmd := devenv.NewServiceCmd("traefik", devenv.ServiceTexts{
		Long: "Traefik is the development environment's default web proxy. It listens on the configured PORT and routes the local virtual hosts and /proxy paths.",
	}, []string{})
	cmd.Long = "Traefik is the development environment's default web proxy. It listens on the configured PORT " +
		"(8080 by default) and routes local virtual hosts and /proxy paths. It discovers services through " +
		"Traefik Compose labels and a read-only Docker API proxy. Authentication is enabled " +
		"by default with a cookie-based gateway login; 'mw dev traefik create' generates a password. " +
		"Sessions expire after 12 hours. HTTP Basic credentials remain supported for scripts. " +
		"Set TRAEFIK_AUTH_MIDDLEWARE=forwarded-https@file " +
		"in the context environment to disable authentication. Configure TRAEFIK_BIND_ADDR to change the bind address."

	var gatewayURL string
	credentialsCmd := &cobra.Command{
		Use:   "credentials",
		Short: "Show gateway credentials and single-use login links",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(devenv.DefaultForUser().Directory(), "compose", "traefik", "auth", "credentials")
			if err := devenv.EnsureTraefikAuth(); err != nil {
				return err
			}
			credentials, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("gateway credentials are not available yet: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(string(credentials)))
			origin := gatewayURL
			if origin == "" {
				port := devenv.DefaultForUser().Env().Get("PORT")
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
			fmt.Fprintf(cmd.OutOrStdout(), "\nLogin: %s\n", link)
			if gatewayURL == "" {
				if tunnel := runningTunnelURL(origin, string(credentials)); tunnel != "" {
					tunnelLink, err := requestLoginLink(tunnel, string(credentials))
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Could not create a tunnel login link: %v\n", err)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "Tunnel login: %s\n", tunnelLink)
					}
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Each link is valid for 5 minutes and works once.")
			return nil
		},
	}
	credentialsCmd.Flags().StringVar(&gatewayURL, "url", "", "Use only this gateway URL instead of localhost and the active tunnel")
	cmd.AddCommand(credentialsCmd)
	return cmd
}

// StartGateway prepares authentication and starts the default Traefik proxy.
func StartGateway() error {
	instance := devenv.DefaultForUser()
	services := []string{"traefik", "docker-socket-proxy"}
	return instance.ChangeServices(context.Background(), services, devenv.ServiceStarted, func() error {
		if err := instance.PrepareGateway(); err != nil {
			return err
		}
		return instance.DockerCompose().Up(services, dockercompose.UpOptions{Detached: true})
	})
}

func gatewayRequest(method, origin, path, credentials string) (*http.Request, error) {
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("expected a gateway origin such as http://localhost:8080")
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
		return nil, fmt.Errorf("gateway credentials are incomplete")
	}
	req, err := http.NewRequest(method, origin+path, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, password)
	return req, nil
}

// The gateway issues a short-lived code; the CLI never puts the password in a URL.
func requestLoginLink(origin, credentials string) (string, error) {
	link, status, err := requestLoginLinkAt(origin, credentials, "/_wm")
	if status == http.StatusNotFound {
		// Existing contexts may still run an older gateway until restarted.
		link, _, err = requestLoginLinkAt(origin, credentials, "/_mwcli")
	}
	return link, err
}

func requestLoginLinkAt(origin, credentials, prefix string) (string, int, error) {
	req, err := gatewayRequest(http.MethodPost, origin, prefix+"/login-link", credentials)
	if err != nil {
		return "", 0, err
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("gateway unavailable; start it with the CLI's dev traefik start command")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", response.StatusCode, fmt.Errorf("gateway returned HTTP %d", response.StatusCode)
	}
	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result); err != nil {
		return "", response.StatusCode, fmt.Errorf("invalid login-link response: %w", err)
	}
	link, err := url.Parse(result.URL)
	if err != nil || link.Scheme != req.URL.Scheme || link.Host != req.URL.Host || link.User != nil || link.Path != prefix+"/login" || link.RawQuery != "" || !strings.HasPrefix(link.Fragment, "login=") || strings.TrimPrefix(link.Fragment, "login=") == "" {
		return "", response.StatusCode, fmt.Errorf("gateway returned an invalid login link")
	}
	return result.URL, response.StatusCode, nil
}

// Reuse the dashboard's readiness check instead of advertising old tunnel logs.
func runningTunnelURL(origin, credentials string) string {
	req, err := gatewayRequest(http.MethodGet, origin, "/proxy/dashboard/api/status", credentials)
	if err != nil {
		return ""
	}
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	var status struct {
		TunnelURL string `json:"tunnel_url"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&status) != nil || !activeQuickTunnelURL.MatchString(status.TunnelURL) {
		return ""
	}
	return status.TunnelURL
}

var activeQuickTunnelURL = regexp.MustCompile(`^https://[a-zA-Z0-9-]+\.trycloudflare\.com$`)
