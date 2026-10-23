package wiki

import (
	"fmt"
	"os"
	"strings"

	mwclient "cgt.name/pkg/go-mwclient"
	"github.com/spf13/cobra"
)

var (
	wiki             string
	wikiUser         string
	wikiPassword     string
	wikiPasswordFile string
	wikiAnon         bool
)

func NewWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "wiki",
		GroupID: "service",
		Short:   "Interact with any MediaWiki wiki",
		RunE:    nil,
	}

	cmd.AddCommand(NewWikiPageCmd())
	cmd.AddCommand(NewWikiExtCmd())
	cmd.AddCommand(NewWikiFileCmd())
	cmd.PersistentFlags().StringVar(&wiki, "wiki", "", "URL of wikis api.php")
	cmd.PersistentFlags().StringVar(&wikiUser, "user", "", "A user to interact using")
	cmd.PersistentFlags().StringVar(&wikiPassword, "password", "", "Password of the user to interact with")
	cmd.PersistentFlags().StringVar(&wikiPasswordFile, "password-file", "", "Read the login password from a file")
	cmd.MarkFlagsMutuallyExclusive("password", "password-file")
	_ = cmd.MarkPersistentFlagFilename("password-file")
	cmd.PersistentFlags().BoolVar(&wikiAnon, "anon", false, "Perform anonymous edits using the anonymous CSRF token")
	err := cmd.MarkPersistentFlagRequired("wiki")
	if err != nil {
		panic(err)
	}

	return cmd
}

func normalizeWiki(wiki string) string {
	// If there is no protocol, assume https
	if !strings.HasPrefix(wiki, "http://") && !strings.HasPrefix(wiki, "https://") {
		wiki = "https://" + wiki
	}
	// If it doesn't end in api.php, assume /w/api.php
	if !strings.HasSuffix(wiki, "/api.php") {
		wiki = strings.TrimSuffix(wiki, "/") + "/w/api.php"
	}
	return wiki
}

// anonCSRFToken is the well-known anonymous CSRF token accepted by MediaWiki
// for edits that do not require authentication.
const anonCSRFToken = `+\`

func loginIfCredentialsProvided(w *mwclient.Client) error {
	if wikiAnon {
		return nil
	}
	password := wikiPassword
	if wikiPasswordFile != "" {
		data, err := os.ReadFile(wikiPasswordFile)
		if err != nil {
			return fmt.Errorf("read password file: %w", err)
		}
		password = strings.TrimRight(string(data), "\r\n")
	}
	if wikiUser == "" && password == "" {
		return nil
	}
	if wikiUser == "" || password == "" {
		return fmt.Errorf("--user and either --password or --password-file must be provided together")
	}
	return w.Login(wikiUser, password)
}
