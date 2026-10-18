package cloudvps

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestProjectInventoryValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       []string
	}{
		{"valid", `{"projects":["mwcli","beta","mwcli","../bad","escape\u001b","a.b",""]}`, 200, []string{"beta", "mwcli"}},
		{"empty", `{"projects":[]}`, 200, []string{}},
		{"missing", `{}`, 200, nil},
		{"malformed", `<html>unavailable</html>`, 200, nil},
		{"server failure", `{}`, 503, nil},
		{"oversized", strings.Repeat("x", 1024*1024+1), 200, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/projects.json" || r.Header.Get("Authorization") != "" || r.Header.Get("User-Agent") == "" {
					t.Error("unexpected request")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			got, err := fetchProjectNames(context.Background(), server.Client(), server.URL)
			if tc.want == nil {
				if err == nil {
					t.Fatal("invalid inventory accepted")
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestProjectCompletionCacheAndOfflineFallback(t *testing.T) {
	var requests atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if fail.Load() {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `{"projects":["mwcli","beta"]}`)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "cache", "projects.json")
	want := []string{"beta", "mwcli"}
	for range 2 {
		if got := cachedProjectNames(context.Background(), server.Client(), server.URL, path); !reflect.DeepEqual(got, want) {
			t.Fatalf("suggestions %v", got)
		}
	}
	if requests.Load() != 1 {
		t.Fatal("fresh cache caused another network request")
	}
	fail.Store(true)
	writeProjectCache(path, projectCache{Projects: want, RefreshAfter: time.Now().Add(-time.Hour)})
	for range 2 {
		if got := cachedProjectNames(context.Background(), server.Client(), server.URL, path); !reflect.DeepEqual(got, want) {
			t.Fatalf("lost stale suggestions: %v", got)
		}
	}
	if requests.Load() != 2 {
		t.Fatal("failed refresh was retried without backoff")
	}
}

func TestProjectCompletionSlowOrMissingInventory(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		<-r.Context().Done()
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "projects.json")
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	start := time.Now()
	if got := cachedProjectNames(ctx, server.Client(), server.URL, path); len(got) != 0 {
		t.Fatalf("unexpected suggestions: %v", got)
	}
	if time.Since(start) > time.Second {
		t.Fatal("lookup did not respect deadline")
	}
	if got := cachedProjectNames(context.Background(), server.Client(), server.URL, path); len(got) != 0 {
		t.Fatal("failed lookup fabricated projects")
	}
	if requests.Load() != 1 {
		t.Fatal("empty-cache failure was not backed off")
	}
}

func TestProjectCompletionCannotWriteCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"projects":["mwcli"]}`) }))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := cachedProjectNames(context.Background(), server.Client(), server.URL, filepath.Join(path, "cache.json"))
	if !reflect.DeepEqual(got, []string{"mwcli"}) {
		t.Fatalf("cache failure prevented suggestions: %v", got)
	}
}

func TestAllProjectFlagsHaveCompletion(t *testing.T) {
	count := 0
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		if cmd.Flags().Lookup("project") != nil {
			count++
			if _, ok := cmd.GetFlagCompletionFunc("project"); !ok {
				t.Errorf("no project completion: %s", cmd.CommandPath())
			}
		}
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(NewCloudVPSCmd())
	if count < 10 {
		t.Fatalf("unexpected project command coverage: %d", count)
	}
	if projectPrompt(context.Background()).Suggest == nil {
		t.Fatal("interactive project prompt has no completion")
	}
}

func TestProjectSuggestionsKeepConfiguredProjectsAndFilterPrefix(t *testing.T) {
	previousCacheHome := xdg.CacheHome
	xdg.CacheHome = t.TempDir()
	cfg := config.State().Effective.CloudVPS
	previousProjects, previousDefault := cfg.Projects, cfg.DefaultProject
	state := config.State().Effective
	state.CloudVPS.Projects = map[string]config.CloudVPSProject{"mwcli-local": {}}
	state.CloudVPS.DefaultProject = "mwcli-default"
	t.Cleanup(func() {
		xdg.CacheHome = previousCacheHome
		state.CloudVPS.Projects, state.CloudVPS.DefaultProject = previousProjects, previousDefault
	})
	writeProjectCache(filepath.Join(xdg.CacheHome, "mwcli", "cloudvps-projects.json"), projectCache{Projects: []string{"mwcli", "beta"}, RefreshAfter: time.Now().Add(time.Hour)})
	got := projectSuggestions(context.Background(), "mwcli")
	if !reflect.DeepEqual(got, []string{"mwcli", "mwcli-default", "mwcli-local"}) {
		t.Fatalf("bad filtered suggestions: %v", got)
	}
}
