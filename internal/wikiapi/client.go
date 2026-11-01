// Package wikiapi provides installation-scoped Action and REST requests.
package wikiapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/restapi"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

// Client never follows redirects, or sends requests outside this installation.
type Client struct {
	Endpoint string
	HTTP     *http.Client
}

func New(endpoint string, authenticated *http.Client) (*Client, error) {
	endpoint, err := wikiauth.Endpoint(endpoint)
	if err != nil {
		return nil, err
	}
	u, _ := url.Parse(endpoint)
	if !wikiauth.AllowsAPIRequest(endpoint, u, false) {
		return nil, fmt.Errorf("wiki endpoint contains a noncanonical path")
	}
	client := *authenticated
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client.Transport = scopedTransport{endpoint, client.Transport}
	return &Client{Endpoint: endpoint, HTTP: &client}, nil
}

type scopedTransport struct {
	endpoint string
	base     http.RoundTripper
}

func (t scopedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !wikiauth.AllowsAPIRequest(t.endpoint, req.URL, true) {
		return nil, fmt.Errorf("request is outside the selected wiki installation")
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

// APIError includes Action errors, which may arrive in an HTTP 200 response.
type APIError struct {
	Code string `json:"code"`
	Info string `json:"info"`
}

func (e *APIError) Error() string { return fmt.Sprintf("Action API error %s: %s", e.Code, e.Info) }

func (c *Client) Action(ctx context.Context, method string, fields url.Values) ([]byte, error) {
	fields = cloneValues(fields)
	if fields.Get("format") == "" {
		fields.Set("format", "json")
	}
	if fields.Get("formatversion") == "" {
		fields.Set("formatversion", "2")
	}
	target := c.Endpoint
	var body io.Reader
	headers := make(http.Header)
	if method == http.MethodGet {
		target += "?" + fields.Encode()
	} else {
		body = strings.NewReader(fields.Encode())
		headers.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	data, err := c.request(ctx, method, target, headers, body)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Error  *APIError  `json:"error"`
		Errors []APIError `json:"errors"`
	}
	if json.Unmarshal(data, &envelope) == nil {
		if envelope.Error != nil {
			return nil, envelope.Error
		}
		if len(envelope.Errors) > 0 {
			return nil, &envelope.Errors[0]
		}
	}
	return data, nil
}

func cloneValues(fields url.Values) url.Values {
	result := make(url.Values, len(fields))
	for k, v := range fields {
		result[k] = append([]string(nil), v...)
	}
	return result
}

// RESTPath resolves a path relative to rest.php, never to an arbitrary URL.
func (c *Client) RESTPath(route string) (string, error) {
	u, err := url.Parse(route)
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil || u.Fragment != "" || strings.HasPrefix(route, "//") {
		return "", fmt.Errorf("REST path must be relative to the selected wiki's rest.php")
	}
	base, _ := url.Parse(strings.TrimSuffix(c.Endpoint, "api.php") + "rest.php/")
	// A leading slash denotes a REST path, not an origin-root path.
	u.Path = strings.TrimLeft(u.Path, "/")
	if u.RawPath != "" {
		u.RawPath = strings.TrimLeft(u.RawPath, "/")
	}
	target := *base
	target.Path += u.Path
	if u.RawPath != "" {
		target.RawPath = base.EscapedPath() + u.RawPath
	}
	target.RawQuery = u.RawQuery
	if !wikiauth.AllowsAPIRequest(c.Endpoint, &target, true) {
		return "", fmt.Errorf("REST path contains a noncanonical or out-of-scope path")
	}
	return target.String(), nil
}

func (c *Client) REST(ctx context.Context, method, route string, fields url.Values, body io.Reader, headers http.Header) ([]byte, error) {
	target, err := c.RESTPath(route)
	if err != nil {
		return nil, err
	}
	u, _ := url.Parse(target)
	query := u.Query()
	for k, values := range fields {
		for _, v := range values {
			query.Add(k, v)
		}
	}
	u.RawQuery = query.Encode()
	return c.request(ctx, method, u.String(), headers, body)
}

func (c *Client) request(ctx context.Context, method, target string, headers http.Header, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	req.Header = headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set("User-Agent", cli.HTTPUserAgent)
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	response, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := restapi.CheckStatus(response); err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	if _, err = io.Copy(&buffer, response.Body); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
