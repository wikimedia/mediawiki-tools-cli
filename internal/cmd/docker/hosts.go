package docker

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/hosts"
)

func NewHostsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hosts",
		Short: "Interact with your system hosts file",
		RunE:  nil,
	}
	cmd.AddCommand(NewHostsAddCmd())
	cmd.AddCommand(NewHostsRemoveCmd())
	cmd.AddCommand(NewHostsWritableCmd())
	return cmd
}

func NewHostsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Adds development environment hosts into your system hosts file (might need sudo)",
		Run: func(cmd *cobra.Command, args []string) {
			changeResult := hosts.AddHosts(
				append(
					[]string{
						// TODO generate these by reading the yml files?
						"proxy.mwdd.localhost",
						"eventlogging.mwdd.localhost",
						"adminer.mwdd.localhost",
						"mailhog.mwdd.localhost",
						"graphite.mwdd.localhost",
						"keycloak.mwdd.localhost",
						"phpmyadmin.mwdd.localhost",
						"default.mediawiki.mwdd.localhost",
					},
					mwdd.DefaultForUser().UsedHosts()...,
				),
			)
			handleChangeResult(changeResult)
		},
	}
}

func NewHostsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Removes development environment hosts from your system hosts file (might need sudo)",
		Run: func(cmd *cobra.Command, args []string) {
			handleChangeResult(hosts.RemoveHostsWithSuffix("mwdd.localhost"))
		},
	}
}

func handleChangeResult(result hosts.ChangeResult) {
	if result.Success && result.Altered {
		fmt.Println("Hosts file altered and updated: " + result.WriteFile)
	} else if result.Altered {
		fmt.Println("Wanted to alter your hosts file bu could not.")
		fmt.Println("You can re-run this command with sudo.")
		fmt.Println("Or edit the hosts file yourself.")
		fmt.Println("Temporary file: " + result.WriteFile)
		fmt.Println("")
		fmt.Println(result.Content)
	} else {
		fmt.Println("No changes needed.")
	}
}

func NewHostsWritableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "writable",
		Short: "Checks if you can write to the needed hosts file",
		Run: func(cmd *cobra.Command, args []string) {
			if hosts.Writable() {
				fmt.Println("Hosts file writable")
			} else {
				fmt.Println("Hosts file not writable")
				os.Exit(1)
			}
		},
	}
}
