package tunnel

import (
	"context"
	"fmt"
	osexec "os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/traefik"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

type provider struct {
	name, service, label string
	endpoint             *regexp.Regexp
}

var providers = []provider{
	{"cloudflare", "cloudflared", "Cloudflare Quick Tunnel", regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)},
	{"holesail", "holesail", "Holesail tunnel", regexp.MustCompile(`hs://s000[a-fA-F0-9]{64}`)},
}

func selectProvider(flag, configured string) (provider, error) {
	name := flag
	if name == "" {
		name = configured
	}
	if name == "" {
		name = "cloudflare"
	}
	for _, candidate := range providers {
		if candidate.name == name {
			return candidate, nil
		}
	}
	return provider{}, fmt.Errorf("unknown tunnel provider %q; choose cloudflare or holesail", name)
}

func NewCmd() *cobra.Command {
	var providerFlag string
	cmd := &cobra.Command{
		Use:   "tunnel",
		Short: "Connect to the development environment through an optional tunnel",
		Long: "Manage a Cloudflare Quick Tunnel or a private Holesail connection to the Traefik gateway. " +
			"Cloudflare gives a public HTTPS URL; Holesail needs a client on the connecting machine. " +
			"Select with --provider or the context's TUNNEL_PROVIDER variable (default: cloudflare). " +
			"A successful start remembers the provider for status, url, logs and stop. " +
			"Gateway authentication remains enabled by default with either provider.",
	}
	cmd.PersistentFlags().StringVar(&providerFlag, "provider", "", "Tunnel provider: cloudflare or holesail (defaults to TUNNEL_PROVIDER)")
	_ = cmd.RegisterFlagCompletionFunc("provider", cobra.FixedCompletions([]string{"cloudflare", "holesail"}, cobra.ShellCompDirectiveNoFileComp))
	selected := func() (mwdd.MWDD, provider, error) {
		instance := mwdd.DefaultForUser()
		instance.EnsureReady()
		p, err := selectProvider(providerFlag, instance.Env().Get("TUNNEL_PROVIDER"))
		return instance, p, err
	}
	cmd.AddCommand(&cobra.Command{
		Use:     "start",
		Short:   "Start a tunnel to the development environment",
		Args:    cobra.NoArgs,
		Example: "start\nstart --provider holesail",
		RunE: func(cmd *cobra.Command, args []string) error {
			instance, p, err := selected()
			if err != nil {
				return err
			}
			if err := traefik.StartGateway(); err != nil {
				return fmt.Errorf("start the Traefik gateway: %w", err)
			}
			project := instance.DockerCompose()
			if err := project.Up([]string{p.service}, dockercompose.UpOptions{Detached: true}); err != nil {
				return fmt.Errorf("start %s: %w", p.label, err)
			}
			endpoint, err := waitForEndpoint(cmd.Context(), project, p, 90*time.Second)
			if err != nil {
				return err
			}
			// Close the old exposure only once the replacement reports readiness.
			if err := stopOtherProviders(project, p); err != nil {
				return err
			}
			instance.Env().Set("TUNNEL_PROVIDER", p.name)
			fmt.Fprintf(cmd.OutOrStdout(), "%s is available at %s\n", p.label, endpoint)
			if p.name == "holesail" {
				fmt.Fprintf(cmd.OutOrStdout(), "On your computer, run:\n  holesail %s --host 127.0.0.1 --port 8082\nThen open http://127.0.0.1:8082/proxy/dashboard/\n", endpoint)
				fmt.Fprintln(cmd.OutOrStdout(), "Treat this connection string as a secret. A Holesail client is required; it is not a public HTTPS URL.")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Show the gateway login with 'mw docker traefik credentials' (authentication is enabled by default).")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop tunnels while leaving the gateway running",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			instance, p, err := selected()
			if err != nil {
				return err
			}
			project := instance.DockerCompose()
			if providerFlag != "" {
				return project.Stop([]string{p.service})
			}
			// Stop all known providers even if the default was changed by hand.
			running, err := project.ServicesWithStatus("running")
			if err != nil {
				return err
			}
			var services []string
			for _, candidate := range providers {
				if slices.Contains(running, candidate.service) {
					services = append(services, candidate.service)
				}
			}
			if len(services) == 0 {
				return nil
			}
			return project.Stop(services)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show whether the selected tunnel is running",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			instance, p, err := selected()
			if err != nil {
				return err
			}
			running, err := instance.DockerCompose().ServicesWithStatus("running")
			if err != nil {
				return err
			}
			if !slices.Contains(running, p.service) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s is stopped.\n", p.label)
				return nil
			}
			endpoint, _, err := readEndpoint(cmd.Context(), instance.DockerCompose(), p)
			if err != nil || endpoint == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s is running; its connection details are not available yet.\n", p.label)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s is running at %s\n", p.label, endpoint)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "url",
		Short: "Print the current tunnel URL or Holesail connection string",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			instance, p, err := selected()
			if err != nil {
				return err
			}
			endpoint, _, err := readEndpoint(cmd.Context(), instance.DockerCompose(), p)
			if err != nil {
				return err
			}
			if endpoint == "" {
				return fmt.Errorf("no %s connection found; start it with 'mw docker tunnel start --provider %s'", p.label, p.name)
			}
			fmt.Fprintln(cmd.OutOrStdout(), endpoint)
			return nil
		},
	})
	var follow bool
	var tail string
	logs := &cobra.Command{
		Use:   "logs",
		Short: "Show or follow the selected tunnel's logs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			instance, p, err := selected()
			if err != nil {
				return err
			}
			composeArgs := []string{"logs", "--no-color", "--tail", tail}
			if follow {
				composeArgs = append(composeArgs, "--follow")
			}
			return instance.DockerCompose().Command(append(composeArgs, p.service)).RunAttached()
		},
	}
	logs.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	logs.Flags().StringVar(&tail, "tail", "100", "Number of recent log lines to show")
	cmd.AddCommand(logs)
	return cmd
}

func stopOtherProviders(project dockercompose.Project, selected provider) error {
	running, err := project.ServicesWithStatus("running")
	if err != nil {
		return err
	}
	for _, candidate := range providers {
		if candidate.name != selected.name && slices.Contains(running, candidate.service) {
			if err := project.Stop([]string{candidate.service}); err != nil {
				return fmt.Errorf("replacement started, but could not stop %s; stop it explicitly with --provider %s: %w", candidate.label, candidate.name, err)
			}
		}
	}
	return nil
}

func waitForEndpoint(ctx context.Context, project dockercompose.Project, p provider, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var logs string
	for {
		endpoint, latest, err := readEndpoint(ctx, project, p)
		logs = latest
		if err == nil && endpoint != "" {
			return endpoint, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("%s did not report a connection before the deadline; check 'mw docker tunnel logs --provider %s': %w\n%s", p.label, p.name, ctx.Err(), logs)
		case <-time.After(time.Second):
		}
	}
}

func readEndpoint(ctx context.Context, project dockercompose.Project, p provider) (string, string, error) {
	ids, stderr, err := project.Command([]string{"ps", "-q", p.service}).RunAndCollect()
	if err != nil {
		return "", "", fmt.Errorf("find %s container: %w: %s", p.label, err, stderr.String())
	}
	id := strings.TrimSpace(ids.String())
	if id == "" {
		return "", "", fmt.Errorf("%s is stopped; start it with 'mw docker tunnel start --provider %s'", p.label, p.name)
	}
	// Logs survive container restarts. Only return a connection from this start,
	// otherwise old Cloudflare URLs or Holesail keys can be reported as ready.
	started, err := osexec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.StartedAt}}", id).Output()
	if err != nil {
		return "", "", fmt.Errorf("inspect %s: %w", p.label, err)
	}
	stdout, stderr, err := project.Command([]string{"logs", "--no-color", "--since", strings.TrimSpace(string(started)), "--tail", "100", p.service}).RunAndCollect()
	logs := stdout.String() + stderr.String()
	if err != nil {
		return "", logs, err
	}
	return endpointFromLogs(p, logs), logs, nil
}

func endpointFromLogs(p provider, logs string) string {
	matches := p.endpoint.FindAllString(logs, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}
