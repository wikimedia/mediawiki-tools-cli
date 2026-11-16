package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	osexec "os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
)

func TestToolsDBForward(t *testing.T) {
	for _, test := range []struct {
		readOnly bool
		host     string
	}{
		{false, "tools.db.svc.wikimedia.cloud"},
		{true, "tools-readonly.db.svc.wikimedia.cloud"},
	} {
		got, err := toolsDBForward("s12345__My_db", test.readOnly, 13306)
		want := replicaForward{database: "s12345__My_db", host: test.host, port: 13306}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, %v; want %#v", got, err, want)
		}
	}
	for _, database := range []string{"", "enwiki", "__db", "s123__", "s123__db/path", "s123__db\\path", "s123__db\n", "s123__db ", strings.Repeat("x", 64) + "__db"} {
		if _, err := toolsDBForward(database, false, 4711); err == nil {
			t.Errorf("accepted database %q", database)
		}
		if _, err := toolsDBForward("s123__Données.test", false, 4711); err != nil {
			t.Fatalf("rejected valid quoted database name: %v", err)
		}
	}
	for _, port := range []int{0, -1, 65536} {
		if _, err := toolsDBForward("s123__db", false, port); err == nil {
			t.Errorf("accepted port %d", port)
		}
	}
}

func TestToolsDBTunnelDryRun(t *testing.T) {
	for _, test := range []struct {
		args []string
		host string
	}{
		{[]string{"s12345__mydb", "--dry-run"}, "tools.db.svc.wikimedia.cloud"},
		{[]string{"s12345__mydb", "--dry-run", "--read-only", "--port", "13306"}, "tools-readonly.db.svc.wikimedia.cloud"},
	} {
		var output bytes.Buffer
		cmd := newDatabaseTunnelCmd(func(*osexec.Cmd) error { t.Fatal("dry run started SSH"); return nil }, true)
		cmd.SetOut(&output)
		cmd.SetArgs(test.args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), test.host+":3306") || !strings.Contains(output.String(), "s12345__mydb") {
			t.Fatalf("unexpected output: %s", output.String())
		}
	}
}

func TestToolsDBCommandsRejectInvalidInputs(t *testing.T) {
	for _, proxy := range []bool{false, true} {
		for _, args := range [][]string{{}, {"invalid"}, {"s123__db", "s123__other"}, {"s123__db", "--port", "0"}, {"s123__db", "--service", "web"}} {
			cmd := newDatabaseTunnelCmd(func(*osexec.Cmd) error { t.Fatal("invalid input started SSH"); return nil }, true)
			if proxy {
				cmd = newDatabaseProxyCmd(true)
			}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Errorf("proxy=%v accepted %q", proxy, args)
			}
		}
	}
}

func TestToolsDBProxyForwardsWrites(t *testing.T) {
	queries := make(chan string, 1)
	h := startDatabaseProxyTest(t, "s12345__mydb", time.Second, func(query string) (*mysql.Result, error) {
		queries <- query
		return replicaProxyTestQuery(query)
	})
	conn, err := client.Connect(h.address, "wmuser", "LOCAL_SESSION_PASSWORD", "s12345__mydb")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if h.dials.Load() != 0 {
		t.Fatal("local login opened an upstream")
	}
	const query = "INSERT INTO example VALUES (1)"
	if _, err := conn.Execute(query); err != nil {
		t.Fatal(err)
	}
	if got := <-queries; got != query {
		t.Fatalf("forwarded %q", got)
	}
	if err := conn.UseDB("other"); err == nil {
		t.Fatal("accepted a different database through COM_INIT_DB")
	}
}

type toolsDBReadyWriter struct{ ready chan struct{} }

func (w toolsDBReadyWriter) Write(p []byte) (int, error) {
	select {
	case w.ready <- struct{}{}:
	default:
	}
	return len(p), nil
}

func TestToolsDBProxyCommandUsesToolCredentialsAndReadOnlyHost(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	credentials := make(chan toolforge.Connection, 1)
	host := make(chan string, 1)
	queries := make(chan string, 1)
	cmd := newDatabaseProxyCmdWithConnections(true,
		func(_ context.Context, conn toolforge.Connection, _ io.Reader, _ io.Writer) (replicaCredentials, error) {
			credentials <- conn
			return replicaCredentials{User: "s12345", Password: "upstream-secret"}, nil
		},
		func(_ context.Context, conn toolforge.Connection, destination string) (net.Conn, error) {
			host <- destination
			front, back := net.Pipe()
			go func() {
				defer back.Close()
				protocol := server.NewServer("8.0.11-test", mysql.DEFAULT_COLLATION_ID, mysql.AUTH_NATIVE_PASSWORD, nil, nil)
				auth := server.NewInMemoryAuthenticationHandler(mysql.AUTH_NATIVE_PASSWORD)
				_ = auth.AddUser("s12345", "upstream-secret")
				backend := &replicaProxyTestBackend{query: func(query string) (*mysql.Result, error) {
					queries <- query
					return replicaProxyTestQuery(query)
				}}
				session, err := protocol.NewCustomizedConn(back, auth, backend)
				if err != nil {
					return
				}
				for !session.Closed() {
					if err := session.HandleCommand(); err != nil {
						return
					}
				}
			}()
			return front, nil
		})
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().String("tool", "", "tool")
	root.AddCommand(cmd)
	ready := make(chan struct{}, 1)
	root.SetOut(toolsDBReadyWriter{ready: ready})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"proxy", "s12345__mydb", "--tool", "mytool", "--read-only", "--port", fmt.Sprint(port), "--user", "local", "--password", "local-secret"})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	root.SetContext(ctx)
	done := make(chan error, 1)
	go func() { done <- root.Execute() }()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("proxy exited before starting: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not start")
	}
	if got := <-credentials; got.ToolUser != "tools.mytool" {
		t.Fatalf("credential account = %q", got.ToolUser)
	}
	connection, err := client.Connect(fmt.Sprintf("127.0.0.1:%d", port), "local", "local-secret", "s12345__mydb")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Execute("SELECT 42"); err != nil {
		t.Fatal(err)
	}
	if got := <-host; got != "tools-readonly.db.svc.wikimedia.cloud" {
		t.Fatalf("upstream host = %q", got)
	}
	if got := <-queries; got != "SELECT 42" {
		t.Fatalf("upstream query = %q", got)
	}
	_ = connection.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not stop")
	}
}
