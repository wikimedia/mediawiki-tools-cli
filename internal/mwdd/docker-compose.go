package mwdd

import (
	"bytes"

	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd/files"
	"gitlab.wikimedia.org/releng/cli/internal/util/strings"
)

// DockerComposeCommand results in something like: `docker-compose <automatic project stuff> <command> <commandArguments>`.
type DockerComposeCommand struct {
	Command          string
	CommandArguments []string
	NoOutput         bool
	MWDD             MWDD
}

func (dcc DockerComposeCommand) composeCommandContext() exec.ComposeCommandContext {
	return exec.ComposeCommandContext{
		ProjectDirectory: dcc.MWDD.Directory(),
		ProjectName:      dcc.MWDD.DockerComposeProjectName(),
		Files:            files.ListRawDcYamlFilesInContextOfProjectDirectory(dcc.MWDD.Directory()),
	}
}

func (dcc DockerComposeCommand) Run() (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	return exec.RunCommandCollect(
		// TODO get rid of the need to pass HandlerOptions if possible
		exec.HandlerOptions{},
		exec.ComposeCommand(
			dcc.composeCommandContext(),
			dcc.Command,
			dcc.CommandArguments...,
		),
	)
}

func (dcc DockerComposeCommand) RunTTY() {
	exec.RunTTYCommand(
		// TODO get rid of the need to pass HandlerOptions if possible
		exec.HandlerOptions{},
		exec.ComposeCommand(
			dcc.composeCommandContext(),
			dcc.Command,
			dcc.CommandArguments...,
		),
	)
}

/*Exec runs `docker-compose exec -T <service> <commandAndArgs>`.*/
func (m MWDD) Exec(service string, commandAndArgs []string, user string) {
	// TODO refactor this code path to make handeling options nicer
	DockerComposeCommand{
		MWDD:             m,
		Command:          "exec",
		CommandArguments: append([]string{"-T", "--user", user, service}, commandAndArgs...),
	}.RunTTY()
}

/*ExecNoOutput runs `docker-compose exec -T <service> <commandAndArgs>` with no output.*/
func (m MWDD) ExecNoOutput(service string, commandAndArgs []string, user string) error {
	_, _, err := DockerComposeCommand{
		MWDD:             m,
		Command:          "exec",
		CommandArguments: append([]string{"-T", "--user", user, service}, commandAndArgs...),
		NoOutput:         true,
	}.Run()
	return err
}

/*UpDetached runs `docker-compose up -d <services>`.*/
func (m MWDD) UpDetached(services []string) {
	DockerComposeCommand{
		MWDD:             m,
		Command:          "up",
		CommandArguments: append([]string{"-d"}, services...),
	}.RunTTY()
}

/*DownWithVolumesAndOrphans runs `docker-compose down --volumes --remove-orphans`.*/
func (m MWDD) DownWithVolumesAndOrphans() {
	DockerComposeCommand{
		MWDD:             m,
		Command:          "down",
		CommandArguments: []string{"--volumes", "--remove-orphans"},
	}.RunTTY()
}

/*Stop runs `docker-compose stop <services>`.*/
func (m MWDD) Stop(services []string) {
	DockerComposeCommand{
		MWDD:             m,
		Command:          "stop",
		CommandArguments: services,
	}.RunTTY()
}

/*Start runs `docker-compose start <services>`.*/
func (m MWDD) Start(services []string) {
	DockerComposeCommand{
		MWDD:             m,
		Command:          "start",
		CommandArguments: services,
	}.RunTTY()
}

/*Rm runs `docker-compose rm --stop --force -v <services>`.*/
func (m MWDD) Rm(services []string) {
	DockerComposeCommand{
		MWDD:             m,
		Command:          "rm",
		CommandArguments: append([]string{"--stop", "--force", "-v"}, services...),
	}.RunTTY()
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
	stdout, stderr, err := DockerComposeCommand{
		MWDD:             m,
		Command:          "ps",
		CommandArguments: []string{"--services", "--filter", "status=" + statusFilter},
	}.Run()

	serviceList := strings.SplitMultiline(stdout.String())
	if stderr.String() != "" || err != nil {
		logrus.Error(stderr.String())
	}

	return serviceList
}
