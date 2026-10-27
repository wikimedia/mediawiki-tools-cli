package wiki

import "github.com/spf13/cobra"

func NewWikiFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Manage files uploaded to a wiki",
	}
	cmd.AddCommand(NewWikiFileUploadCmd())
	cmd.AddCommand(NewWikiFileDownloadCmd())
	return cmd
}
