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
	"bytes"

	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd/files"
	"gitlab.wikimedia.org/releng/cli/internal/util/dotenv"
)

/*DockerComposeProjectName the name of the docker-compose project.*/
func (m MWDD) DockerComposeProjectName() string {
	return "mwcli-mwdd-" + mwddContext()
}

/*Env ...*/
func (m MWDD) Env() dotenv.File {
	return dotenv.FileForDirectory(m.Directory())
}

// DockerComposeCommand results in something like: `docker-compose <automatic project stuff> <command> <commandArguments>`.
type DockerComposeCommand struct {
	Command          string
	CommandArguments []string
	HandlerOptions   exec.HandlerOptions
}

/*DockerCompose runs any docker-compose command for the mwdd project with the correct project settings and all files loaded.*/
func (m MWDD) DockerCompose(command DockerComposeCommand) error {
	context := exec.ComposeCommandContext{
		ProjectDirectory: m.Directory(),
		ProjectName:      m.DockerComposeProjectName(),
		Files:            files.ListRawDcYamlFilesInContextOfProjectDirectory(m.Directory()),
	}

	return exec.RunCommand(
		command.HandlerOptions,
		exec.ComposeCommand(
			context,
			command.Command,
			command.CommandArguments...,
		),
	)
}

/*DockerComposeTTY runs any docker-compose command for the mwdd project with the correct project settings and all files loaded in a TTY.*/
func (m MWDD) DockerComposeTTY(command DockerComposeCommand) {
	context := exec.ComposeCommandContext{
		ProjectDirectory: m.Directory(),
		ProjectName:      m.DockerComposeProjectName(),
		Files:            files.ListRawDcYamlFilesInContextOfProjectDirectory(m.Directory()),
	}

	exec.RunTTYCommand(
		command.HandlerOptions,
		exec.ComposeCommand(
			context,
			command.Command,
			command.CommandArguments...,
		),
	)
}

/*Exec runs `docker-compose exec -T <service> <commandAndArgs>`.*/
func (m MWDD) Exec(service string, commandAndArgs []string, options exec.HandlerOptions, user string) {
	// TODO refactor this code path to make handeling options nicer
	m.DockerComposeTTY(
		DockerComposeCommand{
			Command:          "exec",
			CommandArguments: append([]string{"-T", "--user", user, service}, commandAndArgs...),
			HandlerOptions:   options,
		},
	)
}

/*ExecNoOutput runs `docker-compose exec -T <service> <commandAndArgs>` with no output.*/
func (m MWDD) ExecNoOutput(service string, commandAndArgs []string, options exec.HandlerOptions, user string) error {
	options.HandleStdout = func(stdout bytes.Buffer) {}
	options.HandleError = func(stderr bytes.Buffer, err error) {}
	return m.DockerCompose(
		DockerComposeCommand{
			Command:          "exec",
			CommandArguments: append([]string{"-T", "--user", user, service}, commandAndArgs...),
			HandlerOptions:   options,
		},
	)
}

/*UpDetached runs `docker-compose up -d <services>`.*/
func (m MWDD) UpDetached(services []string, options exec.HandlerOptions) {
	m.DockerComposeTTY(
		DockerComposeCommand{
			Command:          "up",
			CommandArguments: append([]string{"-d"}, services...),
			HandlerOptions:   options,
		},
	)
}

/*DownWithVolumesAndOrphans runs `docker-compose down --volumes --remove-orphans`.*/
func (m MWDD) DownWithVolumesAndOrphans(options exec.HandlerOptions) {
	m.DockerComposeTTY(
		DockerComposeCommand{
			Command:          "down",
			CommandArguments: []string{"--volumes", "--remove-orphans"},
			HandlerOptions:   options,
		},
	)
}

/*Stop runs `docker-compose stop <services>`.*/
func (m MWDD) Stop(services []string, options exec.HandlerOptions) {
	m.DockerComposeTTY(
		DockerComposeCommand{
			Command:          "stop",
			CommandArguments: services,
			HandlerOptions:   options,
		},
	)
}

/*Start runs `docker-compose start <services>`.*/
func (m MWDD) Start(services []string, options exec.HandlerOptions) {
	m.DockerComposeTTY(
		DockerComposeCommand{
			Command:          "start",
			CommandArguments: services,
			HandlerOptions:   options,
		},
	)
}

/*Rm runs `docker-compose rm --stop --force -v <services>`.*/
func (m MWDD) Rm(services []string, options exec.HandlerOptions) {
	m.DockerComposeTTY(
		DockerComposeCommand{
			Command:          "rm",
			CommandArguments: append([]string{"--stop", "--force", "-v"}, services...),
			HandlerOptions:   options,
		},
	)
}

/*RmVolumes runs `docker volume rm <volume names with docker-compose project prefixed>`.*/
func (m MWDD) RmVolumes(dcVolumes []string, options exec.HandlerOptions) {
	dockerVolumes := []string{}
	for _, dcVolume := range dcVolumes {
		dockerVolumes = append(dockerVolumes, m.DockerComposeProjectName()+"_"+dcVolume)
	}
	exec.RunTTYCommand(
		options,
		exec.Command("docker", append([]string{"volume", "rm"}, dockerVolumes...)...),
	)
}
