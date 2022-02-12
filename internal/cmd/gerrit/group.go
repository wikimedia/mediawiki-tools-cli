package gerrit

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/releng/cli/internal/util/cmd"
	stringsutil "gitlab.wikimedia.org/releng/cli/internal/util/strings"
)

func NewGerritGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Interact with Gerrit groups",
	}
	cmd.AddCommand(NewGerritGroupListCmd())
	cmd.AddCommand(NewGerritGroupSearchCmd())
	cmd.AddCommand(NewGerritGroupMembersCmd())
	return cmd
}

func NewGerritGroupListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Gerrit groups",
		Run: func(cmd *cobra.Command, args []string) {
			ssh := cmdutil.AttachAllIO(sshGerritCommand([]string{"ls-groups"}))
			if err := ssh.Run(); err != nil {
				os.Exit(1)
			}
		},
	}
}

func NewGerritGroupSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search [search string]...",
		Short: "Search Gerrit groups",
		Args:  cobra.MinimumNArgs(1),
		Example: `  search wmde
	  search extension Wikibase`,
		Run: func(cmd *cobra.Command, args []string) {
			ssh := cmdutil.AttachInErrIO(sshGerritCommand([]string{"ls-groups"}))
			out := cmdutil.AttachOutputBuffer(ssh)

			if err := ssh.Run(); err != nil {
				os.Exit(1)
			}

			fmt.Println(stringsutil.FilterMultiline(out.String(), args))
		},
	}
}

func NewGerritGroupMembersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "members [group name]",
		Short: "List members of a Gerrit group",
		Args:  cobra.MinimumNArgs(1),
		Example: `  members wmde
	  members mediawiki`,
		Run: func(cmd *cobra.Command, args []string) {
			ssh := cmdutil.AttachAllIO(sshGerritCommand([]string{"ls-members", args[0]}))
			if err := ssh.Run(); err != nil {
				os.Exit(1)
			}
		},
	}
}
