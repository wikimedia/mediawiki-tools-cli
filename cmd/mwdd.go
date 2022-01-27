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
	_ "embed"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/releng/cli/internal/util/ports"
)

var mwddCmd = &cobra.Command{
	Use:   "docker",
	Short: "The MediaWiki-Docker-Dev like development environment",
	RunE:  nil,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Root().PersistentPreRun(cmd, args)
		mwdd := mwdd.DefaultForUser()
		mwdd.EnsureReady()
		if mwdd.Env().Missing("PORT") {
			if !globalOpts.NoInteraction {
				port := ""
				prompt := &survey.Input{
					Message: "What port would you like to use for your development environment?",
					Default: ports.FreeUpFrom("8080"),
				}
				err := survey.AskOne(prompt, &port)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				validityChck := ports.IsValidAndFree(port)
				if validityChck != nil {
					fmt.Println(validityChck)
					os.Exit(1)
				}

				mwdd.Env().Set("PORT", port)
			} else {
				mwdd.Env().Set("PORT", ports.FreeUpFrom("8080"))
			}
		}
	},
}

var mwddWhereCmd = &cobra.Command{
	Use:   "where",
	Short: "States the working directory for the environment",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(mwdd.DefaultForUser().Directory())
	},
}

var mwddDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy all containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().DownWithVolumesAndOrphans(options)
	},
}

var mwddSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend all currently running containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		// Suspend all containers that were running
		mwdd.DefaultForUser().Stop([]string{}, options)
	},
}

var mwddResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume containers that were running before",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Start(mwdd.DefaultForUser().ServicesWithStatus("stopped"), options)
	},
}

//go:embed long/mwdd_elasticsearch.txt
var elasticsearchLong string

//go:embed long/mwdd_eventlogging.txt
var eventLoggingLong string

//go:embed long/mwdd_mailhog.txt
var mailhogLong string

//go:embed long/mwdd_memcached.txt
var memcachedLong string

//go:embed long/mwdd_redis.txt
var redisLong string

//go:embed long/mwdd_custom.txt
var customLong string

func init() {
	mwddCmd.AddCommand(mwddWhereCmd)
	mwddCmd.AddCommand(mwddDestroyCmd)
	mwddCmd.AddCommand(mwddSuspendCmd)
	mwddCmd.AddCommand(mwddResumeCmd)

	adminer := mwdd.NewServiceCmd("adminer", "", []string{})
	mwddCmd.AddCommand(adminer)
	adminer.AddCommand(mwdd.NewServiceCreateCmd("adminer", globalOpts.Verbosity))
	adminer.AddCommand(mwdd.NewServiceDestroyCmd("adminer", globalOpts.Verbosity))
	adminer.AddCommand(mwdd.NewServiceSuspendCmd("adminer", globalOpts.Verbosity))
	adminer.AddCommand(mwdd.NewServiceResumeCmd("adminer", globalOpts.Verbosity))
	adminer.AddCommand(mwdd.NewServiceExecCmd("adminer", "adminer", globalOpts.Verbosity))

	elasticsearch := mwdd.NewServiceCmd("elasticsearch", elasticsearchLong, []string{})
	mwddCmd.AddCommand(elasticsearch)
	elasticsearch.AddCommand(mwdd.NewServiceCreateCmd("elasticsearch", globalOpts.Verbosity))
	elasticsearch.AddCommand(mwdd.NewServiceDestroyCmd("elasticsearch", globalOpts.Verbosity))
	elasticsearch.AddCommand(mwdd.NewServiceSuspendCmd("elasticsearch", globalOpts.Verbosity))
	elasticsearch.AddCommand(mwdd.NewServiceResumeCmd("elasticsearch", globalOpts.Verbosity))
	elasticsearch.AddCommand(mwdd.NewServiceExecCmd("elasticsearch", "elasticsearch", globalOpts.Verbosity))

	eventlogging := mwdd.NewServiceCmd("eventlogging", eventLoggingLong, []string{"eventgate"})
	mwddCmd.AddCommand(eventlogging)
	eventlogging.AddCommand(mwdd.NewServiceCreateCmd("eventlogging", globalOpts.Verbosity))
	eventlogging.AddCommand(mwdd.NewServiceDestroyCmd("eventlogging", globalOpts.Verbosity))
	eventlogging.AddCommand(mwdd.NewServiceSuspendCmd("eventlogging", globalOpts.Verbosity))
	eventlogging.AddCommand(mwdd.NewServiceResumeCmd("eventlogging", globalOpts.Verbosity))
	eventlogging.AddCommand(mwdd.NewServiceExecCmd("eventlogging", "eventlogging", globalOpts.Verbosity))

	graphite := mwdd.NewServiceCmd("graphite", "", []string{})
	mwddCmd.AddCommand(graphite)
	graphite.AddCommand(mwdd.NewServiceCreateCmd("graphite", globalOpts.Verbosity))
	graphite.AddCommand(mwdd.NewServiceDestroyCmd("graphite", globalOpts.Verbosity))
	graphite.AddCommand(mwdd.NewServiceSuspendCmd("graphite", globalOpts.Verbosity))
	graphite.AddCommand(mwdd.NewServiceResumeCmd("graphite", globalOpts.Verbosity))
	graphite.AddCommand(mwdd.NewServiceExecCmd("graphite", "graphite", globalOpts.Verbosity))

	mailhog := mwdd.NewServiceCmd("mailhog", mailhogLong, []string{})
	mwddCmd.AddCommand(mailhog)
	mailhog.AddCommand(mwdd.NewServiceCreateCmd("mailhog", globalOpts.Verbosity))
	mailhog.AddCommand(mwdd.NewServiceDestroyCmd("mailhog", globalOpts.Verbosity))
	mailhog.AddCommand(mwdd.NewServiceSuspendCmd("mailhog", globalOpts.Verbosity))
	mailhog.AddCommand(mwdd.NewServiceResumeCmd("mailhog", globalOpts.Verbosity))
	mailhog.AddCommand(mwdd.NewServiceExecCmd("mailhog", "mailhog", globalOpts.Verbosity))

	memcached := mwdd.NewServiceCmd("memcached", memcachedLong, []string{})
	mwddCmd.AddCommand(memcached)
	memcached.AddCommand(mwdd.NewServiceCreateCmd("memcached", globalOpts.Verbosity))
	memcached.AddCommand(mwdd.NewServiceDestroyCmd("memcached", globalOpts.Verbosity))
	memcached.AddCommand(mwdd.NewServiceSuspendCmd("memcached", globalOpts.Verbosity))
	memcached.AddCommand(mwdd.NewServiceResumeCmd("memcached", globalOpts.Verbosity))
	memcached.AddCommand(mwdd.NewServiceExecCmd("memcached", "memcached", globalOpts.Verbosity))

	mysql := mwdd.NewServiceCmd("mysql", "", []string{})
	mwddCmd.AddCommand(mysql)
	mysql.AddCommand(mwdd.NewServiceCreateCmd("mysql", globalOpts.Verbosity))
	mysql.AddCommand(mwdd.NewServiceDestroyCmd("mysql", globalOpts.Verbosity))
	mysql.AddCommand(mwdd.NewServiceSuspendCmd("mysql", globalOpts.Verbosity))
	mysql.AddCommand(mwdd.NewServiceResumeCmd("mysql", globalOpts.Verbosity))
	mysql.AddCommand(mwdd.NewServiceExecCmd("mysql", "mysql", globalOpts.Verbosity))

	mysqlReplica := mwdd.NewServiceCmd("mysql-replica", "", []string{})
	mwddCmd.AddCommand(mysqlReplica)
	mysqlReplica.AddCommand(mwdd.NewServiceCreateCmd("mysql-replica", globalOpts.Verbosity))
	mysqlReplica.AddCommand(mwdd.NewServiceDestroyCmd("mysql-replica", globalOpts.Verbosity))
	mysqlReplica.AddCommand(mwdd.NewServiceSuspendCmd("mysql-replica", globalOpts.Verbosity))
	mysqlReplica.AddCommand(mwdd.NewServiceResumeCmd("mysql-replica", globalOpts.Verbosity))
	mysqlReplica.AddCommand(mwdd.NewServiceExecCmd("mysql-replica", "mysql-replica", globalOpts.Verbosity))

	phpmyadmin := mwdd.NewServiceCmd("phpmyadmin", "", []string{"ppma"})
	mwddCmd.AddCommand(phpmyadmin)
	phpmyadmin.AddCommand(mwdd.NewServiceCreateCmd("phpmyadmin", globalOpts.Verbosity))
	phpmyadmin.AddCommand(mwdd.NewServiceDestroyCmd("phpmyadmin", globalOpts.Verbosity))
	phpmyadmin.AddCommand(mwdd.NewServiceSuspendCmd("phpmyadmin", globalOpts.Verbosity))
	phpmyadmin.AddCommand(mwdd.NewServiceResumeCmd("phpmyadmin", globalOpts.Verbosity))
	phpmyadmin.AddCommand(mwdd.NewServiceExecCmd("phpmyadmin", "phpmyadmin", globalOpts.Verbosity))

	postgres := mwdd.NewServiceCmd("postgres", "", []string{})
	mwddCmd.AddCommand(postgres)
	postgres.AddCommand(mwdd.NewServiceCreateCmd("postgres", globalOpts.Verbosity))
	postgres.AddCommand(mwdd.NewServiceDestroyCmd("postgres", globalOpts.Verbosity))
	postgres.AddCommand(mwdd.NewServiceSuspendCmd("postgres", globalOpts.Verbosity))
	postgres.AddCommand(mwdd.NewServiceResumeCmd("postgres", globalOpts.Verbosity))
	postgres.AddCommand(mwdd.NewServiceExecCmd("postgres", "postgres", globalOpts.Verbosity))

	redis := mwdd.NewServiceCmd("redis", redisLong, []string{})
	mwddCmd.AddCommand(redis)
	redis.AddCommand(mwdd.NewServiceCreateCmd("redis", globalOpts.Verbosity))
	redis.AddCommand(mwdd.NewServiceDestroyCmd("redis", globalOpts.Verbosity))
	redis.AddCommand(mwdd.NewServiceSuspendCmd("redis", globalOpts.Verbosity))
	redis.AddCommand(mwdd.NewServiceResumeCmd("redis", globalOpts.Verbosity))
	redis.AddCommand(mwdd.NewServiceExecCmd("redis", "redis", globalOpts.Verbosity))
	redis.AddCommand(mwdd.NewServiceCommandCmd("redis", "redis-cli"))

	custom := mwdd.NewServiceCmd("custom", customLong, []string{})
	mwddCmd.AddCommand(custom)
	custom.AddCommand(mwdd.NewServiceCreateCmd("custom", globalOpts.Verbosity))
	custom.AddCommand(mwdd.NewServiceDestroyCmd("custom", globalOpts.Verbosity))
	custom.AddCommand(mwdd.NewServiceSuspendCmd("custom", globalOpts.Verbosity))
	custom.AddCommand(mwdd.NewServiceResumeCmd("custom", globalOpts.Verbosity))
}

func mwddAttachToCmd(rootCmd *cobra.Command) {
	rootCmd.AddCommand(mwddCmd)
}
