package devremote

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// browserEditorProxy authenticates the private editor once and adds its cookie
// to HTTP/WebSocket requests. Neither the browser URL nor process arguments
// need to contain the generated editor password.
func browserEditorProxy(ctx context.Context, upstreamPort, publicPort int, password string) (http.Handler, error) {
	upstream := &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", upstreamPort)}
	client := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, upstream.String()+"/login", strings.NewReader(url.Values{"password": {password}}.Encode()))
	if err != nil {
		return nil, err
	}
	request.Host = fmt.Sprintf("127.0.0.1:%d", publicPort)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connect to the browser editor: %w", err)
	}
	response.Body.Close()
	cookies := response.Cookies()
	if response.StatusCode < 200 || response.StatusCode >= 400 || len(cookies) == 0 {
		return nil, errors.New("could not authenticate the browser editor")
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		existing := r.Cookies()
		r.Header.Del("Cookie")
		for _, cookie := range existing {
			replace := false
			for _, auth := range cookies {
				if auth.Name == cookie.Name {
					replace = true
					break
				}
			}
			if !replace {
				r.AddCookie(cookie)
			}
		}
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
	}
	proxy.Transport = &http.Transport{Proxy: nil}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed := false
		for _, host := range []string{fmt.Sprintf("127.0.0.1:%d", publicPort), fmt.Sprintf("localhost:%d", publicPort), fmt.Sprintf("[::1]:%d", publicPort)} {
			if strings.EqualFold(r.Host, host) {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, "unexpected browser editor host", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Scheme != "http" || !strings.EqualFold(parsed.Host, r.Host) || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
				http.Error(w, "unexpected browser editor origin", http.StatusForbidden)
				return
			}
		}
		proxy.ServeHTTP(w, r)
	}), nil
}
