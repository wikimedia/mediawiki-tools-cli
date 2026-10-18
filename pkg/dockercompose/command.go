package dockercompose

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/sirupsen/logrus"
)

type Command struct {
	Cmd *exec.Cmd
}

var (
	runtimeGOOS   = runtime.GOOS
	runtimeGOARCH = runtime.GOARCH
)

func (c Command) logRun() {
	logrus.Trace(c.Cmd.String())
}

func (c Command) RunAttached() error {
	return c.RunAttachedWithOutput(os.Stdout, os.Stderr)
}

// RunAttachedWithOutput streams output to the supplied writers and attaches stdin.
func (c Command) RunAttachedWithOutput(stdout, stderr io.Writer) error {
	c.Cmd.Stdout = stdout
	c.Cmd.Stdin = os.Stdin
	c.Cmd.Stderr = stderr
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
	if shouldForceDockerDefaultPlatform() {
		// If we are a linux arm machine, we need to force the default platform to linux/amd64
		// As we don't have arm images for all services https://phabricator.wikimedia.org/T355341
		logrus.Trace("Forcing DOCKER_DEFAULT_PLATFORM to linux/amd64")
		c.Cmd.Env = append(c.Cmd.Env, "DOCKER_DEFAULT_PLATFORM=linux/amd64")
	}
	return c.Cmd.Run()
}

func shouldForceDockerDefaultPlatform() bool {
	return isLinux() && isArm() && !isDockerDefaultPlatformDefined()
}

func isDockerDefaultPlatformDefined() bool {
	_, ok := os.LookupEnv("DOCKER_DEFAULT_PLATFORM")
	return ok
}

func isLinux() bool {
	return runtimeGOOS == "linux"
}

func isArm() bool {
	arch := runtimeGOARCH
	return arch == "arm" || arch == "arm64"
}
