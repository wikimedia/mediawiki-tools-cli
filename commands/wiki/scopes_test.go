package wiki

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func TestOAuthPermissionSelection(t *testing.T) {
	for _, test := range []struct {
		name, client                             string
		explicit                                 []string
		all, interactive, wantChooser, wantError bool
		want                                     []string
	}{
		{name: "noninteractive basic", want: []string{"basic"}},
		{name: "interactive choice", interactive: true, wantChooser: true, want: []string{"basic", "editpage"}},
		{name: "explicit basic", explicit: []string{"basic"}, interactive: true, want: []string{"basic"}},
		{name: "explicit upload", explicit: []string{"basic", "uploadfile"}, want: []string{"basic", "uploadfile"}},
		{name: "unsupported registered permission", explicit: []string{"delete"}, wantError: true},
		{name: "custom explicit permission", client: "custom", explicit: []string{"basic", "delete"}, want: []string{"basic", "delete"}},
		{name: "custom default basic", client: "custom", interactive: true, want: []string{"basic"}},
		{name: "custom explicit all", client: "custom", all: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "login"}
			cmd.Flags().StringSlice("scope", nil, "")
			client := test.client
			if client == "" {
				client = wikiauth.DefaultClientID
			}
			if test.explicit != nil {
				_ = cmd.Flags().Set("scope", strings.Join(test.explicit, ","))
			}
			called := false
			got, err := requestedOAuthScopes(cmd, client, test.explicit, test.all, test.interactive, nil, func([]string) ([]string, error) { called = true; return []string{"basic", "editpage"}, nil })
			if (err != nil) != test.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && !reflect.DeepEqual(got, test.want) {
				t.Fatalf("scopes=%v want %v", got, test.want)
			}
			if called != test.wantChooser {
				t.Fatal("unexpected interactive chooser", called)
			}
		})
	}
}

func TestOAuthAllRegisteredScopesAndReauthorizationDefaults(t *testing.T) {
	cmd := &cobra.Command{Use: "login"}
	cmd.Flags().StringSlice("scope", nil, "")
	got, err := requestedOAuthScopes(cmd, wikiauth.DefaultClientID, nil, true, true, nil, nil)
	if err != nil || len(got) != len(defaultOAuthGrants)+1 || got[0] != "basic" {
		t.Fatal(got, err)
	}
	for _, scope := range got {
		if scope == "delete" || scope == "privateinfo" || scope == "#default" {
			t.Fatal("unregistered or broadening permission", scope)
		}
	}
	previous := &config.WikiProfile{ClientID: wikiauth.DefaultClientID, RequestedScope: "basic editpage"}
	_, err = requestedOAuthScopes(cmd, wikiauth.DefaultClientID, nil, false, true, previous, func(defaults []string) ([]string, error) {
		if !reflect.DeepEqual(defaults, []string{"basic", "editpage"}) {
			t.Fatal("previous selection was not retained", defaults)
		}
		return nil, errors.New("cancelled")
	})
	if err == nil {
		t.Fatal("cancelled chooser continued authorization")
	}
}

func TestOAuthScopeOptionsAreExplicit(t *testing.T) {
	isolateWikiProfiles(t)
	for _, args := range [][]string{
		{"auth", "login", "--scope", "basic", "--all-scopes"},
		{"auth", "login", "--method", "password", "--all-scopes"},
		{"auth", "login", "--scope", "delete"},
	} {
		if _, err := runWikiProfile(t, args, ""); err == nil {
			t.Fatalf("invalid permission options accepted: %v", args)
		}
	}
	cmd := NewWikiCmd()
	login, _, err := cmd.Find([]string{"auth", "login"})
	if err != nil {
		t.Fatal(err)
	}
	if login.Flags().Lookup("client-id").DefValue != wikiauth.DefaultClientID || login.Flags().Lookup("redirect-uri").DefValue != "http://localhost:8765/auth/login" {
		t.Fatal("new client defaults not wired")
	}
	values, _ := completeOAuthScopes(nil, nil, "editp")
	if len(values) != 2 {
		t.Fatal("missing scope completions", values)
	}
}
