package docker

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
	"gitlab.wikimedia.org/releng/cli/internal/util/lookpath"
	"gitlab.wikimedia.org/releng/cli/internal/util/ports"
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

	if cli.MwddIsDevAlias {
		cmd.Aliases = []string{"dev"}
		cmd.Short += "\t(alias: dev)"
	}

	// High level commands
	cmd.AddCommand(mwdd.NewWhereCmd(
		"the working directory for the environment",
		func() string { return mwdd.DefaultForUser().Directory() },
	))
	cmd.AddCommand(NewMwddDestroyCmd())
	cmd.AddCommand(NewMwddSuspendCmd())
	cmd.AddCommand(NewMwddResumeCmd())
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
	cmd.AddCommand(mwdd.NewServiceCmd("mysql", "", []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("mysql-replica", "", []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("phpmyadmin", "", []string{"ppma"}))
	cmd.AddCommand(mwdd.NewServiceCmd("postgres", "", []string{}))

	cmd.AddCommand(NewShellboxCmd())

	redis := mwdd.NewServiceCmd("redis", redisLong, []string{})
	redis.AddCommand(mwdd.NewServiceCommandCmd("redis", "redis-cli"))
	cmd.AddCommand(redis)

	// Custom creation of custom command to avoid the exec command being added (for now)
	custom := mwdd.NewServiceCmd("custom", customLong, []string{})
	cmd.AddCommand(custom)
	custom.AddCommand(mwdd.NewWhereCmd(
		"the custom docker-compose yml file",
		func() string { return mwdd.DefaultForUser().Directory() + "/custom.yml" },
	))
	custom.AddCommand(mwdd.NewServiceCreateCmd("custom"))
	custom.AddCommand(mwdd.NewServiceDestroyCmd("custom"))
	custom.AddCommand(mwdd.NewServiceSuspendCmd("custom"))
	custom.AddCommand(mwdd.NewServiceResumeCmd("custom"))

	return cmd
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
