package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
	"gitlab.wikimedia.org/releng/cli/internal/cmd/env"
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
			mwdd.DefaultForUser().DownWithVolumesAndOrphans()
		},
	}
}

func NewMwddSuspendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "suspend",
		Short: "Suspend all currently running containers",
		Run: func(cmd *cobra.Command, args []string) {
			// Suspend all containers that were running
			mwdd.DefaultForUser().Stop([]string{})
		},
	}
}

func NewMwddResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: "Resume containers that were running before",
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().Start(mwdd.DefaultForUser().ServicesWithStatus("stopped"))
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

	mwddCmd.AddCommand(env.Env("Interact with the environment variables", mwdd.DefaultForUser().Directory))

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
	mwddMediawikiCmd.AddCommand(mwdd.NewServiceCreateCmd("mediawiki"))
	mwddMediawikiCmd.AddCommand(mwdd.NewServiceDestroyCmd("mediawiki"))
	mwddMediawikiCmd.AddCommand(mwdd.NewServiceSuspendCmd("mediawiki"))
	mwddMediawikiCmd.AddCommand(mwdd.NewServiceResumeCmd("mediawiki"))
	mwddMediawikiCmd.AddCommand(NewMediaWikiInstallCmd())
	mwddMediawikiCmd.AddCommand(NewMediaWikiComposerCmd())
	mwddMediawikiCmd.AddCommand(NewMediaWikiExecCmd())

	adminer := mwdd.NewServiceCmd("adminer", "", []string{})
	mwddCmd.AddCommand(adminer)
	adminer.AddCommand(mwdd.NewServiceCreateCmd("adminer"))
	adminer.AddCommand(mwdd.NewServiceDestroyCmd("adminer"))
	adminer.AddCommand(mwdd.NewServiceSuspendCmd("adminer"))
	adminer.AddCommand(mwdd.NewServiceResumeCmd("adminer"))
	adminer.AddCommand(mwdd.NewServiceExecCmd("adminer", "adminer"))

	elasticsearch := mwdd.NewServiceCmd("elasticsearch", elasticsearchLong, []string{})
	mwddCmd.AddCommand(elasticsearch)
	elasticsearch.AddCommand(mwdd.NewServiceCreateCmd("elasticsearch"))
	elasticsearch.AddCommand(mwdd.NewServiceDestroyCmd("elasticsearch"))
	elasticsearch.AddCommand(mwdd.NewServiceSuspendCmd("elasticsearch"))
	elasticsearch.AddCommand(mwdd.NewServiceResumeCmd("elasticsearch"))
	elasticsearch.AddCommand(mwdd.NewServiceExecCmd("elasticsearch", "elasticsearch"))

	eventlogging := mwdd.NewServiceCmd("eventlogging", eventLoggingLong, []string{"eventgate"})
	mwddCmd.AddCommand(eventlogging)
	eventlogging.AddCommand(mwdd.NewServiceCreateCmd("eventlogging"))
	eventlogging.AddCommand(mwdd.NewServiceDestroyCmd("eventlogging"))
	eventlogging.AddCommand(mwdd.NewServiceSuspendCmd("eventlogging"))
	eventlogging.AddCommand(mwdd.NewServiceResumeCmd("eventlogging"))
	eventlogging.AddCommand(mwdd.NewServiceExecCmd("eventlogging", "eventlogging"))

	graphite := mwdd.NewServiceCmd("graphite", "", []string{})
	mwddCmd.AddCommand(graphite)
	graphite.AddCommand(mwdd.NewServiceCreateCmd("graphite"))
	graphite.AddCommand(mwdd.NewServiceDestroyCmd("graphite"))
	graphite.AddCommand(mwdd.NewServiceSuspendCmd("graphite"))
	graphite.AddCommand(mwdd.NewServiceResumeCmd("graphite"))
	graphite.AddCommand(mwdd.NewServiceExecCmd("graphite", "graphite"))

	mailhog := mwdd.NewServiceCmd("mailhog", mailhogLong, []string{})
	mwddCmd.AddCommand(mailhog)
	mailhog.AddCommand(mwdd.NewServiceCreateCmd("mailhog"))
	mailhog.AddCommand(mwdd.NewServiceDestroyCmd("mailhog"))
	mailhog.AddCommand(mwdd.NewServiceSuspendCmd("mailhog"))
	mailhog.AddCommand(mwdd.NewServiceResumeCmd("mailhog"))
	mailhog.AddCommand(mwdd.NewServiceExecCmd("mailhog", "mailhog"))

	memcached := mwdd.NewServiceCmd("memcached", memcachedLong, []string{})
	mwddCmd.AddCommand(memcached)
	memcached.AddCommand(mwdd.NewServiceCreateCmd("memcached"))
	memcached.AddCommand(mwdd.NewServiceDestroyCmd("memcached"))
	memcached.AddCommand(mwdd.NewServiceSuspendCmd("memcached"))
	memcached.AddCommand(mwdd.NewServiceResumeCmd("memcached"))
	memcached.AddCommand(mwdd.NewServiceExecCmd("memcached", "memcached"))

	mysql := mwdd.NewServiceCmd("mysql", "", []string{})
	mwddCmd.AddCommand(mysql)
	mysql.AddCommand(mwdd.NewServiceCreateCmd("mysql"))
	mysql.AddCommand(mwdd.NewServiceDestroyCmd("mysql"))
	mysql.AddCommand(mwdd.NewServiceSuspendCmd("mysql"))
	mysql.AddCommand(mwdd.NewServiceResumeCmd("mysql"))
	mysql.AddCommand(mwdd.NewServiceExecCmd("mysql", "mysql"))

	mysqlReplica := mwdd.NewServiceCmd("mysql-replica", "", []string{})
	mwddCmd.AddCommand(mysqlReplica)
	mysqlReplica.AddCommand(mwdd.NewServiceCreateCmd("mysql-replica"))
	mysqlReplica.AddCommand(mwdd.NewServiceDestroyCmd("mysql-replica"))
	mysqlReplica.AddCommand(mwdd.NewServiceSuspendCmd("mysql-replica"))
	mysqlReplica.AddCommand(mwdd.NewServiceResumeCmd("mysql-replica"))
	mysqlReplica.AddCommand(mwdd.NewServiceExecCmd("mysql-replica", "mysql-replica"))

	phpmyadmin := mwdd.NewServiceCmd("phpmyadmin", "", []string{"ppma"})
	mwddCmd.AddCommand(phpmyadmin)
	phpmyadmin.AddCommand(mwdd.NewServiceCreateCmd("phpmyadmin"))
	phpmyadmin.AddCommand(mwdd.NewServiceDestroyCmd("phpmyadmin"))
	phpmyadmin.AddCommand(mwdd.NewServiceSuspendCmd("phpmyadmin"))
	phpmyadmin.AddCommand(mwdd.NewServiceResumeCmd("phpmyadmin"))
	phpmyadmin.AddCommand(mwdd.NewServiceExecCmd("phpmyadmin", "phpmyadmin"))

	postgres := mwdd.NewServiceCmd("postgres", "", []string{})
	mwddCmd.AddCommand(postgres)
	postgres.AddCommand(mwdd.NewServiceCreateCmd("postgres"))
	postgres.AddCommand(mwdd.NewServiceDestroyCmd("postgres"))
	postgres.AddCommand(mwdd.NewServiceSuspendCmd("postgres"))
	postgres.AddCommand(mwdd.NewServiceResumeCmd("postgres"))
	postgres.AddCommand(mwdd.NewServiceExecCmd("postgres", "postgres"))

	redis := mwdd.NewServiceCmd("redis", redisLong, []string{})
	mwddCmd.AddCommand(redis)
	redis.AddCommand(mwdd.NewServiceCreateCmd("redis"))
	redis.AddCommand(mwdd.NewServiceDestroyCmd("redis"))
	redis.AddCommand(mwdd.NewServiceSuspendCmd("redis"))
	redis.AddCommand(mwdd.NewServiceResumeCmd("redis"))
	redis.AddCommand(mwdd.NewServiceExecCmd("redis", "redis"))
	redis.AddCommand(mwdd.NewServiceCommandCmd("redis", "redis-cli"))

	custom := mwdd.NewServiceCmd("custom", customLong, []string{})
	mwddCmd.AddCommand(custom)
	custom.AddCommand(mwdd.NewWhereCmd(
		"the custom docker-compose yml file",
		func() string { return mwdd.DefaultForUser().Directory() + "/custom.yml" },
	))
	custom.AddCommand(mwdd.NewServiceCreateCmd("custom"))
	custom.AddCommand(mwdd.NewServiceDestroyCmd("custom"))
	custom.AddCommand(mwdd.NewServiceSuspendCmd("custom"))
	custom.AddCommand(mwdd.NewServiceResumeCmd("custom"))

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
		shellboxSubCmd.AddCommand(mwdd.NewServiceCreateCmd(dockerComposeName))
		shellboxSubCmd.AddCommand(mwdd.NewServiceDestroyCmd(dockerComposeName))
		shellboxSubCmd.AddCommand(mwdd.NewServiceSuspendCmd(dockerComposeName))
		shellboxSubCmd.AddCommand(mwdd.NewServiceResumeCmd(dockerComposeName))
	}

	return mwddCmd
}
