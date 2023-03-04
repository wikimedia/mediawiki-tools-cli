package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
)

func newClient() *client.Client {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Println("Unable to create docker client")
		panic(err)
	}
	return cli
}

func DockerIsRunning() bool {
	cli := newClient()
	_, err := cli.Ping(context.Background())
	return err == nil
}
