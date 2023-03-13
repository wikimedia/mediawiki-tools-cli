package dockercompose

import (
	"bytes"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
)

type Command struct {
	Cmd *exec.Cmd
}

func (c Command) logRun() {
	// TODO also log method name of caller?
	logrus.Trace(c.Cmd.String())
}

func (c Command) RunAttached() error {
	c.Cmd.Stdout = os.Stdout
	c.Cmd.Stdin = os.Stdin
	c.Cmd.Stderr = os.Stderr
	c.logRun()
	err := c.Cmd.Run()
	return err
}

func (c Command) RunAndCollect() (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	c.Cmd.Stdout = &stdout
	c.Cmd.Stderr = &stderr
	c.logRun()
	err = c.Cmd.Run()
	return stdout, stderr, err
}
