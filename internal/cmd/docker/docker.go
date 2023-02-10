package docker

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/dockercompose"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/hosts"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/keycloak"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/mysql"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/mysqlreplica"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/redis"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/shellbox"
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

	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Core"

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
	cmd.AddCommand(dockercompose.NewCmd())
	cmd.AddCommand(env.Env("Interact with the environment variables", mwdd.DefaultForUser().Directory))
	cmd.AddCommand(hosts.NewHostsCmd())

	// Service commands
	cmd.AddCommand(mediawiki.NewMediaWikiCmd())
	cmd.AddCommand(mwdd.NewServiceCmd("adminer", mwdd.ServiceTexts{Long: adminerLong, OnCreate: envSubst(adminerOnCreate)}, []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("elasticsearch", mwdd.ServiceTexts{Long: elasticsearchLong}, []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("eventlogging", mwdd.ServiceTexts{Long: eventLoggingLong}, []string{"eventgate"}))
	cmd.AddCommand(mwdd.NewServiceCmd("graphite", mwdd.ServiceTexts{Long: graphiteLong, OnCreate: envSubst(graphiteOnCreate)}, []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("mailhog", mwdd.ServiceTexts{Long: mailhogLong, OnCreate: envSubst(mailhogOnCreate)}, []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("memcached", mwdd.ServiceTexts{Long: memcachedLong}, []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("phpmyadmin", mwdd.ServiceTexts{Long: phpmyadminLong, OnCreate: envSubst(phpmyadminOnCreate)}, []string{"ppma"}))
	cmd.AddCommand(mwdd.NewServiceCmd("postgres", mwdd.ServiceTexts{}, []string{}))
	cmd.AddCommand(mysql.NewCmd())
	cmd.AddCommand(mysqlreplica.NewCmd())
	cmd.AddCommand(keycloak.NewCmd())
	cmd.AddCommand(shellbox.NewCmd())
	cmd.AddCommand(redis.NewCmd())

	// Custom creation of custom command to avoid the exec command being added (for now)
	custom := mwdd.NewServiceCmd("custom", mwdd.ServiceTexts{Long: customLong}, []string{})
	cmd.AddCommand(custom)
	custom.AddCommand(mwdd.NewWhereCmd(
		"the custom docker-compose yml file",
		func() string { return mwdd.DefaultForUser().Directory() + "/custom.yml" },
	))

	return cmd
}

func envSubst(s string) string {
	// TODO do this more dynamically... / better...
	os.Setenv("PORT", mwdd.DefaultForUser().Env().Get("PORT"))
	return os.ExpandEnv(s)
}

//go:embed elasticsearch/elasticsearch.long.md
var elasticsearchLong string

//go:embed eventlogging/eventlogging.long.md
var eventLoggingLong string

//go:embed mailhog/mailhog.long.md
var mailhogLong string

//go:embed mailhog/mailhog.oncreate.md
var mailhogOnCreate string

//go:embed graphite/graphite.long.md
var graphiteLong string

//go:embed graphite/graphite.oncreate.md
var graphiteOnCreate string

//go:embed memcached/memcached.long.md
var memcachedLong string

//go:embed custom/custom.long.md
var customLong string

//go:embed adminer/adminer.long.md
var adminerLong string

//go:embed adminer/adminer.oncreate.md
var adminerOnCreate string

//go:embed phpmyadmin/phpmyadmin.long.md
var phpmyadminLong string

//go:embed phpmyadmin/phpmyadmin.oncreate.md
var phpmyadminOnCreate string
