package devenv

import (
	"os"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv/files"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

// Environment is a development environment.
type Environment struct {
	BaseDir string
}

var Context string

// DefaultForUser returns the development environment for the current user.
func DefaultForUser() Environment {
	return Environment{
		BaseDir: devUserDirectory(),
	}
}

func devUserDirectory() string {
	// TODO: Migrate the legacy storage directory while preserving existing contexts.
	return cli.UserDirectoryPathForCmd("mwdd")
}

/*Directory the directory containing the development environment.*/
func (m Environment) Directory() string {
	return m.BaseDir + string(os.PathSeparator) + Context
}

/*EnsureReady ...*/
func (m Environment) EnsureReady() {
	files.EnsureReady(m.Directory())
	m.Env().EnsureExists()
	m.Env().Set("TRAEFIK_DOCKER_NETWORK", m.DockerCompose().NetworkName("dps"))
	m.Env().Set("TRAEFIK_DOCKER_PROJECT", strings.ToLower(m.DockerCompose().Name))
	m.ensureDockerfileComposeFiles()
	if err := m.refreshDashboardServices(); err != nil {
		logrus.Warnf("Could not refresh custom dashboard services: %v", err)
	}
}

func (m Environment) DockerCompose() dockercompose.Project {
	return dockercompose.Project{
		// Keep the existing project identity to reuse containers, networks and volumes.
		Name:      "mwcli-mwdd-" + Context,
		Directory: m.Directory(),
	}
}

/*CommandAndEnvFromArgs takes arguments passed to a cobra command and extracts any prefixing env var definitions from them.*/
func CommandAndEnvFromArgs(args []string) ([]string, []string) {
	extractedArgs := []string{}
	extractedEnvs := []string{}
	regex, _ := regexp.Compile(`^\w+=\w+$`)
	for _, arg := range args {
		matched := regex.MatchString(arg)
		if matched {
			extractedEnvs = append(extractedEnvs, arg)
		} else {
			extractedArgs = append(extractedArgs, arg)
		}
	}
	return extractedArgs, extractedEnvs
}

func (m Environment) dockerNetworkName() string {
	// Default network is always dps...
	return m.DockerCompose().NetworkName("dps")
}
