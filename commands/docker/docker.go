package docker

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/custom"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/dockercompose"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/hosts"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/keycloak"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/mysql"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/mysqlreplica"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/redis"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/shellbox"
	"gitlab.wikimedia.org/repos/releng/cli/commands/docker/wdqs"
	wdqsUi "gitlab.wikimedia.org/repos/releng/cli/commands/docker/wdqs-ui"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/env"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/ports"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
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

func defaultContext() string {
	_, inGitlabCi := os.LookupEnv("GITLAB_CI")
	if !inGitlabCi && os.Getenv("MWCLI_CONTEXT_TEST") != "" {
		return "test"
	}
	// For now, allow the default context to be set using the CONTEXT env var too
	if context, ok := os.LookupEnv("CONTEXT"); ok {
		return context
	}
	return "default"
}

//go:embed docker.long.md
var dockerLong string

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "docker",
		GroupID: "dev",
		Short:   "An advanced docker compose based development environment",
		Long:    cli.RenderMarkdown(dockerLong),
		RunE:    nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cobrautil.CallAllPersistentPreRun(cmd, args)
			if _, err := lookpath.NeedCommands([]string{"docker compose"}); err != nil {
				// We can also allow docker-compose, if docker compose is not available
				if _, err := lookpath.NeedCommands([]string{"docker-compose"}); err != nil {
					fmt.Println("docker compose is not available. Please install it and try again.")
					os.Exit(1)
				}
				fmt.Println(err)
				os.Exit(1)
			}

			// Bail if docker is not running
			// TODO allow some commands to run without docker running, such as "where" and "env"
			if !docker.DockerDaemonIsRunning() {
				fmt.Println("Docker is not running. Please start it and try again.")
				os.Exit(1)
			}

			thisDev := mwdd.DefaultForUser()
			thisDev.EnsureReady()

			// Skip the checks and wizard if "MWCLI_ENV_COMMAND" is defined as an env var
			if _, envCommandDefined := os.LookupEnv("MWCLI_ENV_COMMAND"); envCommandDefined {
				return
			}
			// Skip the checks and wizard for any destroy commands
			if strings.Contains(cobrautil.FullCommandString(cmd), "destroy") {
				return
			}

			// Different subnets are required for different "contexts" if used, so define them in the .env
			if thisDev.Env().Missing("NETWORK_SUBNET_PREFIX") {
				if mwdd.Context == "default" {
					logrus.Trace(".env NETWORK_SUBNET_PREFIX: ", "10.0.0", " (context: ", mwdd.Context, ")")
					thisDev.Env().Set("NETWORK_SUBNET_PREFIX", "10.0.0")
				} else {
					// Do some evil shit to come up with a kind of probably random subnet to use...
					rand1, err := rand.Int(rand.Reader, big.NewInt(10))
					if err != nil {
						panic(err)
					}
					rand2, err := rand.Int(rand.Reader, big.NewInt(10))
					if err != nil {
						panic(err)
					}
					hash := sha256.Sum256([]byte(mwdd.Context))
					hex1 := hex.EncodeToString(hash[rand1.Int64() : rand1.Int64()+1])
					hex2 := hex.EncodeToString(hash[rand2.Int64() : rand2.Int64()+1])
					dec1, _ := strconv.ParseInt(hex1, 16, 32)
					dec2, _ := strconv.ParseInt(hex2, 16, 32)
					logrus.Trace(".env NETWORK_SUBNET_PREFIX: ", fmt.Sprintf("10.%d.%d", dec1, dec2), " (context: ", mwdd.Context, ")")
					thisDev.Env().Set("NETWORK_SUBNET_PREFIX", fmt.Sprintf("10.%d.%d", dec1, dec2))
				}
			}

			if thisDev.Env().Missing("PORT") {
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

					thisDev.Env().Set("PORT", port)
				} else {
					thisDev.Env().Set("PORT", ports.FreeUpFrom("8080"))
				}
			}
		},
	}

	cmd.AddGroup(&cobra.Group{
		ID:    "core",
		Title: "Core Commands",
	})
	cmd.AddGroup(&cobra.Group{
		ID:    "service",
		Title: "Service Commands",
	})

	cmd.PersistentFlags().StringVarP(&mwdd.Context, "context", "c", defaultContext(), "The context to use")
	// Parse PersistentFlags early so that the context is already known to other commands that are added
	err := cmd.PersistentFlags().Parse(os.Args[1:])
	if err != nil {
		logrus.Tracef("Error parsing persistent flags in docker command: %s", err)
	}

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
	cmd.AddCommand(NewMwddUpdateCmd())
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
	cmd.AddCommand(mwdd.NewServiceCmd("citoid", mwdd.ServiceTexts{Long: citoidLong}, []string{}))
	cmd.AddCommand(mwdd.NewServiceCmd("jaeger", mwdd.ServiceTexts{Long: jaegerLong, OnCreate: envSubst(jaegerOnCreate)}, []string{}))

	cmd.AddCommand(mwdd.NewServiceCmd("postgres", mwdd.ServiceTexts{Long: postgresLong}, []string{}))
	cmd.AddCommand(mysql.NewCmd())
	cmd.AddCommand(mysqlreplica.NewCmd())
	cmd.AddCommand(keycloak.NewCmd())
	cmd.AddCommand(shellbox.NewCmd())
	cmd.AddCommand(redis.NewCmd())
	cmd.AddCommand(wdqs.NewCmd())
	cmd.AddCommand(wdqsUi.NewCmd())
	cmd.AddCommand(custom.NewCmd())

	return cmd
}

func envSubst(s string) string {
	// TODO do this more dynamically... / better...
	// Resetting is needed here as docker will look at the existing system / content env vars too

	// Record previous env vars...
	previousPort, portSet := os.LookupEnv("PORT")

	// Set and subst...
	err := os.Setenv("PORT", mwdd.DefaultForUser().Env().Get("PORT"))
	if err != nil {
		panic(err)
	}
	expanded := os.ExpandEnv(s)

	// Reset
	if portSet {
		err := os.Setenv("PORT", previousPort)
		if err != nil {
			panic(err)
		}
	} else {
		os.Unsetenv("PORT")
	}

	return expanded
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

//go:embed citoid/citoid.long.md
var citoidLong string

//go:embed jaeger/jaeger.long.md
var jaegerLong string

//go:embed jaeger/jaeger.oncreate.md
var jaegerOnCreate string

//go:embed postgres/postgres.long.md
var postgresLong string

//go:embed graphite/graphite.oncreate.md
var graphiteOnCreate string

//go:embed memcached/memcached.long.md
var memcachedLong string

//go:embed adminer/adminer.long.md
var adminerLong string

//go:embed adminer/adminer.oncreate.md
var adminerOnCreate string

//go:embed phpmyadmin/phpmyadmin.long.md
var phpmyadminLong string

//go:embed phpmyadmin/phpmyadmin.oncreate.md
var phpmyadminOnCreate string
