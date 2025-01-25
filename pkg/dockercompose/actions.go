package dockercompose

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	// TODO this shoud only use pkgs.
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

type UpOptions struct {
	Detached      bool
	ForceRecreate bool
}

func (p Project) Up(services []string, opts UpOptions) error {
	args := []string{}
	if opts.Detached {
		args = append(args, "-d")
	}
	if opts.ForceRecreate {
		args = append(args, "--force-recreate")
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
	if stderr.String() != "" || err != nil {
		return nil, fmt.Errorf("%s", stderr.String())
	}
	return serviceList, nil
}
