package cloudvps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	osexec "os/exec"
	"os/user"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

const cloudBrowserURL = "https://openstack-browser.toolforge.org"

var sshDNSLabel = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

type sshOptions struct {
	project, name, id, user, jumpHost, region string
	options                                   []string
	tty, noTTY, dryRun                        bool
}

// NewSSHCmd connects by name or discovers instances without changing compute ssh provisioning.
func NewSSHCmd() *cobra.Command {
	var opts sshOptions
	cmd := &cobra.Command{
		Use:     "ssh [instance] [-- command arguments...]",
		Aliases: []string{"connect"},
		Short:   "Open a Cloud VPS session or run a remote command",
		Long: `Connect to a Cloud VPS instance using your system OpenSSH client.
Short instance names use --project or cloud_vps.default_project. Full
*.wikimedia.cloud hostnames work without a project. Omit the instance to choose
from the project's public instance list; OpenStack credentials are only needed
for --id. Project membership and a registered developer SSH key are still required.

Existing SSH configuration, keys, agents and host-key verification are retained.
If no jump host is configured, bastion.wmcloud.org is used automatically.
Set cloud_vps.ssh.user to your Wikimedia developer shell username for future
connections. An existing developer.username setting is also recognized.
Remote arguments are passed literally; use sh -lc for shell expressions.`,
		Example: `  ssh --project mwcli
  ssh gitlab-runner-addshore-1018 --project mwcli
  ssh --project mwcli --name gitlab-runner-addshore-1018
  ssh vm.project.eqiad1.wikimedia.cloud --user addshore
  ssh vm --project project -- uname -a
  ssh vm --project project -- sh -lc 'cd ~/src && git status'
  ssh vm --project project --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCloudVPSSSH(cmd, args, opts)
		},
	}
	cmd.Flags().StringVar(&opts.project, "project", "", "Project name (defaults to cloud_vps.default_project)")
	cmd.Flags().StringVar(&opts.name, "name", "", "Instance name (alternative to a positional instance)")
	cmd.Flags().StringVar(&opts.id, "id", "", "Instance ID (requires configured OpenStack credentials)")
	cmd.Flags().StringVarP(&opts.user, "user", "u", "", "Developer shell username (defaults to CLI or SSH configuration)")
	cmd.Flags().StringVar(&opts.jumpHost, "jump-host", "", "SSH jump host; 'none' disables the default bastion")
	cmd.Flags().StringVar(&opts.region, "region", "", "DNS region (defaults to eqiad1)")
	cmd.Flags().StringArrayVarP(&opts.options, "ssh-option", "o", nil, "OpenSSH -o option (repeatable)")
	cmd.Flags().BoolVarP(&opts.tty, "tty", "t", false, "Allocate a terminal when running a remote command")
	cmd.Flags().BoolVarP(&opts.noTTY, "no-tty", "T", false, "Disable terminal allocation")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Print the SSH command without connecting or saving configuration")
	cmd.MarkFlagsMutuallyExclusive("name", "id")
	cmd.MarkFlagsMutuallyExclusive("tty", "no-tty")
	return cmd
}

func runCloudVPSSSH(cmd *cobra.Command, args []string, opts sshOptions) error {
	// Cobra reports the boundary before --, so remote options never become mw flags.
	n := cmd.ArgsLenAtDash()
	if n < 0 {
		n = len(args)
	}
	if n > 1 || (n == 1 && (opts.name != "" || opts.id != "")) {
		return fmt.Errorf("provide one instance, using a positional name, --name, or --id; put remote commands after --")
	}
	remote := args[n:]
	if n == 1 {
		opts.name = args[0]
	}
	state := config.State()
	defaults := state.Effective.CloudVPS
	if opts.project == "" {
		opts.project = defaults.DefaultProject
	}
	if opts.region == "" {
		opts.region = defaults.SSH.Region
		if opts.region == "" {
			opts.region = "eqiad1"
		}
	}
	interactive := !cli.Opts.NoInteraction && !opts.dryRun
	if opts.project == "" && !strings.HasSuffix(opts.name, ".wikimedia.cloud") {
		if !interactive {
			return fmt.Errorf("specify --project, set cloud_vps.default_project, or provide a full instance hostname")
		}
		if err := survey.AskOne(projectPrompt(cmd.Context()), &opts.project, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}
	if opts.id != "" {
		name, err := instanceNameByID(cmd.Context(), opts.project, opts.id, defaults)
		if err != nil {
			return err
		}
		opts.name = name
	}
	if opts.name == "" {
		if !interactive {
			return fmt.Errorf("specify an instance name; omit it in interactive mode to choose a VM")
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
		defer cancel()
		hosts, err := projectInstances(ctx, http.DefaultClient, cloudBrowserURL, opts.project, opts.region)
		if err != nil {
			return err
		}
		if err := survey.AskOne(&survey.Select{Message: "Cloud VPS instance:", Options: hosts}, &opts.name); err != nil {
			return err
		}
	}
	host, err := instanceHostname(opts.name, opts.project, opts.region)
	if err != nil {
		return err
	}
	if opts.user == "" {
		opts.user = defaults.SSH.User
		if opts.user == "" {
			opts.user = state.EffectiveKoanf.String("developer.username")
		}
	}
	if opts.jumpHost == "" {
		opts.jumpHost = defaults.SSH.JumpHost
	}
	// Query native SSH configuration rather than overwrite a user's working proxy.
	probe := sshBaseArgs(opts)
	probe = append(probe, "-G", host)
	output, err := osexec.CommandContext(cmd.Context(), "ssh", probe...).Output()
	if err != nil {
		return fmt.Errorf("inspect SSH configuration (OpenSSH must be installed): %w", err)
	}
	effective := parseSSHConfig(string(output))
	if opts.user == "" && interactive {
		// A non-default SSH User is already an intentional configuration.
		localUser := os.Getenv("USER")
		if current, err := user.Current(); err == nil {
			localUser = current.Username
		}
		if effective["user"] == localUser || effective["user"] == "" {
			if err := survey.AskOne(&survey.Input{Message: "Wikimedia developer shell username:", Default: effective["user"]}, &opts.user, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
			if err := config.PutKeyValueOnDisk("cloud_vps.ssh.user", opts.user); err != nil {
				return err
			}
		}
	}
	sshArgs := sshSessionArgs(host, remote, opts, effective)
	if opts.dryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "ssh "+shellWords(sshArgs))
		return nil
	}
	process := osexec.CommandContext(cmd.Context(), "ssh", sshArgs...) // #nosec G204 -- arguments passed directly to OpenSSH
	process.Stdin, process.Stdout, process.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := process.Run(); err != nil {
		var exit *osexec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() >= 0 {
			if exit.ExitCode() == 255 {
				cmd.PrintErrln("SSH connection failed. Check your developer shell username, project membership and key at https://idm.wikimedia.org; use mw -vv cloudvps ssh for SSH diagnostics.")
			}
			root := cmd.Root()
			if root.Annotations == nil {
				root.Annotations = make(map[string]string)
			}
			root.Annotations["exitCode"] = strconv.Itoa(exit.ExitCode())
			return nil
		}
		return err
	}
	return nil
}

func sshBaseArgs(opts sshOptions) []string {
	var args []string
	if opts.user != "" {
		args = append(args, "-l", opts.user)
	}
	for _, option := range opts.options {
		args = append(args, "-o", option)
	}
	return args
}

func sshSessionArgs(host string, remote []string, opts sshOptions, effective map[string]string) []string {
	args := sshBaseArgs(opts)
	jump := opts.jumpHost
	if jump == "" && !sshProxyConfigured(effective) && !sshProxyOptionSet(opts.options) {
		jump = "bastion.wmcloud.org"
		user := opts.user
		if user == "" {
			user = effective["user"]
		}
		if user != "" {
			jump = user + "@" + jump
		}
	}
	if jump != "" {
		args = append(args, "-J", jump)
	}
	if effective["connecttimeout"] == "none" || effective["connecttimeout"] == "0" {
		args = append(args, "-o", "ConnectTimeout=15")
	}
	if cli.Opts.NoInteraction {
		args = append(args, "-o", "BatchMode=yes")
	}
	if cli.Opts.Verbosity > 0 {
		args = append(args, "-"+strings.Repeat("v", min(cli.Opts.Verbosity, 3)))
	}
	if opts.tty {
		args = append(args, "-t")
	} else if opts.noTTY {
		args = append(args, "-T")
	}
	args = append(args, host)
	if len(remote) > 0 {
		args = append(args, shellWords(remote))
	}
	return args
}

func parseSSHConfig(output string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, " ")
		if ok {
			values[strings.ToLower(key)] = strings.TrimSpace(value)
		}
	}
	return values
}

func sshProxyConfigured(values map[string]string) bool {
	for _, key := range []string{"proxyjump", "proxycommand"} {
		if value := values[key]; value != "" && value != "none" {
			return true
		}
	}
	return false
}

func sshProxyOptionSet(options []string) bool {
	for _, option := range options {
		fields := strings.FieldsFunc(option, func(r rune) bool { return r == '=' || r == ' ' || r == '\t' })
		if len(fields) > 0 && (strings.EqualFold(fields[0], "ProxyJump") || strings.EqualFold(fields[0], "ProxyCommand")) {
			return true
		}
	}
	return false
}

func shellWords(words []string) string {
	quoted := make([]string, len(words))
	for i, word := range words {
		quoted[i] = "'" + strings.ReplaceAll(word, "'", "'\"'\"'") + "'"
	}
	return strings.Join(quoted, " ")
}

func instanceHostname(name, project, region string) (string, error) {
	if !strings.HasSuffix(name, ".wikimedia.cloud") {
		if !sshDNSLabel.MatchString(name) {
			return "", fmt.Errorf("provide a short VM name or a full *.wikimedia.cloud hostname")
		}
		if !sshDNSLabel.MatchString(project) || !sshDNSLabel.MatchString(region) {
			return "", fmt.Errorf("project and region must be valid DNS labels")
		}
		name += "." + project + "." + region + ".wikimedia.cloud"
	}
	if len(name) > 253 || len(strings.Split(name, ".")) < 5 {
		return "", fmt.Errorf("invalid Cloud VPS instance hostname: %s", name)
	}
	for _, label := range strings.Split(name, ".") {
		if !sshDNSLabel.MatchString(label) {
			return "", fmt.Errorf("invalid Cloud VPS instance hostname: %s", name)
		}
	}
	return name, nil
}

func projectInstances(ctx context.Context, client *http.Client, baseURL, project, region string) ([]string, error) {
	if !sshDNSLabel.MatchString(project) || !sshDNSLabel.MatchString(region) {
		return nil, fmt.Errorf("project and region must be valid DNS labels")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/dsh/project/"+project, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", cli.HTTPUserAgent)
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discover project instances: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instance discovery returned HTTP %d; provide an instance name to connect without discovery", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	var hosts []string
	for _, host := range strings.Fields(string(body)) {
		if _, err := instanceHostname(host, "", ""); err == nil && strings.HasSuffix(host, "."+project+"."+region+".wikimedia.cloud") {
			hosts = append(hosts, host)
		}
	}
	sort.Strings(hosts)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no instances found for project %s in %s; public discovery does not grant SSH access", project, region)
	}
	return hosts, nil
}

func instanceNameByID(ctx context.Context, project, id string, cfg config.CloudVPSConfig) (string, error) {
	entry, ok := cfg.Projects[project]
	if !ok || entry.Credential.ID == "" || entry.Credential.Secret == "" {
		return "", fmt.Errorf("--id needs OpenStack credentials for project %s; use a VM name to connect without API credentials", project)
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	provider, err := openstack.AuthenticatedClient(ctx, gophercloud.AuthOptions{
		IdentityEndpoint: authUrl, ApplicationCredentialID: entry.Credential.ID,
		ApplicationCredentialSecret: entry.Credential.Secret, DomainID: "default",
	})
	if err != nil {
		return "", err
	}
	compute, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return "", err
	}
	server, err := servers.Get(ctx, compute, id).Extract()
	if err != nil {
		return "", err
	}
	return server.Name, nil
}
