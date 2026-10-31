package wikiauth

import (
	"errors"
	"fmt"

	"cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/sirupsen/logrus"
)

// LoginPassword validates the login result even when MediaWiki also returns
// nonfatal warnings. mwclient.Login stops at warnings before checking the result.
func LoginPassword(w *mwclient.Client, username, password string) error {
	response, err := w.Get(params.Values{"action": "query", "meta": "tokens", "type": "login"})
	if err = loginResponseError(err); err != nil {
		return err
	}
	token, err := response.GetString("query", "tokens", "logintoken")
	if err != nil || token == "" {
		return fmt.Errorf("invalid API response: missing login token")
	}
	response, err = w.Post(params.Values{
		"action": "login", "lgname": username, "lgpassword": password, "lgtoken": token,
	})
	if err = loginResponseError(err); err != nil {
		return err
	}
	result, err := response.GetString("login", "result")
	if err != nil || result == "" {
		return fmt.Errorf("invalid API response: missing login result")
	}
	if result != "Success" {
		reason, _ := response.GetString("login", "reason")
		return mwclient.APIError{Code: result, Info: reason}
	}
	return nil
}

func loginResponseError(err error) error {
	var warnings mwclient.APIWarnings
	if errors.As(err, &warnings) {
		logrus.Warn(warnings)
		return nil
	}
	return err
}
