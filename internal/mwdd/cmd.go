package mwdd

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/exec"
)

type ServiceTexts struct {
	// Long description for the top level command
	Long string
	// Output after the service has been created
	OnCreate string
}

/*NewServiceCmd a new command for a single service, such as mailhog.*/
func NewServiceCmd(name string, texts ServiceTexts, aliases []string) *cobra.Command {
	return NewServiceCmdDifferingNames(name, name, texts, aliases)
}

/*NewServiceCmdDifferingNames a new command for a single service, such as mailhog.*/
func NewServiceCmdDifferingNames(commandName string, serviceName string, texts ServiceTexts, aliases []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     commandName,
		Short:   fmt.Sprintf("%s service", commandName),
		Aliases: aliases,
		RunE:    nil,
	}

	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Service"

	if len(texts.Long) > 0 {
		cmd.Long = cli.RenderMarkdown(texts.Long)
	}

	cmd.AddCommand(NewServiceCreateCmd(serviceName, texts.OnCreate))
	cmd.AddCommand(NewServiceDestroyCmd(serviceName))
	cmd.AddCommand(NewServiceStopCmd(serviceName))
	cmd.AddCommand(NewServiceStartCmd(serviceName))
	cmd.AddCommand(NewServiceExposeCmd(serviceName))
	// There is an expectation that the main service for exec has the same name as the service command overall
	cmd.AddCommand(NewServiceExecCmd(serviceName, serviceName))

	return cmd
}

/*NewServicesCmd a new command for a set of grouped services, such as various flavours of shellbox.*/
func NewServicesCmd(groupName string, texts ServiceTexts, aliases []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     groupName,
		Short:   fmt.Sprintf("%s services", groupName),
		Long:    cli.RenderMarkdown(texts.Long),
		Aliases: aliases,
		RunE:    nil,
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Service"
	return cmd
}

func NewServiceCreateCmd(name string, onCreateText string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: fmt.Sprintf("Create the %s containers", name),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerComposeFileExistsOrExit(name)
			services := DefaultForUser().DockerComposeFileServices(name)
			DefaultForUser().UpDetached(services)
			if len(onCreateText) > 0 {
				fmt.Print(cli.RenderMarkdown(onCreateText))
			}
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}

func NewServiceDestroyCmd(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: fmt.Sprintf("Destroy the %s containers", name),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerComposeFileExistsOrExit(name)
			services := DefaultForUser().DockerComposeFileServices(name)
			volumes := DefaultForUser().DockerComposeFileVolumes(name)

			DefaultForUser().Rm(services)
			if len(volumes) > 0 {
				DefaultForUser().RmVolumes(volumes)
			}
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}

func NewServiceStopCmd(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stop",
		Aliases: []string{"suspend"},
		Short:   fmt.Sprintf("Stop the %s containers", name),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerComposeFileExistsOrExit(name)
			services := DefaultForUser().DockerComposeFileServices(name)
			DefaultForUser().Stop(services)
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}

func NewServiceStartCmd(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		Aliases: []string{"resume"},
		Short:   fmt.Sprintf("Start the %s containers", name),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerComposeFileExistsOrExit(name)
			services := DefaultForUser().DockerComposeFileServices(name)
			DefaultForUser().Start(services)
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}

// TODO move these commands into cmd/docker/genericservice ?
// TODO split each cmd into its own file
// TODO exreact the Examples, such as that below into own files

func NewServiceExecCmd(name string, service string) *cobra.Command {
	var User string
	cmd := &cobra.Command{
		Use:     "exec [flags] [command...]",
		Example: "exec bash\nexec -- bash --help\nexec --user root bash\nexec --user root -- bash --help",
		Short:   fmt.Sprintf("Execute a command in the main %s container", name),
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerComposeFileExistsOrExit(name)
			command, env := CommandAndEnvFromArgs(args)
			exitCode := DefaultForUser().DockerExec(
				DockerExecCommand{
					DockerComposeService: service,
					Command:              command,
					Env:                  env,
					User:                 User,
				},
			)
			if exitCode != 0 {
				cmd.Root().Annotations = make(map[string]string)
				cmd.Root().Annotations["exitCode"] = strconv.Itoa(exitCode)
			}
		},
	}
	cmd.Flags().StringVarP(&User, "user", "u", UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}

func NewServiceExposeCmd(name string) *cobra.Command {
	var externalPort string
	var internalPort string
	cmd := &cobra.Command{
		Use:   "expose",
		Short: "Expose a port in a running container",
		Example: heredoc.Doc(`
		expose --external-port 8899
		expose --external-port 8899 --internal-port 80
		`),
		Run: func(cmd *cobra.Command, args []string) {
			m := DefaultForUser()
			m.EnsureReady()
			m.DockerComposeFileExistsOrExit(name)

			ctx := context.Background()
			cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				fmt.Println("Unable to create docker client")
				panic(err)
			}
			containerID := m.containerID(ctx, cli, name)

			// Lookup internal port from an env var if not provided
			if internalPort == "" {
				logrus.Debug("No internal port provided, looking up from container env")
				containerJson, err := cli.ContainerInspect(ctx, containerID)
				if err != nil {
					fmt.Println("Unable to inspect container")
					panic(err)
				}

				// Get the DEFAULT_EXPOSE_PORT environemtn variable from the containerJson if set
				for _, env := range containerJson.Config.Env {
					if strings.HasPrefix(env, "DEFAULT_EXPOSE_PORT=") {
						internalPort = strings.Split(env, "=")[1]
					}
				}
				if internalPort == "" {
					fmt.Println("No known default port to expose, please specify one with --internal-port")
					os.Exit(1)
				}
			}

			var publish string
			if externalPort == "" {
				// Random port will be chosen by docker
				publish = internalPort
			} else {
				publish = externalPort + ":" + internalPort
			}

			network := m.networkName()

			exec.RunTTYCommand(osexec.Command(
				"docker", "run",
				"--publish", publish,
				"--link", containerID,
				"--network", network,
				"alpine/socat:1.7.4.4-r0",
				"tcp-listen:"+internalPort+",fork,reuseaddr", "tcp-connect:"+name+":"+internalPort,
			))
		},
	}
	cmd.Flags().StringVarP(&externalPort, "external-port", "e", "", "External port to expose")
	cmd.Flags().StringVarP(&internalPort, "internal-port", "i", "", "Internal port to expose")
	return cmd
}

func NewServiceCommandCmd(service string, commands []string, aliases []string) *cobra.Command {
	return &cobra.Command{
		Use:     commands[0],
		Aliases: aliases,
		Short:   fmt.Sprintf("Runs %s in the %s container", commands[0], service),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			userCommand, env := CommandAndEnvFromArgs(args)
			exitCode := DefaultForUser().DockerExec(
				DockerExecCommand{
					DockerComposeService: service,
					Command:              append(commands, userCommand...),
					Env:                  env,
				},
			)
			if exitCode != 0 {
				cmd.Root().Annotations = make(map[string]string)
				cmd.Root().Annotations["exitCode"] = strconv.Itoa(exitCode)
			}
		},
	}
}

type WherePathProvider func() string

func NewWhereCmd(description string, pathProvider WherePathProvider) *cobra.Command {
	return &cobra.Command{
		Use:   "where",
		Short: "Outputs the path of " + description,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(pathProvider())
		},
	}
}
