package tools

import (
	"fmt"
	osexec "os/exec"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

func toolsDBForward(database string, readOnly bool, port int) (replicaForward, error) {
	owner, name, found := strings.Cut(database, "__")
	if !found || !replicaUsernamePattern.MatchString(owner) || name == "" ||
		!utf8.ValidString(database) || utf8.RuneCountInString(database) > 64 ||
		strings.HasSuffix(database, " ") || strings.ContainsAny(database, `/\`) ||
		strings.IndexFunc(database, unicode.IsControl) >= 0 {
		return replicaForward{}, fmt.Errorf("invalid ToolsDB database %q: use a full name such as s12345__mydb", database)
	}
	if port < 1 || port > 65535 {
		return replicaForward{}, fmt.Errorf("--port must be between 1 and 65535")
	}
	return replicaForward{database: database, host: toolsDBHost(readOnly), port: port}, nil
}

func toolsDBHost(readOnly bool) string {
	if readOnly {
		return "tools-readonly.db.svc.wikimedia.cloud"
	}
	return "tools.db.svc.wikimedia.cloud"
}

func newToolsDBCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "toolsdb", Short: "Access ToolsDB from your computer through Toolforge"}
	cmd.AddCommand(newDatabaseTunnelCmd(func(cmd *osexec.Cmd) error { return cmd.Run() }, true), newDatabaseProxyCmd(true), newToolsDBListCmd(readReplicaCredentials, replicaSSHStream), newDatabaseSQLCmd(true, runDatabaseSQL))
	return cmd
}
