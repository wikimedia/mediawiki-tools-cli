package hosts

import (
	"fmt"
	"os"

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
	cmd.AddCommand(NewHostsShowCmd())
	cmd.AddCommand(NewHostsWritableCmd())
	return cmd
}

const wslHostsFile = "/mnt/c/Windows/System32/drivers/etc/hosts"

func handleChangeResult(result hosts.ChangeResult) {
	if result.Success && result.Altered {
		fmt.Println("Hosts file altered and updated: " + result.WriteFile)
		if hasWSLHostsFile() {
			fmt.Println("")
			fmt.Println("Note: You appear to be using WSL, so you may want to also run:")
			fmt.Println("sudo cp --no-preserve=mode,ownership " + result.WriteFile + " " + wslHostsFile)
		}
	} else if result.Altered {
		// TODO nicer coloured output
		fmt.Println("Wanted to alter your hosts file bu could not.")
		fmt.Println("You need to edit the file yourself")
		fmt.Println("Temporary file: " + result.WriteFile)
		fmt.Println("")
		fmt.Println("To apply the changes, run:")
		fmt.Println("sudo cp --no-preserve=mode,ownership " + result.WriteFile + " " + hosts.FilePath())
		if hasWSLHostsFile() {
			fmt.Println("")
			fmt.Println("Note: You appear to be using WSL, so you may want to also run:")
			fmt.Println("sudo cp --no-preserve=mode,ownership " + result.WriteFile + " " + wslHostsFile)
		}
		fmt.Println("")
		fmt.Println(result.Content)
	} else {
		fmt.Println("No changes needed.")
		if hasWSLHostsFile() {
			fmt.Println("")
			fmt.Println("Note: You appear to be using WSL. The Windows hosts file was not checked in this process!")
		}
	}
}

func hasWSLHostsFile() bool {
	if _, err := os.Stat(wslHostsFile); os.IsNotExist(err) {
		return false
	}
	return true
}
