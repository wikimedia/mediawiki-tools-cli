package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/restapi"
)

type dashboardSearchResult struct {
	UID         string `json:"uid"`
	Title       string `json:"title"`
	FolderTitle string `json:"folderTitle"`
}

func dashboardSearchEndpoint(query string, limit, page int) string {
	params := url.Values{"type": {"dash-db"}, "limit": {strconv.Itoa(limit)}, "page": {strconv.Itoa(page)}}
	if query != "" {
		params.Set("query", query)
	}
	return "/api/search?" + params.Encode()
}

func readDashboard(ctx context.Context, baseURL, selector string) (json.RawMessage, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, fmt.Errorf("provide a dashboard title or UID")
	}
	if validUID(selector) {
		data, err := readGrafanaJSON(ctx, baseURL, "/api/dashboards/uid/"+url.PathEscape(selector), "dashboard")
		var httpErr *restapi.HTTPError
		if err == nil || !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusNotFound {
			return data, err
		}
	}

	const pageSize = 100
	var exact, partial []dashboardSearchResult
	seen := map[string]bool{}
	for page := 1; ; page++ {
		data, err := readGrafanaJSON(ctx, baseURL, dashboardSearchEndpoint(selector, pageSize, page), "")
		if err != nil {
			return nil, err
		}
		var results []dashboardSearchResult
		if err := json.Unmarshal(data, &results); err != nil {
			return nil, fmt.Errorf("decode dashboard search results: %w", err)
		}
		newCount := 0
		for _, result := range results {
			if !validUID(result.UID) || seen[result.UID] {
				continue
			}
			seen[result.UID] = true
			newCount++
			if strings.EqualFold(result.Title, selector) {
				exact = append(exact, result)
			} else {
				partial = append(partial, result)
			}
		}
		if len(results) < pageSize || newCount == 0 {
			break
		}
	}
	matches := partial
	if len(exact) > 0 {
		matches = exact
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no dashboard matched %q; use dashboard list to search", selector)
	case 1:
		return readGrafanaJSON(ctx, baseURL, "/api/dashboards/uid/"+url.PathEscape(matches[0].UID), "dashboard")
	default:
		var choices []string
		for _, match := range matches[:min(len(matches), 5)] {
			label := fmt.Sprintf("%s (%s)", match.Title, match.UID)
			if match.FolderTitle != "" {
				label = match.FolderTitle + " / " + label
			}
			choices = append(choices, label)
		}
		return nil, fmt.Errorf("dashboard %q matches %d dashboards; use a UID: %s", selector, len(matches), strings.Join(choices, ", "))
	}
}

func selectPanel(panels []dashboardPanel, selector, dashboard string) (dashboardPanel, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return dashboardPanel{}, fmt.Errorf("provide a panel title or ID")
	}
	id, idErr := strconv.Atoi(selector)
	var exact, partial []dashboardPanel
	for _, panel := range panels {
		if idErr == nil && id > 0 && panel.ID == id {
			return panel, nil
		}
		if strings.EqualFold(panel.Title, selector) {
			exact = append(exact, panel)
		} else if strings.Contains(strings.ToLower(panel.Title), strings.ToLower(selector)) {
			partial = append(partial, panel)
		}
	}
	matches := partial
	if len(exact) > 0 {
		matches = exact
	}
	switch len(matches) {
	case 0:
		return dashboardPanel{}, fmt.Errorf("panel %q was not found on dashboard %q", selector, dashboard)
	case 1:
		return matches[0], nil
	default:
		var choices []string
		for _, panel := range matches[:min(len(matches), 5)] {
			choices = append(choices, fmt.Sprintf("%s (%d)", panel.Title, panel.ID))
		}
		return dashboardPanel{}, fmt.Errorf("panel %q matches %d panels; use a panel ID: %s", selector, len(matches), strings.Join(choices, ", "))
	}
}
