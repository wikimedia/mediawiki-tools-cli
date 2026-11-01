package dev

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
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/custom"
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/dockercompose"
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/hosts"
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/keycloak"
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/mediawiki"
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/mysql"
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/mysqlreplica"
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/redis"
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/shellbox"
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/traefik"
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/tunnel"
	"gitlab.wikimedia.org/repos/releng/cli/commands/dev/wdqs"
	wdqsUi "gitlab.wikimedia.org/repos/releng/cli/commands/dev/wdqs-ui"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/env"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
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
		Use:     "dev",
		Aliases: []string{"docker"},
		GroupID: "dev",
		Short:   "Manage a development environment",
		Long:    cli.RenderMarkdown(dockerLong),
		Example: cobrautil.NormalizeExample("dev mediawiki create\ndev mysql create\ndev mediawiki install --dbtype=mysql"),
		RunE:    nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cobrautil.CallAllPersistentPreRun(cmd, args)
			// These file-only command groups mark themselves before calling parent hooks.
			if os.Getenv("MWCLI_MEDIAWIKI_ENV_COMMAND") == "" && os.Getenv("MW_DOCKER_MEDIAWIKI_GET_CODE") == "" {
				if _, err := lookpath.NeedCommands([]string{"docker compose"}); err != nil {
					// We can also allow docker-compose, if docker compose is not available
					if _, err := lookpath.NeedCommands([]string{"docker-compose"}); err != nil {
						fmt.Println("docker compose is not available. Please install it and try again.")
						os.Exit(1)
					}
					fmt.Println(err)
					os.Exit(1)
				}

				// Bail if Docker is unavailable through the selected context.
				if err := docker.CheckDockerDaemon(); err != nil {
					fmt.Fprintf(os.Stderr, "Docker is unavailable: %v\nCheck `docker context show` and `docker info` using the same environment.\n", err)
					os.Exit(1)
				}
			}

			thisDev := devenv.DefaultForUser()
			thisDev.EnsureReady()

			// Skip the checks and wizard if "MWCLI_MEDIAWIKI_ENV_COMMAND" is defined as an env var
			if _, envCommandDefined := os.LookupEnv("MWCLI_MEDIAWIKI_ENV_COMMAND"); envCommandDefined {
				return
			}
			// Skip the checks and wizard for any destroy commands
			if strings.Contains(cobrautil.FullCommandString(cmd), "destroy") {
				return
			}

			// Different subnets are required for different "contexts" if used, so define them in the .env
			if thisDev.Env().Missing("NETWORK_SUBNET_PREFIX") {
				if devenv.Context == "default" {
					logrus.Trace(".env NETWORK_SUBNET_PREFIX: ", "10.0.0", " (context: ", devenv.Context, ")")
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
					hash := sha256.Sum256([]byte(devenv.Context))
					hex1 := hex.EncodeToString(hash[rand1.Int64() : rand1.Int64()+1])
					hex2 := hex.EncodeToString(hash[rand2.Int64() : rand2.Int64()+1])
					dec1, _ := strconv.ParseInt(hex1, 16, 32)
					dec2, _ := strconv.ParseInt(hex2, 16, 32)
					logrus.Trace(".env NETWORK_SUBNET_PREFIX: ", fmt.Sprintf("10.%d.%d", dec1, dec2), " (context: ", devenv.Context, ")")
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
					cmd.Printf("To change this later, see `%s help guides dev ports`.\n", cmd.Root().Name())
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

	cmd.PersistentFlags().StringVarP(&devenv.Context, "context", "c", defaultContext(), "Development context to use (overrides CONTEXT)")
	// Parse PersistentFlags early so that the context is already known to other commands that are added
	err := cmd.PersistentFlags().Parse(os.Args[1:])
	if err != nil {
		logrus.Tracef("Error parsing persistent flags in docker command: %s", err)
	}

	// High level commands
	cmd.AddCommand(devenv.NewWhereCmd(
		"the working directory for the environment",
		func() string { return devenv.DefaultForUser().Directory() },
	))
	cmd.AddCommand(NewDevDestroyCmd())
	cmd.AddCommand(NewDevStopCmd())
	cmd.AddCommand(NewDevStartCmd())
	cmd.AddCommand(NewDevRestartCmd())
	cmd.AddCommand(NewDevUpdateCmd())
	cmd.AddCommand(NewRecipeCmd())
	cmd.AddCommand(dockercompose.NewCmd())
	envCmd := env.Env("Interact with the environment variables", devenv.DefaultForUser().Directory)
	envCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		os.Setenv("MWCLI_MEDIAWIKI_ENV_COMMAND", "1")
		cobrautil.CallAllPersistentPreRun(cmd, args)
	}
	cmd.AddCommand(envCmd)
	cmd.AddCommand(hosts.NewHostsCmd())

	// Service commands
	cmd.AddCommand(cobrautil.WithServiceInfo(mediawiki.NewMediaWikiCmd(), cobrautil.ServiceInfo{Name: "MediaWiki", URL: "https://www.mediawiki.org", Logo: "https://upload.wikimedia.org/wikipedia/commons/a/a6/MediaWiki-2020-icon.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(traefik.NewCmd(), cobrautil.ServiceInfo{Name: "Traefik", URL: "https://traefik.io/traefik/", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/traefikproxy.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(tunnel.NewCmd(), cobrautil.ServiceInfo{Name: "Tunnels", URL: "https://www.mediawiki.org/wiki/Cli"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("adminer", devenv.ServiceTexts{Long: adminerLong, OnCreate: envSubst(adminerOnCreate)}, []string{}), cobrautil.ServiceInfo{Name: "Adminer", URL: "https://www.adminer.org", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/adminer.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("novnc", devenv.ServiceTexts{Long: novncLong}, []string{}), cobrautil.ServiceInfo{Name: "noVNC", URL: "https://novnc.com"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("elasticsearch", devenv.ServiceTexts{Long: elasticsearchLong}, []string{}), cobrautil.ServiceInfo{Name: "Elasticsearch", URL: "https://www.elastic.co/elasticsearch", Logo: "https://upload.wikimedia.org/wikipedia/commons/1/16/OpenSearch_S_logo.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("opensearch", devenv.ServiceTexts{Long: opensearchLong}, []string{}), cobrautil.ServiceInfo{Name: "OpenSearch", URL: "https://opensearch.org", Logo: "https://upload.wikimedia.org/wikipedia/commons/1/16/OpenSearch_S_logo.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmdDifferingNames("logs", "opensearch-dashboards", devenv.ServiceTexts{Long: opensearchDashboardsLong, OnCreate: opensearchDashboardsOnCreate}, []string{"opensearch-dashboards"}), cobrautil.ServiceInfo{Name: "OpenSearch Dashboards", URL: "https://opensearch.org", Logo: "https://upload.wikimedia.org/wikipedia/commons/1/16/OpenSearch_S_logo.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("eventlogging", devenv.ServiceTexts{Long: eventLoggingLong}, []string{"eventgate"}), cobrautil.ServiceInfo{Name: "EventLogging", URL: "https://www.mediawiki.org/wiki/Extension:EventLogging", Logo: "https://upload.wikimedia.org/wikipedia/commons/a/a6/MediaWiki-2020-icon.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("graphite", devenv.ServiceTexts{Long: graphiteLong, OnCreate: envSubst(graphiteOnCreate)}, []string{}), cobrautil.ServiceInfo{Name: "Graphite", URL: "https://graphiteapp.org"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("mailhog", devenv.ServiceTexts{Long: mailhogLong, OnCreate: envSubst(mailhogOnCreate)}, []string{}), cobrautil.ServiceInfo{Name: "MailHog", URL: "https://github.com/mailhog/MailHog"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("memcached", devenv.ServiceTexts{Long: memcachedLong}, []string{}), cobrautil.ServiceInfo{Name: "Memcached", URL: "https://memcached.org"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("phpmyadmin", devenv.ServiceTexts{Long: phpmyadminLong, OnCreate: envSubst(phpmyadminOnCreate)}, []string{"ppma"}), cobrautil.ServiceInfo{Name: "phpMyAdmin", URL: "https://www.phpmyadmin.net", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/phpmyadmin.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("citoid", devenv.ServiceTexts{Long: citoidLong}, []string{}), cobrautil.ServiceInfo{Name: "Citoid", URL: "https://www.mediawiki.org/wiki/Citoid", Logo: "https://upload.wikimedia.org/wikipedia/commons/b/b5/Citoid_logo.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("jaeger", devenv.ServiceTexts{Long: jaegerLong, OnCreate: envSubst(jaegerOnCreate)}, []string{}), cobrautil.ServiceInfo{Name: "Jaeger", URL: "https://www.jaegertracing.io", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/jaeger.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("prometheus", devenv.ServiceTexts{Long: prometheusLong, OnCreate: envSubst(prometheusOnCreate)}, []string{}), cobrautil.ServiceInfo{Name: "Prometheus", URL: "https://prometheus.io", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/prometheus.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("grafana", devenv.ServiceTexts{Long: grafanaLong, OnCreate: envSubst(grafanaOnCreate)}, []string{}), cobrautil.ServiceInfo{Name: "Grafana", URL: "https://grafana.com", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/grafana.svg"}))

	cmd.AddCommand(cobrautil.WithServiceInfo(devenv.NewServiceCmd("postgres", devenv.ServiceTexts{Long: postgresLong}, []string{}), cobrautil.ServiceInfo{Name: "PostgreSQL", URL: "https://www.postgresql.org", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/postgresql.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(mysql.NewCmd(), cobrautil.ServiceInfo{Name: "MySQL", URL: "https://www.mysql.com", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/mysql.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(mysqlreplica.NewCmd(), cobrautil.ServiceInfo{Name: "MySQL replica", URL: "https://www.mysql.com", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/mysql.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(keycloak.NewCmd(), cobrautil.ServiceInfo{Name: "Keycloak", URL: "https://www.keycloak.org", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/keycloak.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(shellbox.NewCmd(), cobrautil.ServiceInfo{Name: "Shellbox", URL: "https://www.mediawiki.org/wiki/Shellbox", Logo: "https://upload.wikimedia.org/wikipedia/commons/a/a6/MediaWiki-2020-icon.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(redis.NewCmd(), cobrautil.ServiceInfo{Name: "Redis", URL: "https://redis.io", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/redis.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(wdqs.NewCmd(), cobrautil.ServiceInfo{Name: "Wikidata Query Service", URL: "https://www.mediawiki.org/wiki/Wikidata_Query_Service", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/wikidata.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(wdqsUi.NewCmd(), cobrautil.ServiceInfo{Name: "Wikidata Query UI", URL: "https://query.wikidata.org", Logo: "https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/wikidata.svg"}))
	cmd.AddCommand(cobrautil.WithServiceInfo(custom.NewCmd(), cobrautil.ServiceInfo{Name: "Custom services"}))

	return cmd
}

func envSubst(s string) string {
	// TODO do this more dynamically... / better...
	// Resetting is needed here as docker will look at the existing system / content env vars too

	// Record previous env vars...
	previousPort, portSet := os.LookupEnv("PORT")

	// Set and subst...
	err := os.Setenv("PORT", devenv.DefaultForUser().Env().Get("PORT"))
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

//go:embed opensearch/opensearch.long.md
var opensearchLong string

//go:embed opensearch-dashboards/opensearch-dashboards.long.md
var opensearchDashboardsLong string

//go:embed opensearch-dashboards/opensearch-dashboards.oncreate.md
var opensearchDashboardsOnCreate string

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

//go:embed novnc/novnc.long.md
var novncLong string

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

//go:embed prometheus/prometheus.long.md
var prometheusLong string

//go:embed prometheus/prometheus.oncreate.md
var prometheusOnCreate string

//go:embed grafana/grafana.long.md
var grafanaLong string

//go:embed grafana/grafana.oncreate.md
var grafanaOnCreate string
