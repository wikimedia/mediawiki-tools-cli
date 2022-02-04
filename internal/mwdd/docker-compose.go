package mwdd

import (
	"bytes"
	"fmt"

	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd/files"
	"gitlab.wikimedia.org/releng/cli/internal/util/strings"
)

// DockerComposeCommand results in something like: `docker-compose <automatic project stuff> <command> <commandArguments>`.
type DockerComposeCommand struct {
	Command          string
	CommandArguments []string
	NoOutput         bool
	HandlerOptions   exec.HandlerOptions
}

/*DockerCompose runs any docker-compose command for the mwdd project with the correct project settings and all files loaded.*/
func (m MWDD) DockerCompose(command DockerComposeCommand) error {
	context := exec.ComposeCommandContext{
		ProjectDirectory: m.Directory(),
		ProjectName:      m.DockerComposeProjectName(),
		Files:            files.ListRawDcYamlFilesInContextOfProjectDirectory(m.Directory()),
	}

	if command.NoOutput {
		// TODO stop setting these on the HandlerOptions in command
		// and just pass in to exec.RunCommand our own options...
		command.HandlerOptions.HandleStdout = func(stdout bytes.Buffer) {}
		command.HandlerOptions.HandleError = func(stderr bytes.Buffer, err error) {}
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
func (m MWDD) ExecNoOutput(service string, commandAndArgs []string, user string) error {
	return m.DockerCompose(
		DockerComposeCommand{
			Command:          "exec",
			CommandArguments: append([]string{"-T", "--user", user, service}, commandAndArgs...),
			NoOutput:         true,
		},
	)
}

/*UpDetached runs `docker-compose up -d <services>`.*/
func (m MWDD) UpDetached(services []string) {
	m.DockerComposeTTY(
		DockerComposeCommand{
			Command:          "up",
			CommandArguments: append([]string{"-d"}, services...),
		},
	)
}

/*DownWithVolumesAndOrphans runs `docker-compose down --volumes --remove-orphans`.*/
func (m MWDD) DownWithVolumesAndOrphans() {
	m.DockerComposeTTY(
		DockerComposeCommand{
			Command:          "down",
			CommandArguments: []string{"--volumes", "--remove-orphans"},
		},
	)
}

/*Stop runs `docker-compose stop <services>`.*/
func (m MWDD) Stop(services []string) {
	m.DockerComposeTTY(
		DockerComposeCommand{
			Command:          "stop",
			CommandArguments: services,
		},
	)
}

/*Start runs `docker-compose start <services>`.*/
func (m MWDD) Start(services []string) {
	m.DockerComposeTTY(
		DockerComposeCommand{
			Command:          "start",
			CommandArguments: services,
		},
	)
}

/*Rm runs `docker-compose rm --stop --force -v <services>`.*/
func (m MWDD) Rm(services []string) {
	m.DockerComposeTTY(
		DockerComposeCommand{
			Command:          "rm",
			CommandArguments: append([]string{"--stop", "--force", "-v"}, services...),
		},
	)
}

/*RmVolumes runs `docker volume rm <volume names with docker-compose project prefixed>`.*/
func (m MWDD) RmVolumes(dcVolumes []string) {
	dockerVolumes := []string{}
	for _, dcVolume := range dcVolumes {
		dockerVolumes = append(dockerVolumes, m.DockerComposeProjectName()+"_"+dcVolume)
	}
	exec.RunTTYCommand(
		exec.HandlerOptions{},
		exec.Command("docker", append([]string{"volume", "rm"}, dockerVolumes...)...),
	)
}

/*ServicesWithStatus lists services in the docker-compose setup that have the given status*/
func (m MWDD) ServicesWithStatus(statusFilter string) []string {
	serviceList := []string{}

	// TODO stop using HandleStdout etc to do the output handeling here
	// This is lame passing this into the DockerComposeCommand, so clean this up
	options := exec.HandlerOptions{}
	options.HandleStdout = func(stdout bytes.Buffer) {
		serviceList = strings.SplitMultiline(stdout.String())
	}
	options.HandleError = func(stderr bytes.Buffer, err error) {
		if stderr.String() != "" {
			fmt.Println(stderr.String())
		}
	}

	m.DockerCompose(
		DockerComposeCommand{
			Command:          "ps",
			CommandArguments: []string{"--services", "--filter", "status=" + statusFilter},
			HandlerOptions:   options,
		},
	)
	return serviceList
}
