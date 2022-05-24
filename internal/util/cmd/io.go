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

/*AttachAllOutputBuffer ...*/
func AttachAllOutputBuffer(cmd *exec.Cmd) *bytes.Buffer {
	var outb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &outb
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
