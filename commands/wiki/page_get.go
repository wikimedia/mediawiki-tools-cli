package wiki

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

func NewWikiPageGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Annotations: commandmeta.Annotations(mcpserver.ReadHints(commandmeta.DataPublic, commandmeta.DataPrivateContent)),
		Use:         "get [TITLE]",
		Short:       "Print a page's source (defaults to the wiki's main page)",
		Args:        cobra.MaximumNArgs(1),
		Example: "  get --profile wikibase-world/addshore\n" +
			"  get --profile wikimedia/addshore --title 'User:Addshore'",
		RunE: func(cmd *cobra.Command, args []string) error {
			title := wikiPageTitle
			if len(args) != 0 {
				if cmd.Flags().Changed("title") {
					return fmt.Errorf("specify a positional title or --title, not both")
				}
				title = args[0]
			}
			client, err := wikiPageReadClient(cmd)
			if err != nil {
				return err
			}
			page, err := client.PageSource(cmd.Context(), title)
			if err != nil {
				return err
			}
			_, err = io.WriteString(cmd.OutOrStdout(), page.Content)
			return err
		},
	}
	cmd.Flags().StringVar(&wikiPageTitle, "title", "", "Title of the page (defaults to the wiki's main page)")
	accesspolicy.Support(cmd)
	return cmd
}
