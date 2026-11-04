package tools

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	osexec "os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/go-mysql-org/go-mysql/stmt"
	_ "github.com/go-sql-driver/mysql"
)

type replicaProxyTestBackend struct {
	server.EmptyHandler
	query func(string) (*mysql.Result, error)
}

func (*replicaProxyTestBackend) UseDB(string) error { return nil }
func (h *replicaProxyTestBackend) HandleQuery(query string) (*mysql.Result, error) {
	return h.query(query)
}

func (*replicaProxyTestBackend) HandleStmtPrepare(string) (int, int, any, error) {
	field := (&mysql.Field{Name: []byte("answer"), Type: mysql.MYSQL_TYPE_LONGLONG, Charset: 63}).Dump()
	return 1, 1, &stmt.PreparedStmt{Params: 1, Columns: 1, RawParamFields: [][]byte{field}, RawColumnFields: [][]byte{field}}, nil
}

func (*replicaProxyTestBackend) HandleStmtExecute(_ any, _ string, args []any) (*mysql.Result, error) {
	value := args[0]
	if typed, ok := value.(mysql.TypedBytes); ok {
		value = typed.Bytes
	}
	result, err := mysql.BuildSimpleResultset([]string{"answer"}, [][]any{{value}}, true)
	if err != nil {
		return nil, err
	}
	return mysql.NewResult(result), nil
}

func (*replicaProxyTestBackend) HandleStmtClose(any) error { return nil }

type replicaCloseObserver struct {
	net.Conn
	once   *sync.Once
	closed chan struct{}
}

func (c *replicaCloseObserver) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

type replicaProxyHarness struct {
	address   string
	cancel    context.CancelFunc
	dials     atomic.Int32
	charset   atomic.Uint32
	closed    chan struct{}
	closeOnce sync.Once
}

func startReplicaProxyTest(t *testing.T, idle time.Duration, query func(string) (*mysql.Result, error)) *replicaProxyHarness {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := &replicaProxyHarness{address: listener.Addr().String(), cancel: cancel, closed: make(chan struct{})}
	config := replicaProxyConfig{
		database: "enwiki_p", user: "wmuser", password: "LOCAL_SESSION_PASSWORD", idle: idle,
		credentials: replicaCredentials{User: "s12345", Password: "REAL_REPLICA_SECRET"},
		dial: func(context.Context) (net.Conn, error) {
			h.dials.Add(1)
			proxy, backend := net.Pipe()
			go func() {
				defer backend.Close()
				protocol := server.NewServer("8.0.11-test", mysql.DEFAULT_COLLATION_ID, mysql.AUTH_NATIVE_PASSWORD, nil, nil)
				auth := server.NewInMemoryAuthenticationHandler(mysql.AUTH_NATIVE_PASSWORD)
				_ = auth.AddUser("s12345", "REAL_REPLICA_SECRET")
				conn, err := protocol.NewCustomizedConn(backend, auth, &replicaProxyTestBackend{query: query})
				if err != nil {
					return
				}
				h.charset.Store(uint32(conn.Charset()))
				conn.SetStatus(mysql.SERVER_STATUS_AUTOCOMMIT)
				for !conn.Closed() {
					if err := conn.HandleCommand(); err != nil {
						return
					}
				}
			}()
			return &replicaCloseObserver{Conn: proxy, closed: h.closed, once: &h.closeOnce}, nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- serveReplicaProxy(ctx, listener, config) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("proxy did not shut down")
		}
	})
	return h
}

func (h *replicaProxyHarness) connect(t *testing.T) *client.Conn {
	t.Helper()
	conn, err := client.Connect(h.address, "wmuser", "LOCAL_SESSION_PASSWORD", "enwiki_p")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func replicaProxyTestQuery(string) (*mysql.Result, error) {
	result, err := mysql.BuildSimpleResultset([]string{"answer"}, [][]any{{int64(42)}}, false)
	if err != nil {
		return nil, err
	}
	return mysql.NewResult(result), nil
}

func waitReplicaClosed(t *testing.T, closed <-chan struct{}) {
	t.Helper()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream connection was kept open")
	}
}

func TestReplicaProxyOpensOnlyOnUseAndClosesOnQuit(t *testing.T) {
	h := startReplicaProxyTest(t, time.Second, replicaProxyTestQuery)
	conn := h.connect(t)
	if err := conn.Ping(); err != nil {
		t.Fatal(err)
	}
	if h.dials.Load() != 0 {
		t.Fatal("login or ping opened a database connection")
	}
	for range 2 {
		result, err := conn.Execute("SELECT 42")
		if err != nil {
			t.Fatal(err)
		}
		value, err := result.GetInt(0, 0)
		if err != nil || value != 42 {
			t.Fatalf("value=%d err=%v", value, err)
		}
	}
	if h.dials.Load() != 1 {
		t.Fatal("queries did not share their client's session")
	}
	if err := conn.Quit(); err != nil {
		t.Fatal(err)
	}
	waitReplicaClosed(t, h.closed)
}

func TestReplicaProxyIdleTimeoutIgnoresPingsAndNeverReconnects(t *testing.T) {
	h := startReplicaProxyTest(t, 100*time.Millisecond, replicaProxyTestQuery)
	conn := h.connect(t)
	if _, err := conn.Execute("SELECT 42"); err != nil {
		t.Fatal(err)
	}
	limit := time.Now().Add(time.Second)
	for {
		time.Sleep(15 * time.Millisecond)
		if err := conn.Ping(); err != nil {
			break
		}
		if time.Now().After(limit) {
			t.Fatal("protocol pings prevented idle expiry")
		}
	}
	waitReplicaClosed(t, h.closed)
	if _, err := conn.Execute("SELECT 42"); err == nil {
		t.Fatal("expired client was silently reconnected")
	}
	if h.dials.Load() != 1 {
		t.Fatal("idle session reopened an upstream")
	}
}

func TestReplicaProxyIdleLoginAndBadAuthNeverDial(t *testing.T) {
	h := startReplicaProxyTest(t, 60*time.Millisecond, replicaProxyTestQuery)
	if conn, err := client.Connect(h.address, "wmuser", "WRONG", "enwiki_p"); err == nil {
		_ = conn.Close()
		t.Fatal("bad local password accepted")
	}
	conn := h.connect(t)
	time.Sleep(120 * time.Millisecond)
	if err := conn.Ping(); err == nil {
		t.Fatal("idle client was not disconnected")
	}
	if h.dials.Load() != 0 {
		t.Fatal("unused or unauthenticated session opened a database")
	}
}

func TestReplicaProxyActiveQueryOutlivesIdleTimeout(t *testing.T) {
	h := startReplicaProxyTest(t, 60*time.Millisecond, func(query string) (*mysql.Result, error) {
		time.Sleep(150 * time.Millisecond)
		return replicaProxyTestQuery(query)
	})
	conn := h.connect(t)
	if _, err := conn.Execute("SELECT slow"); err != nil {
		t.Fatalf("idle timeout interrupted active query: %v", err)
	}
	if _, err := conn.Execute("SELECT slow again"); err != nil {
		t.Fatal(err)
	}
}

func TestReplicaProxyDisconnectAndShutdownCancelActiveQuery(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(fmt.Sprintf("shutdown=%v", shutdown), func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			defer close(release)
			h := startReplicaProxyTest(t, time.Second, func(query string) (*mysql.Result, error) {
				close(entered)
				<-release
				return replicaProxyTestQuery(query)
			})
			conn := h.connect(t)
			done := make(chan struct{})
			go func() { _, _ = conn.Execute("SELECT blocked"); close(done) }()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("query did not reach upstream")
			}
			if shutdown {
				h.cancel()
			} else {
				// Closing a network socket is concurrent-safe; the protocol
				// client's Close also mutates its packet sequence counter.
				_ = conn.Conn.Conn.Close()
			}
			waitReplicaClosed(t, h.closed)
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("query did not unblock")
			}
		})
	}
}

func TestReplicaProxyPreparedStatements(t *testing.T) {
	h := startReplicaProxyTest(t, time.Second, replicaProxyTestQuery)
	conn := h.connect(t)
	prepared, err := conn.Prepare("SELECT ?")
	if err != nil {
		t.Fatal(err)
	}
	fields, err := prepared.GetColumnFields()
	if err != nil || len(fields) != 1 || string(fields[0].Name) != "answer" {
		t.Fatalf("metadata=%v err=%v", fields, err)
	}
	result, err := prepared.Execute(int64(42))
	if err != nil {
		t.Fatal(err)
	}
	value, err := result.GetInt(0, 0)
	if err != nil || value != 42 {
		t.Fatalf("value=%d err=%v", value, err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReplicaProxyResponseLimitAndErrorRedaction(t *testing.T) {
	for _, large := range []bool{false, true} {
		t.Run(fmt.Sprintf("large=%v", large), func(t *testing.T) {
			h := startReplicaProxyTest(t, time.Second, func(string) (*mysql.Result, error) {
				if !large {
					return nil, mysql.NewError(mysql.ER_PARSE_ERROR, "error containing REAL_REPLICA_SECRET")
				}
				result, err := mysql.BuildSimpleResultset([]string{"large"}, [][]any{{strings.Repeat("x", replicaResponseLimit+1)}}, false)
				if err != nil {
					return nil, err
				}
				return mysql.NewResult(result), nil
			})
			conn := h.connect(t)
			_, err := conn.Execute("SELECT result")
			if err == nil || strings.Contains(err.Error(), "REAL_REPLICA_SECRET") {
				t.Fatalf("unexpected error: %v", err)
			}
			if large {
				waitReplicaClosed(t, h.closed)
			}
		})
	}
}

func TestReplicaProxyCommandValidation(t *testing.T) {
	for _, args := range [][]string{{}, {"enwiki", "--user", ""}, {"enwiki", "--user", "bad@user"}, {"enwiki", "--password", ""}, {"enwiki", "--password", "SECRET\n"}, {"enwiki", "dewiki"}, {"example.com"}, {"s1"}, {"enwiki", "--idle-timeout", "0s"}, {"enwiki", "--idle-timeout", "6m"}, {"enwiki", "--port", "0"}} {
		cmd := newReplicaProxyCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted invalid args %q", args)
		}
	}
}

func TestReplicaLimitedConn(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	go func() { _, _ = right.Write([]byte("12345")) }()
	limited := &replicaLimitedConn{Conn: left, remaining: 4}
	data, err := io.ReadAll(limited)
	if err == nil || !bytes.Equal(data, []byte("1234")) {
		t.Fatalf("data=%q err=%v", data, err)
	}
}

func TestReplicaProxyAcceptsFreshSessionsWithoutPooling(t *testing.T) {
	h := startReplicaProxyTest(t, 60*time.Millisecond, replicaProxyTestQuery)
	first := h.connect(t)
	if _, err := first.Execute("SELECT 42"); err != nil {
		t.Fatal(err)
	}
	waitReplicaClosed(t, h.closed)
	second := h.connect(t)
	if _, err := second.Execute("SELECT 42"); err != nil {
		t.Fatal(err)
	}
	if h.dials.Load() != 2 {
		t.Fatal("new session reused or restored an expired upstream")
	}
}

func TestReplicaProxyLimitsConcurrentSessions(t *testing.T) {
	h := startReplicaProxyTest(t, time.Second, replicaProxyTestQuery)
	for range 4 {
		h.connect(t)
	}
	if conn, err := client.Connect(h.address, "wmuser", "LOCAL_SESSION_PASSWORD", "enwiki_p"); err == nil {
		_ = conn.Close()
		t.Fatal("accepted more than four clients")
	}
	if h.dials.Load() != 0 {
		t.Fatal("idle clients opened upstream connections")
	}
}

func TestReplicaProxyWithMySQLClient(t *testing.T) {
	if testing.Short() {
		t.Skip("SQL client integration test")
	}
	binary, err := osexec.LookPath("mysql")
	if err != nil {
		t.Skip("mysql client is not installed")
	}
	h := startReplicaProxyTest(t, time.Second, replicaProxyTestQuery)
	host, port, err := net.SplitHostPort(h.address)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := osexec.CommandContext(ctx, binary, "--no-defaults", "--protocol=TCP", "--host="+host, "--port="+port, "--user=wmuser", "--batch", "--skip-column-names", "enwiki_p", "--execute=SELECT 42")
	command.Env = append(os.Environ(), "MYSQL_PWD=LOCAL_SESSION_PASSWORD")
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "42" {
		t.Fatalf("mysql output=%q err=%v", output, err)
	}
	waitReplicaClosed(t, h.closed)
}

func TestReplicaProxyMalformedPacketClosesSession(t *testing.T) {
	h := startReplicaProxyTest(t, time.Second, replicaProxyTestQuery)
	conn := h.connect(t)
	if _, err := conn.Execute("SELECT 42"); err != nil {
		t.Fatal(err)
	}
	// An empty command packet is invalid. It must not terminate the CLI.
	if _, err := conn.Conn.Conn.Write([]byte{0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	waitReplicaClosed(t, h.closed)
	fresh := h.connect(t)
	if err := fresh.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestReplicaProxyPreservesClientCollation(t *testing.T) {
	h := startReplicaProxyTest(t, time.Minute, replicaProxyTestQuery)
	c, err := client.Connect(h.address, "wmuser", "LOCAL_SESSION_PASSWORD", "enwiki_p", func(c *client.Conn) error { return c.SetCollation("latin1_swedish_ci") })
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Execute("SELECT 'é'"); err != nil {
		t.Fatal(err)
	}
	if got := h.charset.Load(); got != 8 {
		t.Fatalf("upstream collation = %d, want latin1 (8)", got)
	}
}

func TestReplicaProxyRejectsKillBeforeDial(t *testing.T) {
	h := startReplicaProxyTest(t, time.Minute, replicaProxyTestQuery)
	c := h.connect(t)
	for _, query := range []string{"KILL 10001", "kill query 10001", "/* comment */ # comment\n -- comment\n KILL QUERY 10001", "/*! KILL QUERY 10001 */", "/*M! KILL QUERY 10001 */", "/*m! KILL QUERY 10001 */"} {
		if _, err := c.Execute(query); err == nil {
			t.Errorf("accepted %q", query)
		}
		if _, err := c.Prepare(query); err == nil {
			t.Errorf("prepared %q", query)
		}
	}
	if h.dials.Load() != 0 {
		t.Fatal("rejected command opened an upstream session")
	}
	if _, err := c.Execute("SELECT 'KILL QUERY 10001'"); err != nil {
		t.Fatal(err)
	}
}

func TestReplicaProxyCachedPreparedParameterTypes(t *testing.T) {
	h := startReplicaProxyTest(t, time.Minute, replicaProxyTestQuery)
	c := h.connect(t)
	prepared, err := c.Prepare("SELECT ?")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Execute(int64(11)); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		null  byte
		value uint64
		row   []byte
	}{
		{value: 22, row: []byte{0, 0, 22, 0, 0, 0, 0, 0, 0, 0}},
		{null: 1, row: []byte{0, 4}},
		{value: 33, row: []byte{0, 0, 33, 0, 0, 0, 0, 0, 0, 0}},
	} {
		c.ResetSequence()
		data := append(make([]byte, 4), mysql.COM_STMT_EXECUTE)
		data = binary.LittleEndian.AppendUint32(data, prepared.ID)
		data = append(data, 0, 1, 0, 0, 0, test.null, 0) // cursor, iterations, NULL bitmap, cached types
		if test.null == 0 {
			data = binary.LittleEndian.AppendUint64(data, test.value)
		}
		if err := c.WritePacket(data); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 5; i++ {
			packet, err := c.ReadPacket()
			if err != nil {
				t.Fatal(err)
			}
			if i == 3 && !bytes.Equal(packet, test.row) {
				t.Fatalf("binary row = %x, want %x", packet, test.row)
			}
		}
	}
}

func TestReplicaProxyGoSQLDriverStreamsPreparedParameter(t *testing.T) {
	if testing.Short() {
		t.Skip("SQL client integration test")
	}
	h := startReplicaProxyTest(t, time.Minute, replicaProxyTestQuery)
	db, err := sql.Open("mysql", "wmuser:LOCAL_SESSION_PASSWORD@tcp("+h.address+")/enwiki_p?maxAllowedPacket=128")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	prepared, err := db.Prepare("SELECT ?")
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	for _, want := range []string{strings.Repeat("a", 100), "", strings.Repeat("b", 200), "inline"} {
		var got string
		if err := prepared.QueryRow(want).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("prepared value=%q, want %q", got, want)
		}
	}
}
