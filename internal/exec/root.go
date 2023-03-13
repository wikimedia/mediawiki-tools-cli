package exec

import (
	"bytes"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

// ComposeCommandContext ...
type ComposeCommandContext struct {
	ProjectDirectory string
	ProjectName      string
}

/*Command passes through to exec.Command for running generic commands.*/
func Command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

/*ComposeCommand gets a docker-compose command to run.*/
func ComposeCommand(context ComposeCommandContext, command string, arg ...string) *exec.Cmd {
	dcp := dockercompose.Project{
		Name:      context.ProjectName,
		Directory: context.ProjectDirectory,
	}
	return dcp.Cmd(append([]string{command}, arg...))
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
