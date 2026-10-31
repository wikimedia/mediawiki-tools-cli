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
		Short: "MediaWiki Wiki Page Put",
		Example: cobrautil.NormalizeExample(`
# Put "foo" on the "mwcli-test" page on test.wikipedia.org
put --wiki https://test.wikipedia.org/w/api.php --user ${user} --password ${password} --title "mwcli-test" <<< "foo"
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

	cmd.Flags().StringVar(&summary, "summary", cli.DisplayName+" edit", "Summary of the edit")
	cmd.Flags().BoolVar(&minor, "minor", false, "Minor edit")
	cmd.Flags().BoolVar(&bot, "bot", false, "Bot edit")
	cmd.Flags().BoolVar(&recreate, "recreate", false, "Override any errors about the page having been deleted in the meantime.")
	cmd.Flags().BoolVar(&nocreate, "nocreate", false, "Throw an error if the page doesn't exist.")
	cmd.Flags().BoolVar(&createonly, "createonly", false, "Don't edit the page if it exists already.")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "If set, only print the action that would be performed")

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
