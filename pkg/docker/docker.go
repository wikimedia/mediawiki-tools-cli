package docker

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

func NewClientFromEnvOrPanic() *client.Client {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logrus.Error("Unable to create docker client")
		logrus.Panic(err)
	}
	return cli
}

func DockerDaemonIsRunning() bool {
	cli := NewClientFromEnvOrPanic()
	_, err := cli.Ping(context.Background())
	return err == nil
}

/*CurrentUserAndGroupForDockerExecution gets a user and group id combination for the current user that can be used for execution.*/
func CurrentUserAndGroupForDockerExecution() string {
	if runtime.GOOS == "windows" {
		// TODO confirm that just using 2000 will always work on Windows?
		// This user won't exist, but that fact doesn't really matter on pure Windows
		return "2000:2000"
	}
	return fmt.Sprint(os.Getuid(), ":", os.Getgid())
}
