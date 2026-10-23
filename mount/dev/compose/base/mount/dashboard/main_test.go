package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDashboardServesCodexStylesDirectlyAndThroughGateway(t *testing.T) {
	t.Setenv("MWCLI_UI_ASSETS", filepath.Join("..", "ui"))
	previous, previousTime := statusCache, statusCacheTime
	t.Cleanup(func() { statusCache, statusCacheTime = previous, previousTime })
	statusCache = &Status{Services: []Service{{Name: `<script>alert(1)</script>`, Running: true}}}
	statusCacheTime = time.Now()
	for _, prefix := range []string{"", "/proxy/dashboard"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-Prefix", prefix)
		w := httptest.NewRecorder()
		dashboardHandler(w, req)
		body := w.Body.String()
		if w.Code != http.StatusOK || strings.Contains(body, "<script>alert(1)") || !strings.Contains(body, "&lt;script&gt;") {
			t.Fatalf("dashboard render failed or did not escape service name: %d", w.Code)
		}
		for _, asset := range []string{"tokens.css", "tokens-dark.css", "codex.style.css", "codex-links.css", "app.css"} {
			if !strings.Contains(body, `href="`+prefix+`/assets/`+asset+`"`) {
				t.Fatalf("missing asset %s through prefix %q", asset, prefix)
			}
			// Traefik strips the dashboard prefix before forwarding the request.
			css := httptest.NewRecorder()
			dashboardHandler(css, httptest.NewRequest(http.MethodGet, "/assets/"+asset, nil))
			if css.Code != http.StatusOK || !strings.HasPrefix(css.Header().Get("Content-Type"), "text/css") || css.Body.Len() == 0 {
				t.Fatalf("stylesheet %s unavailable: %d", asset, css.Code)
			}
		}
	}
	for _, path := range []string{"/assets/", "/assets/README.md", "/assets/../main.go"} {
		w := httptest.NewRecorder()
		dashboardHandler(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("unexpected asset access %s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	dashboardHandler(w, httptest.NewRequest(http.MethodPost, "/assets/app.css", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatal("stylesheet endpoint accepted a write method")
	}
}

func TestCheckHostRunningTrue(t *testing.T) {
	origLookup := dnsLookupHost
	origTimeout := dnsLookupTimeout
	t.Cleanup(func() {
		dnsLookupHost = origLookup
		dnsLookupTimeout = origTimeout
	})

	dnsLookupTimeout = 50 * time.Millisecond
	dnsLookupHost = func(ctx context.Context, hostname string) ([]string, error) {
		return []string{"10.0.0.2"}, nil
	}

	if !checkHost("mysql") {
		t.Fatal("expected checkHost to return true for resolved service")
	}
}

func TestCheckHostRunningFalseOnLookupError(t *testing.T) {
	origLookup := dnsLookupHost
	origTimeout := dnsLookupTimeout
	t.Cleanup(func() {
		dnsLookupHost = origLookup
		dnsLookupTimeout = origTimeout
	})

	dnsLookupTimeout = 50 * time.Millisecond
	dnsLookupHost = func(ctx context.Context, hostname string) ([]string, error) {
		return nil, errors.New("lookup failed")
	}

	if checkHost("missing") {
		t.Fatal("expected checkHost to return false when lookup errors")
	}
}

func TestCheckHostRespectsTimeout(t *testing.T) {
	origLookup := dnsLookupHost
	origTimeout := dnsLookupTimeout
	t.Cleanup(func() {
		dnsLookupHost = origLookup
		dnsLookupTimeout = origTimeout
	})

	dnsLookupTimeout = 10 * time.Millisecond
	dnsLookupHost = func(ctx context.Context, hostname string) ([]string, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	start := time.Now()
	if checkHost("slow-service") {
		t.Fatal("expected checkHost to return false on timeout")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("expected timeout to be fast, elapsed=%s", elapsed)
	}
}

func TestDashboardGatewayLinksKeepCurrentOrigin(t *testing.T) {
	previous, previousTime := statusCache, statusCacheTime
	t.Cleanup(func() { statusCache, statusCacheTime = previous, previousTime })
	statusCache = &Status{
		Services: []Service{{Name: "graphite", Running: true, HasUI: true, UIURL: "http://graphite.local.wmftest.net:8080/"}},
		Sites:    []Site{{Host: "default.mediawiki.local.wmftest.net", URL: "http://default.mediawiki.local.wmftest.net:8080/"}},
	}
	statusCacheTime = time.Now()
	for _, origin := range []string{"http://localhost:8080", "https://example.trycloudflare.com"} {
		req := httptest.NewRequest("GET", origin+"/", nil)
		req.Header.Set("X-Forwarded-Prefix", "/proxy/dashboard")
		recorder := httptest.NewRecorder()
		dashboardHandler(recorder, req)
		body := recorder.Body.String()
		for _, link := range []string{`href="/proxy/graphite/"`, `href="/proxy/default/"`} {
			if !strings.Contains(body, link) {
				t.Fatalf("%s: missing link %s", origin, link)
			}
		}
	}
	// Visiting the gateway must leave the direct hostname links intact.
	recorder := httptest.NewRecorder()
	dashboardHandler(recorder, httptest.NewRequest("GET", "http://dashboard.local.wmftest.net:8080/", nil))
	for _, link := range []string{`href="http://graphite.local.wmftest.net:8080/"`, `href="http://default.mediawiki.local.wmftest.net:8080/"`} {
		if !strings.Contains(recorder.Body.String(), link) {
			t.Fatalf("direct access lost link %s", link)
		}
	}
}

func TestCurrentTunnelURLTracksReadinessAndHostname(t *testing.T) {
	ready, hostname := true, "first.trycloudflare.com"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ready":
			if !ready {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		case "/quicktunnel":
			fmt.Fprintf(w, `{"hostname":%q}`, hostname)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := server.Client()
	if got := currentTunnelURL(client, server.URL); got != "https://first.trycloudflare.com" {
		t.Fatalf("unexpected connected tunnel: %q", got)
	}
	hostname = "https://second.trycloudflare.com"
	if got := currentTunnelURL(client, server.URL); got != hostname {
		t.Fatalf("kept stale URL after restart: %q", got)
	}
	ready = false
	if got := currentTunnelURL(client, server.URL); got != "" {
		t.Fatalf("advertised disconnected tunnel: %q", got)
	}
	ready = true
	for _, invalid := range []string{"", "javascript:alert(1)", "https://example.com", "https://fake.trycloudflare.com@evil.test", "https://fake.trycloudflare.com/path"} {
		hostname = invalid
		if got := currentTunnelURL(client, server.URL); got != "" {
			t.Fatalf("accepted invalid tunnel %q as %q", invalid, got)
		}
	}
	server.Close()
	if got := currentTunnelURL(client, server.URL); got != "" {
		t.Fatalf("advertised unavailable tunnel: %q", got)
	}
}

func TestDashboardShowsSeparateAccessLinks(t *testing.T) {
	previous, previousTime := statusCache, statusCacheTime
	t.Cleanup(func() { statusCache, statusCacheTime = previous, previousTime })
	statusCache = &Status{
		Services: []Service{
			{Name: "adminer", Running: true, HasUI: true, UIURL: "http://adminer.local.wmftest.net:8080/"},
			{Name: "novnc", Running: true, HasUI: true, UIURL: "http://novnc.local.wmftest.net:8080/"},
			{Name: "stopped", HasUI: true},
		},
		Sites: []Site{{Host: "default.mediawiki.local.wmftest.net", URL: "http://default.mediawiki.local.wmftest.net:8080/"}},
	}
	statusCacheTime = time.Now()
	for _, origin := range []string{"", "https://active.trycloudflare.com"} {
		statusCache.TunnelURL = origin
		recorder := httptest.NewRecorder()
		dashboardHandler(recorder, httptest.NewRequest("GET", "http://dashboard.local.wmftest.net:8080/", nil))
		body := recorder.Body.String()
		for _, expected := range []string{
			`href="http://adminer.local.wmftest.net:8080/"`,
			`href="/proxy/adminer/"`,
			`href="http://default.mediawiki.local.wmftest.net:8080/"`,
			`href="/proxy/default/"`,
			`href="http://novnc.local.wmftest.net:8080/vnc.html"`,
			`href="/proxy/novnc/vnc.html"`,
		} {
			if !strings.Contains(body, expected) {
				t.Fatalf("missing access link: %s", expected)
			}
		}
		if origin == "" {
			if strings.Contains(body, `class="ui-link tunnel-link"`) || strings.Contains(body, `class="site-link tunnel-link"`) {
				t.Fatal("tunnel links shown while disconnected")
			}
		} else {
			for _, suffix := range []string{"adminer/", "default/", "novnc/vnc.html"} {
				if !strings.Contains(body, `href="`+origin+`/proxy/`+suffix+`"`) {
					t.Fatalf("missing tunnel link for %s", suffix)
				}
			}
		}
		if strings.Contains(body, "/proxy/stopped/") {
			t.Fatal("advertised stopped service")
		}
	}
	if statusCache.Services[1].UIURL != "http://novnc.local.wmftest.net:8080/" {
		t.Fatal("changed cached local URL")
	}
}

func TestDashboardLinksRespectCustomProxyPaths(t *testing.T) {
	t.Setenv("GRAFANA_PROXY_PATH", "metrics")
	t.Setenv("WDQS_UI_PROXY_PATH", "query")
	t.Setenv("NOVNC_PROXY_PATH", "desktop")
	status := accessLinks(Status{
		TunnelURL: "https://active.trycloudflare.com",
		Services: []Service{
			{Name: "grafana", Running: true, HasUI: true},
			{Name: "wdqs-ui", Running: true, HasUI: true},
			{Name: "novnc", Running: true, HasUI: true, UIURL: "http://novnc.local.wmftest.net:8080/"},
		},
	})
	for i, path := range []string{"/proxy/metrics/", "/proxy/query/", "/proxy/desktop/vnc.html"} {
		if status.Services[i].ProxyURL != path || status.Services[i].TunnelURL != status.TunnelURL+path {
			t.Fatalf("incorrect custom links: %+v", status.Services[i])
		}
	}
}
