package traefik

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		"by default; 'mw docker traefik create' generates a password. Set TRAEFIK_AUTH_MIDDLEWARE=forwarded-https@file " +
		"in the context environment to disable authentication. Configure TRAEFIK_BIND_ADDR to change the bind address."

	cmd.AddCommand(&cobra.Command{
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
			fmt.Print(strings.TrimSpace(string(credentials)) + "\n")
			return nil
		},
	})
	return cmd
}

// StartGateway prepares authentication and starts the default Traefik proxy.
func StartGateway() error {
	instance := mwdd.DefaultForUser()
	instance.EnsureReady()
	if err := mwdd.EnsureTraefikAuth(); err != nil {
		return err
	}
	return instance.DockerCompose().Up([]string{"traefik", "docker-socket-proxy"}, dockercompose.UpOptions{Detached: true})
}
