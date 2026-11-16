package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
)

func toolsDBTestStream(query func(string) (*mysql.Result, error)) func(context.Context, toolforge.Connection, string) (net.Conn, error) {
	return func(_ context.Context, _ toolforge.Connection, _ string) (net.Conn, error) {
		front, back := net.Pipe()
		go func() {
			defer back.Close()
			protocol := server.NewServer("8.0.11-test", mysql.DEFAULT_COLLATION_ID, mysql.AUTH_NATIVE_PASSWORD, nil, nil)
			auth := server.NewInMemoryAuthenticationHandler(mysql.AUTH_NATIVE_PASSWORD)
			_ = auth.AddUser("s12345", "upstream-secret")
			conn, err := protocol.NewCustomizedConn(back, auth, &replicaProxyTestBackend{query: query})
			if err != nil {
				return
			}
			for !conn.Closed() {
				if err := conn.HandleCommand(); err != nil {
					return
				}
			}
		}()
		return front, nil
	}
}

func toolsDBRows(names ...string) (*mysql.Result, error) {
	rows := make([][]any, 0, len(names))
	for _, name := range names {
		rows = append(rows, []any{name})
	}
	result, err := mysql.BuildSimpleResultset([]string{"SCHEMA_NAME"}, rows, false)
	if err != nil {
		return nil, err
	}
	return mysql.NewResult(result), nil
}

func TestToolsDBListCommand(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		database []string
		host     string
		output   string
	}{
		{"primary", []string{"list", "--tool", "mytool"}, []string{"s12345__z", "s12345__a"}, "tools.db.svc.wikimedia.cloud", "s12345__a\ns12345__z\n"},
		{"replica empty", []string{"list", "--tool", "mytool", "--read-only"}, nil, "tools-readonly.db.svc.wikimedia.cloud", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			var gotConnection toolforge.Connection
			var gotHost, gotQuery string
			open := toolsDBTestStream(func(query string) (*mysql.Result, error) {
				gotQuery = query
				return toolsDBRows(test.database...)
			})
			root := &cobra.Command{Use: "toolforge"}
			root.PersistentFlags().String("tool", "", "tool")
			root.AddCommand(newToolsDBListCmd(
				func(_ context.Context, connection toolforge.Connection, _ io.Reader, _ io.Writer) (replicaCredentials, error) {
					gotConnection = connection
					return replicaCredentials{User: "s12345", Password: "upstream-secret"}, nil
				},
				func(ctx context.Context, connection toolforge.Connection, host string) (net.Conn, error) {
					gotHost = host
					return open(ctx, connection, host)
				},
			))
			root.SetOut(&output)
			root.SetErr(io.Discard)
			root.SetArgs(test.args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			wantQuery := "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE LEFT(SCHEMA_NAME, 8) = BINARY 's12345__' ORDER BY SCHEMA_NAME"
			if gotConnection.ToolUser != "tools.mytool" || gotHost != test.host || gotQuery != wantQuery || output.String() != test.output {
				t.Fatalf("connection=%#v host=%q query=%q output=%q", gotConnection, gotHost, gotQuery, output.String())
			}
		})
	}
}

func TestToolsDBListFailures(t *testing.T) {
	connection := toolforge.Connection{}
	credentials := replicaCredentials{User: "s12345", Password: "upstream-secret"}
	for _, test := range []struct {
		name string
		open func(context.Context, toolforge.Connection, string) (net.Conn, error)
	}{
		{"ssh", func(context.Context, toolforge.Connection, string) (net.Conn, error) {
			return nil, errors.New("upstream-secret")
		}},
		{"query", toolsDBTestStream(func(string) (*mysql.Result, error) {
			return nil, mysql.NewError(mysql.ER_ACCESS_DENIED_ERROR, "upstream-secret")
		})},
		{"unexpected database", toolsDBTestStream(func(string) (*mysql.Result, error) {
			return toolsDBRows("s999__other")
		})},
		{"unsafe database", toolsDBTestStream(func(string) (*mysql.Result, error) {
			return toolsDBRows("s12345__bad\nname")
		})},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := listToolsDB(t.Context(), connection, credentials, toolsDBHost(false), test.open)
			if err == nil || result != nil || strings.Contains(err.Error(), credentials.Password) {
				t.Fatalf("result=%v err=%v", result, err)
			}
		})
	}
	if _, err := listToolsDB(t.Context(), connection, replicaCredentials{User: "bad' OR 1=1", Password: "secret"}, toolsDBHost(false),
		func(context.Context, toolforge.Connection, string) (net.Conn, error) {
			t.Fatal("invalid username opened a connection")
			return nil, nil
		}); err == nil {
		t.Fatal("accepted invalid username")
	}
}

func TestToolsDBListCancellationClosesConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	opened := make(chan net.Conn, 1)
	done := make(chan error, 1)
	go func() {
		_, err := listToolsDB(ctx, toolforge.Connection{}, replicaCredentials{User: "s12345", Password: "secret"}, toolsDBHost(false),
			func(context.Context, toolforge.Connection, string) (net.Conn, error) {
				front, back := net.Pipe()
				opened <- back
				return front, nil
			})
		done <- err
	}()
	back := <-opened
	defer back.Close()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("connection not closed on cancellation")
	}
}

func TestToolsDBListRejectsArguments(t *testing.T) {
	cmd := newToolsDBListCmd(
		func(context.Context, toolforge.Connection, io.Reader, io.Writer) (replicaCredentials, error) {
			t.Fatal("invalid arguments requested credentials")
			return replicaCredentials{}, nil
		},
		func(context.Context, toolforge.Connection, string) (net.Conn, error) {
			return nil, fmt.Errorf("unexpected connection")
		},
	)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"unexpected"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("accepted unexpected argument")
	}
}
