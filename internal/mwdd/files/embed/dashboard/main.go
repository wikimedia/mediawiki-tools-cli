// MWDD Dashboard - A simple status dashboard for MediaWiki Docker Development environment
// Run with: go run main.go
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Service represents a docker service and its status
type Service struct {
	Name      string `json:"name"`
	Running   bool   `json:"running"`
	HasUI     bool   `json:"has_ui"`
	UIHost    string `json:"ui_host,omitempty"`
	UIURL     string `json:"ui_url,omitempty"`
	CheckedAt string `json:"checked_at"`
}

// Site represents an installed MediaWiki site
type Site struct {
	Name string `json:"name"`
	Host string `json:"host"`
	URL  string `json:"url"`
}

// Status represents the overall status response
type Status struct {
	Services  []Service `json:"services"`
	Sites     []Site    `json:"sites"`
	Timestamp string    `json:"timestamp"`
	CacheAge  string    `json:"cache_age"`
}

// ServiceConfig defines a service to check
type ServiceConfig struct {
	Name   string
	HasUI  bool
	UIHost string
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
	}

	// Cache for status
	statusCache     *Status
	statusCacheMu   sync.RWMutex
	statusCacheTime time.Time
	cacheDuration   = 5 * time.Second

	// Port from environment
	port string
)

func init() {
	port = os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
}

// checkHost checks if a hostname resolves to something other than itself
// This is the same check used in MwddSettings.php
func checkHost(hostname string) bool {
	// Set a short timeout for DNS lookups
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return false
	}

	// If we got addresses and the first one isn't the hostname itself, service is running
	if len(addrs) > 0 && addrs[0] != hostname {
		return true
	}
	return false
}

// checkAllServices checks all services in parallel
func checkAllServices() []Service {
	var wg sync.WaitGroup
	services := make([]Service, len(servicesToCheck))
	checkedAt := time.Now().Format(time.RFC3339)

	for i, svc := range servicesToCheck {
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
		logrus.Printf("Could not read record-hosts file: %v", err)
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
		CacheAge:  "0s",
	}
	statusCacheTime = now

	return statusCache
}

// API handler for JSON status
func apiStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := getStatus()

	// Update cache age in response
	response := *status
	response.CacheAge = time.Since(statusCacheTime).Round(time.Millisecond).String()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(response)
}

// API handler for just services
func apiServicesHandler(w http.ResponseWriter, r *http.Request) {
	status := getStatus()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(status.Services)
}

// API handler for just sites
func apiSitesHandler(w http.ResponseWriter, r *http.Request) {
	status := getStatus()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(status.Sites)
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
        .ui-link {
            margin-left: 10px;
            font-size: 0.85em;
        }
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
        
        <div class="api-links">
            <a href="/api/status">API: Full Status</a>
            <a href="/api/services">API: Services</a>
            <a href="/api/sites">API: Sites</a>
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
                            <a href="{{.UIURL}}" class="ui-link" target="_blank">🔗 Open UI</a>
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
                            <a href="{{.URL}}" class="site-link" target="_blank">{{.Name}}</a>
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

	tmpl, err := template.New("dashboard").Parse(dashboardHTML)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		logrus.Printf("Template error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, response); err != nil {
		logrus.Printf("Template execution error: %v", err)
	}
}

// Health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	listenPort := os.Getenv("DASHBOARD_PORT")
	if listenPort == "" {
		listenPort = "8080"
	}

	http.HandleFunc("/", dashboardHandler)
	http.HandleFunc("/api/status", apiStatusHandler)
	http.HandleFunc("/api/services", apiServicesHandler)
	http.HandleFunc("/api/sites", apiSitesHandler)
	http.HandleFunc("/health", healthHandler)

	logrus.Printf("MWDD Dashboard starting on port %s", listenPort)
	logrus.Printf("Dashboard: http://localhost:%s", listenPort)
	logrus.Printf("API endpoints:")
	logrus.Printf("  - GET /api/status   - Full status (services + sites)")
	logrus.Printf("  - GET /api/services - Service status only")
	logrus.Printf("  - GET /api/sites    - Installed sites only")
	logrus.Printf("  - GET /health       - Health check")

	if err := http.ListenAndServe(":"+listenPort, nil); err != nil {
		logrus.Fatalf("Failed to start server: %v", err)
	}
}
