package liftwing

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

const (
	baseURL     = "https://api.wikimedia.org"
	specURL     = baseURL + "/service/lw/specs/openapi.yaml"
	modelPrefix = "/service/lw/inference/v1/models/"
	maxInput    = 4 << 20
	maxResponse = 16 << 20
)

func request(ctx context.Context, method, endpoint string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, application/yaml")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := cli.NewHTTPClient()
	client.Timeout = 60 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Lift Wing request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		retry := ""
		if resp.StatusCode == http.StatusTooManyRequests && resp.Header.Get("Retry-After") != "" {
			retry = "; Retry-After: " + resp.Header.Get("Retry-After")
		}
		return nil, fmt.Errorf("Lift Wing HTTP %d %s%s: %s", resp.StatusCode, http.StatusText(resp.StatusCode), retry, strings.TrimSpace(string(message)))
	}
	return readLimited(resp.Body, maxResponse)
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("Lift Wing data exceeds %d byte limit", limit)
	}
	return data, nil
}
