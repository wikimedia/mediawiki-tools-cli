package devremote

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestEditorProxyAuthenticatesHTTPAndWebSockets(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			if r.FormValue("password") != "generated-editor-password" {
				http.Error(w, "bad login", 401)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "editor", Value: "private-cookie", Path: "/"})
			w.Header().Set("Location", "/")
			w.WriteHeader(http.StatusFound)
			return
		}
		cookie, err := r.Cookie("editor")
		if err != nil || cookie.Value != "private-cookie" {
			http.Error(w, "authentication required", 401)
			return
		}
		if r.URL.Path == "/socket" {
			upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
			connection, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer connection.Close()
			kind, message, err := connection.ReadMessage()
			if err == nil {
				connection.WriteMessage(kind, message)
			}
			return
		}
		io.WriteString(w, "editor workspace")
	}))
	defer backend.Close()
	frontend := httptest.NewUnstartedServer(nil)
	proxy, err := browserEditorProxy(context.Background(), backend.Listener.Addr().(*net.TCPAddr).Port, frontend.Listener.Addr().(*net.TCPAddr).Port, "generated-editor-password")
	if err != nil {
		t.Fatal(err)
	}
	frontend.Config.Handler = proxy
	frontend.Start()
	defer frontend.Close()
	response, err := http.Get(frontend.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || string(data) != "editor workspace" {
		t.Fatalf("editor response %d: %s", response.StatusCode, data)
	}
	for _, tc := range []struct{ host, origin string }{
		{"unrelated.example:6197", ""}, {strings.TrimPrefix(frontend.URL, "http://"), "http://unrelated.example"}, {strings.TrimPrefix(frontend.URL, "http://"), "null"},
	} {
		request, _ := http.NewRequest(http.MethodGet, frontend.URL+"/", nil)
		request.Host = tc.host
		request.Header.Set("Origin", tc.origin)
		rejected, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		rejected.Body.Close()
		if rejected.StatusCode != http.StatusForbidden {
			t.Fatalf("accepted foreign host/origin %+v: %d", tc, rejected.StatusCode)
		}
	}
	connection, handshake, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(frontend.URL, "http")+"/socket", http.Header{"Origin": []string{frontend.URL}})
	if handshake != nil && handshake.Body != nil {
		handshake.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.WriteMessage(websocket.TextMessage, []byte("terminal data")); err != nil {
		t.Fatal(err)
	}
	_, message, err := connection.ReadMessage()
	if err != nil || string(message) != "terminal data" {
		t.Fatalf("websocket echo %q: %v", message, err)
	}
}

func TestSessionCapabilityCannotBeOmitted(t *testing.T) {
	called := false
	handler := authenticatedBridge("session-only-key", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(200) }))
	for _, key := range []string{"", "Bearer wrong", "Bearer session-only-key"} {
		called = false
		request := httptest.NewRequest(http.MethodPost, "/invoke", nil)
		request.Header.Set("Authorization", key)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if called != (key == "Bearer session-only-key") {
			t.Fatalf("capability accepted incorrectly: %q", key)
		}
	}
}
