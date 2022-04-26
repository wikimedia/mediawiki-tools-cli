package mwdd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

/*NewServiceCmd a new command for a single service, such as mailhog*/
func NewServiceCmd(name string, long string, aliases []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     name,
		Short:   fmt.Sprintf("%s service", name),
		Long:    cli.RenderMarkdown(long),
		Aliases: aliases,
		RunE:    nil,
	}

	cmd.AddCommand(NewServiceCreateCmd(name))
	cmd.AddCommand(NewServiceDestroyCmd(name))
	cmd.AddCommand(NewServiceStopCmd(name))
	cmd.AddCommand(NewServiceStartCmd(name))
	// There is an expectation that the main service for exec has the same name as the service command overall
	cmd.AddCommand(NewServiceExecCmd(name, name))

	return cmd
}

/*NewServicesCmd a new command for a set of grouped services, such as various flavours of shellbox*/
func NewServicesCmd(groupName string, long string, aliases []string) *cobra.Command {
	return &cobra.Command{
		Use:     groupName,
		Short:   fmt.Sprintf("%s services", groupName),
		Long:    cli.RenderMarkdown(long),
		Aliases: aliases,
		RunE:    nil,
	}
}

func NewServiceCreateCmd(name string) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: fmt.Sprintf("Create the %s containers", name),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerComposeFileExistsOrExit(name)
			services := DefaultForUser().DockerComposeFileServices(name)
			DefaultForUser().UpDetached(services)
		},
	}
}

func NewServiceDestroyCmd(name string) *cobra.Command {
	return &cobra.Command{
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
}

func NewServiceStopCmd(name string) *cobra.Command {
	return &cobra.Command{
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
}

func NewServiceStartCmd(name string) *cobra.Command {
	return &cobra.Command{
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
}

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
			DefaultForUser().DockerExec(DockerExecCommand{
				DockerComposeService: service,
				Command:              command,
				Env:                  env,
				User:                 User,
			})
		},
	}
	cmd.Flags().StringVarP(&User, "user", "u", UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}

func NewServiceCommandCmd(service string, command string) *cobra.Command {
	return &cobra.Command{
		Use:   command,
		Short: "Runs %s in the %s container",
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			userCommand, env := CommandAndEnvFromArgs(args)
			DefaultForUser().DockerExec(DockerExecCommand{
				DockerComposeService: service,
				Command:              append([]string{command}, userCommand...),
				Env:                  env,
			})
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
