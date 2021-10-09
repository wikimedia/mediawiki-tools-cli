/*Package cmd in internal utils is functionality for interacting with exec.Cmd

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
package cmd

import (
	"bytes"
	"os"
	"os/exec"
)

/*AttachOutputBuffer ...*/
func AttachOutputBuffer(cmd *exec.Cmd) *bytes.Buffer {
	var outb bytes.Buffer
	cmd.Stdout = &outb
	return &outb
}

/*AttachInErrIO ...*/
func AttachInErrIO(cmd *exec.Cmd) *exec.Cmd {
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd
}

/*AttachAllIO ...*/
func AttachAllIO(cmd *exec.Cmd) *exec.Cmd {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
