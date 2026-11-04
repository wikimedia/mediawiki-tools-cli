package tools

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	osexec "os/exec"
	ossignal "os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmdgloss"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	server "gitlab.wikimedia.org/repos/releng/cli/third_party/go-mysql-server"
)

func newReplicaProxyCmd() *cobra.Command {
	var login replicaProxyLoginOptions
	var port int
	var service string
	var idle time.Duration
	cmd := &cobra.Command{
		Use:          "proxy <wiki>",
		Short:        "Run an experimental local proxy with replica authentication",
		SilenceUsage: true,
		Long: `Run on your computer. The proxy retrieves your Toolforge replica username and
password over SSH into memory when it starts. Use --tool to select a tool
account's credentials. Upstream authentication always uses those credentials.

The local proxy defaults to username wmuser and a random password. Use
--use-replica-credentials to accept the selected replica account's login locally.
--user and --password override either local value, including in that mode.
Only generated passwords are printed; supplied or reused passwords stay hidden.

Only the selected wiki's documented endpoint is reachable. Connect a local SQL
client using the selected local login. Startup prints a generated password.
An upstream connection opens only for a database operation after local login.
Each client has its own upstream session; there is no pool, background reconnect
or keepalive. Idle clients are disconnected after --idle-timeout (default 1m).
Protocol pings do not extend that timeout. Active queries are allowed to finish;
disconnecting the client or pressing Ctrl+C closes its upstream connection.

This prototype supports ordinary queries and prepared statements. Results are
buffered with a 16 MiB upstream response limit per command. Multi-statements,
multiple result sets, compression, LOCAL INFILE and KILL are not supported.
Client interrupt requests cannot translate local thread IDs; disconnect the
original client to cancel its upstream session. Leading executable SQL comments
are rejected. After an idle disconnect, explicitly reconnect; session state is never recreated silently.
`,
		Example: cobrautil.NormalizeExample(`
proxy enwiki
proxy enwiki --use-replica-credentials --user wmuser --port 13306
proxy commonswiki --tool mytool --idle-timeout 30s
`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			login.userSet = cmd.Flags().Changed("user")
			login.passwordSet = cmd.Flags().Changed("password")
			if err := login.validate(); err != nil {
				return err
			}
			if idle < time.Second || idle > 5*time.Minute {
				return fmt.Errorf("--idle-timeout must be between 1s and 5m")
			}
			forwards, err := replicaForwards(args, service, port)
			if err != nil {
				return err
			}
			connection, err := replicaConnection(cmd, true)
			if err != nil {
				return err
			}
			listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
			if err != nil {
				return fmt.Errorf("could not bind local proxy: %w", err)
			}
			defer listener.Close()
			ctx, stop := ossignal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			credentials, err := readReplicaCredentials(ctx, connection, cmd.InOrStdin(), cmd.ErrOrStderr())
			if ctx.Err() != nil {
				return nil
			}
			if err != nil {
				return err
			}
			local, generated := login.resolve(credentials)
			forward := forwards[0]
			cmd.Print(cmdgloss.Block([]string{
				"Experimental local Wiki Replica authentication proxy",
				"Host: 127.0.0.1  Port: " + strconv.Itoa(port) + "  Database: " + forward.database,
				replicaProxyLoginDescription(local, generated),
				"Idle timeout: " + idle.String() + "; protocol pings do not keep a session alive.",
				"No database connection is opened until a client uses it. Press Ctrl+C to stop.",
			}))
			dial := func(session context.Context) (net.Conn, error) {
				return replicaSSHStream(session, connection, forward.host)
			}
			config := replicaProxyConfig{
				database: forward.database, credentials: credentials, user: local.User, password: local.Password, idle: idle, dial: dial,
			}

			return serveReplicaProxy(ctx, listener, config)
		},
	}
	cmd.Flags().BoolVar(&login.useReplica, "use-replica-credentials", false, "Accept the selected Toolforge replica account's username and password on the local proxy")
	cmd.Flags().StringVar(&login.user, "user", "wmuser", "Local proxy username; overrides --use-replica-credentials when set")
	cmd.Flags().StringVar(&login.password, "password", "", "Local proxy password; overrides --use-replica-credentials when set (default random; value is visible in command arguments)")
	cmd.Flags().IntVar(&port, "port", 4711, "Local proxy TCP port on 127.0.0.1")
	cmd.Flags().StringVar(&service, "service", "analytics", "Replica endpoint: analytics or web")
	cmd.Flags().DurationVar(&idle, "idle-timeout", time.Minute, "Disconnect idle sessions (1s–5m); pings do not extend the deadline")
	return cmd
}

type replicaProxyConfig struct {
	database    string
	credentials replicaCredentials
	user        string
	password    string
	idle        time.Duration
	dial        func(context.Context) (net.Conn, error)
}

func serveReplicaProxy(ctx context.Context, listener net.Listener, config replicaProxyConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	var sessions sync.WaitGroup
	defer sessions.Wait()
	// Bound the number of client sessions as well as upstream connections.
	slots := make(chan struct{}, 4)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			cancel()
			return fmt.Errorf("local proxy listener failed")
		}
		select {
		case slots <- struct{}{}:
			sessions.Add(1)
			go func() {
				defer sessions.Done()
				defer func() { <-slots }()
				serveReplicaProxyClient(ctx, conn, config)
			}()
		default:
			_ = conn.Close()
		}
	}
}

func serveReplicaProxyClient(parent context.Context, wire net.Conn, config replicaProxyConfig) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	// Treat malformed protocol packets as a failed client session. Do not
	// expose panic values, which can contain packet or credential data.
	defer func() {
		if recover() != nil {
			cancel()
		}
	}()
	front, bridge := net.Pipe()
	stop := context.AfterFunc(ctx, func() {
		_ = wire.Close()
		_ = front.Close()
		_ = bridge.Close()
	})
	defer stop()
	defer wire.Close()
	defer front.Close()
	defer bridge.Close()
	// Detect a client disconnect even while its query is running upstream.
	var copies sync.WaitGroup
	copies.Add(2)
	go func() { defer copies.Done(); _, _ = io.Copy(bridge, wire); cancel() }()
	go func() { defer copies.Done(); _, _ = io.Copy(wire, bridge); cancel() }()
	defer func() { cancel(); _ = wire.Close(); _ = front.Close(); _ = bridge.Close(); copies.Wait() }()
	handler := &replicaProxyHandler{ctx: ctx, config: config, statements: make(map[any]*client.Stmt)}
	defer handler.close()
	_ = front.SetDeadline(time.Now().Add(10 * time.Second))
	protocol := server.NewServer("8.0.11-mw-replica-proxy", mysql.DEFAULT_COLLATION_ID, mysql.AUTH_NATIVE_PASSWORD, nil, nil)
	// There is one result per command; no multi-query routing or federation.
	_ = protocol.UnsetCapability(mysql.CLIENT_MULTI_RESULTS | mysql.CLIENT_PS_MULTI_RESULTS)
	auth := server.NewInMemoryAuthenticationHandler(mysql.AUTH_NATIVE_PASSWORD)
	if err := auth.AddUser(config.user, config.password); err != nil {
		return
	}
	down, err := protocol.NewCustomizedConn(replicaClientConn{Conn: front}, auth, handler)
	if err != nil {
		return
	}
	handler.down = down
	_ = front.SetDeadline(time.Time{})
	deadline := time.Now().Add(config.idle)
	for !down.Closed() {
		_ = front.SetReadDeadline(deadline)
		handler.used = false
		if err := down.HandleCommand(); err != nil || handler.failed {
			return
		}
		// COM_PING is answered by the protocol library, without touching the
		// upstream or this deadline. Long running queries are not idle time.
		if handler.used {
			deadline = time.Now().Add(config.idle)
		}
	}
}

// replicaSSHStream uses the established Toolforge SSH destination and OpenSSH
// authentication. -W creates one fixed direct-tcpip channel, with no remote shell.
func replicaSSHStream(ctx context.Context, connection toolforge.Connection, host string) (net.Conn, error) {
	ctx, cancel := context.WithCancel(ctx)
	args := append([]string(nil), connection.SSHOptions...)
	args = append(args, "-T", "-o", "ClearAllForwardings=yes", "-o", "PermitLocalCommand=no",
		"-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "BatchMode=yes",
		"-o", "IgnoreUnknown=ForkAfterAuthentication", "-o", "ForkAfterAuthentication=no",
		"-W", net.JoinHostPort(host, "3306"), connection.Host)
	command := osexec.CommandContext(ctx, "ssh", args...) // #nosec G204 -- destination is generated by replicaForwards, never an arbitrary host.
	input, err := command.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		cancel()
		_ = input.Close()
		return nil, err
	}
	if err := command.Start(); err != nil {
		cancel()
		_ = input.Close()
		_ = output.Close()
		return nil, err
	}
	local, remote := net.Pipe()
	go func() { _, _ = io.Copy(input, remote); _ = input.Close() }()
	go func() { _, _ = io.Copy(remote, output); _ = remote.Close(); cancel(); _ = command.Wait() }()
	return &replicaSSHConn{Conn: local, cancel: cancel, remote: remote}, nil
}

type replicaSSHConn struct {
	net.Conn
	cancel context.CancelFunc
	remote net.Conn
}

func (c *replicaSSHConn) Close() error {
	c.cancel()
	_ = c.remote.Close()
	return c.Conn.Close()
}
