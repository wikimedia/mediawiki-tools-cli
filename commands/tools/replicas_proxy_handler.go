package tools

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/pingcap/tidb/pkg/parser/charset"
	server "gitlab.wikimedia.org/repos/releng/cli/third_party/go-mysql-server"
)

const replicaResponseLimit = 16 * 1024 * 1024

type replicaProxyHandler struct {
	ctx          context.Context
	config       replicaProxyConfig
	down         *server.Conn
	up           *client.Conn
	transport    *replicaLimitedConn
	stop         func() bool
	statements   map[any]*client.Stmt
	used, failed bool
}

func (h *replicaProxyHandler) close() {
	if h.stop != nil {
		h.stop()
	}
	if h.transport != nil {
		_ = h.transport.Close()
	}
}

func (h *replicaProxyHandler) connect() error {
	h.used = true
	if h.up != nil {
		h.transport.remaining = replicaResponseLimit
		return nil
	}
	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		conn, err := h.config.dial(ctx)
		if err != nil {
			return nil, err
		}
		h.transport = &replicaLimitedConn{Conn: conn, remaining: replicaResponseLimit}
		h.stop = context.AfterFunc(ctx, func() { _ = conn.Close() })
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		return h.transport, nil
	}
	collation, err := charset.GetCollationByID(int(h.down.Charset()))
	if err != nil {
		return mysql.NewError(mysql.ER_NOT_SUPPORTED_YET, "unsupported client collation")
	}
	h.up, err = client.ConnectWithDialer(h.ctx, "tcp", "replica:3306", h.config.credentials.User, h.config.credentials.Password, h.config.database, dial,
		func(up *client.Conn) error {
			if h.down.Capability()&mysql.CLIENT_FOUND_ROWS != 0 {
				up.SetCapability(mysql.CLIENT_FOUND_ROWS)
			}
			return up.SetCollation(collation.Name)
		})
	if err != nil {
		h.failed = true
		return fmt.Errorf("could not authenticate to the selected replica over SSH; check Toolforge access and credentials")
	}
	_ = h.transport.SetDeadline(time.Time{})
	h.transport.remaining = replicaResponseLimit
	return nil
}

func (h *replicaProxyHandler) result(result *mysql.Result, err error) (*mysql.Result, error) {
	if err != nil {
		var sqlError *mysql.MyError
		if errors.As(err, &sqlError) {
			redacted := *sqlError
			redacted.Message = strings.ReplaceAll(redacted.Message, h.config.credentials.Password, "[redacted]")
			return nil, &redacted
		}
		h.failed = true
		return nil, fmt.Errorf("replica connection failed or response exceeded 16 MiB; reconnect explicitly")
	}
	if result != nil && result.Status&mysql.SERVER_MORE_RESULTS_EXISTS != 0 {
		h.failed = true
		return nil, fmt.Errorf("multiple result sets are not supported; reconnect explicitly")
	}
	if result != nil {
		h.down.UnsetStatus(^uint16(0))
		h.down.SetStatus(result.Status)
		h.down.SetWarnings(result.Warnings)
	}
	return result, nil
}

func (h *replicaProxyHandler) UseDB(database string) error {
	if database != "" && database != h.config.database {
		return mysql.NewError(mysql.ER_BAD_DB_ERROR, "this proxy accepts only its selected wiki database")
	}
	// The library calls UseDB during the frontend handshake, before checking
	// its password. Never open an upstream until frontend authentication ends.
	if h.down == nil {
		return nil
	}
	if err := h.connect(); err != nil {
		return err
	}
	err := h.up.UseDB(h.config.database)
	_, err = h.result(nil, err)
	return err
}

// Frontend thread IDs are unrelated to upstream thread IDs. Inspect the
// leading SQL keyword, skipping ordinary MySQL comments, before forwarding.
func validateReplicaQuery(query string) error {
	for {
		query = strings.TrimLeft(query, " \t\r\n\v\f")
		switch {
		case strings.HasPrefix(query, "/*!") || strings.HasPrefix(query, "/*M!") || strings.HasPrefix(query, "/*m!"):
			// Executable comments can contain a complete KILL statement.
			return mysql.NewError(mysql.ER_NOT_SUPPORTED_YET, "leading executable comments are not supported by this experimental proxy")
		case strings.HasPrefix(query, "/*"):
			end := strings.Index(query[2:], "*/")
			if end < 0 {
				return mysql.NewError(mysql.ER_PARSE_ERROR, "unterminated SQL comment")
			}
			query = query[end+4:]
		case strings.HasPrefix(query, "#") || (strings.HasPrefix(query, "--") && (len(query) == 2 || query[2] <= ' ')):
			end := strings.IndexAny(query, "\r\n")
			if end < 0 {
				return nil
			}
			query = query[end+1:]
		default:
			end := 0
			for end < len(query) && ((query[end] >= 'a' && query[end] <= 'z') || (query[end] >= 'A' && query[end] <= 'Z')) {
				end++
			}
			if strings.EqualFold(query[:end], "KILL") {
				return mysql.NewError(mysql.ER_NOT_SUPPORTED_YET, "KILL and client interrupt requests are not supported; disconnect the original client to cancel its upstream session")
			}
			return nil
		}
	}
}

func (h *replicaProxyHandler) HandleQuery(query string) (*mysql.Result, error) {
	if err := validateReplicaQuery(query); err != nil {
		return nil, err
	}
	if err := h.connect(); err != nil {
		return nil, err
	}
	return h.result(h.up.Execute(query))
}

func (h *replicaProxyHandler) HandleFieldList(table, wildcard string) ([]*mysql.Field, error) {
	if err := h.connect(); err != nil {
		return nil, err
	}
	fields, err := h.up.FieldList(table, wildcard)
	_, err = h.result(nil, err)
	return fields, err
}

func (h *replicaProxyHandler) HandleStmtPrepare(query string) (int, int, any, error) {
	if err := validateReplicaQuery(query); err != nil {
		return 0, 0, nil, err
	}
	if err := h.connect(); err != nil {
		return 0, 0, nil, err
	}
	prepared, err := h.up.Prepare(query)
	if err != nil {
		_, err = h.result(nil, err)
		return 0, 0, nil, err
	}
	// Preserve the real column and parameter metadata in the frontend response.
	key := &prepared.PreparedStmt
	h.statements[key] = prepared
	return prepared.ParamNum(), prepared.ColumnNum(), key, nil
}

func (h *replicaProxyHandler) HandleStmtExecute(key any, _ string, args []any) (*mysql.Result, error) {
	prepared, ok := h.statements[key]
	if !ok {
		return nil, fmt.Errorf("unknown prepared statement")
	}
	if err := h.connect(); err != nil {
		return nil, err
	}
	return h.result(prepared.Execute(args...))
}

func (h *replicaProxyHandler) HandleStmtClose(key any) error {
	prepared, ok := h.statements[key]
	if !ok {
		return nil
	}
	h.used = true
	delete(h.statements, key)
	_, err := h.result(nil, prepared.Close())
	return err
}

func (*replicaProxyHandler) HandleOtherCommand(_ byte, _ []byte) error {
	return mysql.NewError(mysql.ER_NOT_SUPPORTED_YET, "command is not supported by this experimental replica proxy")
}

// Cap upstream data before the protocol library buffers a result. No pool or
// automatic retry is used, so an interrupted response must end the session.
type replicaLimitedConn struct {
	net.Conn
	remaining int
}

func (c *replicaLimitedConn) Read(buffer []byte) (int, error) {
	if c.remaining <= 0 {
		_ = c.Conn.Close()
		return 0, fmt.Errorf("replica response limit exceeded")
	}
	if len(buffer) > c.remaining {
		buffer = buffer[:c.remaining]
	}
	n, err := c.Conn.Read(buffer)
	c.remaining -= n
	return n, err
}

// Set a fresh deadline when writing, rather than starting a write timeout
// before a potentially long-running analytics query.
type replicaClientConn struct{ net.Conn }

func (c replicaClientConn) Write(data []byte) (int, error) {
	_ = c.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	return c.Conn.Write(data)
}
