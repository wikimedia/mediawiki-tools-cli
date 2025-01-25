package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	ossignal "os/signal"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/moby/sys/signal"
	"golang.org/x/term"
)

type ExecOptions struct {
	Command    []string
	Env        []string
	WorkingDir string
	User       string
}

func Exec(containerID string, command ExecOptions) (ExitCode int) {
	ctx := context.Background()
	cli := NewClientFromEnvOrPanic()

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

	options := container.ResizeOptions{
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
