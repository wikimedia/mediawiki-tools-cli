/*Package exec is used for executing commands

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
package exec

import (
	"bytes"
	"fmt"
	"github.com/briandowns/spinner"
	"os"
	"os/exec"
	"path/filepath"
)

/*Command passes through to exec.Command for running generic commands*/
func Command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

/*DockerComposeCommand gets a docker-compose command to run*/
func DockerComposeCommand(command string, arg ...string) *exec.Cmd {
	projectDir, _ := os.Getwd()
	projectName := "mw-" + filepath.Base(projectDir)
	arg = append([]string{"-p", projectName, command}, arg...)
	return exec.Command("docker-compose", arg...)
}

/*RunCommand runs a command, handles verbose output and errors, and an optional spinner*/
func RunCommand(verbosity int, cmd *exec.Cmd, s *spinner.Spinner) (bytes.Buffer, bytes.Buffer, error) {
	if s != nil {
		s.Start()
	}
	stdout, stderr, err := runCommand(verbosity, cmd)
	if s != nil {
		s.Stop()
	}
	handleCommandRun(verbosity, cmd, stdout, stderr, err)
	return stdout, stderr, err
}

func runCommand(verbosity int, cmd *exec.Cmd) (bytes.Buffer, bytes.Buffer, error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	return stdoutBuf, stderrBuf, err
}

func handleCommandRun(verbosity int, cmd *exec.Cmd, stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	if verbosity >= 1 {
		fmt.Printf("\n%s\n", cmd.String())
	}
	if verbosity >= 3 && stdout.String() != "" {
		fmt.Printf("\n%s\n%s\n", "STDOUT:", stdout.String())
	}
	if err != nil {
		if verbosity >= 2 && stderr.String() != "" {
			fmt.Printf("\n%s\n%s\n", "STDERR:", stderr.String())
		}
	}
}
