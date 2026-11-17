package tools

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"strings"

	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"golang.org/x/term"
)

func newDatabaseSQLCmd(toolsDB bool, run func(*osexec.Cmd) error) *cobra.Command {
	var service string
	var readOnly bool
	cmd := &cobra.Command{
		Use:   "sql <wiki> [-- mariadb options...]",
		Short: "Run the Toolforge MariaDB client for a Wiki Replica",
		Long: `Run MariaDB on Toolforge over SSH, using the selected account's
~/replica.my.cnf. Use --tool to select a tool account. Credentials stay on
Toolforge. With a terminal, open an interactive session; SQL can also be piped
to stdin. Pass MariaDB options after --; paths in those options refer to
Toolforge, not your computer. SQL and options may be visible in process arguments.

The client uses the selected wiki's analytics endpoint by default. --beta and
--lima are not supported. The remote client can access any database permitted
to the selected account; the selected database is not an authorization boundary.`,
		Example: cobrautil.NormalizeExample(`
sql enwiki
sql enwiki --service web -- -e 'SELECT COUNT(*) FROM page'
sql enwiki --tool mytool -- --batch
`),
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		SilenceUsage:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Client arguments must follow -- so local and remote flags are unambiguous.
			dash := cmd.ArgsLenAtDash()
			if (dash < 0 && len(args) != 1) || (dash >= 0 && dash != 1) {
				return fmt.Errorf("specify one database and put MariaDB options after --")
			}
			var forward replicaForward
			var err error
			if toolsDB {
				forward, err = toolsDBForward(args[0], readOnly, 4711)
			} else {
				var forwards []replicaForward
				forwards, err = replicaForwards(args[:1], service, 4711)
				if err == nil {
					forward = forwards[0]
				}
			}
			if err != nil {
				return err
			}
			connection, err := replicaConnection(cmd, true)
			if err != nil {
				return err
			}
			sshArgs := databaseSQLSSHArgs(connection, forward, args[1:], terminalIO(cmd.InOrStdin(), cmd.OutOrStdout()))
			ssh := osexec.CommandContext(cmd.Context(), "ssh", sshArgs...) // #nosec G204 -- remote arguments are shell-quoted.
			ssh.Stdin, ssh.Stdout, ssh.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
			if err := run(ssh); err != nil {
				if cmd.Context().Err() != nil {
					return cmd.Context().Err()
				}
				return preserveExitCode(cmd, err)
			}
			return nil
		},
	}
	if toolsDB {
		cmd.Use = "sql <database> [-- mariadb options...]"
		cmd.Short = "Run the Toolforge MariaDB client for ToolsDB"
		cmd.Long = strings.Replace(cmd.Long, "The client uses the selected wiki's analytics endpoint by default.", "The client uses the writable ToolsDB primary by default; --read-only selects its read-only replica.", 1)
		cmd.Example = cobrautil.NormalizeExample(`
sql s12345__mydb --tool mytool
sql s12345__mydb --tool mytool --read-only -- -e 'SELECT 1'
`)
		cmd.Flags().BoolVar(&readOnly, "read-only", false, "Use the read-only ToolsDB replica")
	} else {
		cmd.Flags().StringVar(&service, "service", "analytics", "Replica endpoint: analytics or web")
		_ = cmd.RegisterFlagCompletionFunc("service", cobra.FixedCompletions([]string{"analytics", "web"}, cobra.ShellCompDirectiveNoFileComp))
	}
	return cmd
}

func runDatabaseSQL(cmd *osexec.Cmd) error {
	return cmd.Run()
}

func terminalIO(input io.Reader, output io.Writer) bool {
	in, inOK := input.(*os.File)
	out, outOK := output.(*os.File)
	return inOK && outOK && term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}

func databaseSQLSSHArgs(connection toolforge.Connection, forward replicaForward, clientArgs []string, terminal bool) []string {
	client := []string{"--host=" + forward.host, "--database=" + forward.database}
	client = append(client, clientArgs...)
	quoted := make([]string, len(client))
	for i, arg := range client {
		quoted[i] = toolforge.QuoteShellArgument(arg)
	}
	script := `exec mariadb --defaults-file="$HOME/replica.my.cnf" ` + strings.Join(quoted, " ")
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	// sudo -i evaluates its command through a login shell which can expand '$'
	// even inside arguments. Encode the script until after that shell has run.
	launcher := "command -v base64 >/dev/null || exit 127; exec sh -c \"`printf %s " + encoded + " | base64 -d`\""
	sshArgs := append([]string(nil), connection.SSHOptions...)
	if terminal {
		sshArgs = append(sshArgs, "-t")
	} else {
		sshArgs = append(sshArgs, "-T")
	}
	sshArgs = append(sshArgs, "-o", "ClearAllForwardings=yes", "-o", "PermitLocalCommand=no",
		"-o", "ControlMaster=no", "-o", "ControlPath=none",
		"-o", "IgnoreUnknown=ForkAfterAuthentication", "-o", "ForkAfterAuthentication=no",
		connection.Host, connection.RemoteCommand([]string{"sh", "-c", launcher}))
	return sshArgs
}
