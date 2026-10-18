package cloudvps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

const projectCompletionTimeout = 750 * time.Millisecond

type projectCache struct {
	Projects     []string  `json:"projects"`
	RefreshAfter time.Time `json:"refresh_after"`
}

func projectPrompt(ctx context.Context) *survey.Input {
	return &survey.Input{Message: "Cloud VPS project:", Suggest: func(prefix string) []string {
		return projectSuggestions(ctx, prefix)
	}}
}

func registerProjectCompletions(cmd *cobra.Command) {
	if cmd.Flags().Lookup("project") != nil {
		_ = cmd.RegisterFlagCompletionFunc("project", func(cmd *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
			return projectSuggestions(cmd.Context(), prefix), cobra.ShellCompDirectiveNoFileComp
		})
	}
	for _, child := range cmd.Commands() {
		registerProjectCompletions(child)
	}
}

func projectSuggestions(ctx context.Context, prefix string) []string {
	names := cachedProjectNames(ctx, http.DefaultClient, cloudBrowserURL, filepath.Join(xdg.CacheHome, "mwcli", "cloudvps-projects.json"))
	cfg := config.State().Effective.CloudVPS
	names = append(names, cfg.DefaultProject)
	for name := range cfg.Projects {
		names = append(names, name)
	}
	var matches []string
	for _, name := range validProjectNames(names) {
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}
	return matches
}

func cachedProjectNames(ctx context.Context, client *http.Client, baseURL, path string) []string {
	var cache projectCache
	if file, err := os.Open(path); err == nil {
		if err := json.NewDecoder(io.LimitReader(file, 1024*1024)).Decode(&cache); err != nil {
			cache = projectCache{}
		}
		file.Close()
	}
	if time.Now().Before(cache.RefreshAfter) {
		return validProjectNames(cache.Projects)
	}
	ctx, cancel := context.WithTimeout(ctx, projectCompletionTimeout)
	defer cancel()
	projects, err := fetchProjectNames(ctx, client, baseURL)
	if err == nil {
		cache.Projects = projects
		cache.RefreshAfter = time.Now().Add(24 * time.Hour)
	} else {
		// Retain stale suggestions and avoid another slow request on every Tab.
		cache.RefreshAfter = time.Now().Add(time.Minute)
	}
	writeProjectCache(path, cache)
	return validProjectNames(cache.Projects)
}

func fetchProjectNames(ctx context.Context, client *http.Client, baseURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/projects.json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mwcli cloudvps project completion")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("project inventory returned HTTP %d", resp.StatusCode)
	}
	var data struct {
		Projects []string `json:"projects"`
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(body) > 1024*1024 {
		return nil, fmt.Errorf("project inventory response too large")
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	if data.Projects == nil {
		return nil, fmt.Errorf("project inventory has no projects list")
	}
	return validProjectNames(data.Projects), nil
}

func validProjectNames(names []string) []string {
	unique := make(map[string]bool, len(names))
	for _, name := range names {
		if sshDNSLabel.MatchString(name) {
			unique[name] = true
		}
	}
	result := make([]string, 0, len(unique))
	for name := range unique {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// The inventory is public and optional. Cache failures must not block typing.
func writeProjectCache(path string, cache projectCache) {
	data, err := json.Marshal(cache)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".cloudvps-projects-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	closeErr := file.Close()
	if err == nil && closeErr == nil {
		_ = os.Rename(file.Name(), path)
	}
}
