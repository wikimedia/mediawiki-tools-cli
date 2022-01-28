package mwdd

import (
	"context"
	"fmt"
	"io"
	"os"
	gosignal "os/signal"
	"regexp"
	"runtime"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/signal"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
	terminal "golang.org/x/term"
)

// DockerExecCommand to be run with Docker, which directly uses the docker SDK.
type DockerExecCommand struct {
	DockerComposeService string
	Command              []string
	Env                  []string
	WorkingDir           string
	User                 string
	HandlerOptions       exec.HandlerOptions
}

/*CommandAndEnvFromArgs taks arguments passed to a cobra command and extracts any prefixing env var definitions from them*/
func CommandAndEnvFromArgs(args []string) ([]string, []string) {
	extrectedArgs := []string{}
	extrectedEnvs := []string{}
	regex, _ := regexp.Compile(`\w+=\w+`)
	for _, arg := range args {
		matched := regex.MatchString(arg)
		if matched {
			extrectedEnvs = append(extrectedEnvs, arg)
		} else {
			extrectedArgs = append(extrectedArgs, arg)
		}
	}
	return extrectedArgs, extrectedEnvs
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

func (m MWDD) containerID(ctx context.Context, cli *client.Client, service string) string {
	containerFilters := filters.NewArgs()
	containerFilters.Add("label", "com.docker.compose.project="+m.DockerComposeProjectName())
	containerFilters.Add("label", "com.docker.compose.service="+service)
	containerFilters.Add("label", "com.docker.compose.container-number=1")
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
func (m MWDD) DockerExec(command DockerExecCommand) {
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
		return
	}

	execID := response.ID
	if execID == "" {
		fmt.Println("exec ID empty")
		return
	}

	execStartCheck := types.ExecStartCheck{
		Tty: true,
	}

	waiter, err := cli.ContainerExecAttach(ctx, execID, execStartCheck)
	if err != nil {
		fmt.Println(err)
		return
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
	var oldState *terminal.State
	if terminal.IsTerminal(fd) {
		oldState, _ = terminal.MakeRaw(fd)
		defer terminal.Restore(fd, oldState)
	}

	for {
		resp, err := cli.ContainerExecInspect(ctx, execID)
		time.Sleep(50 * time.Millisecond)
		if err != nil {
			break
		}

		if !resp.Running {
			break
		}
	}
}

/*DockerRun runs a docker container using the docker SDK attached to the mwdd network etc...*/
func (m MWDD) DockerRun(command DockerExecCommand) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Println("Unable to create docker client")
		panic(err)
	}

	containerID := m.containerID(ctx, cli, command.DockerComposeService)

	containerJSON, _ := cli.ContainerInspect(context.Background(), containerID)
	containerConfig := containerJSON.Config

	containerConfig.AttachStderr = true
	containerConfig.AttachStdout = true
	containerConfig.AttachStdin = true
	containerConfig.OpenStdin = true
	containerConfig.Tty = true
	containerConfig.WorkingDir = command.WorkingDir
	containerConfig.User = command.User
	containerConfig.Entrypoint = []string{command.Command[0]}
	containerConfig.Cmd = command.Command[1:]
	// Keep the origional docker-compose env vars from the container we are copying, and add any users supplied ones
	// This is probably britle, and if we ever stop copying a container, and removing the old one etc we could perhaps do something else?
	// https://phabricator.wikimedia.org/T294181
	containerConfig.Env = append(containerConfig.Env, command.Env...)

	// Remove the old one and start a new one with new options :)
	cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})

	resp, err := cli.ContainerCreate(
		ctx,
		containerConfig,
		containerJSON.HostConfig,
		&network.NetworkingConfig{
			EndpointsConfig: containerJSON.NetworkSettings.Networks,
		},
		nil,
		containerID,
	)
	if err != nil {
		panic(err)
	}

	waiter, err := cli.ContainerAttach(ctx, resp.ID, types.ContainerAttachOptions{
		Stream: true,
		Stdin:  containerConfig.AttachStdin,
		Stdout: containerConfig.AttachStdout,
		Stderr: containerConfig.AttachStderr,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	if containerConfig.Tty {
		if err := monitorTtySize(ctx, cli, resp.ID, false); err != nil {
			fmt.Println("Error monitoring TTY size:")
			fmt.Println(err)
		}
	}

	cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})

	// When TTY is ON, just copy stdout https://phabricator.wikimedia.org/T282340
	// See: https://github.com/docker/cli/blob/70a00157f161b109be77cd4f30ce0662bfe8cc32/cli/command/container/hijack.go#L121-L130
	go io.Copy(os.Stdout, waiter.Reader)
	go io.Copy(waiter.Conn, os.Stdin)

	fd := int(os.Stdin.Fd())
	var oldState *terminal.State
	if terminal.IsTerminal(fd) {
		oldState, _ = terminal.MakeRaw(fd)
		defer terminal.Restore(fd, oldState)
	}

	for {
		resp, err := cli.ContainerInspect(ctx, resp.ID)
		time.Sleep(50 * time.Millisecond)
		if err != nil {
			break
		}

		if !resp.State.Running {
			break
		}
	}
}

// MonitorTtySize updates the container tty size when the terminal tty changes size.
func monitorTtySize(ctx context.Context, client client.APIClient, id string, isExec bool) error {
	// Source: https://github.com/skiffos/skiff-core/blob/82c430e4961453c250883c2e5ebd4bd360fa13a5/shell/tty.go
	resizeTty := func() {
		width, height, _ := terminal.GetSize(0)
		resizeTtyTo(ctx, client, id, uint(height), uint(width), isExec)
	}

	resizeTty()

	sigchan := make(chan os.Signal, 1)
	gosignal.Notify(sigchan, signal.SIGWINCH)
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
