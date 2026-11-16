package tools

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	ossignal "os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func newToolsDBListCmd(
	readCredentials func(context.Context, toolforge.Connection, io.Reader, io.Writer) (replicaCredentials, error),
	openStream func(context.Context, toolforge.Connection, string) (net.Conn, error),
) *cobra.Command {
	var readOnly bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List databases owned by a Toolforge account",
		Long: `List visible ToolsDB databases whose names start with the selected account's
credential username followed by two underscores. Use --tool to select a tool
account; otherwise your personal Toolforge account is used. Credentials are
retrieved over SSH into memory, and the database connection closes after the
query. Each matching database name is printed on its own line. No output means
no matching databases were visible to the account.`,
		Example:      cobrautil.NormalizeExample("list --tool mytool\nlist --tool mytool --read-only"),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			connection, err := replicaConnection(cmd, true)
			if err != nil {
				return err
			}
			ctx, stop := ossignal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			credentials, err := readCredentials(ctx, connection, cmd.InOrStdin(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			databases, err := listToolsDB(ctx, connection, credentials, toolsDBHost(readOnly), openStream)
			if err != nil {
				return err
			}
			for _, database := range databases {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), database); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Query the read-only ToolsDB replica (which may lag behind the primary)")
	return cmd
}

func listToolsDB(ctx context.Context, connection toolforge.Connection, credentials replicaCredentials, host string,
	openStream func(context.Context, toolforge.Connection, string) (net.Conn, error),
) ([]string, error) {
	if !replicaUsernamePattern.MatchString(credentials.User) {
		return nil, fmt.Errorf("invalid Toolforge database username")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var wire net.Conn
	var stop func() bool
	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		var err error
		wire, err = openStream(ctx, connection, host)
		if err != nil {
			return nil, err
		}
		stop = context.AfterFunc(ctx, func() { _ = wire.Close() })
		if err := wire.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
			_ = wire.Close()
			return nil, err
		}
		return &replicaLimitedConn{Conn: wire, remaining: replicaResponseLimit}, nil
	}
	upstream, err := client.ConnectWithDialer(ctx, "tcp", "toolsdb:3306", credentials.User, credentials.Password, "", dial)
	if stop != nil {
		defer stop()
	}
	if wire != nil {
		defer wire.Close()
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("could not connect to ToolsDB over SSH; check Toolforge access and credentials")
	}
	defer upstream.Close()
	prefix := credentials.User + "__"
	// The username is restricted above, so it cannot change the SQL literal.
	query := fmt.Sprintf("SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE LEFT(SCHEMA_NAME, %d) = BINARY '%s' ORDER BY SCHEMA_NAME", len(prefix), prefix)
	result, err := upstream.Execute(query)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("could not list ToolsDB databases")
	}
	defer result.Close()
	if result.Resultset == nil {
		return nil, fmt.Errorf("ToolsDB returned no database list")
	}
	databases := make([]string, 0, result.RowNumber())
	for row := 0; row < result.RowNumber(); row++ {
		database, err := result.GetString(row, 0)
		if err != nil || !strings.HasPrefix(database, prefix) {
			return nil, fmt.Errorf("ToolsDB returned an invalid database name")
		}
		if _, err := toolsDBForward(database, false, 4711); err != nil {
			return nil, fmt.Errorf("ToolsDB returned an invalid database name")
		}
		databases = append(databases, database)
	}
	sort.Strings(databases)
	return databases, nil
}
