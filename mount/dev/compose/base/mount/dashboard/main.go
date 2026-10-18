// MWDD Dashboard - A simple status dashboard for MediaWiki Docker Development environment
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
    <title>Docker Dashboard</title>
    <style>
        :root {
            --bg-color: #1a1a2e;
            --card-bg: #16213e;
            --text-color: #eee;
            --accent-color: #0f3460;
            --success-color: #00d26a;
            --error-color: #e94560;
            --link-color: #4da6ff;
        }
        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-color);
            padding: 20px;
            min-height: 100vh;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
        }
        h1 {
            text-align: center;
            margin-bottom: 10px;
            font-size: 2em;
        }
        .subtitle {
            text-align: center;
            color: #888;
            margin-bottom: 30px;
        }
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .card {
            background-color: var(--card-bg);
            border-radius: 10px;
            padding: 20px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.3);
        }
        .card h2 {
            margin-bottom: 15px;
            padding-bottom: 10px;
            border-bottom: 2px solid var(--accent-color);
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .service-list, .site-list {
            list-style: none;
        }
        .service-item, .site-item {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 10px;
            margin: 5px 0;
            background-color: var(--accent-color);
            border-radius: 5px;
        }
        .service-name, .site-name {
            font-weight: 500;
        }
        .status-badge {
            padding: 4px 12px;
            border-radius: 20px;
            font-size: 0.85em;
            font-weight: 600;
        }
        .status-running {
            background-color: var(--success-color);
            color: #000;
        }
        .status-stopped {
            background-color: var(--error-color);
            color: #fff;
        }
        a {
            color: var(--link-color);
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
        .access-links {
            display: flex;
            flex-wrap: wrap;
            gap: 12px;
            margin-top: 6px;
            font-size: 0.85em;
        }
        .tunnel-status, .access-note {
            text-align: center;
            margin-bottom: 15px;
        }
        .access-note {
            color: #aaa;
            font-size: 0.85em;
        }
        .service-item, .site-item { gap: 12px; }
        .site-item { flex-wrap: wrap; }
        .site-link {
            color: var(--link-color);
        }
        .meta-info {
            text-align: center;
            color: #666;
            font-size: 0.85em;
            margin-top: 20px;
        }
        .refresh-btn {
            display: block;
            margin: 20px auto;
            padding: 10px 30px;
            background-color: var(--accent-color);
            color: var(--text-color);
            border: none;
            border-radius: 5px;
            cursor: pointer;
            font-size: 1em;
        }
        .refresh-btn:hover {
            background-color: #1a4980;
        }
        .api-links {
            text-align: center;
            margin-bottom: 20px;
        }
        .api-links a {
            margin: 0 10px;
        }
        .empty-message {
            color: #888;
            font-style: italic;
            padding: 10px;
        }
        @media (max-width: 600px) {
            .grid {
                grid-template-columns: 1fr;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🐳 MWDD Dashboard</h1>
        <p class="subtitle">MediaWiki Docker Development Environment Status</p>
        {{if .BasePath}}<p class="access-note"><a href="/_mwcli/logout">Sign out of the gateway</a></p>{{end}}

        <div class="tunnel-status">
            {{if .TunnelURL}}
            <strong>Tunnel connected</strong> · <a href="{{.TunnelURL}}/proxy/dashboard/" target="_blank" rel="noopener noreferrer">Open tunnel dashboard</a>
            {{else}}
            <strong>Tunnel not connected</strong> · Tunnel links appear when a connection is ready.
            {{end}}
        </div>
        <p class="access-note">Local opens the service hostname. Proxy uses this gateway. Tunnel uses the public HTTPS address. Some services need additional setup for Proxy and Tunnel access.</p>

        <div class="api-links">
            <a href="{{.BasePath}}/api/status">API: Full Status</a>
            <a href="{{.BasePath}}/api/services">API: Services</a>
            <a href="{{.BasePath}}/api/sites">API: Sites</a>
        </div>

        <div class="grid">
            <div class="card">
                <h2>📦 Services</h2>
                <ul class="service-list" id="services">
                    {{range .Services}}
                    <li class="service-item">
                        <span class="service-name">
                            {{.Name}}
                            {{if and .Running .HasUI}}
                            <span class="access-links">
                                <a href="{{.UIURL}}" class="ui-link local-link" target="_blank" rel="noopener noreferrer">Local</a>
                                <a href="{{.ProxyURL}}" class="ui-link proxy-link" target="_blank" rel="noopener noreferrer">Proxy</a>
                                {{if .TunnelURL}}<a href="{{.TunnelURL}}" class="ui-link tunnel-link" target="_blank" rel="noopener noreferrer">Tunnel</a>{{end}}
                            </span>
                            {{end}}
                        </span>
                        <span class="status-badge {{if .Running}}status-running{{else}}status-stopped{{end}}">
                            {{if .Running}}Running{{else}}Stopped{{end}}
                        </span>
                    </li>
                    {{else}}
                    <li class="empty-message">No services configured</li>
                    {{end}}
                </ul>
            </div>

            <div class="card">
                <h2>🌐 Installed Sites</h2>
                <ul class="site-list" id="sites">
                    {{range .Sites}}
                    <li class="site-item">
                        <span class="site-name">
                            {{.Name}}
                            <span class="access-links">
                                <a href="{{.URL}}" class="site-link local-link" target="_blank" rel="noopener noreferrer">Local</a>
                                <a href="{{.ProxyURL}}" class="site-link proxy-link" target="_blank" rel="noopener noreferrer">Proxy</a>
                                {{if .TunnelURL}}<a href="{{.TunnelURL}}" class="site-link tunnel-link" target="_blank" rel="noopener noreferrer">Tunnel</a>{{end}}
                            </span>
                        </span>
                        <span style="color: #888; font-size: 0.85em;">{{.Host}}</span>
                    </li>
                    {{else}}
                    <li class="empty-message">No sites installed. Run: mw docker mediawiki install</li>
                    {{end}}
                </ul>
            </div>
        </div>

        <button class="refresh-btn" onclick="location.reload()">🔄 Refresh</button>

        <p class="meta-info">
            Last updated: {{.Timestamp}}<br>
            Cache age: {{.CacheAge}} (cached for 5s)
        </p>
    </div>

    <script>
        // Auto-refresh every 30 seconds
        setTimeout(() => location.reload(), 30000);
    </script>
</body>
</html>
`

// Dashboard handler
func dashboardHandler(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("MWDD Dashboard starting on port %s", listenPort)
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
