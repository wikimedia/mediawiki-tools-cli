package wiki

import (
	_ "embed"
	"errors"
	"fmt"
	"io"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/sirupsen/logrus"
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
		Short: "Create or replace a page using source from stdin",
		Long:  "Read page source from standard input and save it as the complete page content. Existing content is replaced. Use --createonly to refuse existing pages or --nocreate to require an existing page.",
		Example: cobrautil.NormalizeExample(`
# Write source to a page using a saved profile
put --profile wikimedia/addshore --wiki https://test.wikipedia.org/w/api.php --title "User:Addshore/CLI test" < page.wikitext
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				fmt.Println("Dry run mode: Putting page with the following parameters:")
				fmt.Printf("wiki: %s, user: %s, title: %s, summary: %s, minor: %t, bot: %t, recreate: %t, nocreate: %t, createonly: %t\n",
					wiki, wikiUser, wikiPageTitle, summary, minor, bot, recreate, nocreate, createonly)
				return nil
			}

			if wiki == "" {
				return fmt.Errorf("wiki is not set")
			}
			if wikiPageTitle == "" {
				return fmt.Errorf("title is not set")
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
				logrus.Info("edit successful, but did not change page")
				return nil
			}
			if warnings, ok := err.(mwclient.APIWarnings); ok {
				for _, warning := range warnings {
					logrus.Warn(warning)
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

	return cmd
}

type MwClientErrorHandling struct {
	// Returns if the error was handled, or should still be classed as an error
	HandleErr func(mwclient.APIError) bool
	// Returns if the warning was handled, or should still be classed as a warning
	HandleWarn func(mwclient.APIWarnings) bool
	// Returns if the unknown error was handled, or should still be classed as an error
	HandleUnknown func(error) bool
	LogErrors     bool
	LogWarns      bool
	LogUnknown    bool
	PanicUnknown  bool
}

func defaultErrorHandling() MwClientErrorHandling {
	return MwClientErrorHandling{
		LogErrors:    true,
		LogWarns:     true,
		LogUnknown:   true,
		PanicUnknown: true,
	}
}

func (handler MwClientErrorHandling) handle(err error) {
	if err != nil {
		handled := false
		logrus.Trace(err)
		if _, ok := err.(mwclient.APIWarnings); ok {
			mwWarn := err.(mwclient.APIWarnings)
			if handler.HandleWarn != nil {
				handled = handler.HandleWarn(mwWarn)
			}
			if handler.LogWarns && !handled {
				for _, warning := range mwWarn {
					logrus.Warn(warning)
				}
			}
		} else if _, ok := err.(mwclient.APIError); ok {
			mwError := err.(mwclient.APIError)
			if handler.HandleErr != nil {
				handled = handler.HandleErr(mwError)
			}
			if handler.LogErrors && !handled {
				logrus.Errorf("API Error: %s: %s", mwError.Code, mwError.Info)
			}
		} else {
			if handler.HandleUnknown != nil {
				handled = handler.HandleUnknown(err)
			}
			if handler.LogUnknown && !handled {
				logrus.Error(err)
			}
			if handler.PanicUnknown && !handled {
				logrus.Panic(err)
			}
		}
	}
}
