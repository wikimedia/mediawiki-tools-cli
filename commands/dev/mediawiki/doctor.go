package mediawiki

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
)

func NewMediaWikiDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Helps you identify possible issues with your MediaWiki setup",
		Long:  "Inspect code, dependencies, configured sites and Git branches. Known release/development branch mismatches are warnings only: feature branches and detached checkouts require manual compatibility review. This command does not fetch, switch branches, or update source code.",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := devenv.DefaultForUser()
			if err := m.EnsureReady(); err != nil {
				return err
			}

			mw, err := mediawiki.ForDirectory(m.Env().Get("MEDIAWIKI_VOLUMES_CODE"))
			if err != nil {
				return fmt.Errorf("MediaWiki code is unavailable: %w; set MEDIAWIKI_VOLUMES_CODE with mw dev env set or run get-code", err)
			}

			if branches, err := mw.CheckoutBranches(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Could not inspect checkout branches: %v\n", err)
			} else {
				var branchWarnings []string
				for _, checkout := range branches {
					if checkout.Warning != "" {
						branchWarnings = append(branchWarnings, checkout.Warning)
					}
				}
				if len(branchWarnings) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "Git checkout advisory: %s\n", strings.Join(branchWarnings, "; "))
				}
			}
			settings, _ := os.ReadFile(mw.LocalSettingsPath())
			settingsContents := string(settings)

			if !mw.MediaWikiIsPresent() {
				return fmt.Errorf("⚠️ MediaWiki is not present in the code volume")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "✅ MediaWiki is present in the code volume")
			}

			if !mw.VendorDirectoryIsPresent() {
				fmt.Fprintln(cmd.ErrOrStderr(), "⚠️ The vendor directory is not present in the code volume")
				fmt.Fprintln(cmd.ErrOrStderr(), "You may not yet have done a composer install")
				fmt.Fprintln(cmd.ErrOrStderr(), "✨ You can do this with `mw dev mediawiki composer install`")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "✅ The vendor directory is present in the code volume")
			}

			skins, err := mw.SkinsCheckedOut()
			if err != nil {
				return err
			}
			extensions, err := mw.ExtensionsCheckedOut()
			if err != nil {
				return err
			}
			if len(skins) == 0 || !strings.Contains(settingsContents, "wfLoadSkin") {
				fmt.Fprintln(cmd.ErrOrStderr(), "⚠️ You have no skins checked out or loaded in LocalSettings.php")
				fmt.Fprintln(cmd.ErrOrStderr(), "✨ You can checkout the Vector skin with `mw dev mediawiki get-code --skin Vector`")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "✅ A Skin is checked out and loaded in LocalSettings.php")
			}

			if (
			// We have extensions or skins
			len(extensions) != 0 || len(skins) != 0) &&
				// And they are loaded in LocalSettings
				(strings.Contains(settingsContents, "wfLoadExtension") || strings.Contains(settingsContents, "wfLoadSkin")) &&
				!mw.ComposerLocalJsonExists() || !mw.ComposerJsonExists() {
				fmt.Fprintln(cmd.ErrOrStderr(), "⚠️ You have extensions or skins checked out & loaded, but you have not created a composer.local.json file.")
				fmt.Fprintln(cmd.ErrOrStderr(), "If the extensions or skins require additional dependencies, they may not function correctly.")
				fmt.Fprintln(cmd.ErrOrStderr(), "See https://www.mediawiki.org/wiki/Composer#Using_composer-merge-plugin for more information.")
				fmt.Fprintln(cmd.ErrOrStderr(), "✨ You can create a default composer.local.json file with `mw dev mediawiki exec cp /var/www/html/w/composer.local.json-sample /var/www/html/w/composer.local.json`")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "✅ composer.local.json file exists or is likely not needed")
			}

			// TODO check if extension and skin git submodules are

			// Check if a site has been installed
			installedSite := ""
			usedHosts, err := m.UsedHosts()
			if err != nil {
				return err
			}
			for _, host := range usedHosts {
				if strings.Contains(host, "mediawiki.local.wmftest.net") {
					installedSite = host
				}
			}
			if installedSite == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "⚠️ You have not installed a site yet")
				fmt.Fprintln(cmd.ErrOrStderr(), "✨ You can install a site with `mw dev mediawiki install`")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "✅ You have installed a site ("+installedSite+")")

				// Check if the site is accessible
				port := m.Env().Get("PORT")
				url := "http://" + installedSite + ":" + port

				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					return err
				}

				ctx := context.Background()
				c := cli.NewHTTPClient()
				req = req.WithContext(ctx)

				res, err := c.Do(req)
				if err == nil {
					defer res.Body.Close()
				}
				if err != nil || res.StatusCode != 200 {
					fmt.Fprintln(cmd.ErrOrStderr(), "⚠️ That site is not accessible at "+url)
					fmt.Fprintln(cmd.ErrOrStderr(), "✨ You likely need to use the `mw dev hosts` command to add the site to your hosts file")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "✅ That site is accessible at "+url)
				}
			}

			// Check for mediawiki image overrides
			varsToCheck := []string{
				"MEDIAWIKI_IMAGE",
				"MEDIAWIKI_WEB_IMAGE",
			}
			for _, v := range varsToCheck {
				if m.Env().Get(v) != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), "⚠️ You have an override for "+v+" set that might change expected behaviour")
					fmt.Fprintln(cmd.ErrOrStderr(), "✨ You can remove this override with `mw dev env delete "+v+"`")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "✅ You do not have an override for "+v+" set")
				}
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Got more suggestions for things to check? File a ticket!")
			fmt.Fprintln(cmd.ErrOrStderr(), "https://phabricator.wikimedia.org/maniphest/task/edit/form/1/?tags=mwcli")
			return nil
		},
	}
	return cmd
}
