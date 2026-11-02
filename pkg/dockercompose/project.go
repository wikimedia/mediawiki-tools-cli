package dockercompose

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moby/moby/client"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
)

type Project struct {
	Name      string
	Directory string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
}

// WithIO returns a project whose attached commands use the supplied streams.
func (p Project) WithIO(stdin io.Reader, stdout, stderr io.Writer) Project {
	p.Stdin, p.Stdout, p.Stderr = stdin, stdout, stderr
	return p
}

func (p Project) Command(commandAndArgs []string) Command {
	cmd, err := p.Cmd(commandAndArgs)
	return Command{
		Cmd:    cmd,
		err:    err,
		stdin:  p.Stdin,
		stdout: p.Stdout,
		stderr: p.Stderr,
	}
}

// TODO don't use this externally, use Command?
func (p Project) Cmd(commandAndArgs []string) (*exec.Cmd, error) {
	args, err := p.argsForExec(commandAndArgs)
	if err != nil {
		return nil, err
	}
	// If we have docker and a compose sub command, we can run it
	if _, err := lookpath.NeedCommands([]string{"docker compose"}); err == nil {
		return exec.Command("docker", append([]string{"compose"}, args...)...), nil // #nosec G204
	}
	// If we have docker-compose, we can run it
	if _, err := lookpath.NeedExecutables([]string{"docker-compose"}); err == nil {
		return exec.Command("docker-compose", args...), nil // #nosec G204
	}
	return nil, fmt.Errorf("neither docker compose nor docker-compose is available in PATH")
}

func (p Project) argsForExec(commandAndArgs []string) ([]string, error) {
	args := []string{
		"--project-name", p.Name,
		"--project-directory", p.Directory,
	}
	composeFiles, composeFilesErr := p.ComposeFiles()
	if composeFilesErr != nil {
		return nil, fmt.Errorf("discover Compose files in %s: %w", p.Directory, composeFilesErr)
	}
	// Position custom files last, so that they can override defaults
	var lastArgs []string
	for _, fileName := range composeFiles {
		if strings.HasPrefix(fileName, "custom-") || strings.HasPrefix(fileName, "custom.") {
			lastArgs = append(lastArgs, "--file", p.Directory+"/"+fileName)
		} else {
			args = append(args, "--file", p.Directory+"/"+fileName)
		}
	}
	args = append(args, lastArgs...)

	args = append(args, commandAndArgs...)
	return args, nil
}

func (p Project) ComposeFiles() ([]string, error) {
	composeFiles := []string{}

	composeDir := filepath.Join(p.Directory, "compose")
	if stat, statErr := os.Stat(composeDir); statErr == nil && stat.IsDir() {
		walkErr := filepath.Walk(composeDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Base(path), "compose.yml") || strings.EqualFold(filepath.Base(path), "compose.yaml") {
				rel, relErr := filepath.Rel(p.Directory, path)
				if relErr != nil {
					return relErr
				}
				composeFiles = append(composeFiles, filepath.ToSlash(rel))
			}
			return nil
		})
		if walkErr != nil {
			return composeFiles, walkErr
		}
	}

	// Legacy support for custom override files in the project root.
	files, filesErr := p.TopLevelFilePaths()
	if filesErr != nil {
		return composeFiles, filesErr
	}

	for _, file := range files {
		fileBase := filepath.Base(file)
		if strings.HasPrefix(fileBase, "custom-") || strings.HasPrefix(fileBase, "custom.") {
			fileExt := filepath.Ext(file)
			if fileExt == ".yml" || fileExt == ".yaml" {
				composeFiles = append(composeFiles, fileBase)
			}
		}
	}

	sort.Strings(composeFiles)
	return composeFiles, nil
}

func (p Project) TopLevelFilePaths() ([]string, error) {
	var files []string

	err := filepath.Walk(p.Directory, func(path string, info os.FileInfo, err error) error {
		pathMinusDir := strings.TrimPrefix(path, p.Directory+"/")
		slog.Debug(fmt.Sprint("Checking path: " + path + " with pathMinusDir: " + pathMinusDir))
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
	cli, err := docker.NewClientFromEnv()
	if err != nil {
		return "", err
	}
	defer cli.Close()
	ctx := context.Background()

	containerFilters := client.Filters{}
	projectNameLC := strings.ToLower(p.Name)
	containerFilters = containerFilters.Add("label", "com.docker.compose.project="+projectNameLC)
	containerFilters = containerFilters.Add("label", "com.docker.compose.service="+strings.ToLower(service))
	// Only ever retrieve the first container (all mwcli needs)
	containerFilters = containerFilters.Add("label", "com.docker.compose.container-number=1")
	slog.Debug(fmt.Sprint("Getting container ID for service: " + service + " using filters filters: project = " + projectNameLC + ", service = " + service))

	containers, err := cli.ContainerList(ctx, client.ContainerListOptions{Filters: containerFilters})
	if err != nil {
		return "", err
	}
	if len(containers.Items) == 0 {
		return "", fmt.Errorf("unable to execute command, no container found for service: %s", service)
	}
	if len(containers.Items) > 1 {
		return "", fmt.Errorf("more than one container found for service: %s", service)
	}
	return containers.Items[0].ID, nil
}
