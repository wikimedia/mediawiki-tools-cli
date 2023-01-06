package docker

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/eventlogging"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/paths"
)

func NewMediaWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mediawiki",
		Short:   "MediaWiki service",
		Aliases: []string{"mw"},
		RunE:    nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmd.Parent().Parent().PersistentPreRun(cmd, args)
			mwdd := mwdd.DefaultForUser()
			mwdd.EnsureReady()

			// Skip the MediaWiki checks if the user is just trying to destroy the environment
			if strings.Contains(cobrautil.FullCommandString(cmd), "destroy") {
				return
			}

			usr, _ := user.Current()
			usrDir := usr.HomeDir

			if mwdd.Env().Missing("MEDIAWIKI_VOLUMES_CODE") {
				if !cli.Opts.NoInteraction {
					// Prompt the user for a directory or confirmation
					dirValue := ""
					prompt := &survey.Input{
						Message: "What directory would you like to store MediaWiki source code in?",
						Default: mediawiki.GuessMediaWikiDirectoryBasedOnContext(),
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

			setupOpts := mediawiki.CloneSetupOpts{}
			mediawiki, _ := mediawiki.ForDirectory(mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE"))

			// TODO ask a question about what remotes you want to end up using? https vs ssh!
			// TODO ask if they want to get any more skins and extensions?
			// TODO async cloning of repos for speed!
			if !mediawiki.MediaWikiIsPresent() {
				if !cli.Opts.NoInteraction {
					cloneMw := false
					prompt := &survey.Confirm{
						Message: "MediaWiki code not detected in " + mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it now? (Negative answers will abort this command)",
					}
					err := survey.AskOne(prompt, &cloneMw)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					setupOpts.GetMediaWiki = cloneMw
				} else {
					setupOpts.GetMediaWiki = true
				}
			}
			if !mediawiki.VectorIsPresent() {
				if !cli.Opts.NoInteraction {
					cloneVector := false
					prompt := &survey.Confirm{
						Message: "Vector skin is not detected in " + mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it now?",
					}
					err := survey.AskOne(prompt, &cloneVector)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					setupOpts.GetVector = cloneVector
				} else {
					setupOpts.GetVector = true
				}
			}
			if setupOpts.GetMediaWiki || setupOpts.GetVector {
				if !cli.Opts.NoInteraction {
					cloneFromGithub := false
					prompt1 := &survey.Confirm{
						Message: "Do you want to clone from Github for extra speed? (your git remotes will be switched to Gerrit after download)",
					}
					err := survey.AskOne(prompt1, &cloneFromGithub)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					setupOpts.UseGithub = cloneFromGithub

					cloneShallow := false
					prompt2 := &survey.Confirm{
						Message: "Do you want to use shallow clones for extra speed? (You can fetch all history later using `git fetch --unshallow`)",
					}
					err = survey.AskOne(prompt2, &cloneShallow)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					setupOpts.UseShallow = cloneFromGithub

					finalRemoteType := ""
					prompt3 := &survey.Select{
						Message: "How do you want to interact with Gerrit for the cloned repositores?",
						Options: []string{"ssh", "http"},
						Default: "ssh",
					}
					err = survey.AskOne(prompt3, &finalRemoteType)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					setupOpts.GerritInteractionType = finalRemoteType

					if finalRemoteType == "ssh" {
						gerritUsername := ""
						prompt := &survey.Input{
							Message: "What is your Gerrit username?",
						}
						err = survey.AskOne(prompt, &gerritUsername)
						if err != nil {
							fmt.Println(err)
							os.Exit(1)
						}

						if len(gerritUsername) < 1 {
							fmt.Println("Gerrit username required for ssh interaction type.")
							os.Exit(1)
						}
						setupOpts.GerritUsername = gerritUsername
					}
				} else {
					setupOpts.UseGithub = true
					setupOpts.UseShallow = true
					// Default is ssh, but http is the only non interactive choice we can make here..
					setupOpts.GerritInteractionType = "http"
				}
			}

			if setupOpts.GetMediaWiki || setupOpts.GetVector {
				// Clone various things in multiple stages

				// TODO add a spinner back here
				// We removed it while untangling a big old mess
				fmt.Println("Cloning repositories...")
				fmt.Println("This may take a few moments...")
				mediawiki.CloneSetup(setupOpts)

				eventlogging.AddFeatureUsageEvent("clone-repositories", cli.VersionDetails.Version)
				if setupOpts.UseGithub {
					eventlogging.AddFeatureUsageEvent("clone-repositories:use-github", cli.VersionDetails.Version)
				}
				if setupOpts.UseShallow {
					eventlogging.AddFeatureUsageEvent("clone-repositories:use-shallow", cli.VersionDetails.Version)
				}

				// Check that the needed things seem to have happened
				if setupOpts.GetMediaWiki && !mediawiki.MediaWikiIsPresent() {
					fmt.Println("Something went wrong cloning MediaWiki")
					os.Exit(1)
				}
				if setupOpts.GetVector && !mediawiki.VectorIsPresent() {
					fmt.Println("Something went wrong cloning Vector")
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
	cmd.AddCommand(mwdd.NewServiceCreateCmd("mediawiki"))
	cmd.AddCommand(mwdd.NewServiceDestroyCmd("mediawiki"))
	cmd.AddCommand(mwdd.NewServiceStopCmd("mediawiki"))
	cmd.AddCommand(mwdd.NewServiceStartCmd("mediawiki"))
	cmd.AddCommand(NewMediaWikiInstallCmd())
	cmd.AddCommand(NewMediaWikiComposerCmd())
	cmd.AddCommand(NewMediaWikiExecCmd())
	cmd.AddCommand(NewMediaWikiSitesCmd())
	return cmd
}

var applyRelevantMediawikiWorkingDirectory = func(dockerExecCommand mwdd.DockerExecCommand, mountTo string) mwdd.DockerExecCommand {
	if resolvedPath := paths.ResolveMountForCwd(mwdd.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE"), mountTo); resolvedPath != nil {
		dockerExecCommand.WorkingDir = *resolvedPath
	} else {
		dockerExecCommand.WorkingDir = mountTo
	}
	return dockerExecCommand
}
