package wiki

import (
	_ "embed"
	"errors"
	"fmt"
	"io"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func NewWikiPagePutCmd() *cobra.Command {
	var (
		summary    string
		minor      bool
		bot        bool
		recreate   bool
		nocreate   bool
		createonly bool
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "put",
		Args:  cobra.NoArgs,
		Short: "Create or replace a page using source from stdin",
		Long:  "Read page source from standard input and save it as the complete page content. Existing content is replaced. Use --createonly to refuse existing pages or --nocreate to require an existing page.",
		Example: cobrautil.NormalizeExample(`
# Write source to a page using a saved profile
put --profile wikimedia/addshore --wiki https://test.wikipedia.org/w/api.php --title "User:Addshore/CLI test" < page.wikitext
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if wiki == "" {
				return fmt.Errorf("wiki is not set")
			}
			if wikiPageTitle == "" {
				return fmt.Errorf("title is not set")
			}
			if dryRun {
				cmd.Println("Dry run mode: Putting page with the following parameters:")
				cmd.Printf("wiki: %s, user: %s, title: %s, summary: %s, minor: %t, bot: %t, recreate: %t, nocreate: %t, createonly: %t\n",
					wiki, wikiUser, wikiPageTitle, summary, minor, bot, recreate, nocreate, createonly)
				return nil
			}

			bytes, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("could not read page text: %w", err)
			}
			text := string(bytes)

			w, err := mwclient.New(normalizeWiki(wiki), cli.HTTPUserAgent)
			if err != nil {
				return err
			}

			if err := loginIfCredentialsProvided(w); err != nil {
				return fmt.Errorf("wiki authentication failed: %w", err)
			}

			// https://www.mediawiki.org/wiki/API:Edit#Parameters
			editParams := params.Values{
				"title":   wikiPageTitle,
				"text":    text,
				"summary": summary,
			}
			if wikiAnon {
				editParams["token"] = anonCSRFToken
			}
			if minor {
				editParams["minor"] = "1"
			}
			if bot {
				editParams["bot"] = "1"
			}
			if recreate {
				editParams["recreate"] = "1"
			}
			if nocreate {
				editParams["nocreate"] = "1"
			}
			if createonly {
				editParams["createonly"] = "1"
			}

			err = w.Edit(editParams)
			if errors.Is(err, mwclient.ErrEditNoChange) {
				cmd.PrintErrln("Edit successful; page was unchanged.")
				return nil
			}
			if warnings, ok := err.(mwclient.APIWarnings); ok {
				for _, warning := range warnings {
					cmd.PrintErrln("Warning:", warning)
				}
				return nil
			}
			return err
		},
	}

	cmd.Flags().StringVar(&wikiPageTitle, "title", "", "Title of the page to create or replace")
	cmd.Flags().StringVar(&summary, "summary", cli.DisplayName+" edit", "Summary of the edit")
	cmd.Flags().BoolVar(&minor, "minor", false, "Mark the edit as minor")
	cmd.Flags().BoolVar(&bot, "bot", false, "Mark the edit as a bot edit")
	cmd.Flags().BoolVar(&recreate, "recreate", false, "Allow recreating a page deleted since editing began")
	cmd.Flags().BoolVar(&nocreate, "nocreate", false, "Require the page to already exist")
	cmd.Flags().BoolVar(&createonly, "createonly", false, "Create the page only if it does not exist")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the edit parameters without reading stdin or editing the page")

	cmd.MarkFlagsMutuallyExclusive("createonly", "nocreate")
	return cmd
}
