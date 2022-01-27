/*Package mwdd is used to interact a mwdd v2 setup

Copyright © 2020 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package mwdd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
)

func NewServiceCmd(name string, long string, aliases []string) *cobra.Command {
	return &cobra.Command{
		Use:     name,
		Short:   fmt.Sprintf("%s service", name),
		Long:    long,
		Aliases: aliases,
		RunE:    nil,
	}
}

func NewServiceCreateCmd(name string, Verbosity int) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: fmt.Sprintf("Create the %s containers", name),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerComposeFileExistsOrExit(name)
			services := DefaultForUser().DockerComposeFileServices(name)
			DefaultForUser().UpDetached(
				services,
				exec.HandlerOptions{
					Verbosity: Verbosity,
				},
			)
		},
	}
}

func NewServiceDestroyCmd(name string, Verbosity int) *cobra.Command {
	return &cobra.Command{
		Use:   "destroy",
		Short: fmt.Sprintf("Destroy the %s containers", name),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerComposeFileExistsOrExit(name)
			services := DefaultForUser().DockerComposeFileServices(name)
			volumes := DefaultForUser().DockerComposeFileVolumes(name)

			opts := exec.HandlerOptions{
				Verbosity: Verbosity,
			}
			DefaultForUser().Rm(services, opts)
			if len(volumes) > 0 {
				DefaultForUser().RmVolumes(volumes, opts)
			}
		},
	}
}

func NewServiceSuspendCmd(name string, Verbosity int) *cobra.Command {
	return &cobra.Command{
		Use:   "suspend",
		Short: fmt.Sprintf("Suspend the %s containers", name),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerComposeFileExistsOrExit(name)
			services := DefaultForUser().DockerComposeFileServices(name)
			DefaultForUser().Stop(
				services,
				exec.HandlerOptions{
					Verbosity: Verbosity,
				},
			)
		},
	}
}

func NewServiceResumeCmd(name string, Verbosity int) *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: fmt.Sprintf("Resume the %s containers", name),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			DefaultForUser().DockerComposeFileExistsOrExit(name)
			services := DefaultForUser().DockerComposeFileServices(name)
			DefaultForUser().Start(
				services,
				exec.HandlerOptions{
					Verbosity: Verbosity,
				},
			)
		},
	}
}

func NewServiceExecCmd(name string, service string, Verbosity int) *cobra.Command {
	var User string
	cmd := &cobra.Command{
		Use:     "exec [flags] [command...]",
		Example: "  exec bash\n  exec -- bash --help\n  exec --user root bash\n  exec --user root -- bash --help",
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
