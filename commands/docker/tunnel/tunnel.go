package tunnel

import (
	"fmt"
	"regexp"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/traefik"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

var quickTunnelURL = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tunnel",
		Short: "Expose the development environment through a temporary public tunnel",
		Long: "Manage a Cloudflare Quick Tunnel for the development environment. The tunnel forwards " +
			"to the optional Traefik gateway, which routes requests to the configured development services.",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start a temporary public URL for the development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := traefik.StartGateway(); err != nil {
				return fmt.Errorf("start the Traefik gateway: %w", err)
			}
			project := mwdd.DefaultForUser().DockerCompose()
			if err := project.Up([]string{"cloudflared"}, dockercompose.UpOptions{Detached: true}); err != nil {
				return fmt.Errorf("start Cloudflare Quick Tunnel: %w", err)
			}

			deadline := time.Now().Add(45 * time.Second)
			for time.Now().Before(deadline) {
				if url, _, err := readURL(project); err == nil && url != "" {
					fmt.Printf("Cloudflare Quick Tunnel is available at %s\n", url)
					fmt.Println("The Traefik gateway login is required. Show it with 'mw docker traefik credentials'.")
					return nil
				}
				time.Sleep(time.Second)
			}
			_, logs, err := readURL(project)
			if err != nil {
				return err
			}
			return fmt.Errorf("cloudflared started but did not report a Quick Tunnel URL within 45 seconds; check 'mw docker tunnel status' and logs:\n%s", logs)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop the public tunnel",
		RunE: func(cmd *cobra.Command, args []string) error {
			instance := mwdd.DefaultForUser()
			instance.EnsureReady()
			return instance.DockerCompose().Stop([]string{"cloudflared"})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show whether the public tunnel is running",
		RunE: func(cmd *cobra.Command, args []string) error {
			instance := mwdd.DefaultForUser()
			instance.EnsureReady()
			project := instance.DockerCompose()
			services, err := project.ServicesWithStatus("running")
			if err != nil {
				return err
			}
			for _, service := range services {
				if service == "cloudflared" {
					if url, _, err := readURL(project); err == nil && url != "" {
						fmt.Printf("Cloudflare Quick Tunnel is running at %s\n", url)
					} else {
						fmt.Println("Cloudflare Quick Tunnel is running; its URL is not available in the container logs yet.")
					}
					return nil
				}
			}
			fmt.Println("Cloudflare Quick Tunnel is stopped.")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "url",
		Short: "Print the current public tunnel URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			instance := mwdd.DefaultForUser()
			instance.EnsureReady()
			url, _, err := readURL(instance.DockerCompose())
			if err != nil {
				return err
			}
			if url == "" {
				return fmt.Errorf("no Quick Tunnel URL found; start it with 'mw docker tunnel start'")
			}
			fmt.Println(url)
			return nil
		},
	})
	var follow bool
	var tail string
	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Show or follow Cloudflare tunnel logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			instance := mwdd.DefaultForUser()
			instance.EnsureReady()
			composeArgs := []string{"logs", "--no-color", "--tail", tail}
			if follow {
				composeArgs = append(composeArgs, "--follow")
			}
			composeArgs = append(composeArgs, "cloudflared")
			return instance.DockerCompose().Command(composeArgs).RunAttached()
		},
	}
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().StringVar(&tail, "tail", "100", "Number of recent log lines to show")
	cmd.AddCommand(logsCmd)
	return cmd
}

func readURL(project dockercompose.Project) (string, string, error) {
	stdout, stderr, err := project.Command([]string{"logs", "--no-color", "--tail", "100", "cloudflared"}).RunAndCollect()
	logs := stdout.String() + stderr.String()
	if err != nil {
		return "", logs, err
	}
	return quickTunnelURL.FindString(logs), logs, nil
}
