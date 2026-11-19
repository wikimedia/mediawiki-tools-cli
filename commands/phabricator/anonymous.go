package phabricator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

// A configured account keeps the authenticated path, including its errors.
func useAnonymousPhabricator(cmd *cobra.Command, site string) bool {
	return !commandFlagEnabled(cmd, "allow-non-public") && strings.TrimSpace(site) == "" &&
		phabProfiles.Default() == "" && !mwconfig.HasCredential("phabricator.key")
}

var errAnonymousNotVisible = errors.New("not found or not publicly visible")

type anonymousPhabricator struct {
	http *http.Client
}

func newAnonymousPhabricator() anonymousPhabricator {
	return anonymousPhabricator{http: &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: checkAnonymousRedirect,
	}}
}

func checkAnonymousRedirect(req *http.Request, previous []*http.Request) error {
	if req.URL.Scheme != "https" || req.URL.Host != "phabricator.wikimedia.org" || req.URL.User != nil {
		return fmt.Errorf("refusing Phabricator redirect outside %s", defaultPhabricatorURL)
	}
	if len(previous) >= 10 {
		return fmt.Errorf("too many Phabricator redirects")
	}
	return nil
}

// Anonymous reads use the public HTML pages, with no cookie jar or credentials.
func (c anonymousPhabricator) get(ctx context.Context, path string, params url.Values) (*goquery.Document, error) {
	target, err := url.Parse(defaultPhabricatorURL + path)
	if err != nil {
		return nil, err
	}
	target.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", cli.HTTPUserAgent())
	req.Header.Set("Accept", "text/html")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized:
		return nil, fmt.Errorf("Phabricator page %s: %w", path, errAnonymousNotVisible)
	case http.StatusOK:
	default:
		return nil, fmt.Errorf("Phabricator returned HTTP %s for %s", resp.Status, path)
	}
	const maxPageSize = 8 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPageSize {
		return nil, fmt.Errorf("Phabricator page exceeds the %d MiB limit", maxPageSize>>20)
	}
	return goquery.NewDocumentFromReader(bytes.NewReader(body))
}

func anonymousPageCursor(doc *goquery.Document, limit int) pageCursor {
	cursor := pageCursor{Limit: limit}
	doc.Find(".phui-pager-view a").Each(func(_ int, link *goquery.Selection) {
		target, err := url.Parse(link.AttrOr("href", ""))
		if err != nil {
			return
		}
		if after := target.Query().Get("after"); after != "" && strings.TrimSpace(link.Text()) == "Next" {
			cursor.After = &after
		}
		if before := target.Query().Get("before"); before != "" && strings.TrimSpace(link.Text()) == "Previous" {
			cursor.Before = &before
		}
	})
	return cursor
}
