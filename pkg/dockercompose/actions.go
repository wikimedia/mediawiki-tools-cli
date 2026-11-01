package dockercompose

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	// TODO this shoud only use pkgs.
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

type UpOptions struct {
	Detached      bool
	ForceRecreate bool
	Build         bool
}

func (p Project) Up(services []string, opts UpOptions) error {
	args := []string{}
	if opts.Detached {
		args = append(args, "-d")
	}
	if opts.ForceRecreate {
		args = append(args, "--force-recreate")
	}
	if opts.Build {
		args = append(args, "--build")
	}
	args = append(args, services...)
	return p.Command(append([]string{"up"}, args...)).RunAttached()
}

type DownOptions struct {
	Volumes       bool
	RemoveOrphans bool
	Timeout       int
}

func (p Project) Down(opts DownOptions) error {
	args := []string{}
	if opts.Volumes {
		args = append(args, "--volumes")
	}
	if opts.RemoveOrphans {
		args = append(args, "--remove-orphans")
	}
	if opts.Timeout != 0 {
		args = append(args, "--timeout", strconv.Itoa(opts.Timeout))
	}
	return p.Command(append([]string{"down"}, args...)).RunAttached()
}

type ExecOptions struct {
	User                string
	EnableTTYAllocation bool
	CommandAndArgs      []string
}

func (p Project) ExecCommand(service string, opts ExecOptions) Command {
	args := []string{}
	if !opts.EnableTTYAllocation {
		args = append(args, "-T")
	}
	if opts.User != "" {
		args = append(args, "--user", opts.User)
	}
	args = append(args, service)
	args = append(args, opts.CommandAndArgs...)

	return p.Command(append([]string{"exec"}, args...))
}

func (p Project) Exec(service string, opts ExecOptions) error {
	return p.ExecCommand(service, opts).RunAttached()
}

func (p Project) Start(services []string) error {
	return p.Command(append([]string{"start"}, services...)).RunAttached()
}

// StartContainers starts existing containers for the selected services without
// allowing Compose to start their dependencies.
func (p Project) StartContainers(services []string) error {
	return p.StartContainersWithOutput(services, os.Stdout, os.Stderr)
}

// StartContainersWithOutput reports service names instead of Docker's raw IDs.
func (p Project) StartContainersWithOutput(services []string, stdout, stderr io.Writer) error {
	if len(services) == 0 {
		return nil
	}
	listed, listErrors, err := p.Command(append([]string{"ps", "--all", "--quiet"}, services...)).RunAndCollect()
	if err != nil {
		return fmt.Errorf("list containers to start for %s: %w: %s", strings.Join(services, ", "), err, strings.TrimSpace(listErrors.String()))
	}
	ids := strings.Fields(listed.String())
	if len(ids) == 0 {
		return nil
	}
	// Inspect only the selected containers. Compose start can also resume their
	// dependencies, so keep using docker container start with these exact IDs.
	inspectArgs := append([]string{"container", "inspect", "--format", `{{.Id}} {{index .Config.Labels "com.docker.compose.service"}} {{.Name}}`}, ids...)
	labels, inspectErrors, err := (Command{Cmd: exec.Command("docker", inspectArgs...)}).RunAndCollect() // #nosec G204
	if err != nil {
		return fmt.Errorf("identify containers to start for %s: %w: %s", strings.Join(services, ", "), err, strings.TrimSpace(inspectErrors.String()))
	}
	containerServices := map[string]string{}
	containerNames := map[string]string{}
	for _, line := range strings.Split(labels.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			for _, id := range ids {
				if strings.HasPrefix(fields[0], id) {
					containerServices[id] = fields[1]
					containerNames[id] = fields[1]
					if len(fields) >= 3 {
						containerNames[id] = strings.TrimPrefix(fields[2], "/")
					}
				}
			}
		}
	}
	for _, id := range ids {
		if containerServices[id] == "" {
			return fmt.Errorf("could not identify the service for a selected container of %s; no containers started", strings.Join(services, ", "))
		}
	}
	names := startedServiceNames(ids, containerServices)
	progress := newContainerStartProgress(stdout, stderr, ids, containerNames)
	if progress == nil {
		fmt.Fprintf(stderr, "Starting: %s\n", strings.Join(names, ", "))
	}
	started := containerStartOutput{completed: progress.complete}
	var startErrors bytes.Buffer
	err = (Command{Cmd: exec.Command("docker", append([]string{"container", "start"}, ids...)...)}).RunAttachedWithOutput(&started, &startErrors) // #nosec G204
	started.flush()
	progress.finish(err)
	if err == nil {
		fmt.Fprint(stderr, startErrors.String())
		if progress == nil {
			fmt.Fprintf(stdout, "Started: %s\n", strings.Join(names, ", "))
		}
		return nil
	}
	// Docker prints each successfully started ID even when another start fails.
	// A service is complete only when all of its selected replicas started.
	successful := map[string]bool{}
	for _, id := range strings.Fields(started.output.String()) {
		successful[id] = true
	}
	var failedIDs []string
	for _, id := range ids {
		if !successful[id] {
			failedIDs = append(failedIDs, id)
		}
	}
	failed := startedServiceNames(failedIDs, containerServices)
	failedServices := map[string]bool{}
	for _, name := range failed {
		failedServices[name] = true
	}
	var completed []string
	for _, name := range names {
		if !failedServices[name] {
			completed = append(completed, name)
		}
	}
	if progress == nil && len(completed) > 0 {
		fmt.Fprintf(stdout, "Started: %s\n", strings.Join(completed, ", "))
	}
	if len(failed) == 0 {
		failed = names
	}
	return fmt.Errorf("start services %s: %w: %s", strings.Join(failed, ", "), err, strings.TrimSpace(startErrors.String()))
}

func startedServiceNames(ids []string, services map[string]string) []string {
	seen := map[string]bool{}
	for _, id := range ids {
		seen[services[id]] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (p Project) Stop(services []string) error {
	return p.Command(append([]string{"stop"}, services...)).RunAttached()
}

func (p Project) Restart(services []string) error {
	return p.Command(append([]string{"restart"}, services...)).RunAttached()
}

func (p Project) Pull(services []string) error {
	return p.Command(append([]string{"pull"}, services...)).RunAttached()
}

type RmOptions struct {
	Stop                   bool
	Force                  bool
	RemoveAnonymousVolumes bool
}

func (p Project) Rm(services []string, opts RmOptions) error {
	args := []string{}
	if opts.Stop {
		args = append(args, "--stop")
	}
	if opts.Force {
		args = append(args, "--force")
	}
	if opts.RemoveAnonymousVolumes {
		args = append(args, "-v")
	}
	args = append(args, services...)
	return p.Command(append([]string{"rm"}, args...)).RunAttached()
}

func (p Project) VolumesRm(volumes []string) error {
	dockerVolumes := []string{}
	for _, dcVolume := range volumes {
		dockerVolumes = append(dockerVolumes, p.Name+"_"+dcVolume)
	}
	cmd := Command{
		Cmd: exec.Command("docker", append([]string{"volume", "rm"}, dockerVolumes...)...), // #nosec G204
	}
	return cmd.RunAttached()
}

/*ServicesWithStatus lists services in the docker compose setup that have the given status.*/
func (p Project) ServicesWithStatus(statusFilter string) ([]string, error) {
	stdout, stderr, err := p.Command([]string{"ps", "--services", "--filter", "status=" + statusFilter}).RunAndCollect()

	serviceList := stringsutil.SplitMultiline(strings.Trim(stdout.String(), "\n"))
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return nil, fmt.Errorf("docker compose ps: %w: %s", err, message)
		}
		return nil, fmt.Errorf("docker compose ps: %w", err)
	}
	return serviceList, nil
}
