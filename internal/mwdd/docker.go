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
	"context"
	"fmt"
	"io"
	"os"
	gosignal "os/signal"
	"runtime"
	"strings"
	"time"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/signal"
	"golang.org/x/crypto/ssh/terminal"
)

// DockerExecCommand to be run with Docker, which directly uses the docker SDK
type DockerExecCommand struct {
	DockerComposeService string
	Command              []string
	WorkingDir           string
	User                 string
	HandlerOptions       exec.HandlerOptions
}

/*UserAndGroupForDockerExecution gets a user and group id combination for the current user that can be used for execution*/
func UserAndGroupForDockerExecution() string {
	if runtime.GOOS == "windows" {
		// TODO confirm that just using 2000 will always work on Windows?
		// This user won't exist, but that fact doesn't really matter on pure Windows
		return "2000:2000"
	}
	return fmt.Sprint(os.Getuid(), ":", os.Getgid())
}

/*DockerExec runs a docker exec command using the docker SDK*/
func (m MWDD) DockerExec(command DockerExecCommand) {
	containerID := m.DockerComposeProjectName() + "_" + command.DockerComposeService + "_1"

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Println("Unable to create docker client")
		panic(err)
	}

	execConfig := types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		AttachStdin:  true,
		Tty:          true,
		WorkingDir:   command.WorkingDir,
		User:         command.User,
		Cmd:          []string{"/bin/sh", "-c", strings.Join(command.Command, " ")},
	}

	ctx := context.Background()
	response, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
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
		oldState, err = terminal.MakeRaw(fd)
		if err != nil {
			// print error
		}
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

// MonitorTtySize updates the container tty size when the terminal tty changes size
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
