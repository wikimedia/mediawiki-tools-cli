package wiki

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

//go:embed page_delete.example
var pageDeleteExample string

func NewWikiPageDeleteCmd() *cobra.Command {
	var wikiPageDeleteReason string
	var dryRun bool
	var wikiPageTitle string

	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete pages by title or from stdin",
		Long:    "Delete the page selected by --title, the titles supplied as arguments, or newline-separated titles from standard input. Use --dry-run to inspect the selected titles without deleting pages.",
		Example: cobrautil.NormalizeExample(pageDeleteExample),
		RunE: func(cmd *cobra.Command, args []string) error {
			if wiki == "" {
				return fmt.Errorf("wiki is not set")
			}

			var titles []string
			if wikiPageTitle != "" {
				titles = append(titles, wikiPageTitle)
			} else if len(args) > 0 {
				titles = args
			} else {
				bytes, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				titles = strings.Split(strings.TrimSpace(string(bytes)), "\n")
			}

			if dryRun {
				fmt.Println("Dry run mode: Deleting pages with the following parameters:")
				fmt.Printf("wiki: %s, user: %s, reason: %s\n", wiki, wikiUser, wikiPageDeleteReason)
				for _, title := range titles {
					fmt.Printf("title: %s\n", title)
				}
				return nil
			}

			w, err := mwclient.New(normalizeWiki(wiki), cli.HTTPUserAgent())
			if err != nil {
				return err
			}

			err = loginIfCredentialsProvided(w)
			if err != nil {
				return err
			}

			for _, title := range titles {
				if title == "" {
					continue
				}

				// https://www.mediawiki.org/wiki/API:Edit#Parameters
				deleteParams := params.Values{
					"title":  title,
					"reason": wikiPageDeleteReason,
				}

				deleteErr := wikiDelete(w, deleteParams)
				if deleteErr != nil {
					return fmt.Errorf("delete %q: %w", title, deleteErr)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&wikiPageDeleteReason, "reason", cli.DisplayName+" deletion", "Reason for the deletion")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the selected titles without deleting pages")
	cmd.Flags().StringVar(&wikiPageTitle, "title", "", "Title of the page to delete (alternatively, supply arguments or pipe titles to stdin)")

	return cmd
}

// TODO consider pushing this upstream.
func wikiDelete(w *mwclient.Client, p params.Values) error {
	// If edit token not set, obtain one from API or cache
	if p["token"] == "" {
		csrfToken, err := w.GetToken(mwclient.CSRFToken)
		if err != nil {
			return fmt.Errorf("unable to obtain csrf token: %s", err)
		}
		p["token"] = csrfToken
	}

	p["action"] = "delete"

	resp, err := w.Post(p)
	if err != nil {
		return err
	}

	deleteLogID, err := resp.GetNumber("delete", "logid")
	if err != nil {
		return fmt.Errorf("unable to assert 'logid' field to type number")
	}

	if deleteLogID == "" {
		if captcha, err := resp.GetObject("delete", "captcha"); err == nil {
			captchaBytes, err := captcha.Marshal()
			if err != nil {
				return fmt.Errorf("error occurred while creating error message: %s", err)
			}
			var captchaerr mwclient.CaptchaError
			err = json.Unmarshal(captchaBytes, &captchaerr)
			if err != nil {
				return fmt.Errorf("error occurred while creating error message: %s", err)
			}
			return captchaerr
		}

		del, _ := resp.GetValue("delete")
		return fmt.Errorf("unrecognized response: %v", del)
	}

	return nil
}
