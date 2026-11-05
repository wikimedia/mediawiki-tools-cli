package enterprise

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadResumeDoesNotForwardCredentials(t *testing.T) {
	interrupted := true
	var ranges []string
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("bearer token leaked to signed storage")
		}
		w.Header().Set("ETag", `"version-1"`)
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "10")
			return
		}
		ranges = append(ranges, r.Header.Get("Range"))
		if r.Header.Get("Range") == "bytes=5-" {
			if r.Header.Get("If-Range") != `"version-1"` {
				t.Error("missing If-Range")
			}
			w.Header().Set("Content-Range", "bytes 5-9/10")
			w.WriteHeader(206)
			fmt.Fprint(w, "later")
			return
		}
		w.Header().Set("Content-Length", "10")
		if interrupted {
			fmt.Fprint(w, "first")
			interrupted = false
			return
		}
		fmt.Fprint(w, "firstlater")
	}))
	defer storage.Close()
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-secret" {
			t.Error("missing API authentication")
		}
		http.Redirect(w, r, storage.URL+"/file?signature=private-secret", 307)
	})
	destination := filepath.Join(t.TempDir(), "snapshot.tar.gz")
	if err := c.Download(t.Context(), "/v2/snapshots/enwiki_namespace_0/download", destination, false, io.Discard); err == nil {
		t.Fatal("interruption accepted as success")
	}
	part, err := os.ReadFile(destination + ".part")
	if err != nil || string(part) != "first" {
		t.Fatalf("partial download lost: %s %v", part, err)
	}
	state, _ := os.ReadFile(destination + ".part.json")
	if bytes.Contains(state, []byte("secret")) || bytes.Contains(state, []byte("signature")) {
		t.Fatal("signed URL or token persisted")
	}
	if err := c.Download(t.Context(), "/v2/snapshots/enwiki_namespace_0/download", destination, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "firstlater" || len(ranges) != 2 || ranges[1] != "bytes=5-" {
		t.Fatalf("resume failed: %s %v %v", data, err, ranges)
	}
	if err := c.Download(t.Context(), "/v2/snapshots/enwiki_namespace_0/download", destination, true, io.Discard); err == nil {
		t.Fatal("overwrote destination")
	}
}

func TestDownloadRefusesChangedDatasetOrIgnoredRange(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", "10")
				w.Header().Set("ETag", `"original"`)
				if changed {
					w.Header().Set("ETag", `"rotated"`)
				}
				fmt.Fprint(w, "firstlater")
			}))
			defer storage.Close()
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, storage.URL, 307) })
			file := filepath.Join(t.TempDir(), "bundle")
			if err := os.WriteFile(file+".part", []byte("first"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := WriteState(file+".part.json", downloadState{Path: "/download", ETag: `"original"`, Size: 10}); err != nil {
				t.Fatal(err)
			}
			if err := c.Download(t.Context(), "/download", file, true, io.Discard); err == nil {
				t.Fatal("unsafe resume accepted")
			}
			data, _ := os.ReadFile(file + ".part")
			if string(data) != "first" {
				t.Fatal("unsafe resume modified partial data")
			}
		})
	}
}

func TestStorageURLRejectsUnsafeRedirects(t *testing.T) {
	for _, target := range []string{"http://example.org/private", "https://user:password@example.org/file", "file:///tmp/file"} {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target, 307) })
		file := filepath.Join(t.TempDir(), "bundle")
		err := c.Download(t.Context(), "/download", file, false, io.Discard)
		if err == nil {
			t.Fatal("unsafe redirect accepted", target)
		}
		if strings.Contains(err.Error(), "password") {
			t.Fatal("redirect credentials leaked")
		}
	}
}
