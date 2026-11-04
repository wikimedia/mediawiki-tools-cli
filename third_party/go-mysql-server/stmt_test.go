package server

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/go-mysql-org/go-mysql/mysql"
)

type streamedParameterHandler struct {
	EmptyHandler
	values []any
	err    error
}

func (h *streamedParameterHandler) HandleStmtExecute(_ any, _ string, args []any) (*mysql.Result, error) {
	h.values = append([]any(nil), args...)
	return mysql.NewResult(nil), h.err
}

func streamedParameterConn(params int) (*Conn, *Stmt, *streamedParameterHandler) {
	h := &streamedParameterHandler{}
	s := &Stmt{}
	s.ID = 1
	s.Rest(params, 0, nil)
	return &Conn{stmts: map[uint32]*Stmt{1: s}, h: h}, s, h
}

func sendParameterChunk(t *testing.T, c *Conn, index uint16, chunk []byte) {
	t.Helper()
	data := binary.LittleEndian.AppendUint32(nil, 1)
	data = binary.LittleEndian.AppendUint16(data, index)
	data = append(data, chunk...)
	if err := c.handleStmtSendLongData(data); err != nil {
		t.Fatal(err)
	}
}

func executeParameterPacket(types, values []byte) []byte {
	data := binary.LittleEndian.AppendUint32(nil, 1)
	data = append(data, 0, 1, 0, 0, 0, 0) // cursor, iteration count, NULL bitmap
	if types == nil {
		data = append(data, 0)
	} else {
		data = append(data, 1)
		data = append(data, types...)
	}
	return append(data, values...)
}

func TestStreamedParametersMixedWithInlineAndCachedTypes(t *testing.T) {
	c, s, h := streamedParameterConn(2)
	sendParameterChunk(t, c, 0, []byte("hello"))
	sendParameterChunk(t, c, 0, []byte(" world"))
	types := []byte{mysql.MYSQL_TYPE_VAR_STRING, 0, mysql.MYSQL_TYPE_LONGLONG, 0}
	if _, err := c.handleStmtExecute(executeParameterPacket(types, binary.LittleEndian.AppendUint64(nil, 42))); err != nil {
		t.Fatal(err)
	}
	if value, ok := h.values[0].(mysql.TypedBytes); !ok || string(value.Bytes) != "hello world" {
		t.Fatalf("streamed value=%v", h.values[0])
	}
	if h.values[1] != int64(42) {
		t.Fatalf("inline value=%v", h.values[1])
	}
	if len(s.longData) != 0 {
		t.Fatal("execute retained streamed bytes")
	}
	sendParameterChunk(t, c, 0, nil) // an explicitly streamed empty string
	if _, err := c.handleStmtExecute(executeParameterPacket(nil, binary.LittleEndian.AppendUint64(nil, 43))); err != nil {
		t.Fatal(err)
	}
	if value, ok := h.values[0].(mysql.TypedBytes); !ok || len(value.Bytes) != 0 {
		t.Fatalf("empty streamed value=%v", h.values[0])
	}
	if h.values[1] != int64(43) {
		t.Fatalf("inline value=%v", h.values[1])
	}
}

func TestStreamedParameterCleanupAfterErrorResetAndClose(t *testing.T) {
	for _, action := range []string{"handler-error", "malformed-execute", "short-execute", "reset", "close"} {
		t.Run(action, func(t *testing.T) {
			c, s, h := streamedParameterConn(1)
			sendParameterChunk(t, c, 0, []byte("discard"))
			switch action {
			case "handler-error":
				h.err = fmt.Errorf("query failed")
				if _, err := c.handleStmtExecute(executeParameterPacket([]byte{mysql.MYSQL_TYPE_STRING, 0}, nil)); err == nil {
					t.Fatal("missing query error")
				}
			case "malformed-execute":
				if _, err := c.handleStmtExecute(executeParameterPacket(nil, nil)); err == nil {
					t.Fatal("accepted missing parameter types")
				}
			case "short-execute":
				if _, err := c.handleStmtExecute(binary.LittleEndian.AppendUint32(nil, 1)); err == nil {
					t.Fatal("accepted truncated execution")
				}
			case "reset":
				if _, err := c.handleStmtReset(binary.LittleEndian.AppendUint32(nil, 1)); err != nil {
					t.Fatal(err)
				}
			case "close":
				if err := c.handleStmtClose(binary.LittleEndian.AppendUint32(nil, 1)); err != nil {
					t.Fatal(err)
				}
			}
			if len(s.longData) != 0 || s.longDataBytes != 0 || s.longDataErr != nil {
				t.Fatal("statement retained streamed state")
			}
		})
	}
}

func TestStreamedParameterLimitReportedOnExecute(t *testing.T) {
	c, s, _ := streamedParameterConn(1)
	sendParameterChunk(t, c, 0, make([]byte, 16*1024*1024))
	sendParameterChunk(t, c, 0, []byte("excess"))
	if s.longDataErr == nil || len(s.longData) != 0 {
		t.Fatal("oversized stream was not discarded")
	}
	if _, err := c.handleStmtExecute(executeParameterPacket([]byte{mysql.MYSQL_TYPE_BLOB, 0}, nil)); err == nil {
		t.Fatal("execute ignored streamed size error")
	}
	if s.longDataErr != nil || s.longDataBytes != 0 {
		t.Fatal("failed execute retained streamed state")
	}
}
