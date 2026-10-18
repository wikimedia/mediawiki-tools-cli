package events

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

const (
	eventStreamsURL       = "https://stream.wikimedia.org"
	eventStreamsUserAgent = "mwcli WikimediaEventStream/1.0 (https://www.mediawiki.org/wiki/CLI)"
)

type tailOptions struct {
	stream, wiki, title, user string
	namespace, limit          int
	since, until              time.Time
}

type streamEvent struct {
	Meta struct {
		DT     string `json:"dt"`
		Domain string `json:"domain"`
	} `json:"meta"`
	DT        string          `json:"dt"`
	Timestamp json.RawMessage `json:"timestamp"`
}

type recentChange struct {
	Wiki      string `json:"wiki"`
	Title     string `json:"title"`
	User      string `json:"user"`
	Namespace int    `json:"namespace"`
}

func NewTailCmd() *cobra.Command {
	var opts tailOptions
	var since, until string
	cmd := &cobra.Command{
		Use:     "tail",
		Short:   "Follow a Wikimedia EventStreams feed",
		Long:    "Follow Wikimedia EventStreams, reconnecting after interruptions. Events are indented and coloured in a terminal, or compact JSON Lines when piped. Set NO_COLOR to disable colour. Canary test events are excluded.\n\nUse --since and --until with RFC3339 timestamps, for example 2026-10-18T12:00:00Z. The start is inclusive and the end is exclusive, using meta.dt (falling back to dt or timestamp). --until waits until every stream partition reaches the end time. Events can arrive out of order, so this is a best-effort window, not a complete historical export. Available history depends on stream retention.",
		Example: "tail --wiki enwiki\ntail --wiki enwiki --namespace 0\ntail --stream revision-create --limit 20\ntail --since 2026-10-18T12:00:00Z --until 2026-10-18T12:05:00Z",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.limit < 0 {
				return fmt.Errorf("--limit must be zero or greater")
			}
			var err error
			opts.since, err = parseEventTimeFlag("since", since)
			if err != nil {
				return err
			}
			opts.until, err = parseEventTimeFlag("until", until)
			if err != nil {
				return err
			}
			if !opts.since.IsZero() && !opts.until.IsZero() && !opts.until.After(opts.since) {
				return fmt.Errorf("--until must be after --since")
			}
			return tail(cmd.Context(), opts, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&opts.stream, "stream", "recentchange", "EventStreams stream name")
	cmd.Flags().StringVar(&opts.wiki, "wiki", "", "Filter by wiki database name, for example enwiki")
	cmd.Flags().StringVar(&opts.title, "title", "", "Filter by exact page title")
	cmd.Flags().StringVar(&opts.user, "user", "", "Filter by exact username")
	cmd.Flags().IntVar(&opts.namespace, "namespace", -1, "Filter by namespace ID")
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Stop after this many matching events (0 means follow indefinitely)")
	cmd.Flags().StringVar(&since, "since", "", "Start at this RFC3339 timestamp (inclusive; subject to stream retention)")
	cmd.Flags().StringVar(&until, "until", "", "Stop once all partitions reach this RFC3339 timestamp (exclusive)")
	return cmd
}

func tail(ctx context.Context, opts tailOptions, out io.Writer) error {
	stream, wiki, title, user := opts.stream, opts.wiki, opts.title, opts.user
	namespace, limit := opts.namespace, opts.limit
	if stream == "" || strings.Contains(stream, "/") {
		return fmt.Errorf("--stream must be a single stream name")
	}
	if stream != "recentchange" && stream != "mediawiki.recentchange" && (wiki != "" || title != "" || user != "" || namespace >= 0) {
		return fmt.Errorf("--wiki, --title, --user, and --namespace filters are currently supported with --stream recentchange")
	}
	endpoint := eventStreamsURL + "/v2/stream/" + url.PathEscape(stream)
	if !opts.since.IsZero() {
		endpoint += "?" + url.Values{"since": {opts.since.Format(time.RFC3339Nano)}}.Encode()
	}
	client := &http.Client{}
	lastID, printed := "", 0
	delay := time.Second
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("User-Agent", eventStreamsUserAgent)
		if lastID != "" {
			req.Header.Set("Last-Event-ID", lastID)
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if err := wait(ctx, delay); err != nil {
				return nil
			}
			delay = nextDelay(delay)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			return fmt.Errorf("EventStreams returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
		delay = time.Second
		err = readSSE(ctx, resp.Body, func(id, event string, data []byte) (emitErr error) {
			if id != "" {
				lastID = id
			}
			if len(data) == 0 || event == "keepalive" {
				return nil
			}
			if event == "error" {
				return fmt.Errorf("EventStreams error: %s", strings.TrimSpace(string(data)))
			}
			var message streamEvent
			if err := json.Unmarshal(data, &message); err != nil {
				return fmt.Errorf("invalid event JSON: %w", err)
			}
			if !opts.until.IsZero() {
				complete, err := partitionsPastUntil(id, opts.until)
				if err != nil {
					return err
				}
				// Check completion after handling the current event: its timestamp
				// can precede the next position recorded in the SSE checkpoint.
				defer func() {
					if complete && emitErr == nil {
						emitErr = errStop
					}
				}()
			}
			if message.Meta.Domain == "canary" {
				return nil
			}
			if !opts.since.IsZero() || !opts.until.IsZero() {
				stamp, err := message.eventTime()
				if err != nil {
					return err
				}
				if !opts.until.IsZero() && !stamp.Before(opts.until) {
					return nil
				}
				if !opts.since.IsZero() && stamp.Before(opts.since) {
					return nil
				}
			}
			var rc recentChange
			if wiki != "" || title != "" || user != "" || namespace >= 0 {
				if err := json.Unmarshal(data, &rc); err != nil {
					return fmt.Errorf("invalid recentchange event: %w", err)
				}
			}
			if wiki != "" && !strings.EqualFold(wiki, rc.Wiki) {
				return nil
			}
			if title != "" && title != rc.Title {
				return nil
			}
			if user != "" && user != rc.User {
				return nil
			}
			if namespace >= 0 && namespace != rc.Namespace {
				return nil
			}
			if err := output.PrintJSONEvent(out, data); err != nil {
				return err
			}
			printed++
			if limit > 0 && printed >= limit {
				return errStop
			}
			return nil
		})
		resp.Body.Close()
		if errors.Is(err, errStop) || ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		if err := wait(ctx, delay); err != nil {
			return nil
		}
		delay = nextDelay(delay)
	}
}

var errStop = errors.New("stop stream")

func readSSE(ctx context.Context, body io.Reader, emit func(id, event string, data []byte) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var id, event string
	var data []string
	dispatch := func() error {
		if len(data) > 0 {
			if err := emit(id, event, []byte(strings.Join(data, "\n"))); err != nil {
				return err
			}
		}
		id, event, data = "", "", nil
		return nil
	}
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "id":
			id = value
		case "event":
			event = value
		case "data":
			data = append(data, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return dispatch()
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextDelay(delay time.Duration) time.Duration {
	if delay >= 15*time.Second {
		return 30 * time.Second
	}
	delay *= 2
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
}

func parseEventTimeFlag(name, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	stamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("--%s must be an RFC3339 timestamp, for example 2026-10-18T12:00:00Z", name)
	}
	return stamp, nil
}

func (event streamEvent) eventTime() (time.Time, error) {
	value := event.Meta.DT
	if value == "" {
		value = event.DT
	}
	if value != "" {
		stamp, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid event timestamp %q: %w", value, err)
		}
		return stamp, nil
	}
	if len(event.Timestamp) > 0 {
		var seconds int64
		if err := json.Unmarshal(event.Timestamp, &seconds); err != nil {
			return time.Time{}, fmt.Errorf("invalid event timestamp: %w", err)
		}
		return time.Unix(seconds, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("cannot apply time bounds: event has no meta.dt, dt or timestamp")
}

// SSE checkpoints include all assigned topic partitions. An offset-only entry
// has not yielded a timestamp yet, so one faster partition cannot end the window.
func partitionsPastUntil(id string, until time.Time) (bool, error) {
	var positions []struct {
		Timestamp *int64 `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(id), &positions); err != nil {
		return false, fmt.Errorf("cannot track --until: invalid EventStreams partition checkpoint: %w", err)
	}
	if len(positions) == 0 {
		return false, fmt.Errorf("cannot track --until: empty EventStreams partition checkpoint")
	}
	for _, position := range positions {
		if position.Timestamp == nil || time.UnixMilli(*position.Timestamp).Before(until) {
			return false, nil
		}
	}
	return true, nil
}
