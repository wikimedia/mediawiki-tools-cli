package mediawiki

import (
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := os.Setenv("MW_DOCKER_MEDIAWIKI_GET_CODE", "1"); err != nil {
				return err
			}
			if err := cobrautil.CallAllPersistentPreRunE(cmd, args); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			core, _ := cmd.Flags().GetBool("core")
			extensions, _ := cmd.Flags().GetStringSlice("extension")
			skins, _ := cmd.Flags().GetStringSlice("skin")
			shallow, _ := cmd.Flags().GetBool("shallow")
			useGithub, _ := cmd.Flags().GetBool("use-github")
			gerritInteractionType, _ := cmd.Flags().GetString("gerrit-interaction-type")
			gerritUsername, _ := cmd.Flags().GetString("gerrit-username")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			jobs, _ := cmd.Flags().GetInt("jobs")
			if jobs < 1 {
				return fmt.Errorf("jobs must be positive")
			}
			skipCommitHook, _ := cmd.Flags().GetBool("no-commit-hook")

			if gerritUsername == "" {
				c := config.State()
				gerritUsername = c.Effective.Gerrit.Username
			}
			if gerritInteractionType == "" {
				c := config.State()
				gerritInteractionType = c.Effective.Gerrit.InteractionType
			}

			devenv := devenv.DefaultForUser()
			if err := devenv.EnsureReady(); err != nil {
				return err
			}
			codeDir := devenv.Env().Get("MEDIAWIKI_VOLUMES_CODE")
			thisMw, err := mediawiki.ForDirectory(codeDir)
			if err != nil {
				var notMediaWiki *mediawiki.NotMediaWikiDirectory
				if !errors.As(err, &notMediaWiki) {
					return err
				}
				// This command creates a checkout in a clean destination, so a
				// directory without .gitreview means no core is present yet.
				thisMw = mediawiki.MediaWiki(codeDir)
			}
			cloneOpts := mediawiki.CloneOpts{
				GetMediaWiki:          core,
				GetGerritExtensions:   extensions,
				GetGerritSkins:        skins,
				UseShallow:            shallow,
				UseGithub:             useGithub,
				GerritInteractionType: gerritInteractionType,
				GerritUsername:        gerritUsername,
				DryRun:                dryRun,
				SkipCommitHook:        skipCommitHook,
				Jobs:                  jobs,
				In:                    cmd.InOrStdin(),
				Out:                   cmd.OutOrStdout(),
				Err:                   cmd.ErrOrStderr(),
			}

			// If someone runs the command but doesn't ask for anything, run the wizard, or output help in no interaction mode
			if !cloneOpts.GetMediaWiki && len(cloneOpts.GetGerritExtensions) == 0 && len(cloneOpts.GetGerritSkins) == 0 {
				// If we are in no interaction mode, just print the help and exit
				if cli.Opts.NoInteraction {
					err := cmd.Help()
					if err != nil {
						return err
					}
					return fmt.Errorf("specify code to clone with --core, --extension or --skin")
				}

				// Otherwise do the wizard to acquire them...
				if !thisMw.MediaWikiIsPresent() && !core {
					slog.Debug(fmt.Sprint("MediaWiki is missing, and not yet requested"))
					cloneMw := false
					prompt := &survey.Confirm{
						Message: "MediaWiki code not detected in " + devenv.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it now? (Negative answers will abort this command)",
					}
					err := survey.AskOne(prompt, &cloneMw)
					if err != nil {
						return err
					}
					cloneOpts.GetMediaWiki = cloneMw
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "MediaWiki is already present")
				}

				if !thisMw.VectorIsPresent() && !stringsutil.StringInSlice("Vector", skins) {
					slog.Debug(fmt.Sprint("Vector is missing, and not yet requested"))
					cloneVector := false
					prompt := &survey.Confirm{
						Message: "Vector skin is not detected in " + devenv.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it now?",
					}
					err := survey.AskOne(prompt, &cloneVector)
					if err != nil {
						return err
					}
					cloneOpts.GetVector = cloneVector
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "The Vector skin is already present")
				}

				// TODO implement cloning of more skins and extensions in the wizard..
				fmt.Fprintln(cmd.ErrOrStderr(), "If you want to clone more skins and extensions please use the --skin and --extension options...")

				if !cloneOpts.AreThereThingsToClone() {
					fmt.Fprintln(cmd.ErrOrStderr(), "Nothing to do")
					return nil
				}

				finalRemoteType := ""
				prompt3 := &survey.Select{
					Message: "How do you want to interact with Gerrit for the cloned repositories?",
					Options: []string{"ssh", "http"},
					Default: "ssh",
				}
				err := survey.AskOne(prompt3, &finalRemoteType)
				if err != nil {
					return err
				}
				cloneOpts.GerritInteractionType = finalRemoteType

				if finalRemoteType == "ssh" {
					gerritUsername := ""
					prompt := &survey.Input{
						Message: "What is your Gerrit username / shell name? See the Username field on https://gerrit.wikimedia.org/r/settings",
					}
					err = survey.AskOne(prompt, &gerritUsername)
					if err != nil {
						return err
					}

					if len(gerritUsername) < 1 {
						return fmt.Errorf("Gerrit `Username` required for ssh interaction type.")
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
					return err
				}

				if persist {
					if err := config.PutKeyValueOnDisk("gerrit.username", cloneOpts.GerritUsername); err != nil {
						return err
					}
					if err := config.PutKeyValueOnDisk("gerrit.interaction_type", cloneOpts.GerritInteractionType); err != nil {
						return err
					}
				}
			}

			if !cloneOpts.AreThereThingsToClone() {
				fmt.Fprintln(cmd.ErrOrStderr(), "Nothing to do")
				return nil
			}

			// Validate some things that we need
			// Such as IF we are using ssh, we need a username
			if cloneOpts.GerritInteractionType == "ssh" && len(cloneOpts.GerritUsername) < 1 {
				return fmt.Errorf("Gerrit username required for ssh interaction type.")
			}

			cli.NewEvents(cli.EventFilePath()).AddFeatureUsageEvent("mw_docker_mediawiki_get-code", cli.VersionDetails.Version)
			if cloneOpts.UseGithub {
				cli.NewEvents(cli.EventFilePath()).AddFeatureUsageEvent("mw_docker_mediawiki_get-code:use-github", cli.VersionDetails.Version)
			}
			if cloneOpts.UseShallow {
				cli.NewEvents(cli.EventFilePath()).AddFeatureUsageEvent("mw_docker_mediawiki_get-code:use-shallow", cli.VersionDetails.Version)
			}
			return thisMw.CloneSetup(cloneOpts)
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
	cmd.Flags().IntP("jobs", "j", 1, "Maximum simultaneous skin/extension clones (core is cloned first)")
	cmd.Flags().Bool("no-commit-hook", false, "Do not download/install the Gerrit commit-msg hook")
	cmd.Flags().Bool("dry-run", false, "Show what would be done without actually doing it")

	return cmd
}
