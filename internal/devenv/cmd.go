package devenv

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
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
	cmd.AddCommand(NewServiceRebuildCmdP(serviceName, texts.OnCreate))
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
	opts := &dockercompose.UpOptions{Detached: true}
	cmd := newServiceUpCmdP(name, onCreateText, opts)
	cmd.Flags().BoolVar(&opts.ForceRecreate, "force-recreate", false, "Force recreation of containers")
	cmd.Flags().BoolVar(&opts.Build, "build", false, "Build images before starting containers")
	return cmd
}

func NewServiceRebuildCmd(name string, onCreateText string) *cobra.Command {
	return NewServiceRebuildCmdP(&name, onCreateText)
}

func NewServiceRebuildCmdP(name *string, onCreateText string) *cobra.Command {
	cmd := newServiceUpCmdP(name, onCreateText, &dockercompose.UpOptions{
		Detached: true, Build: true, ForceRecreate: true,
	})
	cmd.Use = "rebuild"
	cmd.Short = "Build images and recreate the containers"
	cmd.Long = "Build images and recreate the service containers, preserving their volumes.\nEquivalent to create --build --force-recreate; Docker's build cache is reused."
	return cmd
}

func newServiceUpCmdP(name *string, onCreateText string, opts *dockercompose.UpOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create the containers",
		Long:  fmt.Sprintf("Start the %s service in the local development environment. Use --build to build its image first or --force-recreate to replace its containers.", *name),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedName := *name
			if err := DefaultForUser().PrepareGateway(); err != nil {
				return err
			}
			DefaultForUser().DockerCompose().File(dereferencedName).ExistsOrExit()
			services := DefaultForUser().DockerCompose().File(dereferencedName).Contents().ServiceNames()
			if err := DefaultForUser().DockerCompose().Up(services, *opts); err != nil {
				return err
			}
			if len(onCreateText) > 0 {
				fmt.Print(cli.RenderMarkdown(onCreateText))
			}
			return DefaultForUser().MergeRunningResumeServices()
		},
	}
	cmd.Annotations = map[string]string{"group": "Control"}
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
			return DefaultForUser().UpdateResumeServices(services, false)
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
			if err != nil {
				return err
			}
			return DefaultForUser().UpdateResumeServices(services, false)
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
			if err := DefaultForUser().PrepareGateway(); err != nil {
				return err
			}
			DefaultForUser().DockerCompose().File(dereferencedName).ExistsOrExit()
			services := DefaultForUser().DockerCompose().File(dereferencedName).Contents().ServiceNames()
			err := DefaultForUser().DockerCompose().Up(services, dockercompose.UpOptions{Detached: true})
			if err != nil {
				return err
			}
			return DefaultForUser().MergeRunningResumeServices()
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
		Long:    fmt.Sprintf("Run a command inside the %s container. Arguments after -- are passed to the container command; use --user to choose the execution user.", *service),
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
var devExposeLong string

func NewServiceExposeCmdP(name *string) *cobra.Command {
	var externalPort string
	var internalPort string
	cmd := &cobra.Command{
		Use:    "expose",
		Hidden: true, // Hide as deprecated
		Short:  "DEPRECATED: Expose a port in a running container",
		Long:   cli.RenderMarkdown(devExposeLong),
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
					return fmt.Errorf("unable to inspect container: %w", err)
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
				"alpine/socat:1.8.1.3",
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
		Long:    fmt.Sprintf("Run %s in the service container. Arguments after -- are passed directly to %s.", commands[0], commands[0]),
		Args:    cobra.ArbitraryArgs,
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

	cmd.AddCommand(NewImageDockerfileCmdP(service))

	return cmd
}

func NewImageDockerfileCmd(service string) *cobra.Command {
	return NewImageDockerfileCmdP(&service)
}

// NewImageDockerfileCmdP returns the "image dockerfile" sub-command that lets users
// build a service image from a custom Dockerfile instead of pulling a pre-built one.
//
// Three options are available for how a service image is sourced:
//  1. Default   – no override; the image specified in the compose file is used.
//  2. Custom image (image set) – a pre-built image is pulled from a registry.
//  3. Custom Dockerfile (image dockerfile set) – an image is built locally from a
//     Dockerfile supplied by the user.
func NewImageDockerfileCmdP(service *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dockerfile",
		Short: "Build the service image from a custom Dockerfile",
		Long: `Build a service image from a custom Dockerfile. Use set to activate a file, edit to change it, and rebuild to apply changes.

Guide: mw help guides dev dockerfiles`,
		Example: cobrautil.NormalizeExample(`
dockerfile set
dockerfile set ./Dockerfile
dockerfile edit
# Run this service's rebuild command to build and apply the image.`),
	}

	set := &cobra.Command{
		Use:   "set [path]",
		Short: "Activate a custom Dockerfile for this service",
		Long: `Activate a custom Dockerfile for this service. Without a path, create or reuse Dockerfile.<service> in the environment directory. With a path, use that file; relative paths are resolved from the current directory.

The Dockerfile's directory is the build context. COPY sources must be inside it; use .dockerignore to exclude credentials and unrelated files.

Setting a file does not rebuild running containers. Run the service's rebuild command to build and apply it.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedService := *service
			DefaultForUser().EnsureReady()

			var dockerfilePath string
			if len(args) == 1 {
				// Use the caller-supplied path – it must already exist.
				var err error
				dockerfilePath, err = filepath.Abs(args[0])
				if err != nil {
					return fmt.Errorf("resolve Dockerfile path: %w", err)
				}
				if _, err := os.Stat(dockerfilePath); err != nil {
					return fmt.Errorf("cannot access dockerfile %s: %w", dockerfilePath, err)
				}
			} else {
				// Create (or reuse) a managed Dockerfile in the working directory.
				dockerfilePath = defaultDockerfilePath(DefaultForUser().Directory(), dereferencedService)
				if err := createStarterDockerfileIfNotExists(dockerfilePath, dereferencedService); err != nil {
					return fmt.Errorf("failed to create starter dockerfile: %w", err)
				}
			}

			DefaultForUser().Env().Set(dockerfileEnvKey(dereferencedService), dockerfilePath)
			if err := writeDockerfileComposeFile(DefaultForUser().Directory(), dereferencedService, dockerfilePath); err != nil {
				return fmt.Errorf("failed to write compose override file: %w", err)
			}
			fmt.Printf("Custom Dockerfile activated for service %q.\n", dereferencedService)
			fmt.Printf("Dockerfile: %s\n", dockerfilePath)
			fmt.Printf("Run '%s rebuild' to build and apply the image.\n", cmd.Parent().Parent().Parent().CommandPath())
			fmt.Printf("Run '%s reset' to remove the Dockerfile override.\n", cmd.Parent().CommandPath())
			return nil
		},
	}
	cmd.AddCommand(set)

	dockerfileGet := &cobra.Command{
		Use:   "get",
		Short: "Show the path of the Dockerfile currently configured for this service",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			dereferencedService := *service
			DefaultForUser().EnsureReady()
			path := DefaultForUser().Env().Get(dockerfileEnvKey(dereferencedService))
			if path == "" {
				fmt.Println("No custom Dockerfile set")
			} else {
				fmt.Println(path)
			}
		},
	}
	cmd.AddCommand(dockerfileGet)

	dockerfileCat := &cobra.Command{
		Use:   "cat",
		Short: "Print the contents of the configured Dockerfile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedService := *service
			DefaultForUser().EnsureReady()
			path := DefaultForUser().Env().Get(dockerfileEnvKey(dereferencedService))
			if path == "" {
				return fmt.Errorf("no custom Dockerfile is set for service %q", dereferencedService)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read dockerfile %s: %w", path, err)
			}
			fmt.Print(string(content))
			return nil
		},
	}
	cmd.AddCommand(dockerfileCat)

	dockerfileEdit := &cobra.Command{
		Use:   "edit",
		Short: "Open the configured Dockerfile in your editor",
		Long: `Open the configured Dockerfile in the editor specified by $VISUAL or $EDITOR.

If neither variable is set, vi is used as a fallback.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedService := *service
			DefaultForUser().EnsureReady()
			path := DefaultForUser().Env().Get(dockerfileEnvKey(dereferencedService))
			if path == "" {
				return fmt.Errorf("no custom Dockerfile is set for service %q — run 'image dockerfile set' first", dereferencedService)
			}
			editor := os.Getenv("VISUAL")
			if editor == "" {
				editor = os.Getenv("EDITOR")
			}
			if editor == "" {
				editor = "vi"
			}
			editorCmd := osexec.Command(editor, path) // #nosec G204,G702 -- editor is explicitly selected by the local user.
			editorCmd.Stdout = os.Stdout
			editorCmd.Stdin = os.Stdin
			editorCmd.Stderr = os.Stderr
			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("editor exited with error: %w", err)
			}
			return nil
		},
	}
	cmd.AddCommand(dockerfileEdit)

	dockerfileReset := &cobra.Command{
		Use:   "reset",
		Short: "Remove the custom Dockerfile override",
		Long: `Remove the build override, keeping the Dockerfile on disk. Run the service's create --force-recreate command afterward to replace its containers.

A separate image set override still applies. Reset that with image reset if you want the bundled image.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dereferencedService := *service
			DefaultForUser().EnsureReady()
			DefaultForUser().Env().Delete(dockerfileEnvKey(dereferencedService))
			if err := removeDockerfileComposeFile(DefaultForUser().Directory(), dereferencedService); err != nil {
				return fmt.Errorf("failed to remove compose override file: %w", err)
			}
			fmt.Printf("Custom Dockerfile override removed for service %q.\n", dereferencedService)
			fmt.Printf("Run '%s create --force-recreate' to apply the configured image.\n", cmd.Parent().Parent().Parent().CommandPath())
			return nil
		},
	}
	cmd.AddCommand(dockerfileReset)

	return cmd
}
