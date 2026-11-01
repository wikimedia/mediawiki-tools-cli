package wiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	mwclient "cgt.name/pkg/go-mwclient"
	"github.com/AlecAivazis/survey/v2"
	"github.com/knadh/koanf"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/browser"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore"
)

func newWikiAuthCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Save and manage wiki profiles"}
	profiles := authprofile.Commands{Label: "wiki", Selected: &wikiProfile, Backend: wikiProfiles{}}
	cmd.AddCommand(newWikiLoginCmd(), newWikiStatusCmd(), profiles.RemoveCmd(), profiles.ProfilesCmd(), profiles.DefaultCmd())
	return cmd
}

func newWikiLoginCmd() *cobra.Command {
	var method, provider, clientID, redirectURI string
	var scopes, sites []string
	var manual, noBrowser, passwordStdin, insecure, replace, allScopes bool
	cmd := &cobra.Command{
		Use: "login", Short: "Authorize and save a wiki profile",
		Long: "Save a wiki profile using OAuth or a username and password. When --profile is omitted, the profile name defaults to the verified username.\n\nWikimedia and wikibase.world have built-in public a-wikimedia-cli clients. OAuth uses authorization code with PKCE and defaults to the callback http://localhost:8765/auth/login. No client secret is required. Use --client-id and --redirect-uri for another registered public client, or --method password for a password login.\n\nBasic access is the default. Interactive login offers a permission picker grouped by function and risk; --scope selects permissions explicitly. The wiki validates which scopes the client can request.\n\nUse --manual when the callback port is occupied. You can also paste the callback URL during normal browser login if the redirect fails.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if passwordStdin && (wikiPassword != "" || wikiPasswordFile != "") {
				return fmt.Errorf("--password-stdin cannot be combined with --password or --password-file")
			}
			if err := config.ValidateCredentialProvider(provider); provider != "" && err != nil {
				return err
			}
			if !cmd.Flags().Changed("method") && (wikiUser != "" || wikiPassword != "" || wikiPasswordFile != "" || passwordStdin) {
				method = "password"
			}
			if method == "bot-password" {
				method = "password"
			}
			if method != "oauth" && method != "password" {
				return fmt.Errorf("--method must be oauth or password")
			}
			if (cmd.Flags().Changed("scope") || allScopes) && (method != "oauth" || cmd.Flags().Changed("scope") && len(scopes) == 0) {
				return fmt.Errorf("OAuth scope options require OAuth; --scope needs at least one explicit scope name")
			}
			realm, name, err := splitWikiProfile(wikiProfile)
			if err != nil {
				return err
			}
			target := wiki
			if target == "" && len(sites) > 0 {
				target = sites[0]
			}
			if target == "" {
				presetRealm := realm
				if presetRealm == "" {
					presetRealm = "wikimedia"
				}
				if preset, ok := wikiauth.BuiltinPreset(presetRealm); ok {
					target = preset.API
				}
			}
			endpoint, err := wikiauth.Endpoint(target)
			if err != nil {
				return err
			}
			if realm == "" {
				realm, err = realmForWiki(endpoint)
				if err != nil {
					return err
				}
				if realm == "" {
					u, _ := url.Parse(endpoint)
					realm = strings.NewReplacer(".", "-", ":", "-").Replace(strings.ToLower(u.Hostname()))
				}
			}
			if !wikiauth.ValidName(realm) {
				return fmt.Errorf("provide a valid --realm name")
			}
			if name != "" && !wikiauth.ValidName(name) {
				return fmt.Errorf("provide a valid --profile name")
			}
			var previous *config.WikiProfile
			if name != "" {
				previous, err = checkWikiProfileReplacement(realm, name, method, replace)
				if err != nil {
					return err
				}
			}
			trusted := config.WikiRealmEndpoints(realm)
			if realm != "wikimedia" && len(trusted) == 0 {
				trusted = []string{endpoint}
			}
			explicitSites := cmd.Flags().Changed("site")
			allowedSites, err := normalizeProfileSites(sites, endpoint)
			if err != nil {
				return err
			}
			if explicitSites {
				if realm == wikiauth.WikibaseWorldRealm {
					for _, site := range allowedSites {
						if site != wikiauth.WikibaseWorldAPI {
							return fmt.Errorf("wikibase.world profiles can only include wikibase.world")
						}
					}
				}
				if realm == "wikimedia" {
					for _, site := range allowedSites {
						if !wikiauth.IsWikimedia(site) {
							return fmt.Errorf("Wikimedia profiles can only include Wikimedia sites")
						}
					}
				}
			} else if !wikiauth.Allows(realm, endpoint, trusted) {
				return fmt.Errorf("wiki is not a member of realm %s; explicitly authorize additional endpoints with --site", realm)
			}
			a := config.WikiProfile{Realm: realm, Name: name, Method: method, Wiki: endpoint, Endpoint: endpoint}
			if explicitSites || realm != "wikimedia" && len(config.WikiRealmEndpoints(realm)) == 0 {
				a.Sites = allowedSites
			}
			if previous != nil && !explicitSites {
				a.Sites = previous.Sites
				if !a.Allows(endpoint) {
					return fmt.Errorf("wiki is outside the saved profile's site scope; use --site to explicitly change it")
				}
			}
			if realm == "wikimedia" {
				a.Endpoint = wikiauth.WikimediaAuthority
			}

			var secret string
			if method == "oauth" {
				if !cmd.Flags().Changed("client-id") {
					preset, ok := wikiauth.BuiltinPreset(realm)
					if !ok {
						return fmt.Errorf("provide --client-id for this wiki's registered public OAuth client, or use --method password")
					}
					clientID = preset.ClientID
				}
				if wikiUser != "" || wikiPassword != "" || wikiPasswordFile != "" || passwordStdin {
					return fmt.Errorf("OAuth uses browser authorization; omit password and user options")
				}
				a.ClientID, a.RedirectURI = clientID, redirectURI
				requested, err := requestedOAuthScopes(cmd, clientID, scopes, allScopes, cli.CanAskForInput(), previous, chooseOAuthScopes)
				if err != nil {
					return err
				}

				authorization, err := wikiauth.BeginAt(a.Endpoint, clientID, redirectURI, requested...)
				if err != nil {
					return err
				}
				a.RequestedScope = authorization.Scope
				if a.RequestedScope != "" {
					cmd.PrintErrln("Requesting OAuth scopes: " + a.RequestedScope)
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
				defer cancel()
				callback, err := authorizeWiki(ctx, cmd, authorization, manual, noBrowser)
				if err != nil {
					return err
				}
				code, err := authorization.Code(callback)
				if err != nil {
					return err
				}
				tokens, err := wikiauth.Exchange(ctx, wikiauth.HTTPClient(), clientID, authorization, code)
				if err != nil {
					return err
				}
				httpClient := wikiauth.HTTPClient()
				httpClient.Transport = wikiauth.BearerTransport{Endpoint: endpoint, Token: tokens.AccessToken}
				a.Username, err = wikiauth.Identity(ctx, httpClient, endpoint)
				if err != nil {
					return err
				}
				encoded, err := json.Marshal(tokens)
				if err != nil {
					return err
				}
				secret = string(encoded)
			} else {
				a.Username = wikiUser
				if a.Username == "" {
					return fmt.Errorf("provide --username (use Name@cli for a bot password)")
				}
				secret = wikiPassword
				if passwordStdin {
					secret, err = secrets.ReadInput(cmd.InOrStdin())
				} else if wikiPasswordFile != "" {
					var data []byte
					data, err = os.ReadFile(wikiPasswordFile)
					secret = strings.TrimRight(string(data), "\r\n")
				} else if secret == "" {
					if !cli.CanAskForInput() {
						return fmt.Errorf("provide --password-stdin or --password-file")
					}
					err = survey.AskOne(&survey.Password{Message: "Wiki password:"}, &secret, survey.WithValidator(survey.Required))
				}
				if err != nil {
					return fmt.Errorf("could not read wiki password")
				}
				if secret == "" {
					return fmt.Errorf("wiki password is empty")
				}
				w, err := mwclient.New(endpoint, cli.HTTPUserAgent)
				if err != nil {
					return err
				}
				httpClient := wikiauth.HTTPClient()
				w.SetHTTPClient(httpClient)
				if err := wikiauth.LoginPassword(w, a.Username, secret); err != nil {
					return fmt.Errorf("wiki credentials were not validated or saved")
				}
				a.Identity, err = wikiauth.Identity(cmd.Context(), httpClient, endpoint)
				if err != nil {
					return fmt.Errorf("could not verify the authenticated wiki identity: %w", err)
				}
			}
			if a.Identity == "" {
				a.Identity = a.Username
			}
			if name == "" {
				name = strings.ToLower(strings.ReplaceAll(a.Identity, " ", "-"))
				if !wikiauth.ValidName(name) {
					return fmt.Errorf("provide an explicit --profile name containing letters, digits, underscores or hyphens for this identity")
				}
				a.Name = name
				previous, err = checkWikiProfileReplacement(realm, name, method, replace)
				if err != nil {
					return err
				}
				if previous != nil && !explicitSites {
					a.Sites = previous.Sites
					if !a.Allows(endpoint) {
						return fmt.Errorf("wiki is outside the saved profile's site scope")
					}
				}
			}
			metadata := make(map[string]any)
			for key, value := range a.Metadata() {
				metadata[key] = value
			}
			qualified := realm + "/" + name
			current := config.State().OnDiskKoanf.String("wiki.default_profile")
			changeDefault, err := authprofile.ChooseLoginDefault(cmd, qualified, current)
			if err != nil {
				return err
			}
			realmDefault := "wiki.realms." + realm + ".default_profile"
			if changeDefault || !cmd.Flags().Changed("default") && config.State().OnDiskKoanf.String(realmDefault) == "" {
				metadata[realmDefault] = name
			}
			if changeDefault {
				metadata["wiki.default_profile"] = qualified
			}
			if realm != "wikimedia" {
				// Explicitly authorized sites extend the realm catalog; each profile still
				// enforces its own narrower site scope before opening any credential.
				known := append(config.WikiRealmEndpoints(realm), allowedSites...)
				seen := map[string]bool{}
				var endpoints []string
				for _, site := range known {
					if !seen[site] {
						endpoints = append(endpoints, site)
						seen[site] = true
					}
				}
				metadata["wiki.realms."+realm+".endpoints"] = endpoints
			}
			metadata["wiki.profiles."+realm+"."+name+".sites"] = a.Sites

			cmd.PrintErrln("Saving account credentials; unlock your password manager and approve access if prompted.")
			if err := a.Credential().SaveWithProviderMetadata(secret, insecure, metadata, provider); err != nil {
				if errors.Is(err, secretstore.ErrConflict) {
					return fmt.Errorf("a different credential already exists in the password manager location for %s/%s; nothing was overwritten. Choose another --profile name or review that entry before retrying", realm, name)
				}
				return err
			}
			cmd.Printf("Saved %s/%s as %s (%s).\n", realm, name, a.Identity, method)
			if changeDefault {
				cmd.Printf("Default profile: %s\n", qualified)
			} else if current != "" {
				cmd.Printf("Default profile remains: %s\n", current)
			}
			return nil
		},
	}
	authprofile.AddDefaultFlag(cmd)
	authprofile.AddStorageFlags(cmd, &provider, &insecure)
	cmd.Flags().StringSliceVar(&sites, "site", nil, "Explicitly allowed wiki API endpoints (repeat or comma-separate; includes --wiki)")
	cmd.Flags().StringVar(&method, "method", "oauth", "Authentication method: oauth (public client) or password")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read a wiki password from stdin")
	cmd.Flags().BoolVar(&replace, "replace", false, "Reauthorize an existing profile using its linked credential entry")
	cmd.Flags().StringVar(&clientID, "client-id", wikiauth.DefaultClientID, "Override the selected realm's built-in OAuth public client ID")
	// The actual default depends on the selected realm, not this flag's initializer.
	cmd.Flags().Lookup("client-id").DefValue = ""
	cmd.Flags().StringVar(&redirectURI, "redirect-uri", wikiauth.DefaultRedirectURI, "Exact callback URI registered for this OAuth client")
	cmd.Flags().StringSliceVar(&scopes, "scope", nil, "Explicit OAuth scope names, comma-separated or repeated (basic by default)")
	cmd.Flags().BoolVar(&allScopes, "all-scopes", false, "Explicitly request every grant in the shared MediaWiki catalogue, including internal/unknown grants (the wiki validates availability)")
	cmd.MarkFlagsMutuallyExclusive("scope", "all-scopes")
	_ = cmd.RegisterFlagCompletionFunc("scope", completeOAuthScopes)
	cmd.Flags().BoolVar(&manual, "manual", false, "Paste the callback URL without starting a local listener (pasting is also available alongside normal browser login)")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the authorization URL without opening a browser")
	return cmd
}

func authorizeWiki(ctx context.Context, cmd *cobra.Command, a wikiauth.Authorization, manual, noBrowser bool) (string, error) {
	var listener net.Listener
	var server *http.Server
	callbacks := make(chan string, 1)
	failed := make(chan error, 1)
	if !manual {
		u, _ := url.Parse(a.RedirectURI)
		address := u.Host
		if u.Port() == "" {
			address = net.JoinHostPort(u.Hostname(), "80")
		}
		var err error
		listener, err = net.Listen("tcp", address)
		if err != nil {
			cmd.PrintErrln("Could not listen on the OAuth callback port. Continue authorization in the browser, then paste its callback URL here.")
			manual = true
		} else {
			defer listener.Close()
			server = &http.Server{ReadHeaderTimeout: 5 * time.Second}
			server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Referrer-Policy", "no-referrer")
				if r.Method != http.MethodGet || r.Host != u.Host || r.URL.Path != u.Path {
					http.NotFound(w, r)
					return
				}
				callback := a.RedirectURI + "?" + r.URL.RawQuery
				if _, err := a.Code(callback); err != nil {
					http.Error(w, "Authorization callback invalid or declined.", http.StatusBadRequest)
					return
				}
				select {
				case callbacks <- callback:
					fmt.Fprintln(w, "Authorization received. Return to the terminal.")
				default:
					http.Error(w, "Callback already received.", http.StatusConflict)
				}
			})
			defer func() {
				shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := server.Shutdown(shutdown); err != nil {
					_ = server.Close()
				}
			}()
			// Start serving before launching the browser: a quick redirect must not
			// reach a listening socket whose HTTP handler is not running yet.
			go func() { failed <- server.Serve(listener) }()
		}
	}
	cmd.PrintErrln("Open this URL to authorize the wiki account:\n" + a.URL)
	if !noBrowser {
		if err := browser.OpenURL(a.URL); err != nil {
			cmd.PrintErrln("Could not open a browser automatically; open the authorization URL above manually.")
		}
	}
	paste, err := startCallbackInput(cmd, a)
	if err != nil {
		if manual {
			return "", err
		}
		cmd.PrintErrln("Could not enable hidden callback pasting; browser callbacks are still enabled. Use --manual on a new login if needed.")
	}
	var pasted <-chan callbackInputResult
	if paste != nil {
		defer paste.stop()
		pasted = paste.results
	}
	if manual && paste == nil {
		return readManualCallback(cmd, a)
	}
	if !manual && paste == nil {
		cmd.PrintErrln("Waiting for the browser callback. For pasted input from a non-terminal, use --manual.")
	}
	for {
		select {
		case callback := <-callbacks:
			return callback, nil
		case result := <-pasted:
			return result.URL, result.Err
		case <-ctx.Done():
			return "", fmt.Errorf("OAuth login timed out or was cancelled")
		case <-failed:
			if paste == nil {
				return "", fmt.Errorf("OAuth callback listener stopped; retry login with --manual")
			}
			cmd.PrintErrln("The callback listener stopped. Paste the browser callback URL into the hidden prompt to finish this login.")
		}
	}
}

func accountTokens(ctx context.Context, a config.WikiProfile, force bool) (wikiauth.Tokens, error) {
	return accountTokensWithProgress(ctx, a, force, nil)
}

func accountTokensWithProgress(ctx context.Context, a config.WikiProfile, force bool, progress func(string)) (wikiauth.Tokens, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	report := func(message string) {
		if progress != nil {
			progress(message)
		}
	}
	report("Reading saved OAuth credentials; unlock your password manager and approve access if prompted.")
	var tokens wikiauth.Tokens
	_, err := a.Credential().Update(ctx, func(value string) (string, error) {
		latest, err := config.LoadWikiProfile(a.Realm, a.Name)
		if err != nil {
			return "", err
		}
		if latest.Username != a.Username || latest.Method != a.Method || latest.Endpoint != a.Endpoint || latest.ClientID != a.ClientID || latest.RequestedScope != a.RequestedScope || !slices.Equal(latest.Sites, a.Sites) {
			return "", fmt.Errorf("wiki profile changed while waiting; retry the command")
		}
		if json.Unmarshal([]byte(value), &tokens) != nil || tokens.AccessToken == "" || !strings.EqualFold(tokens.TokenType, "Bearer") {
			return "", fmt.Errorf("invalid saved wiki tokens; log in again")
		}
		if !force && time.Now().Add(time.Minute).Before(tokens.ExpiresAt) {
			return value, nil
		}
		report("Refreshing the OAuth token with " + a.Endpoint + "...")
		tokens, err = wikiauth.RefreshAt(ctx, wikiauth.HTTPClient(), a.Endpoint, a.ClientID, tokens)
		if err != nil {
			return "", fmt.Errorf("could not refresh OAuth tokens: %w; older MediaWiki versions may not support public-client refresh. Reauthorize with wiki auth login --profile %s/%s --wiki %s --replace", err, a.Realm, a.Name, a.Wiki)
		}
		encoded, err := json.Marshal(tokens)
		if err != nil {
			return "", err
		}
		report("Saving refreshed OAuth credentials; approve the entry update in your password manager if prompted.")
		return string(encoded), nil
	})
	return tokens, err
}

func newWikiStatusCmd() *cobra.Command {
	var refresh bool
	cmd := &cobra.Command{Use: "status", Short: "Check the selected profile against the wiki without displaying secrets", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		a, err := selectedWikiProfile(wikiProfile)
		if err != nil {
			return err
		}
		endpoint := a.Wiki
		if wiki != "" {
			endpoint, err = wikiauth.Endpoint(wiki)
			if err != nil {
				return err
			}
		}
		if !a.Allows(endpoint) {
			return fmt.Errorf("wiki is outside the selected profile's site scope")
		}
		httpClient := wikiauth.HTTPClient()
		if a.Method == "oauth" {
			tokens, err := accountTokensWithProgress(cmd.Context(), a, refresh, func(message string) { cmd.PrintErrln(message) })
			if err != nil {
				return err
			}
			httpClient.Transport = wikiauth.BearerTransport{Endpoint: endpoint, Token: tokens.AccessToken}
		} else {
			if refresh {
				return fmt.Errorf("--refresh applies only to OAuth")
			}
			password, err := a.Credential().Read()
			if err != nil {
				return err
			}
			w, err := mwclient.New(endpoint, cli.HTTPUserAgent)
			if err != nil {
				return err
			}
			w.SetHTTPClient(httpClient)
			if err := wikiauth.LoginPassword(w, a.Username, password); err != nil {
				return fmt.Errorf("wiki authentication failed")
			}
		}
		cmd.PrintErrln("Checking the authenticated wiki identity...")
		identity, err := wikiauth.Identity(cmd.Context(), httpClient, endpoint)
		if err != nil {
			return err
		}
		expected := a.Identity
		if identity != expected {
			return fmt.Errorf("authenticated wiki identity does not match the saved account")
		}
		provider, err := a.Credential().CredentialProvider("")
		if err != nil {
			return err
		}
		cmd.Printf("Authenticated as %s on %s using %s/%s (%s, %s).\n", identity, endpoint, a.Realm, a.Name, a.Method, provider)
		if a.Method == "oauth" && a.RequestedScope != "" {
			cmd.Println("Requested OAuth scopes: " + a.RequestedScope)
		}
		return nil
	}}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh OAuth tokens now and save the replacement securely")
	return cmd
}

func newWikiRealmCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "realm", Short: "Configure groups of wikis that share a login"}
	var endpoints []string
	add := &cobra.Command{Use: "add NAME", Short: "Add a custom realm of trusted wiki API endpoints", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		_, builtin := wikiauth.BuiltinPreset(name)
		if !wikiauth.ValidName(name) || builtin {
			return fmt.Errorf("choose a valid custom realm name; built-in wiki realms cannot be overridden")
		}
		if len(endpoints) == 0 {
			return fmt.Errorf("provide --endpoint for each trusted wiki API")
		}
		if len(config.WikiRealmEndpoints(name)) > 0 {
			return fmt.Errorf("realm already exists; choose a new realm name")
		}
		for i, raw := range endpoints {
			endpoint, err := wikiauth.Endpoint(raw)
			if err != nil {
				return err
			}
			if wikiauth.IsWikimedia(endpoint) {
				return fmt.Errorf("Wikimedia endpoints belong to the built-in realm")
			}
			endpoints[i] = endpoint
		}
		return config.UpdateDiskConfig(func(disk *koanf.Koanf) error {
			if len(config.WikiRealmEndpoints(name)) > 0 {
				return fmt.Errorf("realm already exists; choose a new realm name")
			}
			return disk.Set("wiki.realms."+name+".endpoints", endpoints)
		})
	}}
	add.Flags().StringSliceVar(&endpoints, "endpoint", nil, "Trusted wiki API endpoint (repeat or comma-separate for a shared-login farm)")
	cmd.AddCommand(add)
	return cmd
}
