package docker

import (
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
	"gitlab.wikimedia.org/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/util/paths"
)

func NewMediaWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mediawiki",
		Short:   "MediaWiki service",
		Aliases: []string{"mw"},
		RunE:    nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmd.Parent().Parent().PersistentPreRun(cmd, args)
			mwdd := mwdd.DefaultForUser()
			mwdd.EnsureReady()

			// Skip the MediaWiki checks if the user is just trying to destroy the environment
			if strings.Contains(cobrautil.FullCommandString(cmd), "destroy") {
				return
			}

			usr, _ := user.Current()
			usrDir := usr.HomeDir

			if mwdd.Env().Missing("MEDIAWIKI_VOLUMES_CODE") {
				if !cli.Opts.NoInteraction {
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
				if !cli.Opts.NoInteraction {
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
				if !cli.Opts.NoInteraction {
					cloneVector := false
					prompt := &survey.Confirm{
						Message: "Vector skin is not detected in " + mwdd.Env().Get("MEDIAWIKI_VOLUMES_CODE") + ". Do you want to clone it now?",
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
				if !cli.Opts.NoInteraction {
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

				// TODO add a spinner back here
				// We removed it while untangeling a big old mess
				fmt.Println("Cloning repositories...")
				fmt.Println("This may take a few moments...")
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
	cmd.AddCommand(mwdd.NewWhereCmd(
		"the MediaWiki directory",
		func() string { return mwdd.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE") },
	))
	cmd.AddCommand(NewMediaWikiFreshCmd())
	cmd.AddCommand(NewMediaWikiQuibbleCmd())
	cmd.AddCommand(mwdd.NewServiceCreateCmd("mediawiki"))
	cmd.AddCommand(mwdd.NewServiceDestroyCmd("mediawiki"))
	cmd.AddCommand(mwdd.NewServiceSuspendCmd("mediawiki"))
	cmd.AddCommand(mwdd.NewServiceResumeCmd("mediawiki"))
	cmd.AddCommand(NewMediaWikiInstallCmd())
	cmd.AddCommand(NewMediaWikiComposerCmd())
	cmd.AddCommand(NewMediaWikiExecCmd())
	return cmd
}

/*DbType used by the install command.*/
var DbType string

/*DbName used by the install command.*/
var DbName string

//go:embed long/mwdd_mediawiki_install.md
var mwddMediawikiInstallLong string

func NewMediaWikiInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "install",
		Example: `  install --dbtype=mysql                         # Install a MediaWiki site in a database called 'default' backed by MySQL
  install --dbname=enwiki --dbtype=mysql         # Install a MediaWiki site in a databse called 'enwiki' backed by MySQL
  install --dbname=thirdwiki --dbtype=postgres   # Install a MediaWiki site in a databse called 'thirdwiki' backed by Postgres`,
		Short:   "Installs a new MediaWiki site using install.php & update.php",
		Long:    cli.RenderMarkdown(mwddMediawikiInstallLong),
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
			mwdd.DefaultForUser().Exec("mediawiki", []string{"chown", "-R", "nobody", "/var/www/html/w/cache/docker"}, "root")
			mwdd.DefaultForUser().Exec("mediawiki", []string{"chown", "-R", "nobody", "/var/www/html/w/images/docker"}, "root")
			mwdd.DefaultForUser().Exec("mediawiki", []string{"chown", "-R", "nobody", "/var/log/mediawiki"}, "root")

			// Record the wiki domain that we are trying to create
			domain := DbName + ".mediawiki.mwdd.localhost"
			mwdd.DefaultForUser().RecordHostUsageBySite(domain)

			// Figure out what and where we are installing
			serverLink := "http://" + domain + ":" + mwdd.DefaultForUser().Env().Get("PORT")
			const adminUser string = "admin"
			const adminPass string = "mwddpassword"

			// Check composer dependencies are up to date
			checkComposer := func() {
				// overrideConfig is a hack https://phabricator.wikimedia.org/T291613
				// If this gets merged into Mediawiki we can remove it here https://gerrit.wikimedia.org/r/c/mediawiki/core/+/723308/
				composerErr := mwdd.DefaultForUser().ExecNoOutput("mediawiki", []string{
					"php", "-r", "define( 'MW_CONFIG_CALLBACK', 'Installer::overrideConfig' ); require_once('/var/www/html/w/maintenance/checkComposerLockUpToDate.php');",
				},
					User)
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
					}, "root")
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
				}, User)

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
					}, "nobody")
				}
				if DbType == "mysql" {
					mwdd.DefaultForUser().Exec("mediawiki", []string{
						"/wait-for-it.sh",
						"mysql:3306",
					}, "nobody")
				}
				if DbType == "postgres" {
					mwdd.DefaultForUser().Exec("mediawiki", []string{
						"/wait-for-it.sh",
						"postgres:5432",
					}, "nobody")
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
					}, "nobody")
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
				}, "nobody")
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
	cmd.Flags().StringVarP(&DbName, "dbname", "", "default", "Name of the database to install (must be accepted by MediaWiki, stick to letters and numbers)")
	cmd.Flags().StringVarP(&DbType, "dbtype", "", "", "Type of database to install (mysql, postgres, sqlite)")
	return cmd
}

func NewMediaWikiComposerCmd() *cobra.Command {
	cmd := &cobra.Command{
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
	cmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}

func NewMediaWikiExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "exec [flags] [command...]",
		Example: `  exec bash		                                  # Run bash as your system user
	  exec --user root -- bash                                # Run bash as root
	  exec -- composer phpunit:unit                           # Run a composer command (php unit tests)
	  exec -- composer phpunit tests/phpunit/unit/includes/XmlTest.php                 # Run a single test
	  exec -- MW_DB=other composer phpunit tests/phpunit/unit/includes/XmlTest.php     # Run a single test for another database
	  exec -- php maintenance/update.php --quick              # Run a MediaWiki maintenance script
	  exec -- tail -f /var/log/mediawiki/debug.log            # Follow the MediaWiki debug log file`,
		Short: "Executes a command in the MediaWiki container",
		Args:  cobra.MinimumNArgs(1),
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
	cmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}

var applyRelevantMediawikiWorkingDirectory = func(dockerExecCommand mwdd.DockerExecCommand, mountTo string) mwdd.DockerExecCommand {
	if resolvedPath := paths.ResolveMountForCwd(mwdd.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE"), mountTo); resolvedPath != nil {
		dockerExecCommand.WorkingDir = *resolvedPath
	} else {
		dockerExecCommand.WorkingDir = mountTo
	}
	return dockerExecCommand
}
