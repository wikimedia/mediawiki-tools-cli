package mediawiki

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
)

//go:embed update-code.long.md
var updateCodeLong string

func NewMediaWikiUpdateCodeCmd() *cobra.Command {
	var dryRun, composer bool
	cmd := &cobra.Command{
		Use: "update-code", Short: "Fast-forward clean core, skin and extension checkouts", Long: cli.RenderMarkdown(updateCodeLong), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m := devenv.DefaultForUser()
			code, err := mediawiki.ForDirectory(m.Env().Get("MEDIAWIKI_VOLUMES_CODE"))
			if err != nil {
				return err
			}
			repos, err := code.SourceRepositories()
			if err != nil {
				return err
			}
			results, updateErr := mediawiki.UpdateSourceRepositories(cmd.Context(), repos, dryRun)
			skipped := false
			for _, result := range results {
				cmd.Printf("%s: %s", result.Name, result.Status)
				if result.Branch != "" {
					cmd.Printf(" (%s -> %s)", result.Branch, result.Upstream)
				}
				cmd.Println()
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show eligible/skipped repositories without fetching or updating")
	cmd.Flags().BoolVar(&composer, "composer", false, "Run Composer update in the PHP container after all source checkouts succeed")
	cmd.Flags().StringVarP(&User, "user", "u", docker.CurrentUserAndGroupForDockerExecution(), "User for the optional Composer command")
	return cmd
}
