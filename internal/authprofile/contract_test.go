package authprofile_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/commands/cloudvps"
	"gitlab.wikimedia.org/repos/releng/cli/commands/enterprise"
	"gitlab.wikimedia.org/repos/releng/cli/commands/gerrit"
	"gitlab.wikimedia.org/repos/releng/cli/commands/gitlab"
	"gitlab.wikimedia.org/repos/releng/cli/commands/phabricator"
	"gitlab.wikimedia.org/repos/releng/cli/commands/wiki"
)

// Keep the service-independent auth surface stable even as login protocols differ.
func TestServiceProfileCommandContract(t *testing.T) {
	for _, parent := range []*cobra.Command{gerrit.NewGerritCmd(), phabricator.NewPhabricatorCmd(), wiki.NewWikiCmd(), cloudvps.NewCloudVPSCmd(), gitlab.Cmd(), enterprise.NewCmd()} {
		t.Run(parent.Name(), func(t *testing.T) {
			flag := parent.PersistentFlags().Lookup("profile")
			if flag == nil || flag.Value.Type() != "string" {
				t.Fatal("missing persistent --profile")
			}
			if _, ok := parent.GetFlagCompletionFunc("profile"); !ok {
				t.Fatal("missing profile completion")
			}
			auth, _, err := parent.Find([]string{"auth"})
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, child := range auth.Commands() {
				if child.Hidden {
					continue
				}
				names = append(names, child.Name())
				for _, alias := range child.Aliases {
					if alias == "logout" {
						t.Fatal("misleading logout alias")
					}
				}
			}
			sort.Strings(names)
			want := []string{"default", "login", "profiles", "remove", "status"}
			if parent.Name() == "enterprise" {
				want = append(want, "revoke")
				sort.Strings(want)
			}
			if !reflect.DeepEqual(names, want) {
				t.Fatalf("auth commands=%v, want %v", names, want)
			}
			login, _, err := parent.Find([]string{"auth", "login"})
			if err != nil {
				t.Fatal(err)
			}
			defaultFlag := login.Flags().Lookup("default")
			if defaultFlag == nil || defaultFlag.Value.Type() != "bool" || defaultFlag.DefValue != "false" {
				t.Fatal("inconsistent login --default flag")
			}
			if _, ok := login.GetFlagCompletionFunc("provider"); !ok {
				t.Fatal("missing provider completion")
			}
			if login.Flags().Lookup("insecure-storage") == nil {
				t.Fatal("missing plaintext storage flag")
			}
			profiles, _, err := parent.Find([]string{"auth", "list"})
			if err != nil || profiles.Name() != "profiles" {
				t.Fatal("missing list alias")
			}
		})
	}
}
