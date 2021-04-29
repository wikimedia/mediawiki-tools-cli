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
	"time"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/crypto/ssh/terminal"
)

// DockerExecCommand to be run with Docker, which directly uses the docker SDK
type DockerExecCommand struct {
	DockerComposeService      string
	Command      []string
	HandlerOptions exec.HandlerOptions
}

/*DockerExec runs a docker exec command using the docker SDK*/
func (m MWDD) DockerExec( command DockerExecCommand ) {
	containerID := m.DockerComposeProjectName() + "_" + command.DockerComposeService + "_1"

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Println("Unable to create docker client")
		panic(err)
	}

	config :=  types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		AttachStdin: true,
		Tty: true,
		Cmd: command.Command,
	}

	ctx := context.Background()
	response, err := cli.ContainerExecCreate(ctx, containerID, config)
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

	go io.Copy(os.Stdout, waiter.Reader)
	go io.Copy(os.Stderr, waiter.Reader)
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
