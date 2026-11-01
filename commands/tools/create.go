package tools

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

const toolCreateURL = "https://toolsadmin.wikimedia.org/tools/create"

func newCreateToolCmd(openURL func(string) error) *cobra.Command {
	var noBrowser bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Open the Toolforge tool-account creation form",
		Long:  "Open Toolsadmin to create a Toolforge tool account. Sign in with your Wikimedia developer account; approved Toolforge membership is required. Complete and submit the form in your browser. Use --no-browser or --no-interaction to print the link without opening a browser.",
		Args:  cobra.NoArgs,
		Example: cobrautil.NormalizeExample(`
create
create --no-browser
`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, flag := range []string{"beta", "lima"} {
				if cmd.Flags().Lookup(flag) != nil {
					enabled, err := cmd.Flags().GetBool(flag)
					if err != nil {
						return err
					}
					if enabled {
						return fmt.Errorf("the creation form is for production Toolforge; omit --beta and --lima")
					}
				}
			}
			cmd.Println("Create your tool account in Toolsadmin:")
			cmd.Println(toolCreateURL)
			if noBrowser || cli.Opts.NoInteraction {
				return nil
			}
			if err := openURL(toolCreateURL); err != nil {
				return fmt.Errorf("could not open the browser; open the URL above manually or use --no-browser")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the creation URL without opening a browser")
	return cmd
}
