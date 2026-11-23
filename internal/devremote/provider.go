package devremote

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ProviderTarget identifies a machine independently of its current SSH connection.
type ProviderTarget struct {
	Kind          string
	ID            string
	Project       string
	Region        string
	User          string
	Jump          string
	Owned         bool
	Identity      string
	WaitCloudInit bool
}

type Machine struct {
	ID       string
	Name     string
	State    string
	Region   string
	LastUsed time.Time
}

func providerOutput(ctx context.Context, stderr io.Writer, program string, args ...string) ([]byte, error) {
	executable, err := osexec.LookPath(program)
	if err != nil {
		return nil, fmt.Errorf("install and sign in to %s: %w", program, err)
	}
	cmd := osexec.CommandContext(ctx, executable, args...) // #nosec G204 -- provider executable resolved on PATH; arguments passed directly.
	cmd.Stderr = stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", program, strings.Join(args[:2], " "), err)
	}
	return output, nil
}

func CreateProviderMachine(ctx context.Context, provider, name, repo, size, recipe string, idle time.Duration, stderr io.Writer) (ProviderTarget, error) {
	p := ProviderTarget{Kind: provider, Owned: true}
	if !limaInstanceName.MatchString(name) {
		return p, fmt.Errorf("invalid machine name %q", name)
	}
	switch provider {
	case "lima":
		machines, err := ProviderMachines(ctx, p, stderr)
		if err != nil {
			return p, err
		}
		for _, machine := range machines {
			if machine.ID == name {
				return p, fmt.Errorf("Lima instance %q already exists; use remote open --target", name)
			}
		}
		if err := createLima(ctx, name, stderr); err != nil {
			return p, err
		}
		p.ID = name
	case "boxd":
		args := []string{"machine", "new", name, "--isolated", "--json"}
		if idle > 0 {
			args = append(args, "--auto-suspend-timeout", fmt.Sprint(int(idle.Seconds())))
		}
		data, err := providerOutput(ctx, stderr, "boxd", args...)
		if err != nil {
			return p, err
		}
		var machine struct{ ID, Name, Region string }
		if err := json.Unmarshal(data, &machine); err != nil {
			return p, fmt.Errorf("read Boxd creation result for %s: %w", name, err)
		}
		p.ID, p.Region, p.Identity = machine.Name, machine.Region, machine.ID
		if p.Identity == "" {
			details, err := boxdMachine(ctx, p, stderr)
			if err != nil || details.ID == "" {
				return p, fmt.Errorf("Boxd created %q but did not return its immutable machine ID", name)
			}
			p.Identity = details.ID
		}
	case "codespaces":
		args := codespaceCreateArgs(name, repo, size, recipe, idle)
		data, err := providerOutput(ctx, stderr, "gh", args...)
		if err != nil {
			return p, err
		}
		p.ID = strings.TrimSpace(string(data))
	default:
		return p, fmt.Errorf("unknown provider %q", provider)
	}
	if !limaInstanceName.MatchString(p.ID) {
		return p, fmt.Errorf("%s did not report a usable machine ID for %q", provider, name)
	}
	return p, nil
}

func codespaceCreateArgs(name, repo, size, recipe string, idle time.Duration) []string {
	const blankRepository = "github/codespaces-blank"
	if repo == "" {
		repo = blankRepository
	}
	if size == "" && repo == blankRepository {
		size = "basicLinux32gb"
		if recipe == farmRecipe {
			size = "premiumLinux"
		}
	}
	args := []string{"codespace", "create", "--repo=" + repo, "--display-name=" + name, "--default-permissions"}
	if size != "" {
		args = append(args, "--machine="+size)
	}
	if idle > 0 {
		args = append(args, "--idle-timeout="+idle.String())
	}
	return args
}

// ProviderSSH wakes a machine when needed and resolves its SSH transport.
func ProviderSSH(ctx context.Context, p ProviderTarget, name string, stdout, stderr io.Writer) ([]string, error) {
	if !limaInstanceName.MatchString(p.ID) {
		return nil, fmt.Errorf("invalid %s machine ID %q", p.Kind, p.ID)
	}
	switch p.Kind {
	case "lima":
		return LimaSSH(ctx, p.ID, false, stdout, stderr)
	case "boxd":
		machine, err := boxdMachine(ctx, p, stderr)
		if err != nil {
			return nil, err
		}
		if !limaInstanceName.MatchString(machine.Name) {
			return nil, fmt.Errorf("Boxd did not report a usable SSH machine name")
		}
		if strings.EqualFold(machine.Status, "stopped") {
			args := []string{"machine", "start", machine.ID}
			if machine.ID == "" {
				args[2] = p.ID
			}
			if machine.Region != "" {
				args = append(args, "--region="+machine.Region)
			}
			if _, err := providerOutput(ctx, stderr, "boxd", args...); err != nil {
				return nil, err
			}
		}
		return []string{"-o", "ForwardAgent=no", machine.Name + ".boxd"}, nil
	case "codespaces":
		return codespaceSSH(ctx, p.ID, name, stderr)
	default:
		return nil, fmt.Errorf("unknown provider %q", p.Kind)
	}
}

type boxdMachineDetails struct {
	ID, Name, Status, Region string
}

func boxdMachine(ctx context.Context, p ProviderTarget, stderr io.Writer) (boxdMachineDetails, error) {
	target := p.ID
	if p.Identity != "" {
		target = p.Identity
	}
	args := []string{"machine", "get", target, "--json"}
	if p.Region != "" {
		args = append(args, "--region="+p.Region)
	}
	var machine boxdMachineDetails
	data, err := providerOutput(ctx, stderr, "boxd", args...)
	if err != nil {
		return machine, err
	}
	err = json.Unmarshal(data, &machine)
	return machine, err
}

func codespaceSSH(ctx context.Context, id, name string, stderr io.Writer) ([]string, error) {
	dir, err := contextDirectory(name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	// Wake the Codespace and let gh establish its supported SSH identity before
	// generating the relay configuration used by the detached session.
	if _, err := providerOutput(ctx, stderr, "gh", "codespace", "ssh", "-c", id, "--", "-o", "ForwardAgent=no", "true"); err != nil {
		return nil, err
	}
	data, err := providerOutput(ctx, stderr, "gh", "codespace", "ssh", "-c", id, "--config")
	if err != nil {
		return nil, err
	}
	var host string
	config := strings.Builder{}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.EqualFold(fields[0], "Host") {
			if host != "" || ValidateHost(fields[1]) != nil {
				return nil, fmt.Errorf("unexpected SSH host configuration for Codespace %q", id)
			}
			host = fields[1]
			fmt.Fprintln(&config, line)
			fmt.Fprintln(&config, "  ForwardAgent no")
			continue
		}
		fmt.Fprintln(&config, line)
	}
	if host == "" {
		return nil, fmt.Errorf("GitHub CLI returned no SSH host for %q", id)
	}
	filename := filepath.Join(dir, "provider-ssh.conf")
	if err := os.WriteFile(filename, []byte(config.String()), 0o600); err != nil {
		return nil, err
	}
	return []string{"-F", filename, host}, nil
}

func ProviderMachines(ctx context.Context, p ProviderTarget, stderr io.Writer) ([]Machine, error) {
	var data []byte
	var err error
	switch p.Kind {
	case "codespaces":
		data, err = providerOutput(ctx, stderr, "gh", "codespace", "list", "--limit", "1000", "--json", "name,displayName,state,lastUsedAt")
	case "boxd":
		data, err = providerOutput(ctx, stderr, "boxd", "machine", "list", "--region", "all", "--json")
	case "lima":
		data, err = providerOutput(ctx, stderr, "limactl", "list", "--json")
	default:
		return nil, fmt.Errorf("unknown provider %q", p.Kind)
	}
	if err != nil {
		return nil, err
	}
	type record struct {
		Name, DisplayName, State, Status, Region string
		LastUsedAt                               time.Time
	}
	var rows []record
	if p.Kind == "lima" {
		// Lima emits one JSON object per instance.
		for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			var row record
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
	} else if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	machines := make([]Machine, 0, len(rows))
	for _, row := range rows {
		name, state := row.DisplayName, row.State
		if name == "" {
			name = row.Name
		}
		if state == "" {
			state = row.Status
		}
		machines = append(machines, Machine{ID: row.Name, Name: name, State: MachineState(state), Region: row.Region, LastUsed: row.LastUsedAt})
	}
	return machines, nil
}

func MachineState(state string) string {
	switch strings.ToLower(state) {
	case "available", "running", "active":
		return "running"
	case "stopped", "shutdown", "shutoff":
		return "stopped"
	case "":
		return "unknown"
	default:
		return strings.ToLower(state)
	}
}

func ChangeProviderMachine(ctx context.Context, p ProviderTarget, destroy bool, stderr io.Writer) error {
	if !p.Owned || !limaInstanceName.MatchString(p.ID) {
		return fmt.Errorf("machine lifecycle changes require a machine created by mw")
	}
	action := "stop"
	if destroy {
		action = "delete"
	}
	var err error
	switch p.Kind {
	case "lima":
		args := []string{action, "--tty=false", p.ID}
		_, err = providerOutput(ctx, stderr, "limactl", args...)
	case "codespaces":
		args := []string{"codespace", action, "-c", p.ID}
		if destroy {
			args = append(args, "--force")
		}
		_, err = providerOutput(ctx, stderr, "gh", args...)
	case "boxd":
		if !limaInstanceName.MatchString(p.Identity) {
			return fmt.Errorf("cannot verify the identity of Boxd machine %s", p.ID)
		}
		args := []string{"machine", action, p.Identity}
		if destroy {
			args = []string{"machine", "remove", p.Identity, "--confirm"}
		}
		if p.Region != "" {
			args = append(args, "--region="+p.Region)
		}
		_, err = providerOutput(ctx, stderr, "boxd", args...)
	default:
		err = fmt.Errorf("unknown provider %q", p.Kind)
	}
	return err
}
