package tools

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	ossignal "os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmdgloss"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
)

var (
	replicaDatabaseName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
	replicaSectionName  = regexp.MustCompile(`^s[0-9]+$`)
)

type replicaForward struct {
	database, host string
	port           int
}

func replicaForwards(databases []string, service string, port int) ([]replicaForward, error) {
	if service != "analytics" && service != "web" {
		return nil, fmt.Errorf("--service must be analytics or web")
	}
	if port < 1 || port > 65535 || len(databases) > 65536-port {
		return nil, fmt.Errorf("local ports must fit within 1–65535; choose a lower --port or fewer databases")
	}
	seen := make(map[string]bool)
	forwards := make([]replicaForward, 0, len(databases))
	for _, database := range databases {
		wiki := strings.TrimSuffix(database, "_p")
		if !replicaDatabaseName.MatchString(wiki) || replicaSectionName.MatchString(wiki) {
			return nil, fmt.Errorf("invalid wiki database %q: use a database name such as enwiki or enwiki_p, not a hostname or section", database)
		}
		if seen[wiki] {
			return nil, fmt.Errorf("wiki database %q was specified more than once", wiki)
		}
		seen[wiki] = true
		forwards = append(forwards, replicaForward{
			database: wiki + "_p",
			host:     wiki + "." + service + ".db.svc.wikimedia.cloud",
			port:     port + len(forwards),
		})
	}
	return forwards, nil
}

func newReplicasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replicas",
		Short: "Access Wiki Replicas from your computer through Toolforge",
	}
	cmd.AddCommand(newReplicaTunnelCmd(func(cmd *osexec.Cmd) error { return cmd.Run() }), newReplicaUsernameCmd(readReplicaUsername), newReplicaProxyCmd())
	return cmd
}

func newReplicaTunnelCmd(run func(*osexec.Cmd) error) *cobra.Command {
	var service string
	var port int
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "tunnel <wiki> [wiki...]",
		Short: "Forward local ports to Wiki Replicas over one SSH connection",
		Long: `Run on your own computer to reach Wiki Replicas through production Toolforge.
Supply wiki database names, such as enwiki, commonswiki or enwiki_p. Consecutive
ports on 127.0.0.1 forward only to the documented per-wiki analytics or web
endpoints on port 3306. Hostnames, section names and extension databases are
not accepted. Database existence and permissions are checked by your DB client.

Reuses Toolforge's SSH destination and your existing OpenSSH configuration,
keys, agent and host-key verification. No remote shell, background daemon,
database login or credential lookup is started. Keep this foreground command
running while using your DB client; Ctrl+C closes the tunnel. Saved LocalForward, RemoteForward and DynamicForward entries for this host
are rejected; keep those out of its SSH configuration. An occupied local
port fails the SSH connection; use another --port. Forwards are usable only
after SSH authentication succeeds, and DB connectivity is checked on use.

Use 127.0.0.1 explicitly in your DB client, not localhost (which may select a
Unix socket). Close idle DB connections; an open SSH tunnel alone does not
keep database connections open. --tool, --beta and --lima do not apply.
See https://wikitech.wikimedia.org/wiki/Help:Wiki_Replicas`,
		Example: cobrautil.NormalizeExample(`
tunnel enwiki
tunnel enwiki_p dewiki commonswiki --port 4711
tunnel enwiki --service web --port 13306
tunnel enwiki dewiki --dry-run
`),
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			connection, err := replicaConnection(cmd, false)
			if err != nil {
				return err
			}
			forwards, err := replicaForwards(args, service, port)
			if err != nil {
				return err
			}
			sshArgs := append([]string(nil), connection.SSHOptions...)
			// Older OpenSSH has no ForkAfterAuthentication setting and does not
			// fork without -f. Newer clients must override any saved fork setting.
			sshArgs = append(sshArgs, "-N", "-T", "-o", "ClearAllForwardings=no", "-o", "ExitOnForwardFailure=yes",
				"-o", "ControlMaster=no", "-o", "ControlPath=none",
				"-o", "IgnoreUnknown=ForkAfterAuthentication", "-o", "ForkAfterAuthentication=no")
			lines := []string{"Requested Wiki Replica forwards (available after SSH connects):"}
			for _, forward := range forwards {
				local := "127.0.0.1:" + strconv.Itoa(forward.port)
				sshArgs = append(sshArgs, "-L", local+":"+forward.host+":3306")
				lines = append(lines, fmt.Sprintf("%s  %s -> %s:3306", forward.database, local, forward.host))
			}
			sshArgs = append(sshArgs, connection.Host)
			if dryRun {
				lines[0] = "Planned Wiki Replica forwards (no connection opened):"
			} else if _, err := lookpath.NeedExecutables([]string{"ssh"}); err != nil {
				return err
			}
			cmd.Print(cmdgloss.Block(lines))
			if dryRun {
				quoted := []string{"ssh"}
				for _, arg := range sshArgs {
					quoted = append(quoted, toolforge.QuoteShellArgument(arg))
				}
				cmd.Println(strings.Join(quoted, " "))
				return nil
			}
			if err := checkReplicaSSHForwards(cmd.Context(), connection); err != nil {
				return err
			}
			cmd.Println("Connecting through Toolforge. Press Ctrl+C to close all forwards. Close idle connections in your DB client.")
			ctx, stop := ossignal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			ssh := osexec.CommandContext(ctx, "ssh", sshArgs...) // #nosec G204
			ssh.Stdin, ssh.Stdout, ssh.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
			err = run(ssh)
			if ctx.Err() != nil {
				return nil
			}
			if err != nil {
				return preserveExitCode(cmd, fmt.Errorf("Wiki Replica tunnel failed (check SSH access and local port availability): %w", err))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&service, "service", "analytics", "Replica endpoint: analytics or web")
	cmd.Flags().IntVar(&port, "port", 4711, "First local tunnel TCP port; additional wikis use consecutive ports")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print mappings and the SSH command without connecting")
	_ = cmd.RegisterFlagCompletionFunc("service", cobra.FixedCompletions([]string{"analytics", "web"}, cobra.ShellCompDirectiveNoFileComp))
	return cmd
}

// OpenSSH adds saved forwards to command-line -L options. Refuse those rather
// than opening unrelated listeners. Explicit ClearAllForwardings=no prevents a
// saved yes setting from silently removing the forwards we report to the user.
func checkReplicaSSHForwards(ctx context.Context, connection toolforge.Connection) error {
	args := append([]string(nil), connection.SSHOptions...)
	args = append(args, "-G", "-o", "ClearAllForwardings=no", connection.Host)
	config, err := osexec.CommandContext(ctx, "ssh", args...).Output() // #nosec G204
	if err != nil {
		return fmt.Errorf("could not inspect effective Toolforge SSH configuration: %w", err)
	}
	return validateReplicaSSHForwards(string(config))
}

func validateReplicaSSHForwards(config string) error {
	for line := range strings.SplitSeq(config, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "localforward", "remoteforward", "dynamicforward":
			return fmt.Errorf("Toolforge SSH configuration includes saved port forwards; remove LocalForward, RemoteForward and DynamicForward for this host before starting a replica tunnel")
		}
	}
	return nil
}
