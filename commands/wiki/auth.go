package wiki

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	mwclient "cgt.name/pkg/go-mwclient"
	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
	"golang.org/x/term"
)

func newWikiAuthCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Save and manage wiki accounts"}
	cmd.AddCommand(newWikiLoginCmd(), newWikiStatusCmd(), newWikiLogoutCmd(), newWikiAccountsCmd(), newWikiDefaultCmd())
	return cmd
}

func selectedWikiAccount() (config.WikiAccount, error) {
	realm, name := wikiRealm, wikiAccount
	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		if realm != "" && realm != parts[0] {
			return config.WikiAccount{}, fmt.Errorf("--realm and --account disagree")
		}
		realm, name = parts[0], parts[1]
	}
	if realm == "" && wiki != "" {
		endpoint, err := wikiauth.Endpoint(wiki)
		if err != nil {
			return config.WikiAccount{}, err
		}
		realm, err = realmForWiki(endpoint)
		if err != nil {
			return config.WikiAccount{}, err
		}
	}
	if realm == "" {
		realm = config.State().OnDiskKoanf.String("wiki.default_realm")
	}
	if name == "" {
		name = config.State().OnDiskKoanf.String("wiki.realms." + realm + ".default_account")
	}
	return config.LoadWikiAccount(realm, name)
}

func newWikiLoginCmd() *cobra.Command {
	var method, provider, clientID, redirectURI string
	var manual, noBrowser, passwordStdin, insecure bool
	cmd := &cobra.Command{
		Use: "login", Short: "Authorize and save a wiki account",
		Long: "Save a wiki account using OAuth or a bot password. OAuth currently uses an approved basic-access development client, not a production a-wikimedia-cli registration. Its default callback is http://localhost/auth/login; use --manual when port 80 is occupied. No client secret is required.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if passwordStdin && (wikiPassword != "" || wikiPasswordFile != "") {
				return fmt.Errorf("--password-stdin cannot be combined with --password or --password-file")
			}
			if err := config.ValidateCredentialProvider(provider); provider != "" && err != nil {
				return err
			}
			if method != "oauth" && method != "bot-password" {
				return fmt.Errorf("--method must be oauth or bot-password")
			}
			realm, name := wikiRealm, wikiAccount
			if strings.Contains(name, "/") {
				parts := strings.SplitN(name, "/", 2)
				if realm != "" && realm != parts[0] {
					return fmt.Errorf("--realm and --account disagree")
				}
				realm, name = parts[0], parts[1]
			}
			if realm == "" {
				realm = "wikimedia"
			}
			if name == "" {
				name = "default"
			}
			prefix, err := config.WikiAccountPrefix(realm, name)
			if err != nil {
				return err
			}
			if config.State().OnDiskKoanf.Exists(strings.TrimSuffix(prefix, ".")) {
				return fmt.Errorf("account already exists; log out before replacing it")
			}
			target := wiki
			if target == "" && realm == "wikimedia" {
				target = wikiauth.WikimediaAuthority + "/w/api.php"
			}
			endpoint, err := wikiauth.Endpoint(target)
			if err != nil {
				return err
			}
			custom := config.WikiRealmEndpoints(realm)
			if realm != "wikimedia" && len(custom) == 0 {
				custom = []string{endpoint}
			}
			if !wikiauth.Allows(realm, endpoint, custom) {
				return fmt.Errorf("wiki is not a member of realm %s", realm)
			}
			a := config.WikiAccount{Realm: realm, Name: name, Method: method, Wiki: endpoint, Endpoint: endpoint}
			if realm == "wikimedia" {
				a.Endpoint = wikiauth.WikimediaAuthority
			}
			var secret string
			if method == "oauth" {
				if realm != "wikimedia" {
					return fmt.Errorf("OAuth is currently supported only for Wikimedia; use --method bot-password for custom wikis")
				}
				if wikiUser != "" || wikiPassword != "" || wikiPasswordFile != "" || passwordStdin {
					return fmt.Errorf("OAuth uses browser authorization; omit password and user options")
				}
				a.ClientID, a.RedirectURI = clientID, redirectURI
				if clientID == wikiauth.TestClientID {
					cmd.PrintErrln("Using the basic-access development OAuth client; edits and uploads are not authorized.")
				}
				authorization, err := wikiauth.Begin(clientID, redirectURI)
				if err != nil {
					return err
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
					return fmt.Errorf("provide --user (use the bot-password username, such as Name@cli)")
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
					err = survey.AskOne(&survey.Password{Message: "Wiki bot password:"}, &secret, survey.WithValidator(survey.Required))
				}
				if err != nil {
					return fmt.Errorf("could not read bot password")
				}
				if secret == "" {
					return fmt.Errorf("bot password is empty")
				}
				w, err := mwclient.New(endpoint, cli.HTTPUserAgent)
				if err != nil {
					return err
				}
				w.SetHTTPClient(wikiauth.HTTPClient())
				if err := w.Login(a.Username, secret); err != nil {
					return fmt.Errorf("wiki credentials were not validated or saved")
				}
			}
			metadata := a.Metadata()
			// A new custom realm initially trusts only this exact endpoint.
			if realm != "wikimedia" && len(config.WikiRealmEndpoints(realm)) == 0 {
				// Store realm membership only alongside the verified account below.
				metadata["wiki.realms."+realm+".endpoint"] = endpoint
			}
			if err := a.Credential().SaveWithProvider(secret, insecure, metadata, provider); err != nil {
				return err
			}
			cmd.Printf("Saved %s/%s as %s (%s).\n", realm, name, a.Username, method)
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "oauth", "Authentication method: oauth or bot-password")
	cmd.Flags().StringVar(&provider, "provider", "", "Credential provider: keyring, keepassxc or plaintext")
	cmd.Flags().BoolVar(&insecure, "insecure-storage", false, "Explicitly store credentials in plaintext configuration")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read a bot password from stdin")
	cmd.Flags().StringVar(&clientID, "client-id", wikiauth.TestClientID, "OAuth public client ID (development client by default)")
	cmd.Flags().StringVar(&redirectURI, "redirect-uri", wikiauth.TestRedirectURI, "Exact callback URI registered for this OAuth client")
	cmd.Flags().BoolVar(&manual, "manual", false, "Paste the callback URL instead of starting a local listener")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the authorization URL without opening a browser")
	cmd.MarkFlagsMutuallyExclusive("provider", "insecure-storage")
	return cmd
}

func authorizeWiki(ctx context.Context, cmd *cobra.Command, a wikiauth.Authorization, manual, noBrowser bool) (string, error) {
	var listener net.Listener
	if !manual {
		u, _ := url.Parse(a.RedirectURI)
		address := u.Host
		if u.Port() == "" {
			address = net.JoinHostPort(u.Hostname(), "80")
		}
		var err error
		listener, err = net.Listen("tcp", address)
		if err != nil {
			return "", fmt.Errorf("could not listen on OAuth callback port; use --manual or a client registered for another port")
		}
		defer listener.Close()
	}
	cmd.PrintErrln("Open this URL to authorize the wiki account:\n" + a.URL)
	if !noBrowser {
		_ = browser.OpenURL(a.URL)
	}
	if manual {
		cmd.PrintErr("Paste the full callback URL (input hidden on terminals): ")
		if file, ok := cmd.InOrStdin().(*os.File); ok && term.IsTerminal(int(file.Fd())) {
			data, err := term.ReadPassword(int(file.Fd()))
			cmd.PrintErrln()
			return string(data), err
		}
		line, err := bufio.NewReader(io.LimitReader(cmd.InOrStdin(), 16<<10)).ReadString('\n')
		if err != nil && !(err == io.EOF && strings.TrimSpace(line) != "") {
			return "", fmt.Errorf("could not read callback URL")
		}
		return strings.TrimSpace(line), nil
	}
	callbacks := make(chan string, 1)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := url.Parse(a.RedirectURI)
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
		shutdownContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			_ = server.Close()
		}
	}()
	failed := make(chan error, 1)
	go func() { failed <- server.Serve(listener) }()
	select {
	case callback := <-callbacks:
		return callback, nil
	case <-ctx.Done():
		return "", fmt.Errorf("OAuth login timed out or was cancelled")
	case <-failed:
		return "", fmt.Errorf("OAuth callback listener stopped")
	}
}

func accountTokens(ctx context.Context, a config.WikiAccount, force bool) (wikiauth.Tokens, error) {
	value, err := a.Credential().Read()
	if err != nil {
		return wikiauth.Tokens{}, err
	}
	var tokens wikiauth.Tokens
	if json.Unmarshal([]byte(value), &tokens) != nil || tokens.AccessToken == "" || !strings.EqualFold(tokens.TokenType, "Bearer") {
		return tokens, fmt.Errorf("invalid saved wiki tokens; log in again")
	}
	if force || time.Now().Add(time.Minute).After(tokens.ExpiresAt) {
		tokens, err = wikiauth.Refresh(ctx, wikiauth.HTTPClient(), a.ClientID, tokens)
		if err != nil {
			return tokens, err
		}
		encoded, err := json.Marshal(tokens)
		if err != nil {
			return tokens, err
		}
		if err := a.Credential().SaveWithProvider(string(encoded), false, nil, ""); err != nil {
			return tokens, fmt.Errorf("could not save refreshed wiki tokens: %w", err)
		}
	}
	return tokens, nil
}

func newWikiStatusCmd() *cobra.Command {
	var refresh bool
	cmd := &cobra.Command{Use: "status", Short: "Verify the saved wiki account without displaying secrets", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		a, err := selectedWikiAccount()
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
		if !config.WikiRealmAllows(a.Realm, endpoint) {
			return fmt.Errorf("wiki is not a member of the account's realm")
		}
		httpClient := wikiauth.HTTPClient()
		if a.Method == "oauth" {
			tokens, err := accountTokens(cmd.Context(), a, refresh)
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
			if err := w.Login(a.Username, password); err != nil {
				return fmt.Errorf("wiki authentication failed")
			}
		}
		identity, err := wikiauth.Identity(cmd.Context(), httpClient, endpoint)
		if err != nil {
			return err
		}
		expected := a.Username
		if a.Method == "bot-password" {
			expected = strings.SplitN(expected, "@", 2)[0]
		}
		if identity != expected {
			return fmt.Errorf("authenticated wiki identity does not match the saved account")
		}
		provider, err := a.Credential().CredentialProvider("")
		if err != nil {
			return err
		}
		cmd.Printf("Authenticated as %s on %s using %s/%s (%s, %s).\n", identity, endpoint, a.Realm, a.Name, a.Method, provider)
		return nil
	}}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh OAuth tokens now and save the replacement securely")
	return cmd
}

func newWikiLogoutCmd() *cobra.Command {
	return &cobra.Command{Use: "logout", Short: "Remove a saved account and its locally managed credential", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		a, err := selectedWikiAccount()
		if err != nil {
			return err
		}
		if err := a.Credential().Delete(); err != nil {
			return err
		}
		prefix, _ := config.WikiAccountPrefix(a.Realm, a.Name)
		disk := config.State().OnDiskKoanf.Copy()
		disk.Delete(strings.TrimSuffix(prefix, "."))
		key := "wiki.realms." + a.Realm + ".default_account"
		if disk.String(key) == a.Name {
			disk.Delete(key)
		}
		if err := config.PutDiskConfig(disk); err != nil {
			return err
		}
		cmd.Printf("Removed %s/%s locally. OAuth authorization and external KeePassXC entries are retained.\n", a.Realm, a.Name)
		return nil
	}}
}

func newWikiAccountsCmd() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List saved wiki account identities without opening credential stores", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		keys := config.State().OnDiskKoanf.Keys()
		sort.Strings(keys)
		for _, key := range keys {
			parts := strings.Split(key, ".")
			if len(parts) != 5 || parts[0] != "wiki" || parts[1] != "accounts" || parts[4] != "username" {
				continue
			}
			a, err := config.LoadWikiAccount(parts[2], parts[3])
			if err != nil {
				return err
			}
			marker := ""
			if config.State().OnDiskKoanf.String("wiki.realms."+a.Realm+".default_account") == a.Name {
				marker = " (default)"
			}
			cmd.Printf("%s/%s\t%s\t%s%s\n", a.Realm, a.Name, a.Username, a.Method, marker)
		}
		return nil
	}}
}

func newWikiDefaultCmd() *cobra.Command {
	return &cobra.Command{Use: "default", Short: "Make --account the default for its realm", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		a, err := selectedWikiAccount()
		if err != nil {
			return err
		}
		disk := config.State().OnDiskKoanf.Copy()
		_ = disk.Set("wiki.realms."+a.Realm+".default_account", a.Name)
		_ = disk.Set("wiki.default_realm", a.Realm)
		return config.PutDiskConfig(disk)
	}}
}

func newWikiRealmCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "realm", Short: "Configure custom groups of wikis sharing credentials"}
	var endpoints []string
	add := &cobra.Command{Use: "add NAME", Short: "Explicitly authorize exact wiki endpoints to share a login", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if !wikiauth.ValidName(name) || name == "wikimedia" {
			return fmt.Errorf("choose a valid custom realm name; the built-in Wikimedia realm cannot be overridden")
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
		disk := config.State().OnDiskKoanf.Copy()
		if err := disk.Set("wiki.realms."+name+".endpoints", endpoints); err != nil {
			return err
		}
		return config.PutDiskConfig(disk)
	}}
	add.Flags().StringSliceVar(&endpoints, "endpoint", nil, "Exact trusted API endpoint (repeat for a shared-login farm)")
	cmd.AddCommand(add)
	return cmd
}
