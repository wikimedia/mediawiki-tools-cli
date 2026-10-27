package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTunnelProxyRejectsUnauthenticatedRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unauthenticated request reached upstream") }))
	defer upstream.Close()
	g, _ := testGateway(t)
	if err := g.setUpstream(upstream.URL); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/proxy/dashboard/", "/proxy/adminer/", "/w/api.php"} {
		t.Run(path, func(t *testing.T) {
			req := request(http.MethodGet, path, nil)
			req.Host = "adminer.local.wmftest.net"
			req.Header.Set("X-Forwarded-Host", "localhost")
			req.Header.Set("X-Forwarded-Uri", loginPath)
			req.Header.Set("X-Forwarded-Method", "OPTIONS")
			response := httptest.NewRecorder()
			g.ServeHTTP(response, req)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	response := httptest.NewRecorder()
	req := request(http.MethodGet, "/proxy/dashboard/", nil)
	req.Header.Set("Accept", "text/html")
	g.ServeHTTP(response, req)
	if response.Code != http.StatusSeeOther || !strings.HasPrefix(response.Header().Get("Location"), loginPath+"?") {
		t.Fatalf("browser did not get login redirect: %v", response.Result())
	}
	response = httptest.NewRecorder()
	g.ServeHTTP(response, request(http.MethodGet, loginPath, nil))
	if response.Code != http.StatusOK {
		t.Fatal("login form requires authentication")
	}
}

func TestTunnelProxyPreservesApplicationRequest(t *testing.T) {
	var received int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received++
		body, _ := io.ReadAll(r.Body)
		if r.Host != "gateway.example" || r.Header.Get("X-Forwarded-Host") != "gateway.example" || r.Header.Get("X-Forwarded-Proto") != "https" {
			t.Errorf("wrong forwarded origin: host=%s headers=%v", r.Host, r.Header)
		}
		if r.URL.RequestURI() != "/proxy/default/w/api.php?action=edit" || string(body) != "article content" {
			t.Errorf("request changed: %s %q", r.URL, body)
		}
		if auth := r.Header.Get("Authorization"); auth != "" && auth != "Bearer application-token" {
			t.Errorf("gateway credentials leaked: %s", auth)
		}
		fmt.Fprint(w, "upstream response")
	}))
	defer upstream.Close()
	g, _ := testGateway(t)
	if err := g.setUpstream(upstream.URL); err != nil {
		t.Fatal(err)
	}
	session := login(t, g)
	for _, basic := range []bool{true, false} {
		req := request(http.MethodPost, "/proxy/default/w/api.php?action=edit", nil)
		req.Body = io.NopCloser(strings.NewReader("article content"))
		req.ContentLength = int64(len("article content"))
		req.Header.Set("X-Forwarded-Host", "attacker.example")
		if basic {
			req.SetBasicAuth("alice", "secret")
		} else {
			req.AddCookie(session)
			req.Header.Set("Authorization", "Bearer application-token")
		}
		response := httptest.NewRecorder()
		g.ServeHTTP(response, req)
		if response.Code != http.StatusOK || response.Body.String() != "upstream response" {
			t.Fatalf("proxy returned %d %s", response.Code, response.Body.String())
		}
	}
	if received != 2 {
		t.Fatalf("upstream requests=%d", received)
	}
}

func TestTunnelProxyIgnoresLegacyAuthOptOut(t *testing.T) {
	t.Setenv("TRAEFIK_AUTH_MIDDLEWARE", "forwarded-https@file")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "opt-out") }))
	defer upstream.Close()
	g, _ := testGateway(t)
	if err := g.setUpstream(upstream.URL); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	g.ServeHTTP(response, request(http.MethodGet, "/proxy/dashboard/", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("legacy opt-out bypassed authentication: %d %s", response.Code, response.Body.String())
	}
}
