package mediawiki

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
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

func randomString() (string, error) {
	length := 10
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(randomBytes)[:length], nil
}

func NewMediaWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mediawiki",
		Short:   "MediaWiki service",
		Long:    "Create and manage MediaWiki, fetch code, and run maintenance or development tools.\n\nPHP debugging: mw help guides dev xdebug\nBrowser tests and recording: mw help guides dev fresh",
		GroupID: "service",
		Aliases: []string{"mw"},
		RunE:    nil,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cobrautil.CallAllPersistentPreRunE(cmd, args); err != nil {
				return err
			}
			devenv := devenv.DefaultForUser()
			if err := devenv.EnsureReady(); err != nil {
				return err
			}

			// Skip the MediaWiki checks if the user is just trying to destroy the environment
			if strings.Contains(cobrautil.FullCommandString(cmd), "destroy") || cmd.Name() == "doctor" {
				return nil
			}

			usr, err := user.Current()
			if err != nil {
				return err
			}
			usrDir := usr.HomeDir

			// TODO: Might be better placed as a PersistentPreRun of the shellbox commands
			if devenv.Env().Missing("SHELLBOX_SECRET_KEY") {
				secret, err := randomString()
				if err != nil {
					return err
				}
				if err := devenv.Env().Set("SHELLBOX_SECRET_KEY", secret); err != nil {
					return err
				}
			}
			if devenv.Env().Missing("MEDIAWIKI_VOLUMES_CODE") {
				slog.Debug(fmt.Sprint("MEDIAWIKI_VOLUMES_CODE is missing"))
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
						return err
					}
					// TODO check if path looks valid?

					if err == nil {
						if err := devenv.Env().Set("MEDIAWIKI_VOLUMES_CODE", paths.FullifyUserProvidedPath(dirValue)); err != nil {
							return err
						}
					} else {
						return fmt.Errorf("Can't continue without a MediaWiki code directory")
					}
				} else {
					if err := devenv.Env().Set("MEDIAWIKI_VOLUMES_CODE", mediawiki.GuessMediaWikiDirectoryBasedOnContext()); err != nil {
						return err
					}
				}
			}

			// Default the mediawiki container to a .composer directory in the running users home dir
			if !devenv.Env().Has("MEDIAWIKI_VOLUMES_DOT_COMPOSER") {
				slog.Debug(fmt.Sprint("MEDIAWIKI_VOLUMES_DOT_COMPOSER is missing"))
				usrComposerDirectory := usrDir + "/.composer"
				if _, err := os.Stat(usrComposerDirectory); os.IsNotExist(err) {
					err := os.Mkdir(usrComposerDirectory, 0o755) // #nosec G301
					if err != nil {
						return fmt.Errorf("create Composer cache directory: %w", err)
					}
				}
				if err := devenv.Env().Set("MEDIAWIKI_VOLUMES_DOT_COMPOSER", usrDir+"/.composer"); err != nil {
					return err
				}
			}

			// If we are not running get-code command, make sure we have code!
			if os.Getenv("MW_DOCKER_MEDIAWIKI_GET_CODE") == "" && cmd.Name() != "update-code" {
				mediawiki, err := mediawiki.ForDirectory(devenv.Env().Get("MEDIAWIKI_VOLUMES_CODE"))
				if err != nil {
					return err
				}
				if !mediawiki.MediaWikiIsPresent() || !mediawiki.VectorIsPresent() {
					fmt.Fprintln(cmd.ErrOrStderr(), "MediaWiki or Vector is not present in the code directory")
					return fmt.Errorf("You can clone them manually or use `mw dev mediawiki get-code`")
				}
			}
			return nil
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
	cmd.AddCommand(devenv.NewServiceCreateCmd("mediawiki", ""))
	cmd.AddCommand(devenv.NewServiceRebuildCmd("mediawiki", ""))
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
