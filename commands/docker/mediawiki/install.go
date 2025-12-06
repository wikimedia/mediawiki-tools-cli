package mediawiki

import (
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmdgloss"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

//go:embed install.long.md
var mwddMediawikiInstallLong string

//go:embed install.example
var mwddMediawikiInstallExample string

var (
	dbType string
	dbName string
)

func NewMediaWikiInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "install",
		Example: mwddMediawikiInstallExample,
		Short:   "Installs a new MediaWiki site using install.php & update.php",
		Long:    cli.RenderMarkdown(mwddMediawikiInstallLong),
		Aliases: []string{"i"},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cobrautil.CallAllPersistentPreRun(cmd, args)
			dbType, _ = cmd.Flags().GetString("dbtype")
			dbName, _ = cmd.Flags().GetString("dbname")

			if dbType == "" {
				c := config.State()
				dbType = c.Effective.MwDev.Docker.DBType
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbType != "sqlite" && dbType != "mysql" && dbType != "postgres" {
				fmt.Println("You must specify a valid dbtype (mysql, postgres, sqlite)")
				fmt.Println("You can also set the default in the cli config file, for example `mw config set mw_dev.docker.db_type mysql`")
				return fmt.Errorf("invalid dbtype")
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
						return err
					}
				} else {
					createDefaultFile = true
				}

				if createDefaultFile {
					lsPath := mediawiki.Path("LocalSettings.php")

					f, err := os.Create(filepath.Clean(lsPath))
					if err != nil {
						return err
					}
					settingsStringToWrite := "<?php\nrequire_once '/mwdd/MwddSettings.php';\n"
					if mediawiki.VectorIsPresent() {
						settingsStringToWrite += "\nwfLoadSkin('Vector');\n"
					}
					_, err = f.WriteString(settingsStringToWrite)
					if err != nil {
						return err
					}
					err = f.Close()
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("can't install without the expected LocalSettings.php file")
				}
			}

			if !mediawiki.LocalSettingsContains("/mwdd/MwddSettings.php") {
				return fmt.Errorf("LocalSettings.php file exists, but doesn't look right (missing cli development environment shim)")
			}

			// MediaWiki will only create the cache dir sometimes (on some web requests?), make sure it exists.
			err := mwdd.DefaultForUser().DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
				User:           "root",
				CommandAndArgs: []string{"mkdir", "-p", "/var/www/html/w/cache/docker/" + dbName},
			},
			)
			if err != nil {
				return err
			}
			// Fix some container mount permission issues
			// Owned by root, but our webserver needs to be able to write
			err2 := mwdd.DefaultForUser().DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
				User:           "root",
				CommandAndArgs: []string{"chown", "-R", "nobody", "/var/www/html/w/cache/docker", "/var/www/html/w/images/docker", "/var/log/mediawiki"},
			},
			)
			if err2 != nil {
				return err2
			}
			err3 := mwdd.DefaultForUser().DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
				User:           "root",
				CommandAndArgs: []string{"chmod", "-R", "0777", "/var/www/html/w/cache/docker"},
			},
			)
			if err3 != nil {
				return err3
			}

			// Record the wiki domain that we are trying to create
			domain := dbName + ".mediawiki.local.wmftest.net"
			mwdd.DefaultForUser().RecordHostUsageBySite(domain)

			// Figure out what and where we are installing
			serverLink := "http://" + domain + ":" + mwdd.DefaultForUser().Env().Get("PORT")
			const adminUser string = "admin"
			const adminPass string = "mwddpassword"

			// Check composer dependencies are up to date
			checkComposer := func() error {
				// overrideConfig is a hack https://phabricator.wikimedia.org/T291613
				// If this gets merged into Mediawiki we can remove it here https://gerrit.wikimedia.org/r/c/mediawiki/core/+/723308/
				_, _, composerErr := mwdd.DefaultForUser().DockerCompose().ExecCommand("mediawiki", dockercompose.ExecOptions{
					User: User,
					CommandAndArgs: []string{
						"php", "-r", "define( 'MW_CONFIG_CALLBACK', 'MediaWiki\\Installer\\Installer::overrideConfig' ); require_once('/var/www/html/w/maintenance/checkComposerLockUpToDate.php');",
					},
				}).RunAndCollect()
				if composerErr != nil {
					fmt.Println("Composer check failed:", composerErr)

					doComposerInstall := false
					if !cli.Opts.NoInteraction {
						prompt := &survey.Confirm{
							Message: "Composer dependencies are not up to date, do you want to composer install & update?",
						}
						err := survey.AskOne(prompt, &doComposerInstall)
						if err != nil {
							return err
						}
					} else {
						doComposerInstall = true
					}

					if doComposerInstall {
						// Do it twice to make sure we get all the dependencies from the composer merge plugin
						for i := 0; i < 2; i++ {
							containerID, containerIDErr := mwdd.DefaultForUser().DockerCompose().ContainerID("mediawiki")
							if containerIDErr != nil {
								return fmt.Errorf("failed to get container ID %v", containerIDErr)
							}
							docker.Exec(
								containerID,
								docker.ExecOptions{
									Command: []string{"composer", "install", "--ignore-platform-reqs", "--no-interaction"},
									User:    User,
								},
							)
						}
					} else {
						return fmt.Errorf("can't install without up to date composer dependencies")
					}
				}
				return nil
			}

			err = checkComposer()
			if err != nil {
				return err
			}

			// Run install.php
			runInstall := func() error {
				installStartTime := time.Now().Format("20060102150405")

				moveLocalSettingsBack := func() error {
					// Move the LocalSettings.php back after install (or SIGTERM cancellation)
					// TODO Don't do this in docker, do it on disk...
					// TODO Check that the file we are moving does indeed exist, and we are not overwriting what we actually want!
					err := mwdd.DefaultForUser().DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
						User: "root",
						CommandAndArgs: []string{
							"mv",
							"/var/www/html/w/LocalSettings.php.mwdd.bak." + installStartTime,
							"/var/www/html/w/LocalSettings.php",
						},
					})
					return err
				}

				// Set up signal handling for graceful shutdown while LocalSettings.php is moved
				c := make(chan os.Signal, 1)
				signal.Notify(c, os.Interrupt, syscall.SIGTERM)
				go func() {
					<-c
					err := moveLocalSettingsBack()
					if err != nil {
						logrus.Errorf("failed to move LocalSettings.php back after install %v", err)
					}
				}()
				defer func() {
					err := moveLocalSettingsBack()
					if err != nil {
						logrus.Errorf("failed to move LocalSettings.php back after install %v", err)
					}
				}()

				// Move the current LocalSettings "somewhere safe", incase someone needs to restore it
				err := mwdd.DefaultForUser().DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
					User: "root",
					CommandAndArgs: []string{
						"mv",
						"/var/www/html/w/LocalSettings.php",
						"/var/www/html/w/LocalSettings.php.mwdd.bak." + installStartTime,
					},
				})
				if err != nil {
					return err
				}

				// Do a DB type dependant install, writing the output LocalSettings.php to /tmp
				if dbType == "sqlite" {
					err := mwdd.DefaultForUser().DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
						User: "nobody",
						CommandAndArgs: []string{
							"php",
							"/mwdd/MwddInstall.php",
							"--confpath", "/tmp",
							"--server", serverLink,
							"--dbtype", dbType,
							"--dbname", dbName,
							"--dbpath", "/var/www/html/w/cache/docker",
							"--lang", "en",
							"--pass", adminPass,
							"docker-" + dbName,
							adminUser,
						},
					})
					if err != nil {
						return err
					}
				}
				if dbType == "mysql" {
					err := mwdd.DefaultForUser().DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
						User: "nobody",
						CommandAndArgs: []string{
							"/wait-for-it.sh",
							"mysql:3306",
						},
					})
					if err != nil {
						return err
					}
				}
				if dbType == "postgres" {
					err := mwdd.DefaultForUser().DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
						User: "nobody",
						CommandAndArgs: []string{
							"/wait-for-it.sh",
							"postgres:5432",
						},
					})
					if err != nil {
						return err
					}
				}
				if dbType == "mysql" || dbType == "postgres" {
					err := mwdd.DefaultForUser().DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
						User: "nobody",
						CommandAndArgs: []string{
							"php",
							"/mwdd/MwddInstall.php",
							"--confpath", "/tmp",
							"--server", serverLink,
							"--dbtype", dbType,
							"--dbuser", "root",
							"--dbpass", "toor",
							"--dbname", dbName,
							"--dbserver", dbType,
							"--lang", "en",
							"--pass", adminPass,
							"docker-" + dbName,
							adminUser,
						},
					})
					if err != nil {
						return err
					}
				}
				return nil
			}

			err = runInstall()
			if err != nil {
				return err
			}

			// Run update.php
			runUpdate := func() error {
				err := mwdd.DefaultForUser().DockerCompose().Exec("mediawiki", dockercompose.ExecOptions{
					User: "nobody",
					CommandAndArgs: []string{
						"php",
						"/var/www/html/w/maintenance/update.php",
						"--wiki", dbName,
						"--quick",
					},
				})
				return err
			}

			err = runUpdate()
			if err != nil {
				logrus.Error(fmt.Errorf("update.php was unable to run, please fix the issue and run update.php: %s", err))
			}

			outputDetails := make(map[string]string)
			outputDetails["User"] = adminUser
			outputDetails["Pass"] = adminPass
			outputDetails["Link"] = serverLink
			cmdgloss.PrintThreePartBlock(
				cmdgloss.SuccessHeading("Installation successful"),
				outputDetails,
				"If you want to access the wiki from your command line you may need to add it to your hosts file.\n"+
					"You can do this with the `hosts add` command that is part of this development environment.",
			)
			return nil
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Core"

	// Figure out the default DB name for the flag, as this is configurable in the .env file by users
	defaultDbname := "default"
	if mwdd.DefaultForUser().Env().Has("MEDIAWIKI_DEFAULT_DBNAME") {
		defaultDbname = mwdd.DefaultForUser().Env().Get("MEDIAWIKI_DEFAULT_DBNAME")
	}

	cmd.Flags().String("dbname", defaultDbname, "Name of the database to install (must be accepted by MediaWiki, stick to letters and numbers)")
	cmd.Flags().String("dbtype", "", "Type of database to install. One of mysql, postgres, sqlite (overriding config)")

	return cmd
}
