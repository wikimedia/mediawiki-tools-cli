// The gateway's cookie login runs separately from the applications it protects.
package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	loginPath           = "/_wm/login"
	logoutPath          = "/_wm/logout"
	sessionLifetime     = 12 * time.Hour
	loginLinkLifetime   = 5 * time.Minute
	loginLinkPath       = "/_wm/login-link"
	legacyLoginPath     = "/_mwcli/login"
	legacyLogoutPath    = "/_mwcli/logout"
	legacyLoginLinkPath = "/_mwcli/login-link"
)

type gateway struct {
	credentials [32]byte
	key         []byte
	now         func() time.Time
	linksMu     sync.Mutex
	links       map[string]loginLink
}

type loginLink struct {
	origin  string
	expires time.Time
}

type ticket struct {
	Purpose string
	Origin  string
	Expires int64
	Nonce   string
}

func newGateway(directory string) (*gateway, error) {
	credentials, err := os.ReadFile(filepath.Join(directory, "credentials"))
	if err != nil {
		return nil, fmt.Errorf("read gateway credentials: %w", err)
	}
	var username, password string
	for _, line := range strings.Split(string(credentials), "\n") {
		if value, ok := strings.CutPrefix(line, "Username: "); ok {
			username = value
		}
		if value, ok := strings.CutPrefix(line, "Password: "); ok {
			password = value
		}
	}
	if username == "" || password == "" {
		return nil, fmt.Errorf("gateway credentials are incomplete")
	}
	key, err := os.ReadFile(filepath.Join(directory, "session-key"))
	if err != nil {
		return nil, fmt.Errorf("read gateway session key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("gateway session key must contain 32 bytes")
	}
	digest := sha256.Sum256([]byte(username + "\x00" + password))
	// Changing either the credentials or the random key invalidates existing sessions.
	mac := hmac.New(sha256.New, key)
	mac.Write(digest[:])
	return &gateway{credentials: digest, key: mac.Sum(nil), now: time.Now, links: make(map[string]loginLink)}, nil
}

func (g *gateway) credentialsMatch(username, password string) bool {
	digest := sha256.Sum256([]byte(username + "\x00" + password))
	return subtle.ConstantTimeCompare(digest[:], g.credentials[:]) == 1
}

func (g *gateway) sign(purpose, origin string, lifetime time.Duration) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload, err := json.Marshal(ticket{purpose, origin, g.now().Add(lifetime).Unix(), base64.RawURLEncoding.EncodeToString(nonce)})
	if err != nil {
		return "", err
	}
	value := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, g.key)
	mac.Write([]byte(value))
	return value + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (g *gateway) valid(value, purpose, origin string) bool {
	if len(value) > 2048 {
		return false
	}
	payload, signature, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	decodedSignature, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, g.key)
	mac.Write([]byte(payload))
	if !hmac.Equal(decodedSignature, mac.Sum(nil)) {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return false
	}
	var t ticket
	if json.Unmarshal(decoded, &t) != nil {
		return false
	}
	return t.Purpose == purpose && t.Origin == origin && t.Expires > g.now().Unix()
}

// ForwardAuth supplies the original host on /check. Login requests retain Host.
func requestOrigin(r *http.Request) (string, bool) {
	host := r.Host
	if r.URL.Path == "/check" {
		host = r.Header.Get("X-Forwarded-Host")
	}
	scheme := r.Header.Get("X-Forwarded-Proto")
	// Traefik reports WebSocket upgrades as ws/wss; cookies belong to the
	// corresponding HTTP origin used to establish the gateway session.
	if r.URL.Path == "/check" {
		switch scheme {
		case "ws":
			scheme = "http"
		case "wss":
			scheme = "https"
		}
	}
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	origin := scheme + "://" + host
	u, err := url.Parse(origin)
	if err != nil || host == "" || u.Host != host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	return origin, true
}

func cookieName(origin, purpose string) string {
	digest := sha256.Sum256([]byte(origin))
	return fmt.Sprintf("wm_gateway_%x_%s", digest[:6], purpose)
}

// Accept existing signed cookies during the rename, without changing their
// origin binding or expiry. New cookies always use the wm name and route.
func legacyCookieName(origin, purpose string) string {
	digest := sha256.Sum256([]byte(origin))
	return fmt.Sprintf("mwcli_gateway_%x_%s", digest[:6], purpose)
}

func (g *gateway) hasValidCookie(r *http.Request, origin, purpose, submitted string) bool {
	for _, name := range []string{cookieName(origin, purpose), legacyCookieName(origin, purpose)} {
		cookie, err := r.Cookie(name)
		if err == nil && (purpose == "session" || hmac.Equal([]byte(cookie.Value), []byte(submitted))) && g.valid(cookie.Value, purpose, origin) {
			return true
		}
	}
	return false
}

func clearCookie(w http.ResponseWriter, origin, purpose string) {
	writeCookie(w, origin, purpose, "", -time.Second)
	path := "/"
	if purpose == "csrf" {
		path = "/_mwcli/"
	}
	http.SetCookie(w, &http.Cookie{
		Name: legacyCookieName(origin, purpose), Path: path,
		Secure: strings.HasPrefix(origin, "https://"), HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

func writeCookie(w http.ResponseWriter, origin, purpose, value string, lifetime time.Duration) {
	path := "/"
	if purpose == "csrf" {
		path = "/_wm/"
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName(origin, purpose), Value: value, Path: path,
		Secure: strings.HasPrefix(origin, "https://"), HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: int(lifetime.Seconds()),
	})
}

func safeReturn(value string) string {
	u, err := url.ParseRequestURI(value)
	if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") || strings.Contains(u.Path, "\\") || (strings.HasPrefix(u.Path, "/_wm/") || strings.HasPrefix(u.Path, "/_mwcli/")) {
		return "/proxy/dashboard/"
	}
	return value
}

func (g *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	origin, ok := requestOrigin(r)
	if !ok {
		http.Error(w, "Invalid gateway origin", http.StatusBadRequest)
		return
	}
	switch r.URL.Path {
	case "/health":
		w.WriteHeader(http.StatusNoContent)
	case "/check":
		g.check(w, r, origin)
	case loginLinkPath, legacyLoginLinkPath:
		g.createLoginLink(w, r, origin)
	case loginPath, logoutPath, legacyLoginPath, legacyLogoutPath:
		g.form(w, r, origin)
	default:
		http.NotFound(w, r)
	}
}

func (g *gateway) check(w http.ResponseWriter, r *http.Request, origin string) {
	username, password, basic := r.BasicAuth()
	gatewayBasic := basic && g.credentialsMatch(username, password)
	sessionValid := g.hasValidCookie(r, origin, "session", "")
	navigation := r.Header.Get("X-Forwarded-Method") == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html")
	// Browsers need a cookie even if they cached Basic credentials before upgrade:
	// their applications can replace Authorization with a Bearer token later.
	if sessionValid || (gatewayBasic && !navigation) {
		// Traefik replaces Authorization with this response header. Remove only our
		// credentials; retain Bearer tokens and application-specific Basic credentials.
		if !gatewayBasic && r.Header.Get("Authorization") != "" {
			w.Header().Set("Authorization", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if navigation {
		target := safeReturn(r.Header.Get("X-Forwarded-Uri"))
		http.Redirect(w, r, loginPath+"?return="+url.QueryEscape(target), http.StatusSeeOther)
		return
	}
	http.Error(w, "Gateway login required: "+loginPath, http.StatusUnauthorized)
}

// Only a CLI authenticated with the gateway credentials can issue links. Codes are held in memory,
// bound to an origin, and consumed once; a gateway restart invalidates them.
func (g *gateway) createLoginLink(w http.ResponseWriter, r *http.Request, origin string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	username, password, ok := r.BasicAuth()
	if !ok || !g.credentialsMatch(username, password) {
		http.Error(w, "Gateway credentials required", http.StatusUnauthorized)
		return
	}
	if source := r.Header.Get("Origin"); source != "" && source != origin {
		http.Error(w, "Invalid request origin", http.StatusForbidden)
		return
	}
	code := make([]byte, 32)
	if _, err := rand.Read(code); err != nil {
		http.Error(w, "Cannot create login link", http.StatusInternalServerError)
		return
	}
	value := base64.RawURLEncoding.EncodeToString(code)
	now := g.now()
	g.linksMu.Lock()
	for token, link := range g.links {
		if !link.expires.After(now) {
			delete(g.links, token)
		}
	}
	if len(g.links) >= 128 {
		g.linksMu.Unlock()
		http.Error(w, "Too many unused login links; try again in five minutes", http.StatusTooManyRequests)
		return
	}
	g.links[value] = loginLink{origin, now.Add(loginLinkLifetime)}
	g.linksMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		URL string `json:"url"`
	}{origin + loginURLPath(r.URL.Path) + "#login=" + value})
}

func loginURLPath(requestPath string) string {
	if requestPath == legacyLoginLinkPath {
		return legacyLoginPath
	}
	return loginPath
}

func (g *gateway) consumeLoginLink(value, origin string) bool {
	g.linksMu.Lock()
	defer g.linksMu.Unlock()
	link, ok := g.links[value]
	if !ok || link.origin != origin || !link.expires.After(g.now()) {
		return false
	}
	delete(g.links, value)
	return true
}

func (g *gateway) form(w http.ResponseWriter, r *http.Request, origin string) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target := safeReturn(r.URL.Query().Get("return"))
	message := ""
	status := http.StatusOK
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		if r.ParseForm() != nil {
			http.Error(w, "Invalid login form", http.StatusBadRequest)
			return
		}
		submitted := r.PostForm.Get("csrf")
		if !g.hasValidCookie(r, origin, "csrf", submitted) || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != origin) {
			http.Error(w, "Login form expired; reload the page", http.StatusForbidden)
			return
		}
		target = safeReturn(r.PostForm.Get("return"))
		if r.URL.Path == logoutPath || r.URL.Path == legacyLogoutPath {
			clearCookie(w, origin, "session")
			clearCookie(w, origin, "csrf")
			http.Redirect(w, r, loginPath, http.StatusSeeOther)
			return
		}
		authenticated := false
		if value := r.PostForm.Get("login_token"); value != "" {
			authenticated = g.consumeLoginLink(value, origin)
			message = "Login link expired or already used. Run mw docker traefik credentials for a new one."
		} else {
			authenticated = g.credentialsMatch(r.PostForm.Get("username"), r.PostForm.Get("password"))
			message = "Username or password is incorrect."
		}
		if authenticated {
			session, err := g.sign("session", origin, sessionLifetime)
			if err != nil {
				http.Error(w, "Cannot create gateway session", http.StatusInternalServerError)
				return
			}
			writeCookie(w, origin, "session", session, sessionLifetime)
			clearCookie(w, origin, "csrf")
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}
		status = http.StatusUnauthorized
	}
	csrf, err := g.sign("csrf", origin, 15*time.Minute)
	if err != nil {
		http.Error(w, "Cannot create login form", http.StatusInternalServerError)
		return
	}
	writeCookie(w, origin, "csrf", csrf, 15*time.Minute)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	scriptHash := sha256.Sum256([]byte(loginScript))
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'sha256-"+base64.StdEncoding.EncodeToString(scriptHash[:])+"'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.WriteHeader(status)
	_ = loginPage.Execute(w, struct {
		Logout              bool
		CSRF, Return, Error string
	}{(r.URL.Path == logoutPath || r.URL.Path == legacyLogoutPath), csrf, target, message})
}

// A URL fragment never reaches the server. Remove it from browser history before
// submitting the code with the existing form's CSRF token and setting a session.
const loginScript = `const token = new URLSearchParams(location.hash.slice(1)).get('login');
if (token) {
    history.replaceState(null, '', location.pathname + location.search);
    const form = document.querySelector('form');
    form.querySelector('[name="login_token"]').value = token;
    form.submit();
}`

var loginPage = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>a Wikimedia CLI gateway</title>
<style>body{font:16px system-ui;background:#f5f6f8;color:#202122;margin:0;padding:12vh 20px}main{max-width:360px;margin:auto;background:white;padding:28px;border:1px solid #c8ccd1;border-radius:8px}h1{font-size:24px;margin-top:0}label{display:block;margin-top:16px}input,button{box-sizing:border-box;width:100%;padding:10px;font:inherit}input{margin-top:5px;border:1px solid #72777d;border-radius:3px}button{margin-top:22px;background:#36c;color:white;border:0;border-radius:3px;cursor:pointer}.error{color:#b32424}code{overflow-wrap:anywhere}</style>
<main><h1>{{if .Logout}}Sign out of the gateway{{else}}a Wikimedia CLI gateway{{end}}</h1>
{{if not .Logout}}<p>Use the credentials shown by <code>mw docker traefik credentials</code>.</p>{{end}}
{{if .Error}}<p class="error" role="alert">{{.Error}}</p>{{end}}
<form method="post" action="{{if .Logout}}/_wm/logout{{else}}/_wm/login{{end}}">
<input type="hidden" name="csrf" value="{{.CSRF}}"><input type="hidden" name="return" value="{{.Return}}">
{{if not .Logout}}<input type="hidden" name="login_token"><label>Username<input name="username" autocomplete="username" required autofocus></label><label>Password<input name="password" type="password" autocomplete="current-password" required></label>{{end}}
<button type="submit">{{if .Logout}}Sign out{{else}}Sign in{{end}}</button></form></main>{{if not .Logout}}<script>` + loginScript + `</script>{{end}}</html>`))

func main() {
	g, err := newGateway("/auth")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	server := &http.Server{Addr: ":8080", Handler: g, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute, MaxHeaderBytes: 32768}
	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
