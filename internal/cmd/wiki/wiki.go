package wiki

import (
	"strings"

	"github.com/spf13/cobra"
)

var (
	wiki         string
	wikiUser     string
	wikiPassword string
)

func NewWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "wiki",
		GroupID: "service",
		Short:   "Interact with any MediaWiki wiki",
		RunE:    nil,
	}

	cmd.AddCommand(NewWikiPageCmd())
	cmd.PersistentFlags().StringVar(&wiki, "wiki", "", "URL of wikis api.php")
	cmd.PersistentFlags().StringVar(&wikiUser, "user", "", "A user to interact using")
	cmd.PersistentFlags().StringVar(&wikiPassword, "password", "", "Password of the user to interact with")
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
