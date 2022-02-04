package exec

import (
	"bytes"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
)

// ComposeCommandContext ...
type ComposeCommandContext struct {
	ProjectDirectory string
	ProjectName      string
	Files            []string
}

/*Command passes through to exec.Command for running generic commands.*/
func Command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

/*ComposeCommand gets a docker-compose command to run.*/
func ComposeCommand(context ComposeCommandContext, command string, arg ...string) *exec.Cmd {
	arg = append([]string{command}, arg...)
	arg = append([]string{"--project-name", context.ProjectName}, arg...)
	arg = append([]string{"--project-directory", context.ProjectDirectory}, arg...)
	for _, element := range context.Files {
		arg = append([]string{"--file", context.ProjectDirectory + "/" + element}, arg...)
	}
	return exec.Command("docker-compose", arg...)
}

/*RunTTYCommand runs a command in an interactive shell.*/
func RunTTYCommand(cmd *exec.Cmd) {
	logrus.Trace(cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		logrus.Fatal(err)
	}
}

/*RunCommand runs a command, collecting output and errors.*/
func RunCommandCollect(cmd *exec.Cmd) (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	logrus.Trace(cmd.String())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return stdout, stderr, err
}
