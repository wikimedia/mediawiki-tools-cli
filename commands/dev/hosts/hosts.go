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

func handleChangeResult(cmd *cobra.Command, result hosts.ChangeResult) error {
	if result.Success && result.Altered {
		fmt.Fprintln(cmd.ErrOrStderr(), "Hosts file altered and updated: "+result.WriteFile)
		if hasWSLHostsFile() {
			fmt.Fprintln(cmd.ErrOrStderr(), "")
			fmt.Fprintln(cmd.ErrOrStderr(), "Note: You appear to be using WSL, so you may want to also run:")
			fmt.Fprintln(cmd.ErrOrStderr(), "sudo cp --no-preserve=mode,ownership "+result.WriteFile+" "+wslHostsFile)
		}
	} else if result.Altered {
		fmt.Fprintln(cmd.ErrOrStderr(), "Wanted to alter your hosts file but could not.")
		fmt.Fprintln(cmd.ErrOrStderr(), "You need to edit the file yourself")
		fmt.Fprintln(cmd.ErrOrStderr(), "Temporary file: "+result.WriteFile)
		fmt.Fprintln(cmd.ErrOrStderr(), "")
		fmt.Fprintln(cmd.ErrOrStderr(), "To apply the changes, run:")
		fmt.Fprintln(cmd.ErrOrStderr(), "sudo cp --no-preserve=mode,ownership "+result.WriteFile+" "+hosts.FilePath())
		if hasWSLHostsFile() {
			fmt.Fprintln(cmd.ErrOrStderr(), "")
			fmt.Fprintln(cmd.ErrOrStderr(), "Note: You appear to be using WSL, so you may want to also run:")
			fmt.Fprintln(cmd.ErrOrStderr(), "sudo cp --no-preserve=mode,ownership "+result.WriteFile+" "+wslHostsFile)
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "")
		fmt.Fprintln(cmd.ErrOrStderr(), result.Content)
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), "No changes needed.")
		if hasWSLHostsFile() {
			fmt.Fprintln(cmd.ErrOrStderr(), "")
			fmt.Fprintln(cmd.ErrOrStderr(), "Note: You appear to be using WSL. The Windows hosts file was not checked in this process!")
		}
	}
	if result.Altered && !result.Success {
		return fmt.Errorf("hosts file could not be updated; apply the prepared file manually")
	}
	return nil
}

func hasWSLHostsFile() bool {
	if _, err := os.Stat(wslHostsFile); os.IsNotExist(err) {
		return false
	}
	return true
}
