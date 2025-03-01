package wiki

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

//go:embed page_delete.example
var pageDeleteExample string

func NewWikiPageDeleteCmd() *cobra.Command {
	var wikiPageDeleteReason string
	var dryRun bool
	var wikiPageTitle string

	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "MediaWiki Wiki Page Delete",
		RunE:    nil,
		Example: pageDeleteExample,
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

			var titles []string
			if wikiPageTitle != "" {
				titles = append(titles, wikiPageTitle)
			} else if len(args) > 0 {
				titles = args
			} else {
				bytes, err := io.ReadAll(os.Stdin)
				if err != nil {
					panic(err)
				}
				titles = strings.Split(strings.TrimSpace(string(bytes)), "\n")
			}

			if dryRun {
				fmt.Println("Dry run mode: Deleting pages with the following parameters:")
				fmt.Printf("wiki: %s, user: %s, reason: %s\n", wiki, wikiUser, wikiPageDeleteReason)
				for _, title := range titles {
					fmt.Printf("title: %s\n", title)
				}
				return
			}

			w, err := mwclient.New(normalizeWiki(wiki), "mwcli")
			if err != nil {
				panic(err)
			}

			// TODO only login if user and pass is set
			err = w.Login(wikiUser, wikiPassword)
			if err != nil {
				panic(err)
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
					fmt.Println(deleteErr)
				}
			}
		},
	}

	cmd.Flags().StringVar(&wikiPageDeleteReason, "reason", "mwcli deletion", "Reason for the deletion")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "If set, only print the action that would be performed")
	cmd.Flags().StringVar(&wikiPageTitle, "title", "", "Title of the page to delete. You can also pipe the titles to delete to stdin.")

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
