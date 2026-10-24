package docker

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	dockerconfig "github.com/docker/cli/cli/config"
	contextdocker "github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/context/store"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

// NewClientFromEnv uses Docker's own endpoint resolution, including the saved
// currentContext, DOCKER_CONTEXT, DOCKER_HOST, TLS material and SSH helpers.
func NewClientFromEnv() (*client.Client, error) {
	cfg, err := dockerconfig.Load(dockerconfig.Dir())
	if err != nil {
		return nil, fmt.Errorf("load Docker configuration: %w", err)
	}
	// Match Docker CLI v27's resolver: DOCKER_HOST takes precedence over
	// DOCKER_CONTEXT and the saved current context.
	name := "default"
	if os.Getenv("DOCKER_HOST") == "" {
		name = os.Getenv("DOCKER_CONTEXT")
		if name == "" {
			name = cfg.CurrentContext
		}
	}
	if name == "" || name == "default" {
		return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation(), client.WithUserAgent(cli.HTTPUserAgent))
	}
	contexts := store.New(dockerconfig.ContextStoreDir(), dockerContextStoreConfig())
	metadata, err := contexts.GetMetadata(name)
	if err != nil {
		return nil, fmt.Errorf("load Docker context %q: %w", name, err)
	}
	endpointMeta, err := contextdocker.EndpointFromContext(metadata)
	if err != nil {
		return nil, fmt.Errorf("Docker context %q: %w", name, err)
	}
	endpoint, err := contextdocker.WithTLSData(contexts, name, endpointMeta)
	if err != nil {
		return nil, fmt.Errorf("Docker context %q TLS: %w", name, err)
	}
	opts, err := endpoint.ClientOpts()
	if err != nil {
		return nil, fmt.Errorf("Docker context %q client: %w", name, err)
	}
	return client.NewClientWithOpts(append(opts, client.WithUserAgent(cli.HTTPUserAgent))...)
}

func dockerContextStoreConfig() store.Config {
	return store.NewConfig(func() any { return new(map[string]any) },
		store.EndpointTypeGetter(contextdocker.DockerEndpoint, func() any { return new(contextdocker.EndpointMeta) }))
}

func NewClientFromEnvOrPanic() *client.Client {
	cli, err := NewClientFromEnv()
	if err != nil {
		logrus.Panicf("Unable to create Docker client: %v", err)
	}
	return cli
}

// CheckDockerDaemon retains the real endpoint/permission error for diagnostics.
func CheckDockerDaemon() error {
	cli, err := NewClientFromEnv()
	if err != nil {
		return err
	}
	defer cli.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		return fmt.Errorf("cannot connect to Docker at %s: %w", cli.DaemonHost(), err)
	}
	return nil
}

func DockerDaemonIsRunning() bool {
	return CheckDockerDaemon() == nil
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
