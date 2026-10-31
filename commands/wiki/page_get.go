package wiki

import (
	"fmt"
	"io"

	mwclient "cgt.name/pkg/go-mwclient"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func NewWikiPageGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [TITLE]",
		Short: "Print a page's source (defaults to the wiki's main page)",
		Args:  cobra.MaximumNArgs(1),
		Example: "  mw wiki page get --profile wikibase-world/addshore\n" +
			"  mw wiki page get --profile wikimedia/addshore --title 'User:Addshore'",
		RunE: func(cmd *cobra.Command, args []string) error {
			title := wikiPageTitle
			if len(args) != 0 {
				if cmd.Flags().Changed("title") {
					return fmt.Errorf("specify a positional title or --title, not both")
				}
				title = args[0]
			}
			w, err := mwclient.New(normalizeWiki(wiki), cli.HTTPUserAgent)
			if err != nil {
				return err
			}
			if err := loginIfCredentialsProvided(w); err != nil {
				return err
			}
			if title == "" {
				result, err := w.Get(map[string]string{"action": "query", "meta": "siteinfo", "siprop": "general"})
				if err != nil {
					return fmt.Errorf("discover main page: %w", err)
				}
				title, err = result.GetString("query", "general", "mainpage")
				if err != nil || title == "" {
					return fmt.Errorf("wiki did not report a main-page title; specify --title")
				}
			}
			result, err := w.Get(map[string]string{
				"action": "query", "prop": "revisions", "titles": title,
				"rvprop": "content", "rvslots": "main", "formatversion": "2",
			})
			if err != nil {
				return fmt.Errorf("read page %q: %w", title, err)
			}
			pages, err := result.GetObjectArray("query", "pages")
			if err != nil || len(pages) != 1 {
				return fmt.Errorf("wiki did not return page %q", title)
			}
			if missing, _ := pages[0].GetBoolean("missing"); missing {
				return fmt.Errorf("page %q does not exist", title)
			}
			revisions, err := pages[0].GetObjectArray("revisions")
			if err != nil || len(revisions) == 0 {
				return fmt.Errorf("page %q has no readable revision", title)
			}
			content, err := revisions[0].GetString("slots", "main", "content")
			if err != nil {
				return fmt.Errorf("page %q has no readable main-slot content", title)
			}
			_, err = io.WriteString(cmd.OutOrStdout(), content)
			return err
		},
	}
	cmd.Flags().StringVar(&wikiPageTitle, "title", "", "Title of the page (defaults to the wiki's main page)")
	return cmd
}
