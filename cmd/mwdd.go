package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
	"gitlab.wikimedia.org/releng/cli/internal/cmd"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/util/ports"
)

var ignoreMwddPersistentRunForPrefixes = []string{
	// env may be used to initially setup the environment, and thus avoid the wizard
	"mw docker env",
}

func NewMwddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docker",
		Short: "The MediaWiki-Docker-Dev like development environment",
		RunE:  nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmd.Root().PersistentPreRun(cmd, args)
			mwdd := mwdd.DefaultForUser()
			mwdd.EnsureReady()

			// Skip the checks and wizard for some sub commands
			if cobrautil.CommandIsSubCommandOfOneOrMore(cmd, ignoreMwddPersistentRunForPrefixes) {
				return
			}
			// Skip the checks and wizard for any destroy commands
			if strings.Contains(cobrautil.FullCommandString(cmd), "destroy") {
				return
			}

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
}

func NewMwddDestroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy",
		Short: "Destroy all containers",
		Run: func(cmd *cobra.Command, args []string) {
			options := exec.HandlerOptions{
				Verbosity: globalOpts.Verbosity,
			}
			mwdd.DefaultForUser().DownWithVolumesAndOrphans(options)
		},
	}
}

func NewMwddSuspendCmd() *cobra.Command {
	return &cobra.Command{
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
}

func NewMwddResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: "Resume containers that were running before",
		Run: func(cmd *cobra.Command, args []string) {
			options := exec.HandlerOptions{
				Verbosity: globalOpts.Verbosity,
			}
			mwdd.DefaultForUser().Start(mwdd.DefaultForUser().ServicesWithStatus("stopped"), options)
		},
	}
}

//go:embed long/mwdd_elasticsearch.md
var elasticsearchLong string

//go:embed long/mwdd_eventlogging.md
var eventLoggingLong string

//go:embed long/mwdd_mailhog.md
var mailhogLong string

//go:embed long/mwdd_memcached.md
var memcachedLong string

//go:embed long/mwdd_redis.md
var redisLong string

//go:embed long/mwdd_custom.md
var customLong string

//go:embed long/mwdd_shellbox.md
var shellboxLong string

//go:embed long/mwdd_shellbox-media.md
var shellboxLongMedia string

//go:embed long/mwdd_shellbox-php-rpc.md
var shellboxLongPHPRPC string

//go:embed long/mwdd_shellbox-score.md
var shellboxLongScore string

//go:embed long/mwdd_shellbox-syntaxhighlight.md
var shellboxLongSyntaxhighlight string

//go:embed long/mwdd_shellbox-timeline.md
var shellboxLongTimeline string

func mwddAttachToCmd() *cobra.Command {
	mwddCmd := NewMwddCmd()

	if cli.MwddIsDevAlias {
		mwddCmd.Aliases = []string{"dev"}
		mwddCmd.Short += "\t(alias: dev)"
	}

	mwddCmd.AddCommand(mwdd.NewWhereCmd(
		"the working directory for the environment",
		func() string { return mwdd.DefaultForUser().Directory() },
	))
	mwddCmd.AddCommand(NewMwddDestroyCmd())
	mwddCmd.AddCommand(NewMwddSuspendCmd())
	mwddCmd.AddCommand(NewMwddResumeCmd())

	mwddCmd.AddCommand(NewDockerComposerCmd())

	mwddEnvCmd := cmd.Env("Interact with the environment variables")
	mwddCmd.AddCommand(mwddEnvCmd)

	mwddEnvCmd.AddCommand(cmd.EnvDelete(mwdd.DefaultForUser().Directory))
	mwddEnvCmd.AddCommand(cmd.EnvSet(mwdd.DefaultForUser().Directory))
	mwddEnvCmd.AddCommand(cmd.EnvGet(mwdd.DefaultForUser().Directory))
	mwddEnvCmd.AddCommand(cmd.EnvList(mwdd.DefaultForUser().Directory))
	mwddEnvCmd.AddCommand(cmd.EnvWhere(mwdd.DefaultForUser().Directory))
	mwddEnvCmd.AddCommand(cmd.EnvClear(mwdd.DefaultForUser().Directory))

	mwddHostsCmd := NewHostsCmd()
	mwddCmd.AddCommand(mwddHostsCmd)
	mwddHostsCmd.AddCommand(NewHostsAddCmd())
	mwddHostsCmd.AddCommand(NewHostsRemoveCmd())
	mwddHostsCmd.AddCommand(NewHostsWritableCmd())

	mwddMediawikiCmd := NewMediaWikiCmd()
	mwddCmd.AddCommand(mwddMediawikiCmd)
	mwddMediawikiCmd.AddCommand(mwdd.NewWhereCmd(
		"the MediaWiki directory",
		func() string { return mwdd.DefaultForUser().Env().Get("MEDIAWIKI_VOLUMES_CODE") },
	))
	mwddMediawikiCmd.AddCommand(NewMediaWikiFreshCmd())
	mwddMediawikiCmd.AddCommand(NewMediaWikiQuibbleCmd())
	mwddMediawikiCmd.AddCommand(mwdd.NewServiceCreateCmd("mediawiki", globalOpts.Verbosity))
	mwddMediawikiCmd.AddCommand(mwdd.NewServiceDestroyCmd("mediawiki", globalOpts.Verbosity))
	mwddMediawikiCmd.AddCommand(mwdd.NewServiceSuspendCmd("mediawiki", globalOpts.Verbosity))
	mwddMediawikiCmd.AddCommand(mwdd.NewServiceResumeCmd("mediawiki", globalOpts.Verbosity))
	mwddMediawikiCmd.AddCommand(NewMediaWikiInstallCmd())
	mwddMediawikiCmd.AddCommand(NewMediaWikiComposerCmd())
	mwddMediawikiCmd.AddCommand(NewMediaWikiExecCmd())

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
	custom.AddCommand(mwdd.NewWhereCmd(
		"the custom docker-compose yml file",
		func() string { return mwdd.DefaultForUser().Directory() + "/custom.yml" },
	))
	custom.AddCommand(mwdd.NewServiceCreateCmd("custom", globalOpts.Verbosity))
	custom.AddCommand(mwdd.NewServiceDestroyCmd("custom", globalOpts.Verbosity))
	custom.AddCommand(mwdd.NewServiceSuspendCmd("custom", globalOpts.Verbosity))
	custom.AddCommand(mwdd.NewServiceResumeCmd("custom", globalOpts.Verbosity))

	shellbox := mwdd.NewServicesCmd("shellbox", shellboxLong, []string{})
	mwddCmd.AddCommand(shellbox)

	shellBoxFlavours := []string{
		"media",
		"php-rpc",
		"score",
		"syntaxhighlight",
		"timeline",
	}
	shellBoxLongDescs := map[string]string{
		"media":           shellboxLongMedia,
		"php-rpc":         shellboxLongPHPRPC,
		"score":           shellboxLongScore,
		"syntaxhighlight": shellboxLongSyntaxhighlight,
		"timeline":        shellboxLongTimeline,
	}
	for _, flavour := range shellBoxFlavours {
		shellboxSubCmd := mwdd.NewServiceCmd(flavour, shellBoxLongDescs[flavour], []string{})
		shellbox.AddCommand(shellboxSubCmd)
		dockerComposeName := "shellbox-" + flavour
		shellboxSubCmd.AddCommand(mwdd.NewServiceCreateCmd(dockerComposeName, globalOpts.Verbosity))
		shellboxSubCmd.AddCommand(mwdd.NewServiceDestroyCmd(dockerComposeName, globalOpts.Verbosity))
		shellboxSubCmd.AddCommand(mwdd.NewServiceSuspendCmd(dockerComposeName, globalOpts.Verbosity))
		shellboxSubCmd.AddCommand(mwdd.NewServiceResumeCmd(dockerComposeName, globalOpts.Verbosity))
	}

	return mwddCmd
}
