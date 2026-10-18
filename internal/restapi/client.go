// Package restapi provides the HTTP request handling shared by Wikimedia API commands.
package restapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	BaseURL   = "https://wikimedia.org/api/rest_v1/"
	UserAgent = "mwcli (https://www.mediawiki.org/wiki/CLI)"
)

// Request accepts an absolute URL or an endpoint relative to baseURL.
func Request(ctx context.Context, method, baseURL, endpoint string, headers http.Header, body io.Reader) (*http.Response, error) {
	base, err := url.Parse(baseURL)
	if err != nil || !validURL(base) || base.RawQuery != "" {
		return nil, fmt.Errorf("invalid API base URL")
	}
	target, err := url.Parse(endpoint)
	if err != nil || strings.HasPrefix(endpoint, "//") {
		return nil, fmt.Errorf("invalid API endpoint")
	}
	if !target.IsAbs() {
		base.Path = strings.TrimRight(base.Path, "/") + "/"
		base.RawPath = ""
		target, err = url.Parse(strings.TrimLeft(endpoint, "/"))
		if err != nil {
			return nil, fmt.Errorf("invalid API endpoint")
		}
		target = base.ResolveReference(target)
	}
	if !validURL(target) {
		return nil, fmt.Errorf("API URL must use HTTP or HTTPS without credentials or a fragment")
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header = headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent)
	}
	// A REST request should not silently become a GET or move credentials to a
	// different endpoint because of an HTTP redirect.
	client := *http.DefaultClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return client.Do(req)
}

func validURL(u *url.URL) bool {
	return u != nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.Fragment == ""
}

// CheckStatus includes bounded API error details and rate-limit guidance.
func CheckStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := fmt.Sprintf("API returned HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	if retry := resp.Header.Get("Retry-After"); retry != "" {
		message += " (Retry-After: " + retry + ")"
	}
	if detail := strings.TrimSpace(string(body)); detail != "" {
		message += ": " + detail
	}
	return fmt.Errorf("%s", message)
}
