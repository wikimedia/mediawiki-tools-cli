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
	Use:     "mediawiki",
	Short:   "MediaWiki service",
	Aliases: []string{"mw"},
	RunE:    nil,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().Parent().PersistentPreRun(cmd, args)
		mwdd := mwdd.DefaultForUser()
		mwdd.EnsureReady()

		usr, _ := user.Current()
		usrDir := usr.HomeDir

		if mwdd.Env().Missing("MEDIAWIKI_VOLUMES_CODE") {

			// Try to autodetect if we are in a MediaWiki directory at all
			suggestedMwDir, err := os.Getwd()
			if err != nil {
				panic(err)
			}
			for {
				_, checkError := mediawiki.ForDirectory(suggestedMwDir)
				if checkError == nil {
					break
				}
				suggestedMwDir = filepath.Dir(suggestedMwDir)
				if suggestedMwDir == "/" {
					suggestedMwDir = "~/dev/git/gerrit/mediawiki/core"
					break
				}
			}

			// Prompt the user for a directory or confirmation
			dirPrompt := promptui.Prompt{
				Label:   "What directory would you like to store MediaWiki source code in?",
				Default: suggestedMwDir,
			}
			value, err := dirPrompt.Run()

			// Deal with people entering ~/ paths and them not be handled
			if value == "~" {
				// In case of "~", which won't be caught by the "else if"
				value = usrDir
			} else if strings.HasPrefix(value, "~/") {
				// Use strings.HasPrefix so we don't match paths like
				// "/something/~/something/"
				value = filepath.Join(usrDir, value[2:])
			}

			if err == nil {
				mwdd.Env().Set("MEDIAWIKI_VOLUMES_CODE", value)
			} else {
				fmt.Println("Can't continue without a MediaWiki code directory")
				os.Exit(1)
			}

		}

		// Default the mediawiki container to a .composer directory in the running users home dir
		if !mwdd.Env().Has("MEDIAWIKI_VOLUMES_DOT_COMPOSER") {
			usrComposerDirectory := usrDir + "/.composer"
			if _, err := os.Stat(usrComposerDirectory); os.IsNotExist(err) {
				err := os.Mkdir(usrComposerDirectory, 0755)
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
			cloneMwPrompt := promptui.Prompt{
				Label:     "MediaWiki code not detected in " + mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it now?",
				IsConfirm: true,
			}
			_, err := cloneMwPrompt.Run()
			setupOpts.GetMediaWiki = err == nil
		}
		if !mediawiki.VectorIsPresent() {
			cloneMwPrompt := promptui.Prompt{
				Label:     "Vector skin is not detected in " + mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it from Gerrit?",
				IsConfirm: true,
			}
			_, err := cloneMwPrompt.Run()
			setupOpts.GetVector = err == nil
		}
		if setupOpts.GetMediaWiki || setupOpts.GetVector {
			cloneFromGithubPrompt := promptui.Prompt{
				Label:     "Do you want to clone from Github for extra speed? (your git remotes will be switched to Gerrit after download)",
				IsConfirm: true,
			}
			_, err := cloneFromGithubPrompt.Run()
			setupOpts.UseGithub = err == nil

			cloneShallowPrompt := promptui.Prompt{
				Label:     "Do you want to use shallow clones for extra speed? (You can fetch all history later using `git fetch --unshallow`)",
				IsConfirm: true,
			}
			_, err = cloneShallowPrompt.Run()
			setupOpts.UseShallow = err == nil

			finalRemoteTypePrompt := promptui.Prompt{
				Label:   "How do you want to interact with Gerrit for the cloned repositores? (http or ssh)",
				Default: "ssh",
			}
			remoteType, err := finalRemoteTypePrompt.Run()
			if err != nil || (remoteType != "ssh" && remoteType != "http") {
				fmt.Println("Invalid Gerrit interaction type chosen.")
				os.Exit(1)
			}
			setupOpts.GerritInteractionType = remoteType
			if remoteType == "ssh" {
				gerritUsernamePrompt := promptui.Prompt{
					Label: "What is your Gerrit username?",
				}
				gerritUsername, err := gerritUsernamePrompt.Run()
				if err != nil || len(gerritUsername) < 1 {
					fmt.Println("Gerrit username required for ssh interaction type.")
					os.Exit(1)
				}
				setupOpts.GerritUsername = gerritUsername
			}
			setupOpts.UseShallow = err == nil
		}

		if setupOpts.GetMediaWiki || setupOpts.GetVector {
			// Clone various things in multiple stages
			Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
			Spinner.Prefix = "Performing step"
			Spinner.FinalMSG = Spinner.Prefix + "(done)\n"
			setupOpts.Options = exec.HandlerOptions{
				Spinner: Spinner,
			}

			mediawiki.CloneSetup(setupOpts)

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

/*DbType used by the install command*/
var DbType string

/*DbName used by the install command*/
var DbName string

var mwddMediawikiInstallCmd = &cobra.Command{
	Use:     "install",
	Short:   "Installs a new MediaWiki site using install.php",
	Aliases: []string{"i"},
	Run: func(cmd *cobra.Command, args []string) {
		// Make it harder for people to fall over https://phabricator.wikimedia.org/T287654 for now
		if DbType != "sqlite" && DbType != "mysql" && DbType != "postgres" {
			fmt.Println("You must specify a valid dbtype (mysql, postgres, sqlite)")
			os.Exit(1)
		}

		// TODO check that the required DB services is running? OR start it up?

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
				settingsStringToWrite := "<?php\n//require_once \"$IP/includes/PlatformSettings.php\";\nrequire_once '/mwdd/MwddSettings.php';\n"
				if mediawiki.VectorIsPresent() {
					settingsStringToWrite += "\nwfLoadSkin('Vector');\n"
				}
				_, err = f.WriteString(settingsStringToWrite)
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

		if !mediawiki.LocalSettingsContains("/mwdd/MwddSettings.php") {
			fmt.Println("LocalSettings.php file exists, but doesn't look right (missing mwcli mwdd shim)")
			return
		}

		// TODO make sure of composer caches
		composerErr := mwdd.DefaultForUser().ExecNoOutput("mediawiki", []string{
			"php", "/var/www/html/w/maintenance/checkComposerLockUpToDate.php",
		},
			exec.HandlerOptions{}, User)
		if composerErr != nil {
			fmt.Println("Composer check failed:", composerErr)
			prompt := promptui.Prompt{
				IsConfirm: true,
				Label:     "Composer dependencies are not up to date, do you want to composer install?",
			}
			_, err := prompt.Run()
			if err == nil {
				mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
					DockerComposeService: "mediawiki",
					Command:              []string{"composer", "install", "--ignore-platform-reqs", "--no-interaction"},
					User:                 User,
				})
			} else {
				fmt.Println("Can't install without up to date composer dependencies")
				os.Exit(1)
			}
		}

		// Fix some permissions
		mwdd.DefaultForUser().Exec("mediawiki", []string{"chown", "-R", "nobody", "/var/www/html/w/data"}, exec.HandlerOptions{}, "root")
		mwdd.DefaultForUser().Exec("mediawiki", []string{"chown", "-R", "nobody", "/var/log/mediawiki"}, exec.HandlerOptions{}, "root")

		// Record the wiki domain that we are trying to create
		var domain string = DbName + ".mediawiki.mwdd.localhost"
		mwdd.DefaultForUser().RecordHostUsageBySite(domain)

		// Copy current local settings "somewhere safe", incase someone needs to restore it
		currentTime := time.Now()
		currentTimeString := currentTime.Format("20060102150405")
		mwdd.DefaultForUser().Exec("mediawiki", []string{
			"cp",
			"/var/www/html/w/LocalSettings.php",
			"/var/www/html/w/LocalSettings.php.mwdd.bak." + currentTimeString,
		}, exec.HandlerOptions{}, User)

		// Move custom LocalSetting.php so the install doesn't overwrite it
		mwdd.DefaultForUser().Exec("mediawiki", []string{
			"mv",
			"/var/www/html/w/LocalSettings.php",
			"/var/www/html/w/LocalSettings.php.mwdd.tmp",
		}, exec.HandlerOptions{}, "root")

		var serverLink string = "http://" + domain + ":" + mwdd.DefaultForUser().Env().Get("PORT")
		const adminUser string = "admin"
		const adminPass string = "mwddpassword"

		// Do a DB type dependant install, writing the output LocalSettings.php to /tmp
		if DbType == "sqlite" {
			mwdd.DefaultForUser().Exec("mediawiki", []string{
				"php",
				"/var/www/html/w/maintenance/install.php",
				"--confpath", "/tmp",
				"--server", serverLink,
				"--dbtype", DbType,
				"--dbname", DbName,
				"--lang", "en",
				"--pass", adminPass,
				"docker-" + DbName,
				adminUser,
			}, exec.HandlerOptions{}, "nobody")
		}
		if DbType == "mysql" {
			mwdd.DefaultForUser().Exec("mediawiki", []string{
				"/wait-for-it.sh",
				"mysql:3306",
			}, exec.HandlerOptions{}, "nobody")
		}
		if DbType == "postgres" {
			mwdd.DefaultForUser().Exec("mediawiki", []string{
				"/wait-for-it.sh",
				"postgres:5432",
			}, exec.HandlerOptions{}, "nobody")
		}
		if DbType == "mysql" || DbType == "postgres" {
			mwdd.DefaultForUser().Exec("mediawiki", []string{
				"php",
				"/var/www/html/w/maintenance/install.php",
				"--confpath", "/tmp",
				"--server", serverLink,
				"--dbtype", DbType,
				"--dbuser", "root",
				"--dbpass", "toor",
				"--dbname", DbName,
				"--dbserver", DbType,
				"--lang", "en",
				"--pass", adminPass,
				"docker-" + DbName,
				adminUser,
			}, exec.HandlerOptions{}, "nobody")
		}

		// Move the custom one back
		mwdd.DefaultForUser().Exec("mediawiki", []string{
			"mv",
			"/var/www/html/w/LocalSettings.php.mwdd.tmp",
			"/var/www/html/w/LocalSettings.php",
		}, exec.HandlerOptions{}, "root")

		// Run update.php once too
		mwdd.DefaultForUser().Exec("mediawiki", []string{
			"php",
			"/var/www/html/w/maintenance/update.php",
			"--wiki", DbName,
			"--quick",
		}, exec.HandlerOptions{}, "nobody")

		fmt.Println("")
		fmt.Println("***************************************")
		fmt.Println("Installation successful 🎉")
		fmt.Println("User: " + adminUser)
		fmt.Println("Pass: " + adminPass)
		fmt.Println("Link: " + serverLink)
		fmt.Println("")
		fmt.Println("If you want to access the wiki from your command line you may need to add it to your hosts file.")
		fmt.Println("You can do this with the `hosts add` command that is part of this development environment.")
		fmt.Println("***************************************")

		// TODO remove once https://phabricator.wikimedia.org/T287654 is solved
		if DbType == "sqlite" {
			fmt.Println("WARNING: The sqlite development environment currently suffers an issue, https://phabricator.wikimedia.org/T287654")
		}
	},
}

var mwddMediawikiComposerCmd = &cobra.Command{
	Use:     "composer",
	Short:   "Runs composer in a container in the context of MediaWiki",
	Example: "  composer info\n  composer install -- --ignore-platform-reqs",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(applyRelevantWorkingDirectory(mwdd.DockerExecCommand{
			DockerComposeService: "mediawiki",
			Command:              append([]string{"composer"}, args...),
			User:                 User,
		}))
	},
}

var mwddMediawikiCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		// TODO mediawiki should come from some default definition set?
		mwdd.DefaultForUser().UpDetached([]string{"mediawiki", "mediawiki-web"}, options)
	},
}

var mwddMediawikiDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		mwdd.DefaultForUser().Rm([]string{"mediawiki", "mediawiki-web"}, options)
		mwdd.DefaultForUser().RmVolumes([]string{"mediawiki-data", "mediawiki-images", "mediawiki-logs", "mediawiki-dot-composer"}, options)
	},
}

var mwddMediawikiSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		mwdd.DefaultForUser().Stop([]string{"mediawiki", "mediawiki-web"}, options)
	},
}

var mwddMediawikiResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		mwdd.DefaultForUser().Start([]string{"mediawiki", "mediawiki-web"}, options)
	},
}

var mwddMediawikiExecCmd = &cobra.Command{
	Use: "exec [flags] [command...]",
	Example: `  exec bash		                                  # Run bash as your system user
  exec --user root -- bash                                # Run bash as root
  exec -- composer phpunit:unit                           # Run a composer command (php unit tests)
  exec -- composer phpunit tests/phpunit/unit/includes/XmlTest.php                 # Run a single test
  exec -- MW_DB=other composer phpunit tests/phpunit/unit/includes/XmlTest.php     # Run a single test for another database
  exec -- php maintenance/update.php --quick              # Run a MediaWiki maintenance script
  exec -- tail -f /var/log/mediawiki/debug.log            # Follow the MediaWiki debug log file`,
	Short: "Executes a command in the MediaWiki container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(applyRelevantWorkingDirectory(mwdd.DockerExecCommand{
			DockerComposeService: "mediawiki",
			Command:              args,
			User:                 User,
		}))
	},
}

var applyRelevantWorkingDirectory = func(dockerExecCommand mwdd.DockerExecCommand) mwdd.DockerExecCommand {
	currentWorkingDirectory, _ := os.Getwd()
	mountedMwDirectory := mwdd.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE")
	// For paths inside the mediawiki path, rewrite things
	if strings.HasPrefix(currentWorkingDirectory, mountedMwDirectory) {
		dockerExecCommand.WorkingDir = strings.Replace(currentWorkingDirectory, mountedMwDirectory, "/var/www/html/w", 1)
	}

	// Otherwise just use the root of mediawiki
	return dockerExecCommand
}

func init() {
	mwddCmd.AddCommand(mwddMediawikiCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiCreateCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiDestroyCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiSuspendCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiResumeCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiInstallCmd)
	mwddMediawikiInstallCmd.Flags().StringVarP(&DbName, "dbname", "", "default", "Name of the database to install (must be accepted by MediaWiki, stick to letters and numbers)")
	mwddMediawikiInstallCmd.Flags().StringVarP(&DbType, "dbtype", "", "", "Type of database to install (mysql, postgres, sqlite)")
	mwddMediawikiCmd.AddCommand(mwddMediawikiComposerCmd)
	mwddMediawikiComposerCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	mwddMediawikiCmd.AddCommand(mwddMediawikiExecCmd)
	mwddMediawikiExecCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")

}
