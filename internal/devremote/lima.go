package devremote

import (
	"context"
	"fmt"
	"io"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var limaInstanceName = regexp.MustCompile(`^[A-Za-z0-9]+(?:[._-][A-Za-z0-9]+)*$`)

// LimaSSH starts or reuses a named instance and returns ordinary OpenSSH arguments.
func LimaSSH(ctx context.Context, name string, create bool, stdout, stderr io.Writer) ([]string, error) {
	if !limaInstanceName.MatchString(name) {
		return nil, fmt.Errorf("invalid Lima instance name %q", name)
	}
	lima, err := osexec.LookPath("limactl")
	if err != nil {
		return nil, fmt.Errorf("install Lima (https://lima-vm.io/docs/installation/) and retry: %w", err)
	}

	list := osexec.CommandContext(ctx, lima, "list", "--quiet") // #nosec G204 -- limactl is resolved from PATH.
	list.Stderr = stderr
	data, err := list.Output()
	if err != nil {
		return nil, fmt.Errorf("list Lima instances: %w", err)
	}
	args := []string{"start", "--tty=false"}
	if slices.Contains(strings.Fields(string(data)), name) {
		args = append(args, name)
	} else {
		if !create {
			return nil, fmt.Errorf("Lima instance %q is missing; recreate it or choose another remote context", name)
		}
		fmt.Fprintf(stdout, "Creating Lima instance %s with Docker. The first boot downloads a Linux VM image.\n", name)
		// The remote session owns browser forwarding. The checkout stays in the VM.
		args = append(args, "--name="+name, "--cpus=4", "--memory=8", "--disk=40", "--set",
			`.mounts = [] | .portForwards = [{"guestIP":"0.0.0.0","guestIPMustBeZero":false,"proto":"any","ignore":true}]`, "template://docker-rootful")
	}
	start := osexec.CommandContext(ctx, lima, args...) // #nosec G204 -- validated instance name.
	start.Stdout, start.Stderr = stdout, stderr
	if err := start.Run(); err != nil {
		return nil, fmt.Errorf("start Lima instance %q: %w", name, err)
	}

	config := osexec.CommandContext(ctx, lima, "list", "--format={{.SSHConfigFile}}", name) // #nosec G204 -- validated instance name.
	config.Stderr = stderr
	data, err = config.Output()
	if err != nil {
		return nil, fmt.Errorf("locate Lima SSH configuration: %w", err)
	}
	filename := strings.TrimSpace(string(data))
	if !filepath.IsAbs(filename) || strings.ContainsAny(filename, "\r\n") {
		return nil, fmt.Errorf("Lima did not report a usable SSH configuration for %q", name)
	}
	return []string{"-F", filename, "lima-" + name}, nil
}
