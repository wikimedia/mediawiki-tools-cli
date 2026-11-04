package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	ossignal "os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

var replicaUsernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}$`)

func replicaConnection(cmd *cobra.Command, allowTool bool) (toolforge.Connection, error) {
	for _, name := range []string{"beta", "lima"} {
		if cmd.Flags().Lookup(name) != nil {
			enabled, err := cmd.Flags().GetBool(name)
			if err != nil {
				return toolforge.Connection{}, err
			}
			if enabled {
				return toolforge.Connection{}, fmt.Errorf("Wiki Replica access uses production Toolforge; omit --beta and --lima")
			}
		}
	}
	tool := ""
	if cmd.Flags().Lookup("tool") != nil {
		var err error
		tool, err = cmd.Flags().GetString("tool")
		if err != nil {
			return toolforge.Connection{}, err
		}
	}
	if tool != "" && !allowTool {
		return toolforge.Connection{}, fmt.Errorf("omit --tool: the tunnel uses your SSH login; select the replica account in your DB client")
	}
	return toolforge.NewConnection(tool, false, false)
}

func newReplicaUsernameCmd(readUser func(context.Context, toolforge.Connection, io.Reader, io.Writer) (string, error)) *cobra.Command {
	return &cobra.Command{
		Use:          "username",
		Short:        "Print only a Toolforge replica database username",
		SilenceUsage: true,
		Long: `Read the user field from ~/replica.my.cnf over your existing Toolforge SSH
connection. Use --tool to read a tool account's file instead of your personal
account's file. Only the username crosses SSH; no password or credential file
is copied, printed or stored locally. No database connection is opened.

Stdout contains just the username so it can be used in command substitution.
Python 3 is needed on Toolforge, not on your own computer. SSH diagnostics go to
stderr. Missing or malformed files fail without printing their contents.
This command is explicit: replica tunnels never look up credentials themselves.`,
		Example: cobrautil.NormalizeExample(`
username
username --tool mytool
`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			connection, err := replicaConnection(cmd, true)
			if err != nil {
				return err
			}
			ctx, stop := ossignal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			user, err := readUser(ctx, connection, cmd.InOrStdin(), cmd.ErrOrStderr())
			if ctx.Err() != nil {
				return nil
			}
			if err != nil {
				return err
			}
			if !replicaUsernamePattern.MatchString(user) {
				return fmt.Errorf("Toolforge did not return a valid replica username")
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), user)
			return err
		},
	}
}

// Only the user field is printed. Never return parser errors: they can include
// the contents of a malformed credential file, including its password line.
const replicaUsernameScript = `import configparser, os, re, sys
try:
    config = configparser.ConfigParser(interpolation=None)
    with open(os.path.expanduser('~/replica.my.cnf'), encoding='utf-8') as source:
        config.read_file(source)
    user = config.get('client', 'user').strip()
    if len(user) >= 2 and user[0] == user[-1] and user[0] in ('"', "'"):
        user = user[1:-1]
    if not re.fullmatch(r'[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}', user):
        raise ValueError()
except Exception:
    print('Could not read the replica username from ~/replica.my.cnf.', file=sys.stderr)
    sys.exit(1)
print(user)
`

func readReplicaUsername(ctx context.Context, connection toolforge.Connection, input io.Reader, stderr io.Writer) (string, error) {
	value, err := readReplicaFields(ctx, connection, input, stderr, replicaUsernameScript)
	if err != nil {
		return "", fmt.Errorf("Toolforge SSH username lookup failed: %w", err)
	}
	user := strings.TrimSpace(string(value))
	if !replicaUsernamePattern.MatchString(user) {
		return "", fmt.Errorf("Toolforge returned no valid replica username")
	}
	return user, nil
}

func readReplicaFields(ctx context.Context, connection toolforge.Connection, input io.Reader, stderr io.Writer, script string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	args := append([]string(nil), connection.SSHOptions...)
	args = append(args, "-T", "-o", "ClearAllForwardings=yes", "-o", "PermitLocalCommand=no", "-o", "ControlMaster=no", "-o", "ControlPath=none",
		"-o", "IgnoreUnknown=ForkAfterAuthentication", "-o", "ForkAfterAuthentication=no",
		connection.Host, connection.RemoteCommand([]string{"python3", "-c", script}))
	lookup := osexec.CommandContext(ctx, "ssh", args...) // #nosec G204 -- fixed Toolforge destination and fixed, shell-quoted field extraction script.
	lookup.Stdin, lookup.Stderr = input, stderr
	value, err := lookup.Output()
	if err != nil {
		return nil, err
	}
	return value, nil
}
