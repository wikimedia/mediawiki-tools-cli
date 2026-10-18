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

type recentChange struct {
	Wiki      string `json:"wiki"`
	Title     string `json:"title"`
	User      string `json:"user"`
	Namespace int    `json:"namespace"`
}

func NewTailCmd() *cobra.Command {
	var stream, wiki, title, user string
	var namespace, limit int
	cmd := &cobra.Command{
		Use:     "tail",
		Short:   "Follow a Wikimedia EventStreams feed",
		Long:    "Follow Wikimedia EventStreams, reconnecting after interruptions. Events are indented and coloured in a terminal, or compact JSON Lines when piped. Set NO_COLOR to disable colour.",
		Example: "tail --wiki enwiki\ntail --wiki enwiki --namespace 0\ntail --stream revision-create --limit 20",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 0 {
				return fmt.Errorf("--limit must be zero or greater")
			}
			return tail(cmd.Context(), stream, wiki, title, user, namespace, limit, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&stream, "stream", "recentchange", "EventStreams stream name")
	cmd.Flags().StringVar(&wiki, "wiki", "", "Filter by wiki database name, for example enwiki")
	cmd.Flags().StringVar(&title, "title", "", "Filter by exact page title")
	cmd.Flags().StringVar(&user, "user", "", "Filter by exact username")
	cmd.Flags().IntVar(&namespace, "namespace", -1, "Filter by namespace ID")
	cmd.Flags().IntVar(&limit, "limit", 0, "Stop after this many matching events (0 means follow indefinitely)")
	return cmd
}

func tail(ctx context.Context, stream, wiki, title, user string, namespace, limit int, out io.Writer) error {
	if stream == "" || strings.Contains(stream, "/") {
		return fmt.Errorf("--stream must be a single stream name")
	}
	if stream != "recentchange" && (wiki != "" || title != "" || user != "" || namespace >= 0) {
		return fmt.Errorf("--wiki, --title, --user, and --namespace filters are currently supported with --stream recentchange")
	}
	endpoint := "https://stream.wikimedia.org/v2/stream/" + url.PathEscape(stream)
	client := &http.Client{}
	lastID, printed := "", 0
	delay := time.Second
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("User-Agent", "mwcli WikimediaEventStream/1.0 (https://www.mediawiki.org/wiki/CLI)")
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
		err = readSSE(ctx, resp.Body, func(id, event string, data []byte) error {
			if id != "" {
				lastID = id
			}
			if len(data) == 0 || event == "keepalive" {
				return nil
			}
			if event == "error" {
				return fmt.Errorf("EventStreams error: %s", strings.TrimSpace(string(data)))
			}
			var rc recentChange
			_ = json.Unmarshal(data, &rc)
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
