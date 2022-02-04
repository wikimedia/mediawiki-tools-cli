package exec

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/briandowns/spinner"
	log "github.com/sirupsen/logrus"
)

// HandlerOptions options used when handeling executions.
type HandlerOptions struct {
	Spinner      *spinner.Spinner
	HandleStdout func(stdout bytes.Buffer)
	HandleError  func(stderr bytes.Buffer, err error)
}

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
func RunTTYCommand(options HandlerOptions, cmd *exec.Cmd) {
	log.Trace(cmd.String())

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

/*RunCommand runs a command, handles verbose output and errors.*/
func RunCommand(options HandlerOptions, cmd *exec.Cmd) error {
	if options.Spinner != nil {
		options.Spinner.Start()
	}
	stdout, stderr, err := runCommand(cmd)
	if options.Spinner != nil {
		options.Spinner.Stop()
	}
	handleCommandRun(options, cmd, stdout, stderr, err)

	return err
}

func runCommand(cmd *exec.Cmd) (bytes.Buffer, bytes.Buffer, error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	return stdoutBuf, stderrBuf, err
}

func handleCommandRun(options HandlerOptions, cmd *exec.Cmd, stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	log.Trace(cmd.String())
	if options.HandleStdout != nil {
		options.HandleStdout(stdout)
	} else {
		handleStdout(stdout)
	}
	if options.HandleError != nil {
		options.HandleError(stderr, err)
	} else {
		handleError(stderr, err)
	}
}

func handleStdout(stdout bytes.Buffer) {
	if stdout.String() != "" {
		fmt.Printf("\n%s\n%s\n", "STDOUT:", stdout.String())
	}
}

func handleError(stderr bytes.Buffer, err error) {
	if err != nil && stderr.String() != "" {
		fmt.Printf("\n%s\n%s\n", "STDERR:", stderr.String())
	}
}
