/*Package cmd is used for command line.

Copyright © 2020 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/mediawiki"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/mwdd"
	"github.com/briandowns/spinner"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var mwddMediawikiCmd = &cobra.Command{
	Use:   "mediawiki",
	Short: "MediaWiki service",
	RunE:  nil,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().Parent().PersistentPreRun(cmd, args)
		mwdd := mwdd.DefaultForUser()
		mwdd.EnsureReady()
		if(mwdd.Env().Missing("MEDIAWIKI_VOLUMES_CODE")){
			// TODO auto detect if the current (or parent directory) is MediaWiki and suggest that
			dirPrompt := promptui.Prompt{
				Label:     "What directory would you like to store MediaWiki source code in?",
				Default: "~/dev/git/gerrit/mediawiki/core",
			}
			value, err := dirPrompt.Run()

			// Deal with people entering ~/ paths and them not be handeled
			usr, _ := user.Current()
			usrDir := usr.HomeDir
			if value == "~" {
				// In case of "~", which won't be caught by the "else if"
				value = usrDir
			} else if strings.HasPrefix(value, "~/") {
				// Use strings.HasPrefix so we don't match paths like
				// "/something/~/something/"
				value = filepath.Join(usrDir, value[2:])
			}

			if err == nil {
				mwdd.Env().Set("MEDIAWIKI_VOLUMES_CODE",value)
			} else {
				fmt.Println("Can't continue without a MediaWiki code directory")
				os.Exit(1)
			}
		}

		setupOpts := mediawiki.CloneSetupOpts{}
		mediawiki, _ := mediawiki.ForDirectory(mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE"))

		// TODO ask a question about what remotes you want to end up using? https vs ssh!
		// TODO ask if they want to get any more skins and extensions?
		// TODO async cloning of repos for speed!
		if(!mediawiki.MediaWikiIsPresent()) {
			cloneMwPrompt := promptui.Prompt{
				Label:     "MediaWiki code not detected in " + mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it now?",
				IsConfirm: true,
			}
			_, err := cloneMwPrompt.Run()
			setupOpts.GetMediaWiki = err == nil
		}
		if(!mediawiki.VectorIsPresent()) {
			cloneMwPrompt := promptui.Prompt{
				Label:     "Vector skin is not detected in " + mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it from Gerrit?",
				IsConfirm: true,
			}
			_, err := cloneMwPrompt.Run()
			setupOpts.GetVector = err == nil
		}
		if(setupOpts.GetMediaWiki || setupOpts.GetVector) {
			cloneFromGithub := promptui.Prompt{
				Label:     "Do you want to clone from Github for extra speed? (your git remotes will be switched to Gerrit after download)",
				IsConfirm: true,
			}
			_, err := cloneFromGithub.Run()
			setupOpts.UseGithub = err == nil
		}
		if(setupOpts.GetMediaWiki || setupOpts.GetVector) {
			cloneFromGithub := promptui.Prompt{
				Label:     "Do you want to use shallow clones for extra speed? (You can fetch all history later using `git fetch --unshallow`)",
				IsConfirm: true,
			}
			_, err := cloneFromGithub.Run()
			setupOpts.UseShallow = err == nil
		}

		// Clone various things in multiple stages
		Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		Spinner.Prefix = "Performing step"
		Spinner.FinalMSG = Spinner.Prefix + "(done)\n"
		setupOpts.Options = exec.HandlerOptions{
			Spinner: Spinner,
			//Verbosity: Verbosity,
		}
		mediawiki.CloneSetup(setupOpts)

		// Check that the needed things seem to have happened
		if(setupOpts.GetMediaWiki && !mediawiki.MediaWikiIsPresent()) {
			fmt.Println("Something went wrong cloning MediaWiki")
			os.Exit(1);
		}
		if(setupOpts.GetVector && !mediawiki.VectorIsPresent()) {
			fmt.Println("Something went wrong cloning Vector")
			os.Exit(1);
		}
	},
}

/*DbType used by the install command*/
var DbType string;
/*DbName used by the install command*/
var DbName string;

var mwddMediawikiInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Installs a new MediaWiki site using install.php",
	Run: func(cmd *cobra.Command, args []string) {
		mediawiki, _ := mediawiki.ForDirectory(mwdd.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE"))
		if !mediawiki.LocalSettingsIsPresent() {
			prompt := promptui.Prompt{
				IsConfirm: true,
				Label:     "No LocalSettings.php detected. Do you want to create the default mwdd file?",
			}
			_, err := prompt.Run()
			if err == nil {
				lsPath := mediawiki.Path("LocalSettings.php")

				f, err := os.Create(lsPath)
				if err != nil {
					fmt.Println(err)
					return
				}
				// TODO if the Vector skin exists, also load that
				_, err = f.WriteString("<?php\n//require_once \"$IP/includes/PlatformSettings.php\";\nrequire_once '/mwdd/MwddSettings.php';")
				if err != nil {
					fmt.Println(err)
					f.Close()
					return
				}
				err = f.Close()
				if err != nil {
					fmt.Println(err)
					return
				}
			} else {
				fmt.Println("Can't install without the expected LocalSettings.php file")
				return
			}
		}

		if(!mediawiki.LocalSettingsContains("/mwdd/MwddSettings.php")) {
			fmt.Println("LocalSettings.php file exists, but doesn't look right (missing mwcli mwdd shim)")
			return;
		}

		// TODO make sure of composer caches
		composerErr := mwdd.DefaultForUser().Exec("mediawiki",[]string{
			"php", "-r",
			"require_once dirname( __FILE__ ) . '/includes/PHPVersionCheck.php'; $phpVersionCheck = new PHPVersionCheck(); $phpVersionCheck->checkVendorExistence();",
		},
		exec.HandlerOptions{})
		if(composerErr != nil) {
			prompt := promptui.Prompt{
				IsConfirm: true,
				Label:     "Composer dependencies are not up to date, do you want to composer install?",
			}
			_, err := prompt.Run()
			if err == nil {
				mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
					DockerComposeService: "mediawiki",
					Command: []string{"composer","install","--ignore-platform-reqs","--no-interaction"},
					User: User,
				},)
			} else {
				fmt.Println("Can't install without up to date composer dependencies")
				os.Exit(1)
			}
		}

		// Move custom LocalSetting.php so the install doesn't overwrite it
		mwdd.DefaultForUser().Exec("mediawiki",[]string{
			"mv",
			"/var/www/html/w/LocalSettings.php",
			"/var/www/html/w/LocalSettings.php.mwdd.tmp",
			}, exec.HandlerOptions{})

		// Do a DB type dependant install, writing the output LocalSettings.php to /tmp
		if DbType == "sqlite" {
			mwdd.DefaultForUser().Exec("mediawiki",[]string{
				"php",
				"/var/www/html/w/maintenance/install.php",
				"--confpath", "/tmp",
				"--server", "http://" + DbName + ".mediawiki.mwdd.localhost:" + mwdd.DefaultForUser().Env().Get("PORT"),
				"--dbtype", DbType,
				"--dbname", DbName,
				"--lang", "en",
				"--pass", "mwddpassword",
				"docker-" + DbName,
				"admin",
				}, exec.HandlerOptions{})
		}
		if DbType == "mysql" {
			mwdd.DefaultForUser().Exec("mediawiki",[]string{
				"/wait-for-it.sh",
				"mysql:3306",
				}, exec.HandlerOptions{})
		}
		if DbType == "postgres" {
			mwdd.DefaultForUser().Exec("mediawiki",[]string{
				"/wait-for-it.sh",
				"postgres:5432",
				}, exec.HandlerOptions{})
		}
		if DbType == "mysql" || DbType == "postgres" {
			mwdd.DefaultForUser().Exec("mediawiki",[]string{
				"php",
				"/var/www/html/w/maintenance/install.php",
				"--confpath", "/tmp",
				"--server", "http://" + DbName + ".mediawiki.mwdd.localhost:" + mwdd.DefaultForUser().Env().Get("PORT"),
				"--dbtype", DbType,
				"--dbuser", "root",
				"--dbpass", "toor",
				"--dbname", DbName,
				"--dbserver", DbType,
				"--lang", "en",
				"--pass", "mwddpassword",
				"docker-" + DbName,
				"admin",
				}, exec.HandlerOptions{})
		}

		// Move the custom one back
		mwdd.DefaultForUser().Exec("mediawiki",[]string{
			"mv",
			"/var/www/html/w/LocalSettings.php.mwdd.tmp",
			"/var/www/html/w/LocalSettings.php",
			}, exec.HandlerOptions{})

		// Run update.php once too
		mwdd.DefaultForUser().Exec("mediawiki",[]string{
			"php",
			"/var/www/html/w/maintenance/update.php",
			"--wiki", DbName,
			"--quick",
			}, exec.HandlerOptions{})
	},
}

var mwddMediawikiComposerCmd = &cobra.Command{
	Use:   "composer",
	Short: "Runs composer in a container in the context of MediaWiki",
	Example:   "  composer info\n  composer install -- --ignore-platform-reqs",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "mediawiki",
			Command: append([]string{"composer"},args...),
			User: User,
		})
	},
}

var mwddMediawikiCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		// TODO mediawiki should come from some default definition set?
		mwdd.DefaultForUser().UpDetached( []string{"mediawiki"}, options )
		// TODO add functionality for writing to the hosts file...
		//mwdd.DefaultForUser().EnsureHostsFile()
	},
}

var mwddMediawikiDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Rm( []string{"mediawiki"},options)
		mwdd.DefaultForUser().RmVolumes( []string{"mediawiki-data","mediawiki-images","mediawiki-logs"},options)
	},
}

var mwddMediawikiSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Stop( []string{"mediawiki"},options)
	},
}

var mwddMediawikiResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Start( []string{"mediawiki"},options)
	},
}

var mwddMediawikiPhpunitCmd = &cobra.Command{
	Use:   "phpunit",
	Short: "Runs MediaWiki phpunit in the MediaWiki container",
	Run: func(cmd *cobra.Command, args []string) {
		wiki := "default"
		// TODO optionally take a --wiki (use default if not specified?) Maybe this should be done in LocalSettings?
		// if len(args) >= 1 {
		// 	wiki = args[0]
		// }
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "mediawiki",
			Command: append([]string{"php", "/var/www/html/w/tests/phpunit/phpunit.php", "--wiki", wiki},args...),
		})
	},
}

var mwddMediawikiExecCmd = &cobra.Command{
	Use:   "exec [flags] [command...]",
	Example:   "  exec bash\n  exec -- bash --help\n  exec --user root bash\n  exec --user root -- bash --help",
	Short: "Executes a command in the MediaWiki container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "mediawiki",
			Command: args,
			User: User,
		})
	},
}

func init() {
	mwddCmd.AddCommand(mwddMediawikiCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiCreateCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiDestroyCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiSuspendCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiResumeCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiInstallCmd)
	mwddMediawikiInstallCmd.Flags().StringVarP(&DbName, "dbname", "", "default", "Name of the database to install (must be accepted by MediaWiki, stick to letters and numbers)")
	mwddMediawikiInstallCmd.Flags().StringVarP(&DbType, "dbtype", "", "sqlite", "Type of database to install (sqlite, mysql, postgres)")
	mwddMediawikiCmd.AddCommand(mwddMediawikiComposerCmd)
	mwddMediawikiComposerCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	mwddMediawikiCmd.AddCommand(mwddMediawikiPhpunitCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiExecCmd)
	mwddMediawikiExecCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")

}
