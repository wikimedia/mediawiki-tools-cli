package mediawiki

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/eventlogging"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

//go:embed get-code.example
var mediawikiGetCodeExample string

//go:embed get-code.long
var mediawikiGetCodeLong string

func NewMediaWikiGetCodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get-code",
		Example: cobrautil.NormalizeExample(mediawikiGetCodeExample),
		Short:   "Gets MediaWiki code from Gerrit",
		Long:    cli.RenderMarkdown(mediawikiGetCodeLong),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			os.Setenv("MW_DOCKER_MEDIAWIKI_GET_CODE", "1")
			cobrautil.CallAllPersistentPreRun(cmd, args)
		},
		Run: func(cmd *cobra.Command, args []string) {
			core, _ := cmd.Flags().GetBool("core")
			extensions, _ := cmd.Flags().GetStringSlice("extension")
			skins, _ := cmd.Flags().GetStringSlice("skin")
			shallow, _ := cmd.Flags().GetBool("shallow")
			useGithub, _ := cmd.Flags().GetBool("use-github")
			gerritInteractionType, _ := cmd.Flags().GetString("gerrit-interaction-type")
			gerritUsername, _ := cmd.Flags().GetString("gerrit-username")

			if gerritUsername == "" {
				c := config.State()
				gerritUsername = c.Effective.Gerrit.Username
			}
			if gerritInteractionType == "" {
				c := config.State()
				gerritInteractionType = c.Effective.Gerrit.InteractionType
			}

			mwdd := mwdd.DefaultForUser()
			mwdd.EnsureReady()
			thisMw, _ := mediawiki.ForDirectory(mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE"))
			cloneOpts := mediawiki.CloneOpts{
				GetMediaWiki:          core,
				GetGerritExtensions:   extensions,
				GetGerritSkins:        skins,
				UseShallow:            shallow,
				UseGithub:             useGithub,
				GerritInteractionType: gerritInteractionType,
				GerritUsername:        gerritUsername,
			}

			// If someone runs the command but doesn't ask for anything, run the wizard, or output help in no interaction mode
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
					Message: "How do you want to interact with Gerrit for the cloned repositories?",
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
						Message: "What is your Gerrit username / shell name? See the Username field on https://gerrit.wikimedia.org/r/settings",
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

				// Ask about persisting some answers to config
				persist := false
				prompt2 := &survey.Confirm{
					Message: "Do you want to persist Gerrit username and interaction type to config?",
				}
				err = survey.AskOne(prompt2, &persist)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}

				if persist {
					config.PutKeyValueOnDisk("gerrit.username", cloneOpts.GerritUsername)
					config.PutKeyValueOnDisk("gerrit.interaction_type", cloneOpts.GerritInteractionType)
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

	cmd.Flags().Bool("core", false, "Get MediaWiki core")
	cmd.Flags().StringSlice("extension", []string{}, "Get MediaWiki extension")
	cmd.Flags().StringSlice("skin", []string{}, "Get MediaWiki skin")
	cmd.Flags().Bool("shallow", false, "Clone with --depth=1")
	cmd.Flags().Bool("use-github", false, "Use GitHub for speed & switch to Gerrit remotes after")
	cmd.Flags().String("gerrit-interaction-type", "", "How to interact with Gerrit (overriding config) (http, ssh)")
	cmd.Flags().String("gerrit-username", "", "Gerrit username / shell name for ssh interaction type (overriding config)")

	return cmd
}
