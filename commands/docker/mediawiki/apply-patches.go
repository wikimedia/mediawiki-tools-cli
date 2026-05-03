package mediawiki

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

//go:embed apply-patches.long.md
var applyPatchesLong string

//go:embed apply-patches.example
var applyPatchesExample string

func NewMediaWikiApplyPatchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apply-patches",
		Example: cobrautil.NormalizeExample(applyPatchesExample),
		Short:   "Apply Gerrit patches to local MediaWiki code",
		Long:    cli.RenderMarkdown(applyPatchesLong),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			os.Setenv("MW_DOCKER_MEDIAWIKI_GET_CODE", "1")
			cobrautil.CallAllPersistentPreRun(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			changes, _ := cmd.Flags().GetStringSlice("change")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			if len(changes) == 0 {
				return fmt.Errorf("at least one --change is required")
			}

			mwdd := mwdd.DefaultForUser()
			mwdd.EnsureReady()
			thisMw, _ := mediawiki.ForDirectory(mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE"))

			cli.NewEvents(cli.UserDirectoryPath()+string(os.PathSeparator)+".events").AddFeatureUsageEvent("mw_docker_mediawiki_apply-patches", cli.VersionDetails.Version)

			opts := mediawiki.ApplyPatchOpts{
				ChangeIDs: changes,
				DryRun:    dryRun,
			}

			return thisMw.ApplyGerritPatches(cmd.Context(), opts)
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Core"

	cmd.Flags().StringSlice("change", []string{}, "Gerrit change number(s) to apply (repeatable)")
	cmd.Flags().Bool("dry-run", false, "Show what would be done without actually doing it")

	return cmd
}
