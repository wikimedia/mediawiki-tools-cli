package mwdd

import (
	"context"
	"fmt"
	"io"
	"os"
	ossignal "os/signal"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/signal"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

// DockerExecCommand to be run with Docker, which directly uses the docker SDK.
type DockerExecCommand struct {
	DockerComposeService string
	Command              []string
	Env                  []string
	WorkingDir           string
	User                 string
}

/*CommandAndEnvFromArgs takes arguments passed to a cobra command and extracts any prefixing env var definitions from them.*/
func CommandAndEnvFromArgs(args []string) ([]string, []string) {
	extractedArgs := []string{}
	extractedEnvs := []string{}
	regex, _ := regexp.Compile(`\w+=\w+`)
	for _, arg := range args {
		matched := regex.MatchString(arg)
		if matched {
			extractedEnvs = append(extractedEnvs, arg)
		} else {
			extractedArgs = append(extractedArgs, arg)
		}
	}
	return extractedArgs, extractedEnvs
}

/*UserAndGroupForDockerExecution gets a user and group id combination for the current user that can be used for execution.*/
func UserAndGroupForDockerExecution() string {
	if runtime.GOOS == "windows" {
		// TODO confirm that just using 2000 will always work on Windows?
		// This user won't exist, but that fact doesn't really matter on pure Windows
		return "2000:2000"
	}
	return fmt.Sprint(os.Getuid(), ":", os.Getgid())
}

func (m MWDD) networkName() string {
	projectname := strings.ToLower(m.DockerComposeProjectName())
	// Default network is always dps...
	return projectname + "_dps"
}

func (m MWDD) containerID(ctx context.Context, cli *client.Client, service string) string {
	containerFilters := filters.NewArgs()
	projectname := strings.ToLower(m.DockerComposeProjectName())
	containerFilters.Add("label", "com.docker.compose.project="+projectname)
	containerFilters.Add("label", "com.docker.compose.service="+service)
	containerFilters.Add("label", "com.docker.compose.container-number=1")
	logrus.Trace("Getting container ID for service: " + service)
	logrus.Trace("Container filters: project = " + projectname + ", service = " + service)
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: containerFilters})
	if err != nil {
		fmt.Println("Error getting containers for service", service)
		panic(err)
	}
	if len(containers) == 0 {
		fmt.Println("Unable to execute command, no container found for service: " + service)
		fmt.Println("You probably need to create the service first")
		os.Exit(1)
	}
	if len(containers) > 1 {
		panic("More than one container found for service: " + service)
	}
	return containers[0].ID
}

/*DockerExec runs a docker exec command using the docker SDK.*/
func (m MWDD) DockerExec(command DockerExecCommand) (ExitCode int) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Println("Unable to create docker client")
		panic(err)
	}

	containerID := m.containerID(ctx, cli, command.DockerComposeService)

	execConfig := types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		AttachStdin:  true,
		Tty:          true,
		WorkingDir:   command.WorkingDir,
		User:         command.User,
		Cmd:          command.Command,
		Env:          command.Env,
	}

	response, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		fmt.Println("containerExecCreate failed")
		fmt.Println(err)
		return 1
	}

	execID := response.ID
	if execID == "" {
		fmt.Println("exec ID empty")
		return 1
	}

	execStartCheck := types.ExecStartCheck{
		Tty: true,
	}

	waiter, err := cli.ContainerExecAttach(ctx, execID, execStartCheck)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	if execConfig.Tty {
		if err := monitorTtySize(ctx, cli, execID, true); err != nil {
			fmt.Println("Error monitoring TTY size:")
			fmt.Println(err)
		}
	}

	// When TTY is ON, just copy stdout https://phabricator.wikimedia.org/T282340
	// See: https://github.com/docker/cli/blob/70a00157f161b109be77cd4f30ce0662bfe8cc32/cli/command/container/hijack.go#L121-L130
	go io.Copy(os.Stdout, waiter.Reader)
	go io.Copy(waiter.Conn, os.Stdin)

	fd := int(os.Stdin.Fd())
	var oldState *term.State
	if term.IsTerminal(fd) {
		oldState, _ = term.MakeRaw(fd)
		defer term.Restore(fd, oldState)
	}

	for {
		resp, err := cli.ContainerExecInspect(ctx, execID)
		time.Sleep(50 * time.Millisecond)
		if err != nil {
			return 1
		}

		if !resp.Running {
			return resp.ExitCode
		}
	}
}

// MonitorTtySize updates the container tty size when the terminal tty changes size.
func monitorTtySize(ctx context.Context, client client.APIClient, id string, isExec bool) error {
	// Source: https://github.com/skiffos/skiff-core/blob/82c430e4961453c250883c2e5ebd4bd360fa13a5/shell/tty.go
	resizeTty := func() {
		width, height, _ := term.GetSize(0)
		resizeTtyTo(ctx, client, id, uint(height), uint(width), isExec)
	}

	resizeTty()

	sigchan := make(chan os.Signal, 1)
	ossignal.Notify(sigchan, signal.SIGWINCH)
	go func() {
		for range sigchan {
			resizeTty()
		}
	}()

	return nil
}

func resizeTtyTo(ctx context.Context, client client.ContainerAPIClient, id string, height, width uint, isExec bool) {
	// Source: https://github.com/skiffos/skiff-core/blob/82c430e4961453c250883c2e5ebd4bd360fa13a5/shell/tty.go
	if height == 0 && width == 0 {
		return
	}

	options := types.ResizeOptions{
		Height: height,
		Width:  width,
	}

	var err error
	if isExec {
		err = client.ContainerExecResize(ctx, id, options)
	} else {
		err = client.ContainerResize(ctx, id, options)
	}

	_ = err // Ignore this error for now.
	/*
		if err != nil {
		}
	*/
}
