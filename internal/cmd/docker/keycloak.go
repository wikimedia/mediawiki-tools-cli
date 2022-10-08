package docker

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	mwdd "gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed long/mwdd_keycloak.md
var mwddKeycloakLong string

func NewKeycloakCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "keycloak",
		Short:   "Keycloak service",
		Long:    cli.RenderMarkdown(mwddKeycloakLong),
		Aliases: []string{"kc"},
	}
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

func NewKeycloakAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a keycloak realm, client, or user",
	}
	cmd.AddCommand(NewKeycloakAddRealmCmd())
	cmd.AddCommand(NewKeycloakAddClientCmd())
	cmd.AddCommand(NewKeycloakAddUserCmd())
	return cmd
}

func NewKeycloakAddRealmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "realm <realmname>",
		Short: "Add a keycloak realm",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"create",
				"realms",
				"--set", "enabled=true",
				"--set", "realm=" + args[0],
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakAddClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client <clientname> <realmname>",
		Short: "Add a keycloak client to a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/mwdd/create_client.sh",
				args[0],
				args[1],
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakAddUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <username> <password> <realmname>",
		Short: "Add a keycloak user to a realm with a temporary password",
		Args:  cobra.MinimumNArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/mwdd/create_user.sh",
				args[0],
				args[1],
				args[2],
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete keycloak realm, client, or user",
	}
	cmd.AddCommand(NewKeycloakDeleteRealmCmd())
	cmd.AddCommand(NewKeycloakDeleteClientCmd())
	cmd.AddCommand(NewKeycloakDeleteUserCmd())
	return cmd
}

func NewKeycloakDeleteRealmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "realm <realmname>",
		Short: "Delete keycloak realm",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"delete",
				"realms/" + args[0],
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakDeleteClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client <clientname> <realmname>",
		Short: "Delete keycloak client in a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/mwdd/delete_client.sh",
				args[0],
				args[1],
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakDeleteUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <username> <realmname>",
		Short: "Delete keycloak user in a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/mwdd/delete_user.sh",
				args[0],
				args[1],
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List keycloak realms, clients, or users",
	}
	cmd.AddCommand(NewKeycloakListRealmsCmd())
	cmd.AddCommand(NewKeycloakListClientsCmd())
	cmd.AddCommand(NewKeycloakListUsersCmd())
	return cmd
}

func NewKeycloakListRealmsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "realms",
		Short: "List keycloak realms",
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"get",
				"realms",
				"--fields", "realm",
				"--format", "csv",
				"--noquotes",
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakListClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients <realmname>",
		Short: "List keycloak clients in a realm",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"get",
				"clients",
				"--target-realm", args[0],
				"--fields", "clientId",
				"--format", "csv",
				"--noquotes",
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakListUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users <realmname>",
		Short: "List keycloak users in a realm",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"get",
				"users",
				"--target-realm", args[0],
				"--fields", "username",
				"--format", "csv",
				"--noquotes",
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get metadata for keycloak realm, client, or user",
	}
	cmd.AddCommand(NewKeycloakGetRealmCmd())
	cmd.AddCommand(NewKeycloakGetClientCmd())
	cmd.AddCommand(NewKeycloakGetClientSecretCmd())
	cmd.AddCommand(NewKeycloakGetUserCmd())
	return cmd
}

func NewKeycloakGetRealmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "realm <realmname>",
		Short: "Get metadata for keycloak realm",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"get",
				"realms/" + args[0],
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakGetClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client <clientname> <realmname>",
		Short: "Get metadata for keycloak client in a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"get",
				"clients",
				"--query", "clientId=" + args[0],
				"--target-realm", args[1],
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakGetClientSecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clientsecret <clientname> <realmname>",
		Short: "Get client secret for keycloak client in a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/mwdd/get_client_secret.sh",
				args[0],
				args[1],
			}, "root")
		},
	}
	return cmd
}

func NewKeycloakGetUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <username> <realmname>",
		Short: "Get metadata for keycloak user in a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			KeycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"get",
				"users",
				"--query", "username=" + args[0],
				"--target-realm", args[1],
			}, "root")
		},
	}
	return cmd
}

func KeycloakLogin() {
	mwdd.DefaultForUser().ExecNoOutput("keycloak", []string{
		"/mwdd/login.sh",
	}, "root")
}
