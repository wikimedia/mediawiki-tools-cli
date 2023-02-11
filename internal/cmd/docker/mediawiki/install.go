package mediawiki

import (
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmdgloss"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

/*DbType used by the install command.*/
var DbType string

/*DbName used by the install command.*/
var DbName string

//go:embed install.long.md
var mwddMediawikiInstallLong string

//go:embed install.example
var mwddMediawikiInstallExample string

func NewMediaWikiInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "install",
		Example: mwddMediawikiInstallExample,
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
				if !cli.Opts.NoInteraction {
					createDefaultFile = false
					prompt := &survey.Confirm{
						Message: "No LocalSettings.php detected. Do you want to create the default mwdd file?",
					}
					err := survey.AskOne(prompt, &createDefaultFile)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
				} else {
					createDefaultFile = true
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
					if !cli.Opts.NoInteraction {
						prompt := &survey.Confirm{
							Message: "Composer dependencies are not up to date, do you want to composer install & update?",
						}
						err := survey.AskOne(prompt, &doComposerInstall)
						if err != nil {
							fmt.Println(err)
							os.Exit(1)
						}
					} else {
						doComposerInstall = true
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

			outputDetails := make(map[string]string)
			outputDetails["User"] = adminUser
			outputDetails["Pass"] = adminPass
			outputDetails["Link"] = serverLink
			cmdgloss.PrintThreePartBlock(
				cmdgloss.SuccessHeding("Installation successful"),
				outputDetails,
				"If you want to access the wiki from your command line you may need to add it to your hosts file.\n"+
					"You can do this with the `hosts add` command that is part of this development environment.",
			)
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Core"
	cmd.Flags().StringVarP(&DbName, "dbname", "", "default", "Name of the database to install (must be accepted by MediaWiki, stick to letters and numbers)")
	cmd.Flags().StringVarP(&DbType, "dbtype", "", "", "Type of database to install (mysql, postgres, sqlite)")
	return cmd
}
