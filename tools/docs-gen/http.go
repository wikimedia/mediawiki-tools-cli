package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"time"

	mwclient "cgt.name/pkg/go-mwclient"
)

// The API library decodes error pages as JSON without checking their HTTP
// status. Keep diagnostics useful without printing response bodies or tokens.
type documentationTransport struct {
	base http.RoundTripper
	wait func(context.Context, time.Duration) error
}

func waitForDocumentationRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (t documentationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		response, err := t.base.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 32<<20+1))
		response.Body.Close()
		if readErr == nil && len(body) <= 32<<20 && response.StatusCode >= 200 && response.StatusCode < 300 && json.Valid(body) {
			response.Body = io.NopCloser(bytes.NewReader(body))
			return response, nil
		}
		transient := response.StatusCode == 429 || response.StatusCode == 502 || response.StatusCode == 503 || response.StatusCode == 504 || (response.StatusCode >= 200 && response.StatusCode < 300)
		// Never replay a write here: the edit may have reached the wiki even
		// when a gateway returned HTML. publishDocumentationEdit rechecks it.
		if req.Method == http.MethodGet && transient && attempt < 2 {
			delay := time.Duration(attempt+1) * time.Second
			if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds > 0 {
				delay = time.Duration(seconds) * time.Second
			} else if date, err := http.ParseTime(response.Header.Get("Retry-After")); err == nil && time.Until(date) > delay {
				delay = time.Until(date)
			}
			if delay <= time.Minute {
				if err := t.wait(req.Context(), delay); err != nil {
					return nil, err
				}
				continue
			}
		}
		return nil, fmt.Errorf("documentation API returned HTTP %d (%s), content type %q; response was not usable JSON", response.StatusCode, http.StatusText(response.StatusCode), response.Header.Get("Content-Type"))
	}
}

func documentationHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar, Timeout: time.Minute,
		Transport:     documentationTransport{base: http.DefaultTransport, wait: waitForDocumentationRetry},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

var publicationRetryWait = time.Sleep

func publishDocumentationEdit(client *mwclient.Client, title, desired, summary, timestamp string, original snapshot) error {
	for attempt := 0; ; attempt++ {
		err := client.Edit(editParameters(title, desired, summary, timestamp, original))
		if err == nil || errors.Is(err, mwclient.ErrEditNoChange) {
			return nil
		}
		// Permission and edit-conflict errors are definitive. Unknown HTTP or
		// decoding failures have an uncertain outcome, so inspect before retrying.
		var apiError mwclient.APIError
		if errors.As(err, &apiError) || strings.Contains(err.Error(), "documentation API returned HTTP 4") {
			return err
		}
		latest, readErr := fetchPages(client, []string{title})
		if readErr != nil {
			return fmt.Errorf("%w; could not check whether the edit completed: %v", err, readErr)
		}
		current := latest.Pages[title]
		if !current.Missing && samePublicationText(current.Text, desired) {
			return nil
		}
		if current.Missing != original.Missing || current.Revision != original.Revision || current.Text != original.Text {
			return fmt.Errorf("%w; page changed after the failed response; refresh and review before publishing again", err)
		}
		if attempt >= 2 {
			return err
		}
		fmt.Printf("Retrying %s after an API failure; its revision is unchanged.\n", title)
		publicationRetryWait(time.Duration(attempt+1) * time.Second)
	}
}
