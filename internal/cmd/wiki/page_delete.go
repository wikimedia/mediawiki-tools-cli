package wiki

import (
	_ "embed"
	"encoding/json"
	"fmt"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

//go:embed page_delete_example.txt
var pageDeleteExample string

func NewWikiPageDeleteCmd() *cobra.Command {
	var wikiPageDeleteReason string

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
			if wikiPageTitle == "" {
				logrus.Fatal("title is not set")
			}

			w, err := mwclient.New(wiki, "mwcli")
			if err != nil {
				panic(err)
			}

			// TODO only login if user and pass is set
			err = w.Login(wikiUser, wikiPassword)
			if err != nil {
				panic(err)
			}

			// https://www.mediawiki.org/wiki/API:Edit#Parameters
			deleteParams := params.Values{
				"title":  wikiPageTitle,
				"reason": wikiPageDeleteReason,
			}

			deleteErr := wikiDelete(w, deleteParams)
			if deleteErr != nil {
				fmt.Println(deleteErr)
			}
		},
	}

	cmd.Flags().StringVar(&wikiPageDeleteReason, "reason", "mwcli deletion", "Reason for the deletion")

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

		delete, _ := resp.GetValue("delete")
		return fmt.Errorf("unrecognized response: %v", delete)
	}

	return nil
}
