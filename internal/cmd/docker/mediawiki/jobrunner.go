package mediawiki

import (
	_ "embed"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/files"
)

func NewMediaWikiJobRunnerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobrunner",
		Short: "Controls a continuous jobrunner.",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(mwdd.NewServiceCreateCmd("mediawiki-jobrunner", ""))
	cmd.AddCommand(mwdd.NewServiceDestroyCmd("mediawiki-jobrunner"))
	cmd.AddCommand(mwdd.NewServiceStopCmd("mediawiki-jobrunner"))
	cmd.AddCommand(mwdd.NewServiceStartCmd("mediawiki-jobrunner"))
	cmd.AddCommand(NewMediaWikiJobRunnerAddSiteCmd())
	cmd.AddCommand(NewMediaWikiJobRunnerRemoveSiteCmd())
	cmd.AddCommand(NewMediaWikiJobRunnerGetSitesCmd())

	return cmd
}

func NewMediaWikiJobRunnerAddSiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-site",
		Short: "Add a site to the jobrunner.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd := mwdd.DefaultForUser()
			mwdd.EnsureReady()
			file := mwdd.Directory() + string(os.PathSeparator) + "mediawiki" + string(os.PathSeparator) + "jobrunner-sites"
			// TODO check the site exists?
			files.AddLineUnique(args[0], file)
			// TODO different output if it was already running?
			logrus.Info("Added site to jobrunner: " + args[0])
		},
	}
	return cmd
}

func NewMediaWikiJobRunnerRemoveSiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove-site",
		Aliases: []string{"rm-site"},
		Short:   "Remove a site to the jobrunner.",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd := mwdd.DefaultForUser()
			mwdd.EnsureReady()
			file := mwdd.Directory() + string(os.PathSeparator) + "mediawiki" + string(os.PathSeparator) + "jobrunner-sites"
			files.RemoveAllLinesMatching(args[0], file)
			logrus.Info("Removed site from jobrunner: " + args[0])
		},
	}
	return cmd
}

func NewMediaWikiJobRunnerGetSitesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get-sites",
		Aliases: []string{"sites"},
		Short:   "Get a list of sites in the jobrunner.",
		Args:    cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd := mwdd.DefaultForUser()
			mwdd.EnsureReady()
			file := mwdd.Directory() + string(os.PathSeparator) + "mediawiki" + string(os.PathSeparator) + "jobrunner-sites"
			lines := files.Lines(file)
			for _, line := range lines {
				logrus.Info(line)
			}
		},
	}
	return cmd
}
