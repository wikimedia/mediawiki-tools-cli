package wiki

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestWikiCompletionsAreLocalAndRespectRealms(t *testing.T) {
	isolateWikiProfiles(t)
	seedWikiOAuth(t, "personal")
	seedWikiOAuth(t, "bot")
	custom := config.WikiProfile{Realm: "independent", Name: "personal", Username: "Other", Method: "bot-password", Endpoint: "https://wikibase.world/w/api.php", Wiki: "https://wikibase.world/w/api.php"}
	for key, value := range custom.Metadata() {
		if err := config.PutKeyValueOnDisk(key, value); err != nil {
			t.Fatal(err)
		}
	}
	disk := config.State().OnDiskKoanf.Copy()
	if err := disk.Set("wiki.realms.independent.endpoints", []string{custom.Endpoint}); err != nil {
		t.Fatal(err)
	}
	if err := config.PutDiskConfig(disk); err != nil {
		t.Fatal(err)
	}
	if err := config.PutKeyValueOnDisk("wiki.default_profile", "wikimedia/personal"); err != nil {
		t.Fatal(err)
	}
	// Completion must still work when accessing a credential would fail.
	keyring.MockInitWithError(io.ErrUnexpectedEOF)
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = wikiRoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("completion contacted the network")
		return nil, io.ErrUnexpectedEOF
	})
	for _, tc := range []struct {
		flag, prefix, realm, account string
		want                         []string
	}{
		{"profile", "", "", "", []string{"independent/personal", "wikimedia/bot", "wikimedia/personal"}},
		{"profile", "pers", "", "", []string{"personal"}},
		{"profile", "", "independent", "", []string{"personal"}},
		{"profile", "wikimedia/b", "", "", []string{"wikimedia/bot"}},
		{"realm", "", "", "", []string{"independent", "wikimedia"}},
		{"wiki", "https://en.wikipedia.", "", "", []string{"https://en.wikipedia.org/w/api.php"}},
		{"wiki", "test.wikipedia.", "", "", []string{"test.wikipedia.org/w/api.php"}},
		{"wiki", "", "independent", "", []string{custom.Endpoint}},
		{"wiki", "", "", "independent/personal", []string{custom.Endpoint}},
		{"wiki", "https://wikibase.world", "", "wikimedia/bot", nil},
	} {
		t.Run(tc.flag+"/"+tc.prefix+"/"+tc.realm+"/"+tc.account, func(t *testing.T) {
			cmd := NewWikiCmd()
			if err := cmd.PersistentFlags().Set("realm", tc.realm); err != nil {
				t.Fatal(err)
			}
			if err := cmd.PersistentFlags().Set("profile", tc.account); err != nil {
				t.Fatal(err)
			}
			callback, exists := cmd.GetFlagCompletionFunc(tc.flag)
			if !exists || callback == nil {
				t.Fatal("flag completion not registered")
			}
			values, directive := callback(cmd, nil, tc.prefix)
			var got []string
			for _, value := range values {
				name, _, _ := strings.Cut(value, "\t")
				got = append(got, name)
			}
			if !reflect.DeepEqual(got, tc.want) || directive != cobra.ShellCompDirectiveNoFileComp {
				t.Fatalf("got %v (directive=%v), want %v", got, directive, tc.want)
			}
		})
	}
}

func TestAccountCompletionIsInheritedByWikiSubcommands(t *testing.T) {
	isolateWikiProfiles(t)
	seedWikiOAuth(t, "personal")
	root := &cobra.Command{Use: "mw"}
	root.AddGroup(&cobra.Group{ID: "service", Title: "Services"})
	root.AddCommand(NewWikiCmd())
	var output strings.Builder
	root.SetOut(&output)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"__completeNoDesc", "wiki", "auth", "status", "--profile", "wikimedia/p"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "wikimedia/personal\n") || !strings.Contains(output.String(), ":4") {
		t.Fatalf("inherited shell completion failed: %s", output.String())
	}
}
