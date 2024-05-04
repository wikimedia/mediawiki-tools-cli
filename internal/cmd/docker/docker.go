package docker

import (
	"crypto/md5"
	_ "embed"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/custom"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/dockercompose"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/hosts"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/keycloak"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/mysql"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/mysqlreplica"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/redis"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/shellbox"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/wdqs"
	wdqsUi "gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker/wdqs-ui"
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

//go:embed docker.long.md
var dockerLong string

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "The MediaWiki-Docker-Dev like development environment",
		Long:  cli.RenderMarkdown(dockerLong),
		RunE:  nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmd.Root().PersistentPreRun(cmd, args)
			if _, err := lookpath.NeedCommands([]string{"docker compose"}); err != nil {
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

			// Skip the checks and wizard for some sub commands
			if cobrautil.CommandIsSubCommandOfOneOrMore(cmd, ignoreMwddPersistentRunForPrefixes) {
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
					rand1 := rand.Intn(10)
					rand2 := rand.Intn(10)
					hash := md5.Sum([]byte(mwdd.Context))
					hex1 := hex.EncodeToString(hash[rand1 : rand1+1])
					hex2 := hex.EncodeToString(hash[rand2 : rand2+1])
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

	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Core"

	cmd.PersistentFlags().StringVarP(&mwdd.Context, "context", "c", defaultContext(), "The context to use")
	// Parse PersistentFlags early so that the context is already known to other commands that are added
	cmd.PersistentFlags().Parse(os.Args[1:])

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

	cmd.AddCommand(mwdd.NewServiceCmd("postgres", mwdd.ServiceTexts{}, []string{}))
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
	os.Setenv("PORT", mwdd.DefaultForUser().Env().Get("PORT"))
	expanded := os.ExpandEnv(s)

	// Reset
	if portSet {
		os.Setenv("PORT", previousPort)
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
