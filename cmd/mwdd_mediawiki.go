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
	"os/signal"
	"os/user"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/releng/cli/internal/util/paths"
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
			if !globalOpts.NoInteraction {
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
			if !globalOpts.NoInteraction {
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
			if !globalOpts.NoInteraction {
				cloneVector := false
				prompt := &survey.Confirm{
					Message: "Vector skin is not detected in " + mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it from Gerrit?",
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
			if !globalOpts.NoInteraction {
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

/*DbType used by the install command.*/
var DbType string

/*DbName used by the install command.*/
var DbName string

var mwddMediawikiInstallCmd = &cobra.Command{
	Use: "install",
	Example: `  install --dbtype=mysql                         # Install a MediaWiki site in a database called 'default' backed by MySQL
  install --dbname=enwiki --dbtype=mysql         # Install a MediaWiki site in a databse called 'enwiki' backed by MySQL
  install --dbname=thirdwiki --dbtype=postgres   # Install a MediaWiki site in a databse called 'thirdwiki' backed by Postgres`,
	Short: "Installs a new MediaWiki site using install.php & update.php",
	Long: `Installs a new MediaWiki site using install.php & update.php

The process hidden within this command is:
 - Ensure we know where MediaWiki is
 - Ensure a LocalSettings.php file exists with the shim needed by this development environment
 - Ensure composer dependencies are up to date, or run composer install & update
 - Move LocalSettings.php to a temporary location, as MediaWiki can't install with it present
 - Wait for any needed databases to be ready
 - Run install.php
 - Move LocalSettings.php back
 - Run update.php`,
	Aliases: []string{"i"},
	Run: func(cmd *cobra.Command, args []string) {
		if DbType != "sqlite" && DbType != "mysql" && DbType != "postgres" {
			fmt.Println("You must specify a valid dbtype (mysql, postgres, sqlite)")
			os.Exit(1)
		}

		// TODO check that the required DB services is running? OR start it up?

		mediawiki, _ := mediawiki.ForDirectory(mwdd.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE"))
		if !mediawiki.LocalSettingsIsPresent() {
			createDefaultFile := false
			prompt := &survey.Confirm{
				Message: "No LocalSettings.php detected. Do you want to create the default mwdd file?",
			}
			err := survey.AskOne(prompt, &createDefaultFile)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			if createDefaultFile {
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

		// Fix some container mount permission issues
		// Owned by root, but our webserver needs to be able to write
		mwdd.DefaultForUser().Exec("mediawiki", []string{"chown", "-R", "nobody", "/var/www/html/w/cache/docker"}, exec.HandlerOptions{}, "root")
		mwdd.DefaultForUser().Exec("mediawiki", []string{"chown", "-R", "nobody", "/var/www/html/w/images/docker"}, exec.HandlerOptions{}, "root")
		mwdd.DefaultForUser().Exec("mediawiki", []string{"chown", "-R", "nobody", "/var/log/mediawiki"}, exec.HandlerOptions{}, "root")

		// Record the wiki domain that we are trying to create
		var domain string = DbName + ".mediawiki.mwdd.localhost"
		mwdd.DefaultForUser().RecordHostUsageBySite(domain)

		// Figure out what and where we are installing
		var serverLink string = "http://" + domain + ":" + mwdd.DefaultForUser().Env().Get("PORT")
		const adminUser string = "admin"
		const adminPass string = "mwddpassword"

		// Check composer dependencies are up to date
		checkComposer := func() {
			// overrideConfig is a hack https://phabricator.wikimedia.org/T291613
			// If this gets merged into Mediawiki we can remove it here https://gerrit.wikimedia.org/r/c/mediawiki/core/+/723308/
			composerErr := mwdd.DefaultForUser().ExecNoOutput("mediawiki", []string{
				"php", "-r", "define( 'MW_CONFIG_CALLBACK', 'Installer::overrideConfig' ); require_once('/var/www/html/w/maintenance/checkComposerLockUpToDate.php');",
			},
				exec.HandlerOptions{}, User)
			if composerErr != nil {
				fmt.Println("Composer check failed:", composerErr)

				doComposerInstall := false
				prompt := &survey.Confirm{
					Message: "Composer dependencies are not up to date, do you want to composer install & update?",
				}
				err := survey.AskOne(prompt, &doComposerInstall)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				if doComposerInstall {
					mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
						DockerComposeService: "mediawiki",
						Command:              []string{"composer", "install", "--ignore-platform-reqs", "--no-interaction"},
						User:                 User,
					})
					mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
						DockerComposeService: "mediawiki",
						Command:              []string{"composer", "update", "--ignore-platform-reqs", "--no-interaction"},
						User:                 User,
					})
				} else {
					fmt.Println("Can't install without up to date composer dependencies")
					os.Exit(1)
				}
			}
		}
		checkComposer()

		// Run install.php
		runInstall := func() {
			installStartTime := time.Now().Format("20060102150405")

			moveLocalSettingsBack := func() {
				// Move the LocalSettings.php back after install (or SIGTERM cancellation)
				// TODO Don't do this in docker, do it on disk...
				// TODO Check that the file we are moving does indeed exist, and we are not overwriting what we actually want!
				mwdd.DefaultForUser().Exec("mediawiki", []string{
					"mv",
					"/var/www/html/w/LocalSettings.php.mwdd.bak." + installStartTime,
					"/var/www/html/w/LocalSettings.php",
				}, exec.HandlerOptions{}, "root")
			}

			// Set up signal handling for graceful shutdown while LocalSettings.php is moved
			c := make(chan os.Signal)
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-c
				moveLocalSettingsBack()
				os.Exit(1)
			}()
			defer func() {
				moveLocalSettingsBack()
			}()

			// Move the current LocalSettings "somewhere safe", incase someone needs to restore it
			mwdd.DefaultForUser().Exec("mediawiki", []string{
				"mv",
				"/var/www/html/w/LocalSettings.php",
				"/var/www/html/w/LocalSettings.php.mwdd.bak." + installStartTime,
			}, exec.HandlerOptions{}, User)

			// Do a DB type dependant install, writing the output LocalSettings.php to /tmp
			if DbType == "sqlite" {
				mwdd.DefaultForUser().Exec("mediawiki", []string{
					"php",
					"/mwdd/MwddInstall.php",
					"--confpath", "/tmp",
					"--server", serverLink,
					"--dbtype", DbType,
					"--dbname", DbName,
					"--dbpath", "/var/www/html/w/cache/docker",
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
					"/mwdd/MwddInstall.php",
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
		}
		runInstall()

		// Run update.php
		runUpdate := func() {
			mwdd.DefaultForUser().Exec("mediawiki", []string{
				"php",
				"/var/www/html/w/maintenance/update.php",
				"--wiki", DbName,
				"--quick",
			}, exec.HandlerOptions{}, "nobody")
		}
		// TODO if update fails, still output the install message section, BUT tell them they need to fix the issue and run update.php
		runUpdate()

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
	},
}

var mwddMediawikiComposerCmd = &cobra.Command{
	Use:     "composer",
	Short:   "Runs composer in a container in the context of MediaWiki",
	Example: "  composer info\n  composer install -- --ignore-platform-reqs",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		command, env := mwdd.CommandAndEnvFromArgs(args)
		mwdd.DefaultForUser().DockerExec(applyRelevantMediawikiWorkingDirectory(mwdd.DockerExecCommand{
			DockerComposeService: "mediawiki",
			Command:              append([]string{"composer"}, command...),
			Env:                  env,
			User:                 User,
		}, "/var/www/html/w"))
	},
}

var (
	mwddMediawikiCreateCmd  = mwdd.NewServiceCreateCmd("mediawiki", []string{"mediawiki", "mediawiki-web"}, globalOpts.Verbosity)
	mwddMediawikiDestroyCmd = mwdd.NewServiceDestroyCmd(
		"mediawiki",
		[]string{"mediawiki", "mediawiki-web"},
		[]string{"mediawiki-data", "mediawiki-images", "mediawiki-logs", "mediawiki-dot-composer"},
		globalOpts.Verbosity,
	)
	mwddMediawikiSuspendCmd = mwdd.NewServiceSuspendCmd("mediawiki", []string{"mediawiki", "mediawiki-web"}, globalOpts.Verbosity)
	mwddMediawikiResumeCmd  = mwdd.NewServiceResumeCmd("mediawiki", []string{"mediawiki", "mediawiki-web"}, globalOpts.Verbosity)
)

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
		command, env := mwdd.CommandAndEnvFromArgs(args)
		mwdd.DefaultForUser().DockerExec(applyRelevantMediawikiWorkingDirectory(mwdd.DockerExecCommand{
			DockerComposeService: "mediawiki",
			Command:              command,
			Env:                  env,
			User:                 User,
		}, "/var/www/html/w"))
	},
}

var applyRelevantMediawikiWorkingDirectory = func(dockerExecCommand mwdd.DockerExecCommand, mountTo string) mwdd.DockerExecCommand {
	if resolvedPath := paths.ResolveMountForCwd(mwdd.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE"), mountTo); resolvedPath != nil {
		dockerExecCommand.WorkingDir = *resolvedPath
	} else {
		dockerExecCommand.WorkingDir = mountTo
	}
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
