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
	wikiProfile      string
	savedWikiProfile *config.WikiProfile
	wikiContext      context.Context
)

func NewWikiCmd() *cobra.Command {
	savedWikiProfile = nil
	wikiContext = context.Background()
	cmd := &cobra.Command{
		Use:     "wiki",
		GroupID: "service",
		Short:   "Interact with any MediaWiki wiki",
		Long:    "Read and manage pages, files and extension data on MediaWiki wikis. Use wiki auth login to save a profile, then select it with --profile. A saved default profile is used when no profile is specified. Use --wiki to choose a site within the profile's scope.\n\nFor one-off access, supply --username with --password or --password-file. Use --anon to bypass saved credentials.",
		RunE:    nil,
	}

	cmd.AddCommand(NewWikiPageCmd())
	cmd.AddCommand(NewWikiExtCmd())
	cmd.AddCommand(NewWikiFileCmd())
	cmd.AddCommand(newWikiAPICmd(), newWikiRESTCmd())
	cmd.AddCommand(newWikiAuthCmd(), newWikiRealmCmd())
	cmd.PersistentFlags().StringVar(&wiki, "wiki", "", "Wiki URL or API endpoint")
	cmd.PersistentFlags().StringVar(&wikiUser, "username", "", "Wiki login username (use Name@bot for a bot password)")
	cmd.PersistentFlags().StringVar(&wikiUser, "user", "", "Deprecated alias for --username")
	cmd.MarkFlagsMutuallyExclusive("username", "user")
	_ = cmd.PersistentFlags().MarkHidden("user")
	cmd.PersistentFlags().StringVar(&wikiPassword, "password", "", "Wiki login password (or bot password)")
	cmd.PersistentFlags().StringVar(&wikiPasswordFile, "password-file", "", "Read the login password from a file")
	cmd.MarkFlagsMutuallyExclusive("password", "password-file")
	_ = cmd.MarkPersistentFlagFilename("password-file")
	cmd.PersistentFlags().BoolVar(&wikiAnon, "anon", false, "Use anonymous access instead of saved credentials")
	cmd.PersistentFlags().StringVar(&wikiRealm, "realm", "", "Shared-login realm (wikimedia and wikibase-world are built in)")
	cmd.PersistentFlags().StringVar(&wikiProfile, "profile", "", "Saved profile name or REALM/NAME")
	cmd.PersistentFlags().StringVar(&wikiProfile, "account", "", "Deprecated alias for --profile")
	cmd.MarkFlagsMutuallyExclusive("profile", "account")
	// TODO(early 2027): remove --account and --user compatibility aliases.
	_ = cmd.PersistentFlags().MarkHidden("account")
	registerWikiCompletions(cmd)
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
	httpClient := wikiauth.HTTPClient()
	if len(clients) > 0 {
		httpClient = clients[0]
		httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	w.SetHTTPClient(httpClient)
	if wikiAnon {
		// SetHTTPClient inherits mwclient's fresh jar; anonymous mode deliberately
		// disables it so even response-set cookies are not reused across requests.
		httpClient.Jar = nil
		return nil
	}
	if savedWikiProfile != nil {
		a := *savedWikiProfile
		endpoint, err := wikiauth.Endpoint(wiki)
		if err != nil {
			return err
		}
		if !a.Allows(endpoint) {
			return fmt.Errorf("wiki is outside the saved profile's site scope")
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
		if err := wikiauth.LoginPassword(w, a.Username, password); err != nil {
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
		return fmt.Errorf("--username and either --password or --password-file must be provided together")
	}
	return wikiauth.LoginPassword(w, wikiUser, password)
}

func hasWikiCredentials() bool {
	return savedWikiProfile != nil || (wikiUser != "" && (wikiPassword != "" || wikiPasswordFile != ""))
}

func realmForWiki(endpoint string) (string, error) {
	if realm := wikiauth.BuiltinRealm(endpoint); realm != "" {
		return realm, nil
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
		if child.RunE == nil && child.Run == nil {
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
			return prepareWikiProfile(cmd)
		}
	}
}

func prepareWikiProfile(cmd *cobra.Command) error {
	savedWikiProfile = nil
	wikiContext = cmd.Context()
	explicitCredentials := wikiUser != "" || wikiPassword != "" || wikiPasswordFile != ""
	if wikiAnon {
		if explicitCredentials {
			return fmt.Errorf("--anon cannot be combined with explicit credentials")
		}
		// Site selection reads public metadata only. Anonymous access may use a
		// profile's default site but must never open its credentials or enforce its
		// credential scope on an explicitly selected public wiki.
		if wiki == "" {
			a, err := selectedWikiProfile(wikiProfile)
			if err != nil {
				return fmt.Errorf("provide --wiki or select a saved profile: %w", err)
			}
			wiki = a.Wiki
		}
		return nil
	}
	if explicitCredentials {
		if wikiProfile != "" || wikiRealm != "" {
			return fmt.Errorf("saved profile selection cannot be combined with --anon or explicit credentials")
		}
		if wiki == "" {
			return fmt.Errorf("provide --wiki")
		}
		return nil
	}
	// Resolve defaults only from the CLI's own configuration, never from wiki responses.
	if wiki == "" && wikiProfile == "" && wikiRealm == "" && wikiDefaultRealm() == "" {
		return fmt.Errorf("provide --wiki or save a default profile with wiki auth login")
	}
	var endpoint string
	if wiki != "" {
		var err error
		endpoint, err = wikiauth.Endpoint(wiki)
		if err != nil {
			return err
		}
	}
	if wikiRealm == "" && wikiProfile == "" && endpoint != "" {
		realm, err := realmForWiki(endpoint)
		if err != nil {
			return err
		}
		if realm == "" || config.State().OnDiskKoanf.String("wiki.realms."+realm+".default_profile") == "" {
			return nil
		}
	}
	a, err := selectedWikiProfile(wikiProfile)
	if err != nil {
		return err
	}
	if endpoint == "" {
		endpoint = a.Wiki
		wiki = a.Wiki
	}
	if !a.Allows(endpoint) {
		return fmt.Errorf("wiki is outside profile realm/scope %s; credentials were not opened", a.Realm)
	}
	savedWikiProfile = &a
	wikiUser = a.Username
	return nil
}
