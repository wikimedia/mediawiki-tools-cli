package mwdd

import (
	"context"
	_ "embed"
	"fmt"
	osexec "os/exec"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/exec"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

type ServiceTexts struct {
	// Long description for the top level command
	Long string
	// Output after the service has been created
	OnCreate string
}

func NewServiceCmd(name string, texts ServiceTexts, aliases []string) *cobra.Command {
	return NewServiceCmdP(&name, texts, aliases)
}

/*NewServiceCmd a new command for a single service, such as mailhog.*/
func NewServiceCmdP(name *string, texts ServiceTexts, aliases []string) *cobra.Command {
	return NewServiceCmdDifferingNamesP(name, name, texts, aliases)
}

func NewServiceCmdDifferingNames(commandName string, serviceName string, texts ServiceTexts, aliases []string) *cobra.Command {
	return NewServiceCmdDifferingNamesP(&commandName, &serviceName, texts, aliases)
}

/*NewServiceCmdDifferingNames a new command for a single service, such as mailhog.*/
func NewServiceCmdDifferingNamesP(commandName *string, serviceName *string, texts ServiceTexts, aliases []string) *cobra.Command {
	dereferencedCommandName := *commandName
	cmd := &cobra.Command{
		Use:     *commandName,
		GroupID: "service",
		Short:   fmt.Sprintf("%s service", dereferencedCommandName),
		Aliases: aliases,
		RunE:    nil,
	}

	if len(texts.Long) > 0 {
		cmd.Long = cli.RenderMarkdown(texts.Long)
	}

	cmd.AddCommand(NewImageCmdP(serviceName))
	cmd.AddCommand(NewServiceCreateCmdP(serviceName, texts.OnCreate))
	cmd.AddCommand(NewServiceDestroyCmdP(serviceName))
	cmd.AddCommand(NewServiceStopCmdP(serviceName))
	cmd.AddCommand(NewServiceStartCmdP(serviceName))
	cmd.AddCommand(NewServiceExposeCmdP(serviceName))
	// There is an expectation that the main service for exec has the same name as the service command overall
	cmd.AddCommand(NewServiceExecCmdP(serviceName, serviceName))

	return cmd
}

/*NewServicesCmd a new command for a set of grouped services, such as various flavours of shellbox.*/
func NewServicesCmd(groupName string, texts ServiceTexts, aliases []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     groupName,
		GroupID: "service",
		Short:   fmt.Sprintf("%s services", groupName),
		Long:    cli.RenderMarkdown(texts.Long),
		Aliases: aliases,
		RunE:    nil,
	}
	cmd.AddGroup(&cobra.Group{
		ID:    "service",
		Title: "Service Commands",
	})
	return cmd
}

func NewServiceCreateCmd(name string, onCreateText string) *cobra.Command {
	return NewServiceCreateCmdP(&name, onCreateText)
}

func NewServiceCreateCmdP(name *string, onCreateText string) *cobra.Command {
	var forceRecreate bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create the containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedName := *name
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerCompose().File(dereferencedName).ExistsOrExit()
			services := DefaultForUser().DockerCompose().File(dereferencedName).Contents().ServiceNames()
			err := DefaultForUser().DockerCompose().Up(services, dockercompose.UpOptions{
				Detached:      true,
				ForceRecreate: forceRecreate,
			})
			if err != nil {
				return err
			}
			if len(onCreateText) > 0 {
				fmt.Print(cli.RenderMarkdown(onCreateText))
			}
			return nil
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	cmd.Flags().BoolVar(&forceRecreate, "force-recreate", false, "Force recreation of containers")
	return cmd
}

func NewServiceDestroyCmd(name string) *cobra.Command {
	return NewServiceDestroyCmdP(&name)
}

func NewServiceDestroyCmdP(name *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedName := *name
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerCompose().File(dereferencedName).ExistsOrExit()
			services := DefaultForUser().DockerCompose().File(dereferencedName).Contents().ServiceNames()
			volumes := DefaultForUser().DockerCompose().File(dereferencedName).Contents().VolumeNames()

			err := DefaultForUser().DockerCompose().Rm(services, dockercompose.RmOptions{
				Stop:                   true,
				Force:                  true,
				RemoveAnonymousVolumes: true,
			})
			if err != nil {
				return err
			}
			if len(volumes) > 0 {
				err := DefaultForUser().DockerCompose().VolumesRm(volumes)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}

func NewServiceStopCmd(name string) *cobra.Command {
	return NewServiceStopCmdP(&name)
}

func NewServiceStopCmdP(name *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stop",
		Aliases: []string{"suspend"},
		Short:   "Stop the containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedName := *name
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerCompose().File(dereferencedName).ExistsOrExit()
			services := DefaultForUser().DockerCompose().File(dereferencedName).Contents().ServiceNames()
			err := DefaultForUser().DockerCompose().Stop(services)
			return err
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}

func NewServiceStartCmd(name string) *cobra.Command {
	return NewServiceStartCmdP(&name)
}

func NewServiceStartCmdP(name *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		Aliases: []string{"resume"},
		Short:   "Start the containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedName := *name
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerCompose().File(dereferencedName).ExistsOrExit()
			services := DefaultForUser().DockerCompose().File(dereferencedName).Contents().ServiceNames()
			err := DefaultForUser().DockerCompose().Start(services)
			return err
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
	return NewServiceExecCmdP(&name, &service)
}

func NewServiceExecCmdP(name *string, service *string) *cobra.Command {
	var User string
	cmd := &cobra.Command{
		Use:     "exec [flags] [command...]",
		Example: cobrautil.NormalizeExample("exec bash\nexec -- bash --help\nexec --user root bash\nexec --user root -- bash --help"),
		Short:   "Execute a command in the main container",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedName := *name
			dereferencedService := *service
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerCompose().File(dereferencedName).ExistsOrExit()
			command, env := CommandAndEnvFromArgs(args)
			containerID, containerIDErr := DefaultForUser().DockerCompose().ContainerID(dereferencedService)
			if containerIDErr != nil {
				return containerIDErr
			}
			exitCode := docker.Exec(
				containerID,
				docker.ExecOptions{
					Command: command,
					Env:     env,
					User:    User,
				},
			)
			if exitCode != 0 {
				cmd.Root().Annotations = make(map[string]string)
				cmd.Root().Annotations["exitCode"] = strconv.Itoa(exitCode)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&User, "user", "u", docker.CurrentUserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}

func NewServiceExposeCmd(name string) *cobra.Command {
	return NewServiceExposeCmdP(&name)
}

//go:embed expose.long.md
var mwddExposeLong string

func NewServiceExposeCmdP(name *string) *cobra.Command {
	var externalPort string
	var internalPort string
	cmd := &cobra.Command{
		Use:    "expose",
		Hidden: true, // Hide as deprecated
		Short:  "DEPRECATED: Expose a port in a running container",
		Long:   cli.RenderMarkdown(mwddExposeLong),
		Example: cobrautil.NormalizeExample(`
		expose
		expose --external-port 1234
		expose --external-port 1234 --internal-port 80
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedName := *name
			m := DefaultForUser()
			m.EnsureReady()
			m.DockerCompose().File(dereferencedName).ExistsOrExit()

			ctx := context.Background()
			cli := docker.NewClientFromEnvOrPanic()
			containerID, err := m.DockerCompose().ContainerID(dereferencedName)
			if err != nil {
				// unable to execute command, no container found for service: mysql
				if strings.Contains(err.Error(), "no container found") {
					return fmt.Errorf("Container must be running before you can expose a port")
				}
				return err
			}

			// Lookup internal port from an env var if not provided
			if internalPort == "" {
				logrus.Debug("No internal port provided, looking up from container env")
				containerJson, err := cli.ContainerInspect(ctx, containerID)
				if err != nil {
					return fmt.Errorf("Unable to inspect container" + err.Error())
				}

				// Get the DEFAULT_EXPOSE_PORT environment variable from the containerJson if set
				for _, env := range containerJson.Config.Env {
					if strings.HasPrefix(env, "DEFAULT_EXPOSE_PORT=") {
						internalPort = strings.Split(env, "=")[1]
					}
				}
				if internalPort == "" {
					return fmt.Errorf("No known default port to expose, please specify one with --internal-port")
				}
			}

			var publish string
			if externalPort == "" {
				// Random port will be chosen by docker
				publish = internalPort
			} else {
				publish = externalPort + ":" + internalPort
			}

			network := m.dockerNetworkName()

			exec.RunTTYCommand(osexec.Command(
				"docker", "run",
				"--publish", publish,
				"--link", containerID,
				"--network", network,
				"alpine/socat:1.7.4.4-r0",
				"tcp-listen:"+internalPort+",fork,reuseaddr", "tcp-connect:"+dereferencedName+":"+internalPort,
			)) // #nosec G204
			return nil
		},
	}
	cmd.Flags().StringVarP(&externalPort, "external-port", "e", "", "External port to expose")
	cmd.Flags().StringVarP(&internalPort, "internal-port", "i", "", "Internal port to expose")
	return cmd
}

func NewServiceCommandCmd(service string, commands []string, aliases []string) *cobra.Command {
	return NewServiceCommandCmdP(&service, commands, aliases)
}

func NewServiceCommandCmdP(service *string, commands []string, aliases []string) *cobra.Command {
	return &cobra.Command{
		// Mention that additional flags will be passed on to the original command
		Use:     fmt.Sprintf("%s [flags] -- [%s flags]", commands[0], commands[0]),
		Aliases: aliases,
		Short:   fmt.Sprintf("Runs %s in the container", commands[0]),
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedName := *service
			DefaultForUser().EnsureReady()
			userCommand, env := CommandAndEnvFromArgs(args)
			containerId, containerIDErr := DefaultForUser().DockerCompose().ContainerID(dereferencedName)
			if containerIDErr != nil {
				return containerIDErr
			}
			exitCode := docker.Exec(
				containerId,
				docker.ExecOptions{
					Command: append(commands, userCommand...),
					Env:     env,
				},
			)
			if exitCode != 0 {
				cmd.Root().Annotations = make(map[string]string)
				cmd.Root().Annotations["exitCode"] = strconv.Itoa(exitCode)
			}
			return nil
		},
		DisableFlagsInUseLine: true,
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

func NewImageCmd(service string) *cobra.Command {
	return NewImageCmdP(&service)
}

func NewImageCmdP(service *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Interact with the image used for the service",
	}

	get := &cobra.Command{
		Use:   "get",
		Short: "Outputs the name of the image currently being used",
		Run: func(cmd *cobra.Command, args []string) {
			dereferencedService := *service
			DefaultForUser().EnsureReady()
			fmt.Println(DefaultForUser().DockerCompose().Config().Services[dereferencedService].Image)
		},
	}
	cmd.AddCommand(get)

	set := &cobra.Command{
		Use:   "set",
		Short: "Sets the image to use for the service as an overrides environment variable",
		Run: func(cmd *cobra.Command, args []string) {
			dereferencedService := *service
			DefaultForUser().EnsureReady()
			DefaultForUser().Env().Set(strings.ToUpper(dereferencedService)+"_IMAGE", args[0])
		},
	}
	cmd.AddCommand(set)

	reset := &cobra.Command{
		Use:   "reset",
		Args:  cobra.NoArgs,
		Short: "Resets the image to use for the service to the default image specified in the docker-compose.yml file",
		Run: func(cmd *cobra.Command, args []string) {
			dereferencedService := *service
			DefaultForUser().EnsureReady()
			DefaultForUser().Env().Delete(strings.ToUpper(dereferencedService) + "_IMAGE")
		},
	}
	cmd.AddCommand(reset)

	return cmd
}
