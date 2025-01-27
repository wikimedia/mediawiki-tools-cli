package wiki

import (
	_ "embed"
	"io"
	"os"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
	)

	cmd := &cobra.Command{
		Use:   "put",
		Short: "MediaWiki Wiki Page Put",
		RunE:  nil,
		Example: cobrautil.NormalizeExample(`
# Put "foo" on the "mwcli-test" page on test.wikipedia.org
put --wiki https://test.wikipedia.org/w/api.php --user ${user} --password ${password} --title "mwcli-test" <<< "foo"
`),
		Run: func(cmd *cobra.Command, args []string) {
			if wiki == "" {
				logrus.Fatal("wiki is not set")
			}
			if wikiUser == "" {
				logrus.Fatal("wiki is not set")
			}
			if wikiPassword == "" {
				logrus.Fatal("wiki is not set")
			}
			if wikiPageTitle == "" {
				logrus.Fatal("title is not set")
			}

			bytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				panic(err)
			}
			text := string(bytes)

			w, err := mwclient.New(wiki, "mwcli")
			if err != nil {
				panic(err)
			}

			// TODO only login if user and pass is set
			defaultErrorHandling().handle(w.Login(wikiUser, wikiPassword))

			// https://www.mediawiki.org/wiki/API:Edit#Parameters
			editParams := params.Values{
				"title":   wikiPageTitle,
				"text":    text,
				"summary": summary,
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

			editErrorHandling := defaultErrorHandling()
			editErrorHandling.HandleUnknown = func(err error) bool {
				if err.Error() == "edit successful, but did not change page" {
					logrus.Info("edit successful, but did not change page")
					return true
				}
				return false
			}

			editErrorHandling.handle(w.Edit(editParams))
		},
	}

	cmd.Flags().StringVar(&summary, "summary", "mwcli edit", "Summary of the edit")
	cmd.Flags().BoolVar(&minor, "minor", false, "Minor edit")
	cmd.Flags().BoolVar(&bot, "bot", false, "Bot edit")
	cmd.Flags().BoolVar(&recreate, "recreate", false, "Override any errors about the page having been deleted in the meantime.")
	cmd.Flags().BoolVar(&nocreate, "nocreate", false, "Throw an error if the page doesn't exist.")
	cmd.Flags().BoolVar(&createonly, "createonly", false, "Don't edit the page if it exists already.")

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
