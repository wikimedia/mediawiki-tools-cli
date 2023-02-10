package hosts

import (
	"fmt"

	"github.com/spf13/cobra"
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
	cmd.AddCommand(NewHostsWhereCmd())
	cmd.AddCommand(NewHostsWritableCmd())
	return cmd
}

func handleChangeResult(result hosts.ChangeResult) {
	if result.Success && result.Altered {
		fmt.Println("Hosts file altered and updated: " + result.WriteFile)
	} else if result.Altered {
		fmt.Println("Wanted to alter your hosts file bu could not.")
		fmt.Println("You need to edit the file yourself")
		fmt.Println("Temporary file: " + result.WriteFile)
		fmt.Println("")
		fmt.Println("To apply the changes, run:")
		fmt.Println("sudo cp --no-preserve=mode,ownership " + result.WriteFile + " " + hosts.FilePath())
		fmt.Println("")
		fmt.Println(result.Content)
	} else {
		fmt.Println("No changes needed.")
	}
}
