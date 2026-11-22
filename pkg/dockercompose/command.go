package dockercompose

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
)

type Command struct {
	Cmd    *exec.Cmd
	err    error
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

var (
	runtimeGOOS   = runtime.GOOS
	runtimeGOARCH = runtime.GOARCH
)

func (c Command) logRun() {
	slog.Debug(fmt.Sprint("Running docker compose (arguments omitted)"))
}

func (c Command) RunAttached() error {
	stdin, stdout, stderr := c.stdin, c.stdout, c.stderr
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return c.RunAttachedWithIO(stdin, stdout, stderr)
}

// RunAttachedWithOutput streams output to the supplied writers and attaches stdin.
func (c Command) RunAttachedWithOutput(stdout, stderr io.Writer) error {
	stdin := c.stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	return c.RunAttachedWithIO(stdin, stdout, stderr)
}

// RunAttachedWithIO attaches the supplied input and output streams.
func (c Command) RunAttachedWithIO(stdin io.Reader, stdout, stderr io.Writer) error {
	if c.err != nil {
		return c.err
	}
	c.Cmd.Stdout = stdout
	c.Cmd.Stdin = stdin
	c.Cmd.Stderr = stderr
	err := c.run()
	return err
}

func (c Command) RunAndCollect() (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	if c.err != nil {
		return stdout, stderr, c.err
	}
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
		slog.Debug(fmt.Sprint("Forcing DOCKER_DEFAULT_PLATFORM to linux/amd64"))
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
