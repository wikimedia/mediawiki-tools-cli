package dev

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devremote"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func newRemoteRemoveCmd() *cobra.Command {
	var deleteWorkspace, yes bool
	cmd := &cobra.Command{Use: "remove CONTEXT", Short: "Disconnect and forget a remote context", Long: "Remove the local registration, session logs and Git mirror. Remote files and services are retained unless --delete-workspace is supplied. That option also destroys the context’s containers and volumes and deletes its remote checkout, including uncommitted work. Local source directories and the VM itself are retained.", Args: cobra.ExactArgs(1), PersistentPreRun: cobrautil.CallAllPersistentPreRun, RunE: func(cmd *cobra.Command, args []string) error {
		c, err := devremote.LoadContext(args[0])
		if err != nil {
			return err
		}
		if deleteWorkspace {
			cmd.Printf("Delete remote workspace %s on %s, its containers and volumes, including database data and uncommitted source changes.\n", c.Workspace, c.SSHArgs[len(c.SSHArgs)-1])
			if err := confirmRemoteCleanup(yes, "Delete this remote workspace and its data?"); err != nil {
				return err
			}
		}
		if err := devremote.RemoveContext(cmd.Context(), c.Context, deleteWorkspace, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
			return err
		}
		if deleteWorkspace {
			cmd.Printf("Removed %s and its remote workspace. Local source and the VM were retained.\n", c.Context)
		} else {
			cmd.Printf("Forgot %s. Remote files and services were retained.\n", c.Context)
		}
		return nil
	}}
	cmd.Flags().BoolVar(&deleteWorkspace, "delete-workspace", false, "Also destroy remote containers/volumes and delete the workspace")
	cmd.Flags().BoolVar(&yes, "yes", false, "Approve deletion without an interactive confirmation")
	return cmd
}

func newRemoteCacheCmd() *cobra.Command {
	var host string
	var yes, dryRun bool
	cmd := &cobra.Command{Use: "cache", Short: "Manage cached remote CLI and browser editor downloads"}
	cleanup := &cobra.Command{Use: "cleanup [context]", Short: "Remove unused downloads from local and optionally remote caches", Long: "Without a context, clean the local Linux release-binary cache. Supply a saved context or --ssh HOST to also clean that VM’s shared CLI/editor caches. Active or referenced downloads are retained; source, Docker images and volumes are not removed. Use --dry-run to preview without changing anything.", Args: cobra.MaximumNArgs(1), PersistentPreRun: cobrautil.CallAllPersistentPreRun, RunE: func(cmd *cobra.Command, args []string) error {
		var ssh []string
		if host != "" {
			if len(args) > 0 {
				return fmt.Errorf("use either a saved context or --ssh")
			}
			if err := devremote.ValidateHost(host); err != nil {
				return err
			}
			ssh = []string{host}
		} else if len(args) > 0 {
			c, err := devremote.LoadContext(args[0])
			if err != nil {
				return err
			}
			ssh = c.SSHArgs
		}
		entries, err := devremote.UnusedCaches(cmd.Context(), ssh)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			cmd.Println("No unused cache entries found. Referenced and active downloads were retained.")
			return nil
		}
		cmd.Println("Unused cache entries:")
		for _, entry := range entries {
			cmd.Printf("  %s: %s\n", entry.Location, entry.Path)
		}
		if dryRun {
			cmd.Println("Preview only; nothing changed.")
			return nil
		}
		if err := confirmRemoteCleanup(yes, "Delete these unused cache entries?"); err != nil {
			return err
		}
		removed, err := devremote.CleanupCaches(cmd.Context(), ssh, entries)
		if err != nil {
			return err
		}
		cmd.Printf("Removed %d unused cache entries. Workspaces and development data were retained.\n", removed)
		return nil
	}}
	cleanup.Flags().StringVar(&host, "ssh", "", "Also clean a VM’s shared caches using this SSH host or alias")
	cleanup.Flags().BoolVar(&dryRun, "dry-run", false, "Preview unused cache entries without deleting them")
	cleanup.Flags().BoolVar(&yes, "yes", false, "Approve deletion without an interactive confirmation")
	cmd.AddCommand(cleanup)
	return cmd
}

func confirmRemoteCleanup(yes bool, message string) error {
	if yes {
		return nil
	}
	if !cli.CanAskForInput() {
		return fmt.Errorf("review the listed paths and supply --yes to approve deletion")
	}
	confirmed := false
	if err := survey.AskOne(&survey.Confirm{Message: message, Default: false}, &confirmed); err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("cancelled; nothing deleted")
	}
	return nil
}
