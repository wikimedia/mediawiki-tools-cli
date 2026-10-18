package mediawiki

import (
	"testing"

	"github.com/spf13/cobra"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func TestUpdateCodeOnlyInitializesDockerForActualComposer(t *testing.T) {
	for _, tc := range []struct {
		name             string
		composer, dryRun bool
		wantDockerHooks  int
	}{
		{name: "git-only"},
		{name: "dry-run", dryRun: true},
		{name: "composer-preview", composer: true, dryRun: true},
		{name: "composer", composer: true, wantDockerHooks: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rootCalls, dockerCalls := 0, 0
			root := &cobra.Command{Use: "mw", PersistentPreRun: func(cmd *cobra.Command, args []string) { rootCalls++ }}
			dev := &cobra.Command{Use: "dev"}
			wiki := &cobra.Command{Use: "mediawiki"}
			for _, parent := range []*cobra.Command{dev, wiki} {
				parent.PersistentPreRun = func(cmd *cobra.Command, args []string) {
					dockerCalls++
					cobrautil.CallAllPersistentPreRun(cmd, args)
				}
			}
			root.AddCommand(dev)
			dev.AddCommand(wiki)
			command := NewMediaWikiUpdateCodeCmd()
			wiki.AddCommand(command)
			if tc.composer {
				if err := command.Flags().Set("composer", "true"); err != nil {
					t.Fatal(err)
				}
			}
			if tc.dryRun {
				if err := command.Flags().Set("dry-run", "true"); err != nil {
					t.Fatal(err)
				}
			}
			called := false
			command.RunE = func(cmd *cobra.Command, args []string) error { called = true; return nil }
			root.SetArgs([]string{"dev", "mediawiki", "update-code"})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if !called || rootCalls != 1 || dockerCalls != tc.wantDockerHooks {
				t.Fatalf("run=%v root hooks=%d docker hooks=%d (want %d)", called, rootCalls, dockerCalls, tc.wantDockerHooks)
			}
		})
	}
}
