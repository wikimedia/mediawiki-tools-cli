package mediawiki

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/paths"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
)

// User run docker command with the specified -u.
var User string

func NewMediaWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mediawiki",
		Short:   "MediaWiki service",
		Long:    "Create and manage MediaWiki, fetch code, and run maintenance or development tools.\n\nPHP debugging: mw help guides dev xdebug\nBrowser tests and recording: mw help guides dev fresh",
		GroupID: "service",
		Aliases: []string{"mw"},
		RunE:    nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cobrautil.CallAllPersistentPreRun(cmd, args)
			environment := devenv.DefaultForUser()
			environment.EnsureReady()

			// Skip the MediaWiki checks if the user is just trying to destroy the environment
			if strings.Contains(cobrautil.FullCommandString(cmd), "destroy") || cmd.Name() == "doctor" {
				return
			}

			usr, _ := user.Current()
			usrDir := usr.HomeDir

			if environment.Env().Missing("MEDIAWIKI_VOLUMES_CODE") {
				logrus.Debug("MEDIAWIKI_VOLUMES_CODE is missing")
				if !cli.Opts.NoInteraction {
					// Prompt the user for a directory or confirmation
					dirValue := ""
					prompt := &survey.Input{
						Message: "What directory would you like to store MediaWiki source code in?",
						Default: mediawiki.GuessMediaWikiDirectoryBasedOnContext(),
						Suggest: func(toComplete string) []string {
							files, _ := filepath.Glob(toComplete + "*")
							return files
						},
					}
					err := survey.AskOne(prompt, &dirValue)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					// TODO check if path looks valid?

					if err == nil {
						environment.Env().Set("MEDIAWIKI_VOLUMES_CODE", paths.FullifyUserProvidedPath(dirValue))
					} else {
						fmt.Println("Can't continue without a MediaWiki code directory")
						os.Exit(1)
					}
				} else {
					environment.Env().Set("MEDIAWIKI_VOLUMES_CODE", mediawiki.GuessMediaWikiDirectoryBasedOnContext())
				}
			}

			// Default the mediawiki container to a .composer directory in the running users home dir
			if !environment.Env().Has("MEDIAWIKI_VOLUMES_DOT_COMPOSER") {
				logrus.Debug("MEDIAWIKI_VOLUMES_DOT_COMPOSER is missing")
				usrComposerDirectory := usrDir + "/.composer"
				if _, err := os.Stat(usrComposerDirectory); os.IsNotExist(err) {
					err := os.Mkdir(usrComposerDirectory, 0o755) // #nosec G301
					if err != nil {
						fmt.Println("Failed to create directory needed for a composer cache")
						os.Exit(1)
					}
				}
				environment.Env().Set("MEDIAWIKI_VOLUMES_DOT_COMPOSER", usrDir+"/.composer")
			}

			// If we are not running get-code command, make sure we have code!
			if os.Getenv("MW_DOCKER_MEDIAWIKI_GET_CODE") == "" && cmd.Name() != "update-code" {
				mediawiki, _ := mediawiki.ForDirectory(environment.Env().Get("MEDIAWIKI_VOLUMES_CODE"))
				if !mediawiki.MediaWikiIsPresent() || !mediawiki.VectorIsPresent() {
					fmt.Fprintf(cmd.ErrOrStderr(), "MediaWiki core or Vector is missing from %s.\n", mediawiki.Directory())
					fmt.Fprintf(cmd.ErrOrStderr(), "Run `%s dev --context %q mediawiki get-code` to fetch the missing code, then retry.\n", cmd.Root().Name(), devenv.Context)
					os.Exit(1)
				}
			}
		},
	}

	cmd.AddCommand(devenv.NewWhereCmd(
		"the MediaWiki directory",
		func() string { return devenv.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE") },
	))
	cmd.AddCommand(NewMediaWikiFreshCmd())
	cmd.AddCommand(NewMediaWikiNPMCmd())
	cmd.AddCommand(NewMediaWikiNPXCmd())
	cmd.AddCommand(NewMediaWikiQuibbleCmd())
	cmd.AddCommand(devenv.NewImageCmd("mediawiki"))
	for _, serviceUp := range []*cobra.Command{
		devenv.NewServiceCreateCmd("mediawiki", ""),
		devenv.NewServiceRebuildCmd("mediawiki", ""),
	} {
		serviceUp.PostRun = func(cmd *cobra.Command, args []string) {
			if url := devenv.DefaultForUser().RunningDashboardURL(); url != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Dashboard: %s\n", url)
			}
		}
		cmd.AddCommand(serviceUp)
	}
	cmd.AddCommand(devenv.NewServiceDestroyCmd("mediawiki"))
	cmd.AddCommand(devenv.NewServiceStopCmd("mediawiki"))
	cmd.AddCommand(devenv.NewServiceStartCmd("mediawiki"))
	cmd.AddCommand(NewMediaWikiInstallCmd())
	cmd.AddCommand(NewMediaWikiGetCodeCmd())
	cmd.AddCommand(NewMediaWikiUpdateCodeCmd())
	cmd.AddCommand(NewMediaWikiApplyPatchesCmd())
	cmd.AddCommand(NewMediaWikiComposerCmd())
	cmd.AddCommand(NewMediaWikiJobRunnerCmd())
	cmd.AddCommand(NewMediaWikiExecCmd())
	cmd.AddCommand(NewMediaWikiForeachwikiCmd())
	cmd.AddCommand(NewMediaWikiMWScriptCmd())
	cmd.AddCommand(NewMediaWikiSitesCmd())
	cmd.AddCommand(NewMediaWikiDoctorCmd())
	return cmd
}

var applyRelevantMediawikiWorkingDirectory = func(options docker.ExecOptions, mountTo string) docker.ExecOptions {
	if resolvedPath := paths.ResolveMountForCwd(devenv.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE"), mountTo); resolvedPath != nil {
		options.WorkingDir = *resolvedPath
	} else {
		options.WorkingDir = mountTo
	}
	return options
}
