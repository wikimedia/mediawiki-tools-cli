package mediawiki

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/eventlogging"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

//go:embed get-code.example
var mediawikiGetCodeExample string

func NewMediaWikiGetCodeCmd() *cobra.Command {
	core := false
	extensions := []string{}
	skins := []string{}

	shallow := false
	useGithub := false
	gerritInteractionType := "http"
	gerritUsername := ""

	cmd := &cobra.Command{
		Use:     "get-code",
		Example: mediawikiGetCodeExample,
		Short:   "Gets MediaWiki code from Gerrit",
		Run: func(cmd *cobra.Command, args []string) {
			mwdd := mwdd.DefaultForUser()
			mwdd.EnsureReady()
			thisMw, _ := mediawiki.ForDirectory(mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE"))
			cloneOpts := mediawiki.CloneOpts{}

			// Set any known options from the command line
			cloneOpts.GetMediaWiki = core
			cloneOpts.GetGerritExtensions = extensions
			cloneOpts.GetGerritSkins = skins
			cloneOpts.UseShallow = shallow
			cloneOpts.UseGithub = useGithub
			cloneOpts.GerritInteractionType = gerritInteractionType
			cloneOpts.GerritUsername = gerritUsername

			// If someone runs the command but doesnt ask for anything, run the wizard, or output help in no interaction mode
			if !cloneOpts.GetMediaWiki && len(cloneOpts.GetGerritExtensions) == 0 && len(cloneOpts.GetGerritSkins) == 0 {
				// If we are in no interaction mode, just print the help and exit
				if cli.Opts.NoInteraction {
					err := cmd.Help()
					if err != nil {
						panic(err)
					}
					os.Exit(1)
				}

				// Otherwise do the wizard to acquire them...
				if !thisMw.MediaWikiIsPresent() && !core {
					logrus.Debug("MediaWiki is missing, and not yet requested")
					cloneMw := false
					prompt := &survey.Confirm{
						Message: "MediaWiki code not detected in " + mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it now? (Negative answers will abort this command)",
					}
					err := survey.AskOne(prompt, &cloneMw)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					cloneOpts.GetMediaWiki = cloneMw
				} else {
					fmt.Println("MediaWiki is already present")
				}

				if !thisMw.VectorIsPresent() && !stringsutil.StringInSlice("Vector", skins) {
					logrus.Debug("Vector is missing, and not yet requested")
					cloneVector := false
					prompt := &survey.Confirm{
						Message: "Vector skin is not detected in " + mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it now?",
					}
					err := survey.AskOne(prompt, &cloneVector)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					cloneOpts.GetVector = cloneVector
				} else {
					fmt.Println("The Vector skin is already present")
				}

				// TODO implement cloning of more skins and extensions in the wizard..
				fmt.Println("If you want to clone more skins and extensions please use the --skin and --extension options...")

				if !cloneOpts.AreThereThingsToClone() {
					fmt.Println("Nothing to do")
					os.Exit(0)
				}

				finalRemoteType := ""
				prompt3 := &survey.Select{
					Message: "How do you want to interact with Gerrit for the cloned repositores?",
					Options: []string{"ssh", "http"},
					Default: "ssh",
				}
				err := survey.AskOne(prompt3, &finalRemoteType)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				cloneOpts.GerritInteractionType = finalRemoteType

				if finalRemoteType == "ssh" {
					gerritUsername := ""
					prompt := &survey.Input{
						Message: "What is your Gerrit username? See https://gerrit.wikimedia.org/r/settings",
					}
					err = survey.AskOne(prompt, &gerritUsername)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}

					if len(gerritUsername) < 1 {
						fmt.Println("Gerrit `Username` required for ssh interaction type.")
						os.Exit(1)
					}
					cloneOpts.GerritUsername = gerritUsername
				}
			}

			if !cloneOpts.AreThereThingsToClone() {
				fmt.Println("Nothing to do")
				os.Exit(0)
			}

			// Validate some things that we need
			// Such as IF we are using ssh, we need a username
			if cloneOpts.GerritInteractionType == "ssh" && len(cloneOpts.GerritUsername) < 1 {
				fmt.Println("Gerrit username required for ssh interaction type.")
				os.Exit(1)
			}

			eventlogging.AddFeatureUsageEvent("mw_docker_mediawiki_get-code", cli.VersionDetails.Version)
			if cloneOpts.UseGithub {
				eventlogging.AddFeatureUsageEvent("mw_docker_mediawiki_get-code:use-github", cli.VersionDetails.Version)
			}
			if cloneOpts.UseShallow {
				eventlogging.AddFeatureUsageEvent("mw_docker_mediawiki_get-code:use-shallow", cli.VersionDetails.Version)
			}
			thisMw.CloneSetup(cloneOpts)
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Core"

	cmd.Flags().BoolVar(&core, "core", false, "Get MediaWiki core")
	cmd.Flags().StringSliceVar(&extensions, "extension", []string{}, "Get MediaWiki extension")
	cmd.Flags().StringSliceVar(&skins, "skin", []string{}, "Get MediaWiki skin")
	cmd.Flags().BoolVar(&shallow, "shallow", false, "Clone with --depth=1")
	cmd.Flags().BoolVar(&useGithub, "use-github", false, "Use GitHub for speed & switch to Gerrit remotes after")
	cmd.Flags().StringVar(&gerritInteractionType, "gerrit-interaction-type", "ssh", "How to interact with Gerrit (http, ssh)")
	cmd.Flags().StringVar(&gerritUsername, "gerrit-username", "", "Gerrit username for ssh interaction type")

	return cmd
}
