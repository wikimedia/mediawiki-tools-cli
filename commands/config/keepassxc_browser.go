package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func NewKeePassXCConnectCmd() *cobra.Command {
	var proxy string
	cmd := &cobra.Command{
		Use: "connect", Short: "Pair with KeePassXC",
		Long: "Enable Browser Integration and unlock the intended database in KeePassXC. Approve and name the connection there. The pairing key is stored separately in a private file; ordinary config stores only entry identifiers. This command can open an association dialog in KeePassXC.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cli.CanAskForInput() {
				return fmt.Errorf("pairing needs an interactive terminal; unlock KeePassXC and run this command yourself")
			}
			cmd.PrintErrln("In KeePassXC, unlock the intended database and approve the connection request. Give it a name such as wm-cli.")
			if err := secrets.PairKeePassXCBrowser(cmd.Context(), proxy); err != nil {
				return err
			}
			cmd.Println("Paired with KeePassXC.")
			return nil
		},
		Args: cobra.NoArgs,
	}
	cmd.Flags().StringVar(&proxy, "proxy", "", "KeePassXC native messaging proxy executable (WSL can use keepassxc-proxy.exe)")
	return cmd
}

func NewKeePassXCStatusCmd() *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Show the saved KeePassXC pairing", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		status, err := secrets.KeePassXCBrowserStatus()
		if err != nil {
			return err
		}
		cmd.Print(status)
		return nil
	}}
}

func NewKeePassXCDisconnectCmd() *cobra.Command {
	return &cobra.Command{Use: "disconnect", Short: "Forget the local KeePassXC pairing", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := secrets.ForgetKeePassXCBrowser(); err != nil {
			return err
		}
		cmd.Println("Local pairing removed. Revoke its stored key in KeePassXC Database Settings > Browser Integration as well.")
		return nil
	}}
}

func NewKeePassXCCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "keepassxc", Short: "Manage the KeePassXC connection"}
	cmd.AddCommand(NewKeePassXCConnectCmd(), NewKeePassXCStatusCmd(), NewKeePassXCDisconnectCmd())
	return cmd
}
