package docker

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/env"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/lookpath"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/ports"
)

// Detach run docker command with -d.
var Detach bool

// Privileged run docker command with --privileged.
var Privileged bool

// User run docker command with the specified -u.
var User string

// NoTTY run docker command with -T.
var NoTTY bool

// Index run the docker command with the specified --index.
var Index string

// Env run the docker command with the specified env vars.
var Env []string

// Workdir run the docker command with this working directory.
var Workdir string

var ignoreMwddPersistentRunForPrefixes = []string{
	// env may be used to initially setup the environment, and thus avoid the wizard
	"mw docker env",
}

func defaultContext() string {
	_, inGitlabCi := os.LookupEnv("GITLAB_CI")
	if !inGitlabCi && os.Getenv("MWCLI_CONTEXT_TEST") != "" {
		return "test"
	}
	return "default"
}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "The MediaWiki-Docker-Dev like development environment",
		RunE:  nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmd.Root().PersistentPreRun(cmd, args)
			if _, err := lookpath.NeedExecutables([]string{"docker", "docker-compose"}); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

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
				if !cli.Opts.NoInteraction {
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
					validityCheck := ports.IsValidAndFree(port)
					if validityCheck != nil {
						fmt.Println(validityCheck)
						os.Exit(1)
					}

					mwdd.Env().Set("PORT", port)
				} else {
					mwdd.Env().Set("PORT", ports.FreeUpFrom("8080"))
				}
			}
		},
	}

	cmd.PersistentFlags().StringVarP(&mwdd.Context, "context", "c", defaultContext(), "The context to use")

	if cli.MwddIsDevAlias {
		cmd.Aliases = []string{"dev"}
		cmd.Short += " (alias: dev)"
	}

	// High level commands
	cmd.AddCommand(mwdd.NewWhereCmd(
		"the working directory for the environment",
		func() string { return mwdd.DefaultForUser().Directory() },
	))
	cmd.AddCommand(NewMwddDestroyCmd())
	cmd.AddCommand(NewMwddStopCmd())
	cmd.AddCommand(NewMwddStartCmd())
	cmd.AddCommand(NewMwddRestartCmd())
	cmd.AddCommand(NewDockerComposerCmd())
	cmd.AddCommand(env.Env("Interact with the environment variables", mwdd.DefaultForUser().Directory))
	cmd.AddCommand(NewHostsCmd())

	// Service commands
	cmd.AddCommand(NewMediaWikiCmd())
	cmd.AddCommand(mwdd.NewServiceCmd("adminer", "", []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("elasticsearch", elasticsearchLong, []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("eventlogging", eventLoggingLong, []string{"eventgate"}))
	cmd.AddCommand(mwdd.NewServiceCmd("graphite", "", []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("mailhog", mailhogLong, []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("memcached", memcachedLong, []string{}))

	mysql := mwdd.NewServiceCmd("mysql", "", []string{})
	mysql.AddCommand(mwdd.NewServiceCommandCmd("mysql", []string{"mysql", "-uroot", "-ptoor"}, []string{"cli"}))
	cmd.AddCommand(mysql)
	mysqlReplica := mwdd.NewServiceCmd("mysql-replica", "", []string{})
	mysqlReplica.AddCommand(mwdd.NewServiceCommandCmd("mysql-replica", []string{"mysql", "-uroot", "-ptoor"}, []string{"cli"}))
	cmd.AddCommand(mysqlReplica)

	cmd.AddCommand(mwdd.NewServiceCmd("phpmyadmin", "", []string{"ppma"}))
	cmd.AddCommand(mwdd.NewServiceCmd("postgres", "", []string{}))

	cmd.AddCommand(NewKeycloakCmd())

	cmd.AddCommand(NewShellboxCmd())

	redis := mwdd.NewServiceCmd("redis", redisLong, []string{})
	redis.AddCommand(mwdd.NewServiceCommandCmd("redis", []string{"redis-cli"}, []string{"cli"}))
	cmd.AddCommand(redis)

	// Custom creation of custom command to avoid the exec command being added (for now)
	custom := mwdd.NewServiceCmd("custom", customLong, []string{})
	cmd.AddCommand(custom)
	custom.AddCommand(mwdd.NewWhereCmd(
		"the custom docker-compose yml file",
		func() string { return mwdd.DefaultForUser().Directory() + "/custom.yml" },
	))

	return cmd
}

func NewMwddDestroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy",
		Short: "Destroy all containers and data",
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().DownWithVolumesAndOrphans()
		},
	}
}

func NewMwddStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "stop",
		Aliases: []string{"suspend"},
		Short:   "Stop all currently running containers",
		Run: func(cmd *cobra.Command, args []string) {
			// Stop all containers that were running
			mwdd.DefaultForUser().Stop([]string{})
		},
	}
}

func NewMwddStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "start",
		Aliases: []string{"resume"},
		Short:   "Start containers that were running before",
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().Start(mwdd.DefaultForUser().ServicesWithStatus("stopped"))
		},
	}
}

func NewMwddRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the running containers",
		Run: func(cmd *cobra.Command, args []string) {
			NewMwddStopCmd().Execute()
			NewMwddStartCmd().Execute()
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
