package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

//go:embed keycloak.long.md
var devKeycloakLong string

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "keycloak",
		GroupID: "service",
		Short:   "Keycloak service",
		Long:    cli.RenderMarkdown(devKeycloakLong),
		Aliases: []string{"kc"},
	}
	cmd.AddCommand(devenv.NewImageCmd("keycloak"))
	cmd.AddCommand(devenv.NewServiceCreateCmd("keycloak", ""))
	cmd.AddCommand(devenv.NewServiceDestroyCmd("keycloak"))
	cmd.AddCommand(devenv.NewServiceStopCmd("keycloak"))
	cmd.AddCommand(devenv.NewServiceStartCmd("keycloak"))
	cmd.AddCommand(devenv.NewServiceExecCmd("keycloak", "keycloak"))
	cmd.AddCommand(NewKeycloakAddCmd())
	cmd.AddCommand(NewKeycloakDeleteCmd())
	cmd.AddCommand(NewKeycloakListCmd())
	cmd.AddCommand(NewKeycloakGetCmd())
	return cmd
}

func keycloakLogin() {
	_, _, err := devenv.DefaultForUser().DockerCompose().ExecCommand("keycloak", dockercompose.ExecOptions{
		User:           "root",
		CommandAndArgs: []string{"/mwdd/login.sh"},
	}).RunAndCollect()
	if err != nil {
		panic(err)
	}
}
