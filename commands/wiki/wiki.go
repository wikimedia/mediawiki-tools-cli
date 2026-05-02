package wiki

import (
	"fmt"
	"strings"

	mwclient "cgt.name/pkg/go-mwclient"
	"github.com/spf13/cobra"
)

var (
	wiki         string
	wikiUser     string
	wikiPassword string
	wikiAnon     bool
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
	cmd.PersistentFlags().StringVar(&wiki, "wiki", "", "URL of wikis api.php")
	cmd.PersistentFlags().StringVar(&wikiUser, "user", "", "A user to interact using")
	cmd.PersistentFlags().StringVar(&wikiPassword, "password", "", "Password of the user to interact with")
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
	if wikiUser == "" && wikiPassword == "" {
		return nil
	}
	if wikiUser == "" || wikiPassword == "" {
		return fmt.Errorf("--user and --password must either both be set or both be omitted")
	}
	return w.Login(wikiUser, wikiPassword)
}
