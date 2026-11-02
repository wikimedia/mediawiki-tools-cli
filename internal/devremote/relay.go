package devremote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// relayMessage multiplexes HTTP requests over an ordinary SSH exec channel.
// This also works with SSH gateways that do not implement reverse forwarding.
type relayMessage struct {
	ID            uint64 `json:"id,omitempty"`
	Port          int    `json:"port,omitempty"`
	Path          string `json:"path,omitempty"`
	Authorization string `json:"authorization,omitempty"`
	Data          []byte `json:"data,omitempty"`
	Status        int    `json:"status,omitempty"`
	Done          bool   `json:"done,omitempty"`
}

type relayPending struct {
	replies chan relayMessage
	done    <-chan struct{}
}

type relayWriter struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

func (w *relayWriter) send(m relayMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.encoder.Encode(m)
}

// serveRelay exposes a loopback HTTP endpoint only while its SSH channel lives.
func serveRelay(ctx context.Context, input io.Reader, output io.Writer) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	writer := &relayWriter{encoder: json.NewEncoder(output)}
	if err := writer.send(relayMessage{Port: listener.Addr().(*net.TCPAddr).Port}); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var next atomic.Uint64
	var pending sync.Map
	slots := make(chan struct{}, 4)
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || (r.URL.Path != "/invoke" && r.URL.Path != "/git") {
			http.Error(w, "unsupported bridge request", http.StatusNotFound)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		case <-ctx.Done():
			return
		default:
			http.Error(w, "bridge busy", http.StatusTooManyRequests)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 12<<20))
		if err != nil {
			http.Error(w, "bridge request too large", http.StatusRequestEntityTooLarge)
			return
		}
		id := next.Add(1)
		replies := make(chan relayMessage, 8)
		pending.Store(id, relayPending{replies: replies, done: r.Context().Done()})
		defer pending.Delete(id)
		if err := writer.send(relayMessage{ID: id, Path: r.URL.Path, Authorization: r.Header.Get("Authorization"), Data: body}); err != nil {
			return
		}
		for {
			select {
			case reply := <-replies:
				if reply.Status != 0 {
					w.WriteHeader(reply.Status)
				}
				if len(reply.Data) > 0 {
					if _, err := w.Write(reply.Data); err != nil {
						return
					}
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
				}
				if reply.Done {
					return
				}
			case <-ctx.Done():
				return
			case <-r.Context().Done():
				return
			}
		}
	})}
	defer server.Close()
	go server.Serve(listener)
	go func() { <-ctx.Done(); server.Close() }()
	decoder := json.NewDecoder(input)
	for {
		var m relayMessage
		if err := decoder.Decode(&m); err != nil {
			return err
		}
		if ch, ok := pending.Load(m.ID); ok {
			if err := deliverRelayReply(ctx, ch.(relayPending), m); err != nil {
				return err
			}
		}
	}
}

func startRelay(ctx context.Context, t transport, binary string, port int) (int, *exec.Cmd, error) {
	child := t.command(ctx, nil, quote(binary)+" __remote-relay")
	input, err := child.StdinPipe()
	if err != nil {
		return 0, nil, err
	}
	output, err := child.StdoutPipe()
	if err != nil {
		input.Close()
		return 0, nil, err
	}
	if err := child.Start(); err != nil {
		input.Close()
		return 0, nil, err
	}
	decoder := json.NewDecoder(output)
	var ready relayMessage
	if err := decoder.Decode(&ready); err != nil {
		child.Process.Kill()
		child.Wait()
		return 0, nil, fmt.Errorf("start remote session relay: %w", err)
	}
	if ready.Port < 1 || ready.Port > 65535 {
		child.Process.Kill()
		child.Wait()
		return 0, nil, fmt.Errorf("invalid relay port")
	}
	writer := &relayWriter{encoder: json.NewEncoder(input)}
	go func() {
		defer input.Close()
		client := &http.Client{Transport: &http.Transport{Proxy: nil}}
		defer client.CloseIdleConnections()
		var workers sync.WaitGroup
		defer workers.Wait()
		for {
			var request relayMessage
			if err := decoder.Decode(&request); err != nil {
				return
			}
			workers.Add(1)
			go func(m relayMessage) { defer workers.Done(); relayRequest(ctx, client, writer, port, m) }(request)
		}
	}()
	return ready.Port, child, nil
}

func relayRequest(ctx context.Context, client *http.Client, writer *relayWriter, port int, m relayMessage) {
	// Only the fixed loopback bridge is reachable, never an arbitrary URL.
	if m.Path != "/invoke" && m.Path != "/git" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d%s", port, m.Path), bytes.NewReader(m.Data))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", m.Authorization)
	response, err := client.Do(req)
	if err != nil {
		writer.send(relayMessage{ID: m.ID, Status: http.StatusBadGateway, Data: []byte("local session unavailable"), Done: true})
		return
	}
	defer response.Body.Close()
	writer.send(relayMessage{ID: m.ID, Status: response.StatusCode})
	buffer := make([]byte, 32<<10)
	for {
		n, err := response.Body.Read(buffer)
		if n > 0 {
			if writer.send(relayMessage{ID: m.ID, Data: buffer[:n]}) != nil {
				return
			}
		}
		if err != nil {
			break
		}
	}
	writer.send(relayMessage{ID: m.ID, Done: true})
}

func deliverRelayReply(ctx context.Context, pending relayPending, message relayMessage) error {
	select {
	case pending.replies <- message:
		return nil
	case <-pending.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
