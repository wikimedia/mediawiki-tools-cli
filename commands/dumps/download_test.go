package dumps

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/progress"
)

func TestDownloadProgressUpdatesOneLine(t *testing.T) {
	var output, destination bytes.Buffer
	tracker := newDownloadProgress(&destination, &output, 40, 100)
	tracker.terminal = true
	tracker.bar = progress.New(progress.WithWidth(20))
	if err := tracker.render(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "40%") || !strings.Contains(output.String(), "40 / 100 bytes") {
		t.Fatalf("missing resumed progress: %q", output.String())
	}
	output.Reset()
	tracker.last = time.Now().Add(-time.Second)
	if n, err := tracker.Write([]byte("ten bytes!")); err != nil || n != 10 {
		t.Fatalf("write=%d, %v", n, err)
	}
	if !strings.HasPrefix(output.String(), "\r\033[K") || strings.Contains(output.String(), "\n") ||
		!strings.Contains(output.String(), "50%") || !strings.Contains(output.String(), "50 / 100 bytes") {
		t.Fatalf("expected in-place progress: %q", output.String())
	}
	if destination.String() != "ten bytes!" || tracker.done != 50 {
		t.Fatalf("destination=%q, done=%d", destination.String(), tracker.done)
	}
	output.Reset()
	if err := tracker.clear(); err != nil || output.String() != "\r\033[K" {
		t.Fatalf("clear=%q, err=%v", output.String(), err)
	}
}

func TestDownloadProgressRedirectedOutput(t *testing.T) {
	var output, destination bytes.Buffer
	tracker := newDownloadProgress(&destination, &output, 0, 3)
	if err := tracker.render(); err != nil {
		t.Fatal(err)
	}
	if _, err := tracker.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if err := tracker.clear(); err != nil || output.Len() != 0 {
		t.Fatalf("redirected progress=%q, err=%v", output.String(), err)
	}
	client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "3")
		if r.Method != http.MethodHead {
			fmt.Fprint(w, "abc")
		}
	})
	file := filepath.Join(t.TempDir(), "dump.gz")
	if err := client.download(t.Context(), base, file, false, "", &output); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("Downloading %s: 0 / 3 bytes\nSaved %s (3 bytes).\n", file, file)
	if output.String() != want {
		t.Fatalf("output=%q; want %q", output.String(), want)
	}
}

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestExportChecksumUsesOfficialManifest(t *testing.T) {
	client := newClient(func(*url.URL) error { return nil })
	hash := strings.Repeat("a", 64)
	status := http.StatusOK
	calls := 0
	client.http.Transport = testTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() != officialURL+"other/mediawiki_content_current/testwiki/2026-11-01/xml/bzip2/SHA256SUMS" {
			t.Fatalf("unexpected manifest request %s", req.URL)
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(hash + "  file.xml.bz2\n"))}, nil
	})
	relative := "other/mediawiki_content_current/testwiki/2026-11-01/xml/bzip2/file.xml.bz2"
	if got, err := client.exportChecksum(t.Context(), relative, ""); err != nil || got != hash {
		t.Fatalf("checksum=%s err=%v", got, err)
	}
	if _, err := client.exportChecksum(t.Context(), relative, strings.Repeat("b", 64)); err == nil {
		t.Fatal("accepted conflicting checksum")
	}
	status = http.StatusNotFound
	if _, err := client.exportChecksum(t.Context(), relative, ""); err == nil {
		t.Fatal("accepted export without completion manifest")
	}
	if got, err := client.exportChecksum(t.Context(), "wikidatawiki/entities/file.json.gz", hash); err != nil || got != hash || calls != 3 {
		t.Fatal("ordinary formats should not require a content-export manifest")
	}
}

func TestDownloadCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		if r.Method == http.MethodHead {
			return
		}
		fmt.Fprint(w, "partial")
		w.(http.Flusher).Flush()
		cancel()
		<-r.Context().Done()
	})
	destination := filepath.Join(t.TempDir(), "file")
	if err := client.download(ctx, base, destination, false, "", io.Discard); err == nil {
		t.Fatal("cancelled download succeeded")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("cancelled download published destination")
	}
}

func TestDownloadPreservesFormatsAndVerifiesHash(t *testing.T) {
	payload := []byte{0x1f, 0x8b, 0, 0xff, '\n', 'x'}
	client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		w.Header().Set("ETag", `"stable"`)
		if r.Method != http.MethodHead {
			_, _ = w.Write(payload)
		}
	})
	for _, extension := range []string{"json.gz", "ttl.bz2", "sql.gz", "xml.bz2", "parquet", "7z", "zst"} {
		t.Run(extension, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "dump."+extension)
			sum := sha256.Sum256(payload)
			if err := client.download(t.Context(), base.JoinPath("file"), destination, false, hex.EncodeToString(sum[:]), io.Discard); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(destination)
			if err != nil || string(got) != string(payload) {
				t.Fatalf("changed file bytes: %q, %v", got, err)
			}
			for _, suffix := range []string{".part", ".part.json"} {
				if _, err := os.Stat(destination + suffix); !os.IsNotExist(err) {
					t.Fatalf("left partial state %s: %v", suffix, err)
				}
			}
			if err := client.download(t.Context(), base.JoinPath("file"), destination, false, "", io.Discard); err == nil {
				t.Fatal("overwrote completed file")
			}
		})
	}
}

func TestDownloadResumeAndValidation(t *testing.T) {
	const payload = "abcdefghij"
	for _, scenario := range []string{"success", "changed", "ignored-range", "wrong-range", "weak", "missing-metadata", "checksum"} {
		t.Run(scenario, func(t *testing.T) {
			client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				tag := `"stable"`
				if scenario == "changed" {
					tag = `"changed"`
				} else if scenario == "weak" {
					tag = `W/"weak"`
				}
				w.Header().Set("ETag", tag)
				if r.Method == http.MethodHead {
					w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
					return
				}
				if r.Header.Get("Range") != "bytes=4-" || r.Header.Get("If-Range") != `"stable"` {
					t.Error("resume did not send correct validators")
				}
				if scenario == "ignored-range" {
					fmt.Fprint(w, payload)
					return
				}
				header := "bytes 4-9/10"
				if scenario == "wrong-range" {
					header = "bytes 0-5/10"
				}
				w.Header().Set("Content-Range", header)
				w.WriteHeader(http.StatusPartialContent)
				fmt.Fprint(w, payload[4:])
			})
			destination := filepath.Join(t.TempDir(), "file")
			source := base.JoinPath("file")
			if err := os.WriteFile(destination+".part", []byte(payload[:4]), 0o600); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256([]byte(payload))
			checksum := hex.EncodeToString(sum[:])
			if scenario == "checksum" {
				checksum = strings.Repeat("0", 64)
			}
			state := downloadState{URL: source.String(), ETag: `"stable"`, Size: int64(len(payload)), SHA256: checksum}
			data, _ := json.Marshal(state)
			if scenario != "missing-metadata" {
				if err := os.WriteFile(destination+".part.json", data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := client.download(t.Context(), source, destination, true, checksum, io.Discard)
			if scenario == "success" {
				got, readErr := os.ReadFile(destination)
				if err != nil || readErr != nil || string(got) != payload {
					t.Fatalf("resume: %q, %v, %v", got, err, readErr)
				}
			} else {
				if err == nil {
					t.Fatal("unsafe resume succeeded")
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatal("failed download published a completed file")
				}
				if _, err := os.Stat(destination + ".part"); err != nil {
					t.Fatal("failed resume removed partial file")
				}
			}
		})
	}
}

func TestDownloadFailurePreservesPartialAndRefusesOverwrite(t *testing.T) {
	client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.Header().Set("ETag", `"stable"`)
		if r.Method != http.MethodHead {
			fmt.Fprint(w, "truncated")
		}
	})
	destination := filepath.Join(t.TempDir(), "file")
	if err := client.download(t.Context(), base.JoinPath("file"), destination, false, "", io.Discard); err == nil {
		t.Fatal("accepted truncated transfer")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("published incomplete file")
	}
	data, err := os.ReadFile(destination + ".part")
	if err != nil || string(data) != "truncated" {
		t.Fatalf("lost partial data: %q, %v", data, err)
	}
	if err := client.download(t.Context(), base.JoinPath("file"), destination, false, "", io.Discard); err == nil {
		t.Fatal("overwrote partial without --resume")
	}
}

func TestDownloadNeverOverwritesRacingDestination(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "file")
	client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "3")
		if r.Method != http.MethodHead {
			if err := os.WriteFile(destination, []byte("keep me"), 0o600); err != nil {
				t.Error(err)
			}
			fmt.Fprint(w, "new")
		}
	})
	if err := client.download(t.Context(), base.JoinPath("file"), destination, false, "", io.Discard); err == nil {
		t.Fatal("overwrote racing destination")
	}
	if data, err := os.ReadFile(destination); err != nil || string(data) != "keep me" {
		t.Fatalf("destination changed: %q, %v", data, err)
	}
}

func TestContentExportChecksum(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, filename := range []string{"file.xml.bz2", "*file.xml.bz2", "./file.xml.bz2"} {
		got, err := checksumFor([]byte(hash+"  "+filename+"\n"), "file.xml.bz2")
		if err != nil || got != hash {
			t.Fatalf("manifest %s = %s, %v", filename, got, err)
		}
	}
	for _, manifest := range []string{"", hash + "  other.xml.bz2", "bad  file.xml.bz2"} {
		if _, err := checksumFor([]byte(manifest), "file.xml.bz2"); err == nil {
			t.Errorf("accepted manifest %q", manifest)
		}
	}
	if !contentExport("other/mediawiki_content_current/enwiki/2026-11-01/xml/bzip2/file.xml.bz2") ||
		contentExport("wikidatawiki/entities/file.json.bz2") {
		t.Fatal("incorrect content export detection")
	}
}

func TestChecksumFailureAndDestinationTypes(t *testing.T) {
	client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "3")
		if r.Method != http.MethodHead {
			fmt.Fprint(w, "abc")
		}
	})
	target := filepath.Join(t.TempDir(), "file")
	if err := client.download(t.Context(), base, target, false, strings.Repeat("0", 64), io.Discard); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("checksum failure=%v", err)
	}
	for _, destination := range []string{"", "-"} {
		if err := client.download(t.Context(), &url.URL{}, destination, false, "", io.Discard); err == nil {
			t.Fatal("accepted invalid destination")
		}
	}
}
