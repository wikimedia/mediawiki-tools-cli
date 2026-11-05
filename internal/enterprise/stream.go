package enterprise

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

type StreamOptions struct {
	Resource   string
	Query      map[string]any
	Parts      []int
	Since      time.Time
	Limit      int
	Checkpoint string
}

type streamState struct {
	Scope string            `json:"scope"`
	Since map[string]string `json:"since_per_partition"`
}

// Stream emits one validated NDJSON record at a time. State is advanced only
// after the writer accepts an event. Replay can produce duplicates; consumers
// must be idempotent. Checkpoints are bound to the resource and query.
func (c *Client) Stream(ctx context.Context, opts StreamOptions, writer, diagnostics io.Writer) error {
	if opts.Resource != "articles" && opts.Resource != "wikidata" {
		return fmt.Errorf("invalid stream resource")
	}
	if opts.Limit < 0 {
		return fmt.Errorf("stream limit must be zero or greater")
	}
	if len(opts.Parts) == 0 {
		opts.Parts = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	}
	seen := map[int]bool{}
	for _, part := range opts.Parts {
		if part < 0 || part > 9 || seen[part] {
			return fmt.Errorf("stream parts must be unique integers from 0 to 9")
		}
		seen[part] = true
	}
	query := map[string]any{}
	for key, value := range opts.Query {
		query[key] = value
	}
	// Resume state always needs these fields, even when callers project fields.
	if fields, ok := query["fields"].([]string); ok && len(fields) != 0 {
		fields = append(append([]string{}, fields...), "event.partition", "event.date_published")
		query["fields"] = fields
	}
	query["parts"] = opts.Parts
	scope, err := json.Marshal(map[string]any{"endpoint": c.RealtimeURL + "/v2/" + opts.Resource, "query": query})
	if err != nil {
		return err
	}
	start := opts.Since
	if start.IsZero() {
		start = time.Now().UTC()
	}
	state := streamState{Scope: string(scope), Since: map[string]string{}}
	for _, part := range opts.Parts {
		state.Since[strconv.Itoa(part)] = start.Format(time.RFC3339Nano)
	}
	if opts.Checkpoint != "" {
		lock := flock.New(opts.Checkpoint + ".lock")
		ok, err := lock.TryLock()
		if err != nil || !ok {
			return fmt.Errorf("stream checkpoint is locked by another process")
		}
		defer lock.Unlock()
		var saved streamState
		if err := ReadState(opts.Checkpoint, &saved); err == nil {
			if !opts.Since.IsZero() {
				return fmt.Errorf("--since cannot override an existing checkpoint")
			}
			if saved.Scope != state.Scope || len(saved.Since) != len(state.Since) {
				return fmt.Errorf("checkpoint belongs to a different stream query")
			}
			for key := range state.Since {
				if _, err := time.Parse(time.RFC3339Nano, saved.Since[key]); err != nil {
					return fmt.Errorf("invalid timestamp in stream checkpoint")
				}
			}
			state = saved
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("could not load stream checkpoint: %w", err)
		}
		if err := WriteState(opts.Checkpoint, state); err != nil {
			return err
		}
	}
	count := 0
	delay := time.Second
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		query["since_per_partition"] = state.Since
		resp, err := c.Authorized(ctx, http.MethodPost, c.RealtimeURL+"/v2/"+opts.Resource, query, "application/x-ndjson")
		if err != nil {
			var status *HTTPError
			if errors.As(err, &status) || ctx.Err() != nil {
				return err
			}
			// Credential/provider errors must not trigger endless reconnects.
			if !errors.Is(err, errConnection) {
				return err
			}
		} else if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			if resp.StatusCode != 429 && resp.StatusCode < 500 {
				return &HTTPError{resp.StatusCode}
			}
		} else {
			if content := resp.Header.Get("Content-Type"); content != "" && !strings.Contains(content, "ndjson") && !strings.Contains(content, "jsonl") {
				resp.Body.Close()
				return fmt.Errorf("Enterprise did not return an NDJSON stream")
			}
			before := count
			err = readStream(resp.Body, func(line []byte) error {
				var record struct {
					Event struct {
						Partition *int   `json:"partition"`
						Published string `json:"date_published"`
					} `json:"event"`
				}
				if json.Unmarshal(line, &record) != nil || record.Event.Partition == nil || !seen[*record.Event.Partition] {
					return fmt.Errorf("invalid Enterprise stream event")
				}
				published, err := time.Parse(time.RFC3339Nano, record.Event.Published)
				if err != nil {
					return fmt.Errorf("invalid Enterprise event timestamp")
				}
				if _, err := writer.Write(append(append([]byte{}, line...), '\n')); err != nil {
					return &streamWriteError{err}
				}
				part := strconv.Itoa(*record.Event.Partition)
				previous, _ := time.Parse(time.RFC3339Nano, state.Since[part])
				if published.After(previous) {
					state.Since[part] = published.Format(time.RFC3339Nano)
				}
				if opts.Checkpoint != "" {
					if err := WriteState(opts.Checkpoint, state); err != nil {
						return &streamWriteError{err}
					}
				}
				count++
				if opts.Limit > 0 && count >= opts.Limit {
					return errStreamLimit
				}
				return nil
			})
			resp.Body.Close()
			if errors.Is(err, errStreamLimit) {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var failure *streamWriteError
			if errors.As(err, &failure) {
				return failure.err
			}
			if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
				// Retry transport failures, but surface malformed or oversized records.
				var networkError net.Error
				if !errors.As(err, &networkError) {
					return err
				}
			}
			if count > before {
				delay = time.Second
			}
		}
		if diagnostics != nil {
			fmt.Fprintf(diagnostics, "Enterprise stream disconnected; reconnecting in %s (replayed events may repeat).\n", delay)
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if delay < 30*time.Second {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}
}

var errStreamLimit = errors.New("stream limit reached")

type streamWriteError struct{ err error }

func (e *streamWriteError) Error() string { return e.err.Error() }

func readStream(reader io.Reader, emit func([]byte) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 64<<20)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if i := strings.IndexByte(string(data), '\n'); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF {
			// A dropped connection can end halfway through a record. Discard
			// that fragment; recovery starts from the last completed event.
			if json.Valid(data) {
				return len(data), data, nil
			}
			return len(data), nil, nil
		}
		return 0, nil, nil
	})
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		if err := emit(line); err != nil {
			return err
		}
	}
	return scanner.Err()
}
