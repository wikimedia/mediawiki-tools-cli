// Development dashboard for a Wikimedia CLI
// Run with: go run main.go
// //nolint:depguard
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Service represents a docker service and its status
type Service struct {
	Name      string `json:"name"`
	Running   bool   `json:"running"`
	HasUI     bool   `json:"has_ui"`
	UIHost    string `json:"ui_host,omitempty"`
	UIURL     string `json:"ui_url,omitempty"`
	ProxyURL  string `json:"proxy_url,omitempty"`
	TunnelURL string `json:"tunnel_url,omitempty"`
	CheckedAt string `json:"checked_at"`
}

// Site represents an installed MediaWiki site
type Site struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	URL       string `json:"url"`
	ProxyURL  string `json:"proxy_url,omitempty"`
	TunnelURL string `json:"tunnel_url,omitempty"`
}

// Status represents the overall status response
type Status struct {
	Services  []Service `json:"services"`
	Sites     []Site    `json:"sites"`
	Timestamp string    `json:"timestamp"`
	CacheAge  string    `json:"cache_age"`
	BasePath  string    `json:"-"`
	TunnelURL string    `json:"tunnel_url,omitempty"`
}

// ServiceConfig defines a service to check
type ServiceConfig struct {
	Name   string `json:"name"`
	HasUI  bool   `json:"has_ui"`
	UIHost string `json:"ui_host,omitempty"`
}

var (
	// Services to check - these match the services in MwddSettings.php
	servicesToCheck = []ServiceConfig{
		{Name: "mysql", HasUI: false},
		{Name: "mysql-replica", HasUI: false},
		{Name: "postgres", HasUI: false},
		{Name: "redis", HasUI: false},
		{Name: "memcached", HasUI: false},
		{Name: "elasticsearch", HasUI: false},
		{Name: "opensearch", HasUI: false},
		{Name: "eventlogging", HasUI: false},
		{Name: "graphite", HasUI: true, UIHost: "graphite.local.wmftest.net"},
		{Name: "citoid", HasUI: false},
		{Name: "jaeger", HasUI: true, UIHost: "jaeger.local.wmftest.net"},
		{Name: "mailhog", HasUI: true, UIHost: "mailhog.local.wmftest.net"},
		{Name: "phpmyadmin", HasUI: true, UIHost: "phpmyadmin.local.wmftest.net"},
		{Name: "adminer", HasUI: true, UIHost: "adminer.local.wmftest.net"},
		{Name: "wdqs-ui", HasUI: true, UIHost: "wdqs-ui.local.wmftest.net"},
		{Name: "shellbox-media", HasUI: false},
		{Name: "shellbox-php-rpc", HasUI: false},
		{Name: "shellbox-score", HasUI: false},
		{Name: "shellbox-syntaxhighlight", HasUI: false},
		{Name: "shellbox-timeline", HasUI: false},
		{Name: "keycloak", HasUI: true, UIHost: "keycloak.local.wmftest.net"},
		{Name: "novnc", HasUI: true, UIHost: "novnc.local.wmftest.net"},
		{Name: "prometheus", HasUI: true, UIHost: "prometheus.local.wmftest.net"},
		{Name: "grafana", HasUI: true, UIHost: "grafana.local.wmftest.net"},
	}

	// Cache for status
	statusCache     *Status
	statusCacheMu   sync.RWMutex
	statusCacheTime time.Time
	cacheDuration   = 5 * time.Second

	// Port from environment
	port string

	// DNS lookup timeout for service checks.
	dnsLookupTimeout = 500 * time.Millisecond

	// dnsLookupHost is swappable for tests.
	dnsLookupHost = func(ctx context.Context, hostname string) ([]string, error) {
		return net.DefaultResolver.LookupHost(ctx, hostname)
	}
)

func init() {
	port = os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if timeoutMs := os.Getenv("DASHBOARD_DNS_TIMEOUT_MS"); timeoutMs != "" {
		if parsed, err := strconv.Atoi(timeoutMs); err == nil && parsed > 0 {
			dnsLookupTimeout = time.Duration(parsed) * time.Millisecond
		}
	}
}

// checkHost checks if a hostname resolves to something other than itself
// This is the same check used in MwddSettings.php
func checkHost(hostname string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), dnsLookupTimeout)
	defer cancel()

	addrs, err := dnsLookupHost(ctx, hostname)
	if err != nil {
		return false
	}

	// If we got addresses and the first one isn't the hostname itself, service is running
	if len(addrs) > 0 && addrs[0] != hostname {
		return true
	}
	return false
}

// serviceConfigs merges generated custom-service metadata with bundled entries.
// No Docker socket or YAML parser is needed inside the dashboard.
func serviceConfigs() []ServiceConfig {
	result := append([]ServiceConfig{}, servicesToCheck...)
	path := os.Getenv("DASHBOARD_SERVICES_FILE")
	if path == "" {
		path = "/data/dashboard-services.json"
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return result
	}
	if err != nil {
		log.Printf("Could not read custom services: %v", err)
		return result
	}
	var custom []ServiceConfig
	if err := json.Unmarshal(data, &custom); err != nil {
		log.Printf("Could not parse custom services: %v", err)
		return result
	}
	positions := map[string]int{}
	for i, service := range result {
		positions[service.Name] = i
	}
	for _, service := range custom {
		if service.Name == "" {
			continue
		}
		if i, exists := positions[service.Name]; exists {
			// A build-only override must not erase bundled UI metadata.
			if service.HasUI {
				result[i] = service
			}
			continue
		}
		positions[service.Name] = len(result)
		result = append(result, service)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// checkAllServices checks all services in parallel
func checkAllServices() []Service {
	var wg sync.WaitGroup
	configs := serviceConfigs()
	services := make([]Service, len(configs))
	checkedAt := time.Now().Format(time.RFC3339)

	for i, svc := range configs {
		wg.Add(1)
		go func(idx int, config ServiceConfig) {
			defer wg.Done()
			running := checkHost(config.Name)
			services[idx] = Service{
				Name:      config.Name,
				Running:   running,
				HasUI:     config.HasUI,
				UIHost:    config.UIHost,
				CheckedAt: checkedAt,
			}
			if config.HasUI && running {
				services[idx].UIURL = fmt.Sprintf("http://%s:%s", config.UIHost, port)
			}
		}(i, svc)
	}

	wg.Wait()
	return services
}

// getSites reads sites from the record-hosts file
func getSites() []Site {
	// The record-hosts file is mounted at /data/record-hosts
	data, err := os.ReadFile("/data/record-hosts")
	if err != nil {
		log.Printf("Could not read record-hosts file: %v", err)
		return []Site{}
	}

	var sites []Site
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Only include mediawiki hosts
		if strings.Contains(line, "mediawiki.local.wmftest.net") {
			name := strings.Split(line, ".")[0]
			sites = append(sites, Site{
				Name: name,
				Host: line,
				URL:  fmt.Sprintf("http://%s:%s", line, port),
			})
		}
	}

	sort.Slice(sites, func(i, j int) bool {
		return sites[i].Name < sites[j].Name
	})

	return sites
}

// getStatus returns cached status or refreshes if cache is stale
func getStatus() *Status {
	statusCacheMu.RLock()
	if statusCache != nil && time.Since(statusCacheTime) < cacheDuration {
		status := statusCache
		statusCacheMu.RUnlock()
		return status
	}
	statusCacheMu.RUnlock()

	// Need to refresh cache
	statusCacheMu.Lock()
	defer statusCacheMu.Unlock()

	// Double-check after acquiring write lock
	if statusCache != nil && time.Since(statusCacheTime) < cacheDuration {
		return statusCache
	}

	now := time.Now()
	statusCache = &Status{
		Services:  checkAllServices(),
		Sites:     getSites(),
		Timestamp: now.Format(time.RFC3339),
		TunnelURL: currentTunnelURL(&http.Client{Timeout: 500 * time.Millisecond}, "http://cloudflared:20241"),
		CacheAge:  "0s",
	}
	*statusCache = accessLinks(*statusCache)
	statusCacheTime = now

	return statusCache
}

// A Quick Tunnel's internal status endpoint disappears when its container stops.
// Readiness prevents advertising a URL while it is still starting or disconnected.
var quickTunnelHostname = regexp.MustCompile(`^(?:https://)?([a-zA-Z0-9-]+\.trycloudflare\.com)/?$`)

func currentTunnelURL(client *http.Client, endpoint string) string {
	ready, err := client.Get(endpoint + "/ready")
	if err != nil {
		return ""
	}
	ready.Body.Close()
	if ready.StatusCode != http.StatusOK {
		return ""
	}
	response, err := client.Get(endpoint + "/quicktunnel")
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	var tunnel struct {
		Hostname string `json:"hostname"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&tunnel) != nil {
		return ""
	}
	matches := quickTunnelHostname.FindStringSubmatch(tunnel.Hostname)
	if len(matches) != 2 {
		return ""
	}
	return "https://" + matches[1]
}

// Keep access methods separate and leave the cached status unchanged.
func accessLinks(status Status) Status {
	status.Services = append([]Service(nil), status.Services...)
	status.Sites = append([]Site(nil), status.Sites...)
	for i := range status.Services {
		service := &status.Services[i]
		service.ProxyURL, service.TunnelURL = "", ""
		if !service.HasUI || !service.Running {
			continue
		}
		path := os.Getenv(strings.ToUpper(strings.ReplaceAll(service.Name, "-", "_")) + "_PROXY_PATH")
		if path == "" {
			path = service.Name
		}
		service.ProxyURL = "/proxy/" + strings.Trim(path, "/") + "/"
		if service.Name == "novnc" {
			service.ProxyURL += "vnc.html"
			if !strings.HasSuffix(service.UIURL, "/vnc.html") {
				service.UIURL = strings.TrimRight(service.UIURL, "/") + "/vnc.html"
			}
		}
		if status.TunnelURL != "" {
			service.TunnelURL = status.TunnelURL + service.ProxyURL
		}
	}
	for i := range status.Sites {
		site := &status.Sites[i]
		dbName := strings.SplitN(site.Host, ".", 2)[0]
		site.ProxyURL = "/proxy/" + dbName + "/"
		site.TunnelURL = ""
		if status.TunnelURL != "" {
			site.TunnelURL = status.TunnelURL + site.ProxyURL
		}
	}
	return status
}

// API handler for JSON status
func apiStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := getStatus()

	// Update cache age in response
	response := *status
	response.CacheAge = time.Since(statusCacheTime).Round(time.Millisecond).String()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "JSON encode error", http.StatusInternalServerError)
	}
}

// API handler for just services
func apiServicesHandler(w http.ResponseWriter, r *http.Request) {
	status := getStatus()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := json.NewEncoder(w).Encode(status.Services); err != nil {
		http.Error(w, "JSON encode error", http.StatusInternalServerError)
	}
}

// API handler for just sites
func apiSitesHandler(w http.ResponseWriter, r *http.Request) {
	status := getStatus()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := json.NewEncoder(w).Encode(status.Sites); err != nil {
		http.Error(w, "JSON encode error", http.StatusInternalServerError)
	}
}

// Dashboard HTML template
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>a Wikimedia CLI · Development dashboard</title>
    <link rel="stylesheet" href="{{.BasePath}}/assets/tokens.css">
    <link rel="stylesheet" href="{{.BasePath}}/assets/tokens-dark.css" media="(prefers-color-scheme: dark)">
    <link rel="stylesheet" href="{{.BasePath}}/assets/codex.style.css">
    <link rel="stylesheet" href="{{.BasePath}}/assets/codex-links.css">
    <link rel="stylesheet" href="{{.BasePath}}/assets/app.css">
</head>
<body>
    <a class="cdx-link wm-skip-link" href="#main">Skip to content</a>
    <header class="wm-header">
        <span class="wm-brand">a Wikimedia CLI</span>
        <span>Docker development</span>
        {{if .BasePath}}<nav aria-label="Gateway"><a class="cdx-link" href="/_wm/logout">Sign out of the gateway</a></nav>{{end}}
    </header>
    <main id="main" class="wm-page wm-stack">
        <div class="wm-page-heading">
            <div>
                <h1>Development dashboard</h1>
                <p class="wm-description">MediaWiki and service status</p>
            </div>
            <form method="get" action="{{.BasePath}}/">
                <button class="cdx-button cdx-button--action-progressive cdx-button--weight-normal" type="submit">Refresh</button>
            </form>
        </div>
        <div class="cdx-message cdx-message--block {{if .TunnelURL}}cdx-message--success{{else}}cdx-message--notice{{end}}" aria-live="polite">
            <span class="cdx-message__icon" aria-hidden="true"></span>
            <div class="cdx-message__content">
                {{if .TunnelURL}}
                <strong>Tunnel connected</strong> · <a class="cdx-link" href="{{.TunnelURL}}/proxy/dashboard/" target="_blank" rel="noopener noreferrer">Open tunnel dashboard</a>
                {{else}}
                <strong>Tunnel not connected</strong> · Tunnel links appear when a connection is ready.
                {{end}}
            </div>
        </div>
        <p class="wm-description">Local opens the service hostname. Proxy uses this gateway. Tunnel uses the public HTTPS address. Some services need additional setup for Proxy and Tunnel access.</p>
        <div class="wm-columns">
            <section class="wm-section" aria-labelledby="services-heading">
                <h2 id="services-heading">Services</h2>
                <ul class="wm-card-list wm-card-list--services" id="services">
                    {{range .Services}}
                    <li class="cdx-card">
                        <div class="cdx-card__text">
                            <div class="cdx-card__text__title wm-item-heading">
                                <span>{{.Name}}</span>
                                <span class="cdx-info-chip {{if .Running}}cdx-info-chip--success{{else}}cdx-info-chip--notice{{end}}">
                                    <span class="cdx-info-chip__icon" aria-hidden="true"></span>
                                    <span class="cdx-info-chip__text">{{if .Running}}Running{{else}}Stopped{{end}}</span>
                                </span>
                            </div>
                            {{if and .Running .HasUI}}
                            <div class="cdx-card__text__supporting-text wm-links" aria-label="Access {{.Name}}">
                                <a href="{{.UIURL}}" class="cdx-link ui-link local-link" target="_blank" rel="noopener noreferrer">Local</a>
                                <a href="{{.ProxyURL}}" class="cdx-link ui-link proxy-link" target="_blank" rel="noopener noreferrer">Proxy</a>
                                {{if .TunnelURL}}<a href="{{.TunnelURL}}" class="cdx-link ui-link tunnel-link" target="_blank" rel="noopener noreferrer">Tunnel</a>{{end}}
                            </div>
                            {{end}}
                        </div>
                    </li>
                    {{else}}
                    <li class="cdx-card"><span class="cdx-card__text">No services configured</span></li>
                    {{end}}
                </ul>
            </section>
            <section class="wm-section" aria-labelledby="sites-heading">
                <h2 id="sites-heading">Installed Sites</h2>
                <ul class="wm-card-list" id="sites">
                    {{range .Sites}}
                    <li class="cdx-card">
                        <div class="cdx-card__text">
                            <span class="cdx-card__text__title">{{.Name}}</span>
                            <span class="cdx-card__text__description wm-host">{{.Host}}</span>
                            <div class="cdx-card__text__supporting-text wm-links" aria-label="Access {{.Name}}">
                                <a href="{{.URL}}" class="cdx-link site-link local-link" target="_blank" rel="noopener noreferrer">Local</a>
                                <a href="{{.ProxyURL}}" class="cdx-link site-link proxy-link" target="_blank" rel="noopener noreferrer">Proxy</a>
                                {{if .TunnelURL}}<a href="{{.TunnelURL}}" class="cdx-link site-link tunnel-link" target="_blank" rel="noopener noreferrer">Tunnel</a>{{end}}
                            </div>
                        </div>
                    </li>
                    {{else}}
                    <li class="cdx-card"><div class="cdx-card__text"><span class="cdx-card__text__title">No sites installed</span><span class="cdx-card__text__description">Run <code>mw docker mediawiki install</code> to add your first site.</span></div></li>
                    {{end}}
                </ul>
            </section>
        </div>
        <footer class="wm-meta wm-stack wm-stack--compact">
            <nav class="wm-links" aria-label="Dashboard APIs">
                <a class="cdx-link" href="{{.BasePath}}/api/status">API: Full Status</a>
                <a class="cdx-link" href="{{.BasePath}}/api/services">API: Services</a>
                <a class="cdx-link" href="{{.BasePath}}/api/sites">API: Sites</a>
            </nav>
            <p>Last updated: {{.Timestamp}} · Cache age: {{.CacheAge}} (cached for 5s). Refreshes automatically every 30 seconds.</p>
        </footer>
    </main>
    <script>setTimeout(() => location.reload(), 30000);</script>
</body>
</html>
`

// Dashboard handler
func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		uiAssetHandler(w, r)
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	status := getStatus()
	response := *status
	response.CacheAge = time.Since(statusCacheTime).Round(time.Millisecond).String()
	response.BasePath = strings.TrimRight(r.Header.Get("X-Forwarded-Prefix"), "/")
	response = accessLinks(response)

	tmpl, err := template.New("dashboard").Parse(dashboardHTML)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, response); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

// Serve only the shared UI styles, including when the dashboard is proxied.
func uiAssetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/assets/")
	switch name {
	case "tokens.css", "tokens-dark.css", "codex.style.css", "codex-links.css", "app.css":
	default:
		http.NotFound(w, r)
		return
	}
	directory := os.Getenv("MWCLI_UI_ASSETS")
	if directory == "" {
		directory = "/ui"
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(directory, name))
}

// Health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		http.Error(w, "JSON encode error", http.StatusInternalServerError)
	}
}

func main() {
	listenPort := os.Getenv("DASHBOARD_PORT")
	if listenPort == "" {
		listenPort = "8080"
	}
	if _, err := strconv.Atoi(listenPort); err != nil {
		log.Fatalf("Invalid dashboard port: %q", listenPort)
	}

	http.HandleFunc("/", dashboardHandler)
	http.HandleFunc("/api/status", apiStatusHandler)
	http.HandleFunc("/api/services", apiServicesHandler)
	http.HandleFunc("/api/sites", apiSitesHandler)
	http.HandleFunc("/health", healthHandler)

	log.Printf("Development dashboard starting on port %s", listenPort)
	log.Printf("Dashboard: http://localhost:%s", listenPort)
	log.Printf("API endpoints:")
	log.Printf("  - GET /api/status   - Full status (services + sites)")
	log.Printf("  - GET /api/services - Service status only")
	log.Printf("  - GET /api/sites    - Installed sites only")
	log.Printf("  - GET /health       - Health check")

	server := &http.Server{
		Addr:              ":" + listenPort,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
