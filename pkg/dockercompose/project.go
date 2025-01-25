package dockercompose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
)

type Project struct {
	Name      string
	Directory string
}

func (p Project) Command(commandAndArgs []string) Command {
	return Command{
		Cmd: p.Cmd(commandAndArgs),
	}
}

// TODO don't use this externally, use Command?
func (p Project) Cmd(commandAndArgs []string) *exec.Cmd {
	// If we have docker and a compose sub command, we can run it
	if _, err := lookpath.NeedCommands([]string{"docker compose"}); err == nil {
		return exec.Command("docker", append([]string{"compose"}, p.argsForExec(commandAndArgs)...)...) // #nosec G204
	}
	// If we have docker-compose, we can run it
	if _, err := lookpath.NeedExecutables([]string{"docker-compose"}); err == nil {
		return exec.Command("docker-compose", p.argsForExec(commandAndArgs)...) // #nosec G204
	}
	// Otherwise we have no option (but this should already be checked by callers)
	panic("No docker-compose or docker compose found in PATH")
}

func (p Project) argsForExec(commandAndArgs []string) []string {
	args := []string{
		"--project-name", p.Name,
		"--project-directory", p.Directory,
	}
	composeFiles, composeFilesErr := p.ComposeFiles()
	if composeFilesErr != nil {
		// TODO bubble this error up?
		panic(composeFilesErr)
	}
	for _, fileName := range composeFiles {
		args = append(args, "--file", p.Directory+"/"+fileName)
	}

	args = append(args, commandAndArgs...)
	return args
}

func (p Project) ComposeFiles() ([]string, error) {
	var composeFiles []string
	files, filesErr := p.TopLevelFilePaths()

	for _, file := range files {
		fileExt := filepath.Ext(file)
		if fileExt == ".yml" || fileExt == ".yaml" {
			composeFiles = append(composeFiles, filepath.Base(file))
		}
	}

	return composeFiles, filesErr
}

func (p Project) TopLevelFilePaths() ([]string, error) {
	var files []string

	err := filepath.Walk(p.Directory, func(path string, info os.FileInfo, err error) error {
		pathMinusDir := strings.TrimPrefix(path, p.Directory+"/")
		logrus.Trace("Checking path: " + path + " with pathMinusDir: " + pathMinusDir)
		if !info.IsDir() && !strings.Contains(pathMinusDir, "/") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func (p Project) NetworkName(name string) string {
	return strings.ToLower(p.Name) + "_" + name
}

func (p Project) ContainerID(service string) (string, error) {
	cli := docker.NewClientFromEnvOrPanic()
	ctx := context.Background()

	containerFilters := filters.NewArgs()
	projectNameLC := strings.ToLower(p.Name)
	containerFilters.Add("label", "com.docker.compose.project="+projectNameLC)
	containerFilters.Add("label", "com.docker.compose.service="+strings.ToLower(service))
	// Only ever retrieve the first container (all mwcli needs)
	containerFilters.Add("label", "com.docker.compose.container-number=1")
	logrus.Trace("Getting container ID for service: " + service + " using filters filters: project = " + projectNameLC + ", service = " + service)

	containers, err := cli.ContainerList(ctx, container.ListOptions{Filters: containerFilters})
	if err != nil {
		return "", err
	}
	if len(containers) == 0 {
		return "", fmt.Errorf("unable to execute command, no container found for service: %s", service)
	}
	if len(containers) > 1 {
		return "", fmt.Errorf("more than one container found for service: %s", service)
	}
	return containers[0].ID, nil
}
