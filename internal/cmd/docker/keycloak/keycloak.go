package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	mwdd "gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed keycloak.long.md
var mwddKeycloakLong string

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "keycloak",
		Short:   "Keycloak service",
		Long:    cli.RenderMarkdown(mwddKeycloakLong),
		Aliases: []string{"kc"},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Service"
	cmd.AddCommand(mwdd.NewServiceCreateCmd("keycloak"))
	cmd.AddCommand(mwdd.NewServiceDestroyCmd("keycloak"))
	cmd.AddCommand(mwdd.NewServiceStopCmd("keycloak"))
	cmd.AddCommand(mwdd.NewServiceStartCmd("keycloak"))
	cmd.AddCommand(mwdd.NewServiceExecCmd("keycloak", "keycloak"))
	cmd.AddCommand(NewKeycloakAddCmd())
	cmd.AddCommand(NewKeycloakDeleteCmd())
	cmd.AddCommand(NewKeycloakListCmd())
	cmd.AddCommand(NewKeycloakGetCmd())
	return cmd
}

func keycloakLogin() {
	mwdd.DefaultForUser().ExecNoOutput("keycloak", []string{
		"/mwdd/login.sh",
	}, "root")
}