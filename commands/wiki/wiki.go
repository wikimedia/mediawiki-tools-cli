package wiki

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	mwclient "cgt.name/pkg/go-mwclient"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

var (
	wiki             string
	wikiUser         string
	wikiPassword     string
	wikiPasswordFile string
	wikiAnon         bool
	wikiRealm        string
	wikiAccount      string
	savedWikiAccount *config.WikiAccount
	wikiContext      context.Context
)

func NewWikiCmd() *cobra.Command {
	savedWikiAccount = nil
	wikiContext = context.Background()
	cmd := &cobra.Command{
		Use:     "wiki",
		GroupID: "service",
		Short:   "Interact with any MediaWiki wiki",
		RunE:    nil,
	}

	cmd.AddCommand(NewWikiPageCmd())
	cmd.AddCommand(NewWikiExtCmd())
	cmd.AddCommand(NewWikiFileCmd())
	cmd.AddCommand(newWikiAuthCmd(), newWikiRealmCmd())
	cmd.PersistentFlags().StringVar(&wiki, "wiki", "", "URL of wikis api.php")
	cmd.PersistentFlags().StringVar(&wikiUser, "user", "", "A user to interact using")
	cmd.PersistentFlags().StringVar(&wikiPassword, "password", "", "Password of the user to interact with")
	cmd.PersistentFlags().StringVar(&wikiPasswordFile, "password-file", "", "Read the login password from a file")
	cmd.MarkFlagsMutuallyExclusive("password", "password-file")
	_ = cmd.MarkPersistentFlagFilename("password-file")
	cmd.PersistentFlags().BoolVar(&wikiAnon, "anon", false, "Perform anonymous edits using the anonymous CSRF token")
	cmd.PersistentFlags().StringVar(&wikiRealm, "realm", "", "Shared-login realm (Wikimedia is built in)")
	cmd.PersistentFlags().StringVar(&wikiAccount, "account", "", "Saved account name or REALM/NAME")
	prepareWikiCommands(cmd)

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

func loginIfCredentialsProvided(w *mwclient.Client, clients ...*http.Client) error {
	if wikiAnon {
		return nil
	}
	httpClient := wikiauth.HTTPClient()
	if len(clients) > 0 {
		httpClient = clients[0]
		httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	w.SetHTTPClient(httpClient)
	if savedWikiAccount != nil {
		a := *savedWikiAccount
		endpoint, err := wikiauth.Endpoint(wiki)
		if err != nil {
			return err
		}
		if !config.WikiRealmAllows(a.Realm, endpoint) {
			return fmt.Errorf("wiki is not a member of the saved account's realm")
		}
		if a.Method == "oauth" {
			tokens, err := accountTokens(wikiContext, a, false)
			if err != nil {
				return err
			}
			httpClient.Jar = nil
			httpClient.Transport = wikiauth.BearerTransport{Base: httpClient.Transport, Endpoint: endpoint, Token: tokens.AccessToken}
			w.Assert = mwclient.AssertUser
			return nil
		}
		password, err := a.Credential().Read()
		if err != nil {
			return err
		}
		if err := w.Login(a.Username, password); err != nil {
			return fmt.Errorf("saved wiki login failed")
		}
		w.Assert = mwclient.AssertUser
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

func hasWikiCredentials() bool {
	return savedWikiAccount != nil || (wikiUser != "" && (wikiPassword != "" || wikiPasswordFile != ""))
}

func realmForWiki(endpoint string) (string, error) {
	if wikiauth.IsWikimedia(endpoint) {
		return "wikimedia", nil
	}
	var matched string
	for _, realm := range config.State().OnDiskKoanf.MapKeys("wiki.realms") {
		if realm == "wikimedia" {
			continue
		}
		if config.WikiRealmAllows(realm, endpoint) {
			if matched != "" {
				return "", fmt.Errorf("wiki belongs to multiple realms; choose --realm explicitly")
			}
			matched = realm
		}
	}
	return matched, nil
}

// Run after a leaf's existing preparation (including file URL inference), without
// overriding the root command's logging and configuration hook.
func prepareWikiCommands(parent *cobra.Command) {
	for _, child := range parent.Commands() {
		if child.Name() == "auth" || child.Name() == "realm" {
			continue
		}
		prepareWikiCommands(child)
		if child.HasSubCommands() {
			continue
		}
		oldE, old := child.PreRunE, child.PreRun
		child.PreRun = nil
		child.PreRunE = func(cmd *cobra.Command, args []string) error {
			if oldE != nil {
				if err := oldE(cmd, args); err != nil {
					return err
				}
			} else if old != nil {
				old(cmd, args)
			}
			return prepareWikiAccount(cmd)
		}
	}
}

func prepareWikiAccount(cmd *cobra.Command) error {
	savedWikiAccount = nil
	wikiContext = cmd.Context()
	explicitCredentials := wikiUser != "" || wikiPassword != "" || wikiPasswordFile != ""
	if wikiAnon || explicitCredentials {
		if wikiAccount != "" || wikiRealm != "" {
			return fmt.Errorf("saved account selection cannot be combined with --anon or explicit credentials")
		}
		if wiki == "" {
			return fmt.Errorf("provide --wiki")
		}
		return nil
	}
	// Resolve defaults only from the CLI's own configuration, never from wiki responses.
	if wiki == "" && wikiAccount == "" && wikiRealm == "" && config.State().OnDiskKoanf.String("wiki.default_realm") == "" {
		return fmt.Errorf("provide --wiki or save a default account with wiki auth login")
	}
	var endpoint string
	if wiki != "" {
		var err error
		endpoint, err = wikiauth.Endpoint(wiki)
		if err != nil {
			return err
		}
	}
	if wikiRealm == "" && wikiAccount == "" && endpoint != "" {
		realm, err := realmForWiki(endpoint)
		if err != nil {
			return err
		}
		if realm == "" || config.State().OnDiskKoanf.String("wiki.realms."+realm+".default_account") == "" {
			return nil
		}
	}
	a, err := selectedWikiAccount()
	if err != nil {
		return err
	}
	if endpoint == "" {
		endpoint = a.Wiki
		wiki = a.Wiki
	}
	if !config.WikiRealmAllows(a.Realm, endpoint) {
		return fmt.Errorf("wiki is not a member of account realm %s; credentials were not opened", a.Realm)
	}
	savedWikiAccount = &a
	wikiUser = a.Username
	return nil
}
