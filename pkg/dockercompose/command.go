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
	err := c.run()
	return err
}

func (c Command) RunAndCollect() (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	c.Cmd.Stdout = &stdout
	c.Cmd.Stderr = &stderr
	err = c.run()
	return stdout, stderr, err
}

func (c Command) run() error {
	c.logRun()
	if isLinux() && isArm() && !isDockerDefaultPlatformDefined() {
		// If we are a linux arm machine, we need to force the default platform to linux/amd64
		// As we don't have arm images for all services https://phabricator.wikimedia.org/T355341
		logrus.Trace("Forcing DOCKER_DEFAULT_PLATFORM to linux/amd64")
		c.Cmd.Env = append(c.Cmd.Env, "DOCKER_DEFAULT_PLATFORM=linux/amd64")
	}
	return c.Cmd.Run()
}

func isDockerDefaultPlatformDefined() bool {
	_, ok := os.LookupEnv("DOCKER_DEFAULT_PLATFORM")
	return ok
}

func isLinux() bool {
	return os.Getenv("GOOS") == "linux"
}

func isArm() bool {
	arch := os.Getenv("GOARCH")
	return arch == "arm" || arch == "arm64"
}
