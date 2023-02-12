package mediawiki

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewMediaWikiDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Helps you identify possible issues with your MediaWiki setup",
		Run: func(cmd *cobra.Command, args []string) {
			m := mwdd.DefaultForUser()
			m.EnsureReady()

			mw, err := mediawiki.ForDirectory(m.Env().Get("MEDIAWIKI_VOLUMES_CODE"))
			if err != nil {
				logrus.Fatal(err)
				os.Exit(1)
			}

			if !mw.MediaWikiIsPresent() {
				logrus.Fatal("MediaWiki is not present in the code volume")
			} else {
				logrus.Info("✅ MediaWiki is present in the code volume")
			}

			if len(m.SkinsCheckedOut()) == 0 || !strings.Contains(m.LocalSettingsContents(), "wfLoadSkin") {
				logrus.Warn("You have no skins checked out or loaded in LocalSettings.php")
				logrus.Info("You can check the Vector skin with `mwdev docker mediawiki get-code --skin Vector`")
			} else {
				logrus.Info("✅ A Skin is checked out and loaded in LocalSettings.php")
			}

			if (
			// We have extensions or skins
			len(m.ExtensionsCheckedOut()) != 0 || len(m.SkinsCheckedOut()) != 0) &&
				// And they are loaded in LocalSettings
				(strings.Contains(m.LocalSettingsContents(), "wfLoadExtension") || strings.Contains(m.LocalSettingsContents(), "wfLoadSkin")) &&
				!m.ComposerLocalJsonExists() {
				logrus.Warn("You have extensions or skins checked out & loaded, but you have not created a composer.local.json file.")
				logrus.Info("If the extensions or skins require additional dependencies, they may not function correctly.")
				logrus.Info("See https://www.mediawiki.org/wiki/Composer#Using_composer-merge-plugin for more information.")
				logrus.Trace("composer.local.json path: " + m.ComposerLocalJsonPath())
			} else {
				logrus.Info("✅ composer.local.json file exists or is likely not needed")
			}

			// TODO check if extension and skin git submodules are loaded

			logrus.Info("Got more suggestions for things to check? File a ticket!")
			logrus.Info("https://phabricator.wikimedia.org/maniphest/task/edit/form/1/?tags=mwcli")
		},
	}
	return cmd
}
