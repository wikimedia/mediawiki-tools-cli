package mediawiki

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/paths"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
)

// User run docker command with the specified -u.
var User string

func findParentCommandWithUse(cmd *cobra.Command, use string) *cobra.Command {
	if cmd.Use == use {
		return cmd
	}
	if cmd.Parent() == nil {
		return nil
	}
	return findParentCommandWithUse(cmd.Parent(), use)
}

func randomString() string {
	length := 10
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	return base32.StdEncoding.EncodeToString(randomBytes)[:length]
}

func NewMediaWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mediawiki",
		Short:   "MediaWiki service",
		Aliases: []string{"mw"},
		RunE:    nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Allways run the root level PersistentPreRun first
			findParentCommandWithUse(cmd, "mw").PersistentPreRun(cmd, args)
			findParentCommandWithUse(cmd, "docker").PersistentPreRun(cmd, args)
			mwdd := mwdd.DefaultForUser()
			mwdd.EnsureReady()

			// Skip the MediaWiki checks if the user is just trying to destroy the environment
			if strings.Contains(cobrautil.FullCommandString(cmd), "destroy") {
				return
			}

			usr, _ := user.Current()
			usrDir := usr.HomeDir

			// TODO: Might be better placed as a PersistentPreRun of the shellbox commands
			if mwdd.Env().Missing("SHELLBOX_SECRET_KEY") {
				mwdd.Env().Set("SHELLBOX_SECRET_KEY", randomString())
			}
			if mwdd.Env().Missing("MEDIAWIKI_VOLUMES_CODE") {
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
						mwdd.Env().Set("MEDIAWIKI_VOLUMES_CODE", paths.FullifyUserProvidedPath(dirValue))
					} else {
						fmt.Println("Can't continue without a MediaWiki code directory")
						os.Exit(1)
					}
				} else {
					mwdd.Env().Set("MEDIAWIKI_VOLUMES_CODE", mediawiki.GuessMediaWikiDirectoryBasedOnContext())
				}
			}

			// Default the mediawiki container to a .composer directory in the running users home dir
			if !mwdd.Env().Has("MEDIAWIKI_VOLUMES_DOT_COMPOSER") {
				logrus.Debug("MEDIAWIKI_VOLUMES_DOT_COMPOSER is missing")
				usrComposerDirectory := usrDir + "/.composer"
				if _, err := os.Stat(usrComposerDirectory); os.IsNotExist(err) {
					err := os.Mkdir(usrComposerDirectory, 0o755)
					if err != nil {
						fmt.Println("Failed to create directory needed for a composer cache")
						os.Exit(1)
					}
				}
				mwdd.Env().Set("MEDIAWIKI_VOLUMES_DOT_COMPOSER", usrDir+"/.composer")
			}

			// If we are not running get-code command, make sure we have code!
			if cmd.Use != "get-code" {
				mediawiki, _ := mediawiki.ForDirectory(mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE"))
				if !mediawiki.MediaWikiIsPresent() || !mediawiki.VectorIsPresent() {
					fmt.Println("MediaWiki or Vector is not present in the code directory")
					fmt.Println("You can clone them manually or use `mw docker mediawiki get-code`")
					os.Exit(1)
				}
			}
		},
	}

	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Service"

	cmd.AddCommand(mwdd.NewWhereCmd(
		"the MediaWiki directory",
		func() string { return mwdd.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE") },
	))
	cmd.AddCommand(NewMediaWikiFreshCmd())
	cmd.AddCommand(NewMediaWikiQuibbleCmd())
	cmd.AddCommand(mwdd.NewImageCmd("mediawiki"))
	cmd.AddCommand(mwdd.NewServiceCreateCmd("mediawiki", ""))
	cmd.AddCommand(mwdd.NewServiceDestroyCmd("mediawiki"))
	cmd.AddCommand(mwdd.NewServiceStopCmd("mediawiki"))
	cmd.AddCommand(mwdd.NewServiceStartCmd("mediawiki"))
	cmd.AddCommand(NewMediaWikiInstallCmd())
	cmd.AddCommand(NewMediaWikiGetCodeCmd())
	cmd.AddCommand(NewMediaWikiComposerCmd())
	cmd.AddCommand(NewMediaWikiJobRunnerCmd())
	cmd.AddCommand(NewMediaWikiExecCmd())
	cmd.AddCommand(NewMediaWikiMWScriptCmd())
	cmd.AddCommand(NewMediaWikiSitesCmd())
	cmd.AddCommand(NewMediaWikiDoctorCmd())
	return cmd
}

var applyRelevantMediawikiWorkingDirectory = func(options docker.ExecOptions, mountTo string) docker.ExecOptions {
	if resolvedPath := paths.ResolveMountForCwd(mwdd.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE"), mountTo); resolvedPath != nil {
		options.WorkingDir = *resolvedPath
	} else {
		options.WorkingDir = mountTo
	}
	return options
}
