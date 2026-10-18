package updater

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestDownloadFileResponseHTTP(t *testing.T) {
	const payload = "replacement executable fixture"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/200", http.StatusFound)
			return
		}
		status, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, payload)
	}))
	defer server.Close()

	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError, http.StatusNotModified} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			response, err := DownloadFileResponse(server.URL + "/" + strconv.Itoa(status))
			if response != nil {
				_ = response.Body.Close()
				t.Fatal("unsuccessful response returned to caller")
			}
			if err == nil || !strings.Contains(err.Error(), strconv.Itoa(status)) || !strings.Contains(err.Error(), http.StatusText(status)) {
				t.Fatalf("expected an error containing HTTP status, got %v", err)
			}
		})
	}
	for _, path := range []string{"/200", "/redirect"} {
		t.Run(path, func(t *testing.T) {
			response, err := DownloadFileResponse(server.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || string(body) != payload {
				t.Fatalf("body = %q, err = %v", body, err)
			}
		})
	}
}

func TestDownloadFileResponseLocalFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local update fixture")
	const payload = "local executable fixture"
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	for _, location := range []string{path, fileURL} {
		t.Run(location, func(t *testing.T) {
			if runtime.GOOS == "windows" && strings.HasPrefix(location, "file://") {
				t.Skip("file URLs use POSIX paths; native Windows paths are covered separately")
			}
			response, err := DownloadFileResponse(location)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || string(body) != payload || response.ContentLength != int64(len(payload)) {
				t.Fatalf("body = %q, length = %d, err = %v", body, response.ContentLength, err)
			}
		})
	}
}

type downloadRoundTripper func(*http.Request) (*http.Response, error)

func (f downloadRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type downloadBody struct {
	io.Reader
	closes int
}

func (b *downloadBody) Close() error {
	b.closes++
	return nil
}

func TestDownloadFileResponseBodyOwnership(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, status := range []int{http.StatusOK, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			body := &downloadBody{Reader: strings.NewReader("fixture")}
			http.DefaultTransport = downloadRoundTripper(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Status: strconv.Itoa(status) + " " + http.StatusText(status), Body: body}, nil
			})
			response, err := DownloadFileResponse("http://update.invalid/fixture")
			if status != http.StatusOK {
				if err == nil || response != nil || body.closes != 1 {
					t.Fatalf("rejected response: err=%v, response=%v, closes=%d", err, response, body.closes)
				}
				return
			}
			if err != nil || response == nil || body.closes != 0 {
				t.Fatalf("successful response closed early: err=%v, closes=%d", err, body.closes)
			}
			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if body.closes != 1 {
				t.Fatalf("body close count = %d", body.closes)
			}
		})
	}
}

func TestDownloadFileResponseReturnsErrors(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	failure := errors.New("connection failed")
	http.DefaultTransport = downloadRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, failure
	})
	response, err := DownloadFileResponse("http://update.invalid/fixture")
	if response != nil {
		_ = response.Body.Close()
		t.Fatal("response returned with a transport error")
	}
	if !errors.Is(err, failure) {
		t.Fatalf("expected the transport error, got %v", err)
	}
	for _, location := range []string{"file://%invalid", "file:///missing-mwcli-update-fixture"} {
		response, err := DownloadFileResponse(location)
		if response != nil {
			_ = response.Body.Close()
			t.Fatalf("%s: response returned for an invalid file", location)
		}
		if err == nil {
			t.Fatalf("%s: expected a file error", location)
		}
	}
}
