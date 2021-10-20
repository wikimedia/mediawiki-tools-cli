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
	Short: "Destroy the all containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().DownWithVolumesAndOrphans(options)
	},
}

var mwddSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Default containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Stop([]string{}, options)
	},
}

var mwddResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Default containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		fmt.Println("Any services that you have not already created will show as 'failed'")
		mwdd.DefaultForUser().Start([]string{}, options)
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

func init() {
	mwddCmd.AddCommand(mwddWhereCmd)
	mwddCmd.AddCommand(mwddDestroyCmd)
	mwddCmd.AddCommand(mwddSuspendCmd)
	mwddCmd.AddCommand(mwddResumeCmd)

	rootCmd.AddCommand(mwddCmd)

	adminer := mwdd.NewServiceCmd("adminer", "", []string{})
	mwddCmd.AddCommand(adminer)
	adminer.AddCommand(mwdd.NewServiceCreateCmd("adminer", []string{"adminer"}, globalOpts.Verbosity))
	adminer.AddCommand(mwdd.NewServiceDestroyCmd("adminer", []string{"adminer"}, []string{}, globalOpts.Verbosity))
	adminer.AddCommand(mwdd.NewServiceSuspendCmd("adminer", []string{"adminer"}, globalOpts.Verbosity))
	adminer.AddCommand(mwdd.NewServiceResumeCmd("adminer", []string{"adminer"}, globalOpts.Verbosity))
	adminer.AddCommand(mwdd.NewServiceExecCmd("adminer", "adminer", globalOpts.Verbosity))

	elasticsearch := mwdd.NewServiceCmd("elasticsearch", elasticsearchLong, []string{})
	mwddCmd.AddCommand(elasticsearch)
	elasticsearch.AddCommand(mwdd.NewServiceCreateCmd("elasticsearch", []string{"elasticsearch"}, globalOpts.Verbosity))
	elasticsearch.AddCommand(mwdd.NewServiceDestroyCmd("elasticsearch", []string{"elasticsearch"}, []string{"elasticsearch-data"}, globalOpts.Verbosity))
	elasticsearch.AddCommand(mwdd.NewServiceSuspendCmd("elasticsearch", []string{"elasticsearch"}, globalOpts.Verbosity))
	elasticsearch.AddCommand(mwdd.NewServiceResumeCmd("elasticsearch", []string{"elasticsearch"}, globalOpts.Verbosity))
	elasticsearch.AddCommand(mwdd.NewServiceExecCmd("elasticsearch", "elasticsearch", globalOpts.Verbosity))

	eventlogging := mwdd.NewServiceCmd("eventlogging", eventLoggingLong, []string{"eventgate"})
	mwddCmd.AddCommand(eventlogging)
	eventlogging.AddCommand(mwdd.NewServiceCreateCmd("eventlogging", []string{"eventlogging"}, globalOpts.Verbosity))
	eventlogging.AddCommand(mwdd.NewServiceDestroyCmd("eventlogging", []string{"eventlogging"}, []string{}, globalOpts.Verbosity))
	eventlogging.AddCommand(mwdd.NewServiceSuspendCmd("eventlogging", []string{"eventlogging"}, globalOpts.Verbosity))
	eventlogging.AddCommand(mwdd.NewServiceResumeCmd("eventlogging", []string{"eventlogging"}, globalOpts.Verbosity))
	eventlogging.AddCommand(mwdd.NewServiceExecCmd("eventlogging", "eventlogging", globalOpts.Verbosity))

	graphite := mwdd.NewServiceCmd("graphite", "", []string{})
	mwddCmd.AddCommand(graphite)
	graphite.AddCommand(mwdd.NewServiceCreateCmd("graphite", []string{"graphite"}, globalOpts.Verbosity))
	graphite.AddCommand(mwdd.NewServiceDestroyCmd("graphite", []string{"graphite"}, []string{"graphite-storage", "graphite-logs"}, globalOpts.Verbosity))
	graphite.AddCommand(mwdd.NewServiceSuspendCmd("graphite", []string{"graphite"}, globalOpts.Verbosity))
	graphite.AddCommand(mwdd.NewServiceResumeCmd("graphite", []string{"graphite"}, globalOpts.Verbosity))
	graphite.AddCommand(mwdd.NewServiceExecCmd("graphite", "graphite", globalOpts.Verbosity))

	mailhog := mwdd.NewServiceCmd("mailhog", mailhogLong, []string{})
	mwddCmd.AddCommand(mailhog)
	mailhog.AddCommand(mwdd.NewServiceCreateCmd("mailhog", []string{"mailhog"}, globalOpts.Verbosity))
	mailhog.AddCommand(mwdd.NewServiceDestroyCmd("mailhog", []string{"mailhog"}, []string{}, globalOpts.Verbosity))
	mailhog.AddCommand(mwdd.NewServiceSuspendCmd("mailhog", []string{"mailhog"}, globalOpts.Verbosity))
	mailhog.AddCommand(mwdd.NewServiceResumeCmd("mailhog", []string{"mailhog"}, globalOpts.Verbosity))
	mailhog.AddCommand(mwdd.NewServiceExecCmd("mailhog", "mailhog", globalOpts.Verbosity))

	memcached := mwdd.NewServiceCmd("memcached", memcachedLong, []string{})
	mwddCmd.AddCommand(memcached)
	memcached.AddCommand(mwdd.NewServiceCreateCmd("memcached", []string{"memcached"}, globalOpts.Verbosity))
	memcached.AddCommand(mwdd.NewServiceDestroyCmd("memcached", []string{"memcached"}, []string{}, globalOpts.Verbosity))
	memcached.AddCommand(mwdd.NewServiceSuspendCmd("memcached", []string{"memcached"}, globalOpts.Verbosity))
	memcached.AddCommand(mwdd.NewServiceResumeCmd("memcached", []string{"memcached"}, globalOpts.Verbosity))
	memcached.AddCommand(mwdd.NewServiceExecCmd("memcached", "memcached", globalOpts.Verbosity))

	mysql := mwdd.NewServiceCmd("mysql", "", []string{})
	mwddCmd.AddCommand(mysql)
	mysql.AddCommand(mwdd.NewServiceCreateCmd("mysql", []string{"mysql", "mysql-configure-replication"}, globalOpts.Verbosity))
	mysql.AddCommand(mwdd.NewServiceDestroyCmd("mysql", []string{"mysql", "mysql-configure-replication"}, []string{"mysql-data", "mysql-configure-replication-data"}, globalOpts.Verbosity))
	mysql.AddCommand(mwdd.NewServiceSuspendCmd("mysql", []string{"mysql", "mysql-configure-replication"}, globalOpts.Verbosity))
	mysql.AddCommand(mwdd.NewServiceResumeCmd("mysql", []string{"mysql", "mysql-configure-replication"}, globalOpts.Verbosity))
	mysql.AddCommand(mwdd.NewServiceExecCmd("mysql", "mysql", globalOpts.Verbosity))

	mysqlReplica := mwdd.NewServiceCmd("mysql-replica", "", []string{})
	mwddCmd.AddCommand(mysqlReplica)
	mysqlReplica.AddCommand(mwdd.NewServiceCreateCmd("mysql-replica", []string{"mysql-replica", "mysql-replica-configure-replication"}, globalOpts.Verbosity))
	mysqlReplica.AddCommand(mwdd.NewServiceDestroyCmd("mysql-replica", []string{"mysql-replica", "mysql-replica-configure-replication"}, []string{"mysql-replica-data"}, globalOpts.Verbosity))
	mysqlReplica.AddCommand(mwdd.NewServiceSuspendCmd("mysql-replica", []string{"mysql-replica", "mysql-replica-configure-replication"}, globalOpts.Verbosity))
	mysqlReplica.AddCommand(mwdd.NewServiceResumeCmd("mysql-replica", []string{"mysql-replica", "mysql-replica-configure-replication"}, globalOpts.Verbosity))
	mysqlReplica.AddCommand(mwdd.NewServiceExecCmd("mysql-replica", "mysql-replica", globalOpts.Verbosity))

	phpmyadmin := mwdd.NewServiceCmd("phpmyadmin", "", []string{"ppma"})
	mwddCmd.AddCommand(phpmyadmin)
	phpmyadmin.AddCommand(mwdd.NewServiceCreateCmd("phpmyadmin", []string{"phpmyadmin"}, globalOpts.Verbosity))
	phpmyadmin.AddCommand(mwdd.NewServiceDestroyCmd("phpmyadmin", []string{"phpmyadmin"}, []string{}, globalOpts.Verbosity))
	phpmyadmin.AddCommand(mwdd.NewServiceSuspendCmd("phpmyadmin", []string{"phpmyadmin"}, globalOpts.Verbosity))
	phpmyadmin.AddCommand(mwdd.NewServiceResumeCmd("phpmyadmin", []string{"phpmyadmin"}, globalOpts.Verbosity))
	phpmyadmin.AddCommand(mwdd.NewServiceExecCmd("phpmyadmin", "phpmyadmin", globalOpts.Verbosity))

	postgres := mwdd.NewServiceCmd("postgres", "", []string{})
	mwddCmd.AddCommand(postgres)
	postgres.AddCommand(mwdd.NewServiceCreateCmd("postgres", []string{"postgres"}, globalOpts.Verbosity))
	postgres.AddCommand(mwdd.NewServiceDestroyCmd("postgres", []string{"postgres"}, []string{"postgres-data"}, globalOpts.Verbosity))
	postgres.AddCommand(mwdd.NewServiceSuspendCmd("postgres", []string{"postgres"}, globalOpts.Verbosity))
	postgres.AddCommand(mwdd.NewServiceResumeCmd("postgres", []string{"postgres"}, globalOpts.Verbosity))
	postgres.AddCommand(mwdd.NewServiceExecCmd("postgres", "postgres", globalOpts.Verbosity))

	redis := mwdd.NewServiceCmd("redis", redisLong, []string{})
	mwddCmd.AddCommand(redis)
	redis.AddCommand(mwdd.NewServiceCreateCmd("redis", []string{"redis"}, globalOpts.Verbosity))
	redis.AddCommand(mwdd.NewServiceDestroyCmd("redis", []string{"redis"}, []string{}, globalOpts.Verbosity))
	redis.AddCommand(mwdd.NewServiceSuspendCmd("redis", []string{"redis"}, globalOpts.Verbosity))
	redis.AddCommand(mwdd.NewServiceResumeCmd("redis", []string{"redis"}, globalOpts.Verbosity))
	redis.AddCommand(mwdd.NewServiceExecCmd("redis", "redis", globalOpts.Verbosity))
	redis.AddCommand(mwdd.NewServiceCommandCmd("redis", "redis-cli"))
}
