package mediawiki

import (
	"context"
	"net/http"
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
				logrus.Fatal("⚠️ MediaWiki is not present in the code volume")
			} else {
				logrus.Info("✅ MediaWiki is present in the code volume")
			}

			if !mw.VendorDirectoryIsPresent() {
				logrus.Warn("⚠️ The vendor directory is not present in the code volume")
				logrus.Warn("You may not yet have done a composer install")
				logrus.Warn("✨ You can do this with `mw docker mediawiki composer install`")
			} else {
				logrus.Info("✅ The vendor directory is present in the code volume")
			}

			if len(mw.SkinsCheckedOut()) == 0 || !strings.Contains(mw.LocalSettingsContents(), "wfLoadSkin") {
				logrus.Warn("⚠️ You have no skins checked out or loaded in LocalSettings.php")
				logrus.Warn("✨ You can checkout the Vector skin with `mw docker mediawiki get-code --skin Vector`")
			} else {
				logrus.Info("✅ A Skin is checked out and loaded in LocalSettings.php")
			}

			if (
			// We have extensions or skins
			len(mw.ExtensionsCheckedOut()) != 0 || len(mw.SkinsCheckedOut()) != 0) &&
				// And they are loaded in LocalSettings
				(strings.Contains(mw.LocalSettingsContents(), "wfLoadExtension") || strings.Contains(mw.LocalSettingsContents(), "wfLoadSkin")) &&
				!mw.ComposerLocalJsonExists() || !mw.ComposerJsonExists() {
				logrus.Warn("⚠️ You have extensions or skins checked out & loaded, but you have not created a composer.local.json file.")
				logrus.Warn("If the extensions or skins require additional dependencies, they may not function correctly.")
				logrus.Warn("See https://www.mediawiki.org/wiki/Composer#Using_composer-merge-plugin for more information.")
				logrus.Warn("✨ You can create a default composer.local.json file with `mw docker mediawiki exec cp /var/www/html/w/composer.local.json-sample /var/www/html/w/composer.local.json`")
			} else {
				logrus.Info("✅ composer.local.json file exists or is likely not needed")
			}

			// TODO check if extension and skin git submodules are

			// Check if a site has been installed
			installedSite := ""
			for _, host := range m.UsedHosts() {
				if strings.Contains(host, "mediawiki.local.wmftest.net") {
					installedSite = host
				}
			}
			if installedSite == "" {
				logrus.Warn("⚠️ You have not installed a site yet")
				logrus.Warn("✨ You can install a site with `mw docker mediawiki install`")
			} else {
				logrus.Info("✅ You have installed a site (" + installedSite + ")")

				// Check if the site is accessible
				port := m.Env().Get("PORT")
				url := "http://" + installedSite + ":" + port

				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					panic(err)
				}

				ctx := context.Background()
				c := http.Client{}
				req = req.WithContext(ctx)

				res, err := c.Do(req)
				if err == nil {
					defer res.Body.Close()
				}
				if err != nil || res.StatusCode != 200 {
					logrus.Warn("⚠️ That site is not accessible at " + url)
					logrus.Warn("✨ You likely need to use the `mw docker hosts` command to add the site to your hosts file")
				} else {
					logrus.Info("✅ That site is accessible at " + url)
				}
			}

			// Check for mediawiki image overrides
			varsToCheck := []string{
				"MEDIAWIKI_IMAGE",
				"MEDIAWIKI_WEB_IMAGE",
			}
			for _, v := range varsToCheck {
				if m.Env().Get(v) != "" {
					logrus.Warn("⚠️ You have an override for " + v + " set that might change expected behaviour")
					logrus.Warn("✨ You can remove this override with `mw docker env delete " + v + "`")
				} else {
					logrus.Info("✅ You do not have an override for " + v + " set")
				}
			}

			logrus.Print("Got more suggestions for things to check? File a ticket!")
			logrus.Print("https://phabricator.wikimedia.org/maniphest/task/edit/form/1/?tags=mwcli")
		},
	}
	return cmd
}
