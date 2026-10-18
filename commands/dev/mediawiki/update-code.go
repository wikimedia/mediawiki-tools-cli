package mediawiki

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
)

//go:embed update-code.long.md
var updateCodeLong string

//go:embed update-code.example
var updateCodeExample string

func NewMediaWikiUpdateCodeCmd() *cobra.Command {
	var dryRun, composer bool
	var mode string
	cmd := &cobra.Command{
		Example: cobrautil.NormalizeExample(updateCodeExample),
		Use:     "update-code", Short: "Update core, skin and extension checkouts", Long: cli.RenderMarkdown(updateCodeLong), Args: cobra.NoArgs,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if !composer || dryRun {
				// Git-only updates need mw's setup, but no Docker daemon,
				// environment provisioning or MediaWiki/container wizards.
				root := cmd.Root()
				if root != cmd && root.PersistentPreRun != nil {
					root.PersistentPreRun(cmd, args)
				}
				return
			}
			cobrautil.CallAllPersistentPreRun(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			m := devenv.DefaultForUser()
			env, err := godotenv.Read(m.Env().Path())
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			directory := env["MEDIAWIKI_VOLUMES_CODE"]
			if directory == "" {
				return fmt.Errorf("MEDIAWIKI_VOLUMES_CODE is not configured; run mw dev mediawiki get-code first")
			}
			code, err := mediawiki.ForDirectory(directory)
			if err != nil {
				return err
			}
			repos, err := code.SourceRepositories()
			if err != nil {
				return err
			}
			results, updateErr := mediawiki.UpdateSourceRepositoriesWithOptions(cmd.Context(), repos, mediawiki.SourceUpdateOptions{DryRun: dryRun, Mode: mode})
			skipped := false
			for _, result := range results {
				cmd.Printf("%s: %s", result.Name, result.Status)
				if result.Branch != "" {
					cmd.Printf(" (%s -> %s)", result.Branch, result.Upstream)
				}
				cmd.Println()
				if result.Detail != "" {
					cmd.Printf("  %s\n", result.Detail)
				}
				skipped = skipped || strings.HasPrefix(result.Status, "skipped-")
			}
			if updateErr != nil {
				return updateErr
			}
			if !composer {
				return nil
			}
			if skipped {
				return fmt.Errorf("some checkouts were skipped; review them before running composer update explicitly")
			}
			if dryRun {
				cmd.Println("Would run composer update after source updates")
				return nil
			}
			containerID, err := m.DockerCompose().ContainerID("mediawiki")
			if err != nil {
				return err
			}
			codeStatus := docker.Exec(containerID, docker.ExecOptions{Command: composerCommand("update", "--no-interaction"), WorkingDir: "/var/www/html/w", User: User})
			if codeStatus != 0 {
				return fmt.Errorf("composer update failed (exit code %d)", codeStatus)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "ff-only", "Update mode: ff-only, stash (reapply changes), or mop (save changes and reset to the default branch)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show eligible/skipped repositories without fetching or updating")
	cmd.Flags().BoolVar(&composer, "composer", false, "Run Composer update in the PHP container after all source checkouts succeed")
	cmd.Flags().StringVarP(&User, "user", "u", docker.CurrentUserAndGroupForDockerExecution(), "User for the optional Composer command")
	return cmd
}
