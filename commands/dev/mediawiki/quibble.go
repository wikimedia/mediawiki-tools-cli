package mediawiki

import (
	_ "embed"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

//go:embed quibble.long.md
var devMediawikiQuibbleLong string

//go:embed quibble.example
var mediawikiQuibbleExample string

func NewMediaWikiQuibbleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "quibble [flags] [quibble-commands]... -- [quibble-args]",
		Short:   "Runs commands in a 'quibble' container.",
		Args:    cobra.MinimumNArgs(1),
		Long:    cli.RenderMarkdown(devMediawikiQuibbleLong),
		Example: cobrautil.NormalizeExample(mediawikiQuibbleExample),
		Run:     runMediaWikiQuibble,
	}
	cmd.PersistentFlags().StringVarP(&User, "user", "u", docker.CurrentUserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	cmd.AddCommand(&cobra.Command{
		Use:     "record [env...] [spec]",
		Short:   "Record a Selenium test run as video",
		Long:    "Run Selenium through Quibble with video recording enabled, a non-headless browser, and one worker. Optionally select a spec; otherwise run the full suite. Uses the internal development wiki with Admin/mwddpassword and skips cloning and installation steps. Run quibble npm ci first. Videos normally appear in tests/selenium/log/ in the host checkout. Override MW_SERVER, MW_SCRIPT_PATH, MEDIAWIKI_USER, MEDIAWIKI_PASSWORD or LOG_DIR with NAME=value arguments. Targets must be reachable from the container; gateway authentication is not bypassed.",
		Example: cobrautil.NormalizeExample("record\nrecord tests/selenium/specs/recentchanges.js\nrecord MW_SERVER=http://traefik:8080 MW_SCRIPT_PATH=/proxy/default/w tests/selenium/specs/recentchanges.js"),
		Args: func(cmd *cobra.Command, args []string) error {
			spec, _ := cobrautil.CommandAndEnvFromArgs(args)
			return cobra.MaximumNArgs(1)(cmd, spec)
		},
		Run: func(cmd *cobra.Command, args []string) {
			runMediaWikiQuibble(cmd, quibbleRecordingArgs(args))
		},
	})
	return cmd
}

func quibbleRecordingArgs(args []string) []string {
	spec, overrides := cobrautil.CommandAndEnvFromArgs(args)
	settings := map[string]string{
		"MW_SERVER":             "http://mediawiki-web:8080",
		"MW_SCRIPT_PATH":        "/w",
		"MEDIAWIKI_USER":        "Admin",
		"MEDIAWIKI_PASSWORD":    "mwddpassword",
		"LOG_DIR":               "/workspace/src/tests/selenium/log",
		"CI":                    "1",
		"WDIO_USE_NATIVE_FETCH": "1",
	}
	for _, override := range overrides {
		key, value, _ := strings.Cut(override, "=")
		settings[key] = value
	}
	env := make([]string, 0, len(settings))
	for key, value := range settings {
		env = append(env, key+"="+value)
	}
	sort.Strings(env)
	command := seleniumRecordingCommand(spec)
	for i, arg := range command {
		// Quibble's --command runs in a POSIX shell. Quote every argument so
		// spec paths containing spaces, quotes or shell syntax stay literal.
		command[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return append(env,
		"quibble", "--skip-zuul", "--skip-deps", "--skip-install", "--db-is-external",
		"--web-backend", "external", "--web-url", settings["MW_SERVER"]+settings["MW_SCRIPT_PATH"],
		"--log-dir", settings["LOG_DIR"], "--command", strings.Join(command, " "),
	)
}

func runMediaWikiQuibble(cmd *cobra.Command, args []string) {
	devenv.DefaultForUser().EnsureReady()
	err := devenv.DefaultForUser().DockerCompose().Up([]string{"mediawiki-quibble"}, dockercompose.UpOptions{
		Detached: true,
	})
	if err != nil {
		panic(err)
	}
	command, env := cobrautil.CommandAndEnvFromArgs(args)
	containerID, containerIDErr := devenv.DefaultForUser().DockerCompose().ContainerID("mediawiki-quibble")
	if containerIDErr != nil {
		panic(containerIDErr)
	}
	exitCode := docker.Exec(
		containerID,
		applyRelevantMediawikiWorkingDirectory(
			docker.ExecOptions{
				Command: command,
				Env:     env,
				User:    User,
			},
			"/workspace/src",
		),
	)
	if exitCode != 0 {
		cmd.Root().Annotations = make(map[string]string)
		cmd.Root().Annotations["exitCode"] = strconv.Itoa(exitCode)
	}
}
