package wiki

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

// A transparent 4×2 lossless WebP, generated specifically for these tests.
const downloadWebPFixture = "UklGRhoAAABXRUJQVlA4TA0AAAAvA0AAEAcQERGIiP4HAA=="

type wikiDownloadFixture struct {
	t                           *testing.T
	mu                          sync.Mutex
	base                        string
	preview                     []byte
	queries, assets             int
	widths                      []int
	status                      int
	missing, private, omitThumb bool
	thumbError                  string
}

func (f *wikiDownloadFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !strings.Contains(r.UserAgent(), cli.HTTPUserAgent) {
		f.t.Error("missing CLI user agent")
	}
	if err := r.ParseForm(); err != nil {
		f.t.Error(err)
	}
	if r.URL.Path == "/w/api.php" {
		w.Header().Set("Content-Type", "application/json")
		write := func(value interface{}) { _ = json.NewEncoder(w).Encode(value) }
		if r.Form.Get("meta") == "tokens" {
			write(map[string]interface{}{"query": map[string]interface{}{"tokens": map[string]string{"logintoken": "login-token"}}})
			return
		}
		if r.Form.Get("action") == "login" {
			if r.Form.Get("lgname") != "Name@download" || r.Form.Get("lgpassword") != "fixture-secret" {
				f.t.Error("wrong login credentials")
			}
			http.SetCookie(w, &http.Cookie{Name: "wiki-session", Value: "authenticated", Path: "/"})
			write(map[string]interface{}{"login": map[string]string{"result": "Success"}})
			return
		}
		f.queries++
		if r.Form.Get("action") != "query" || r.Form.Get("prop") != "imageinfo" || r.Form.Get("titles") != "File:Example.svg" {
			f.t.Errorf("unexpected file query: %v", r.Form)
		}
		if f.private {
			if _, err := r.Cookie("wiki-session"); err != nil || r.Form.Get("assert") != "user" {
				f.t.Error("API lost authenticated session")
			}
		}
		width, _ := strconv.Atoi(r.Form.Get("iiurlwidth"))
		f.widths = append(f.widths, width)
		info := map[string]interface{}{"url": f.base + "/source", "width": 4, "height": 2}
		if width > 0 && !f.omitThumb {
			info["thumburl"] = f.base + "/thumb.png"
		}
		if f.thumbError != "" {
			info["thumberror"] = f.thumbError
		}
		page := map[string]interface{}{"title": "File:Example.svg", "missing": true}
		// Shared-repository files can have imageinfo even when their local
		// description page is missing. Only absent imageinfo means no file.
		if !f.missing {
			page["imageinfo"] = []interface{}{info}
		}
		write(map[string]interface{}{"query": map[string]interface{}{"pages": []interface{}{page}}})
		return
	}
	f.assets++
	if f.private {
		if _, err := r.Cookie("wiki-session"); err != nil {
			f.t.Error("asset request lost authenticated cookie jar")
		}
	}
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte("upstream unavailable"))
		return
	}
	if r.URL.Path == "/source" {
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="4" height="2"/>`))
		return
	}
	if r.Header.Get("Accept") != "image/png" && r.Header.Get("Accept") != "image/jpeg" {
		f.t.Errorf("unexpected thumbnail Accept: %q", r.Header.Get("Accept"))
	}
	// Deliberately claim PNG while supplying WebP bytes: the decoder must
	// inspect the content instead of trusting the extension or HTTP metadata.
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(f.preview)
}

func newWikiDownloadFixture(t *testing.T) *wikiDownloadFixture {
	t.Helper()
	preview, err := base64.StdEncoding.DecodeString(downloadWebPFixture)
	if err != nil {
		t.Fatal(err)
	}
	f := &wikiDownloadFixture{t: t, preview: preview}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	f.base = server.URL
	t.Cleanup(server.Close)
	return f
}

func runWikiDownload(t *testing.T, f *wikiDownloadFixture, args ...string) ([]byte, error) {
	t.Helper()
	cmd := NewWikiCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs(append([]string{"file", "download", "File:Example.svg", "--wiki", f.base + "/w/api.php"}, args...))
	err := cmd.Execute()
	return output.Bytes(), err
}

func TestWikiFileDownloadFormatsAndExactWidth(t *testing.T) {
	for _, tc := range []struct {
		format  string
		width   int
		private bool
	}{
		{format: "png", width: 2},
		{format: "png", width: 4},
		{format: "png", width: 8},
		{format: "jpeg", width: 4},
		{format: "jpg", width: 4},
		{format: "png", private: true},
	} {
		t.Run(tc.format+strconv.Itoa(tc.width)+strconv.FormatBool(tc.private), func(t *testing.T) {
			f := newWikiDownloadFixture(t)
			f.private = tc.private
			output := filepath.Join(t.TempDir(), "result")
			args := []string{"--format", tc.format, "--width", strconv.Itoa(tc.width), "--output", output}
			if tc.private {
				args = append(args, "--user", "Name@download", "--password", "fixture-secret")
			}
			if _, err := runWikiDownload(t, f, args...); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			decoded, format, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			wantFormat := tc.format
			if wantFormat == "jpg" {
				wantFormat = "jpeg"
			}
			width := tc.width
			if width == 0 {
				width = 4
			}
			if format != wantFormat || decoded.Bounds().Dx() != width || decoded.Bounds().Dy() != width/2 {
				t.Fatalf("downloaded %s %v; want %s at width %d", format, decoded.Bounds(), wantFormat, width)
			}
			r, g, b, a := decoded.At(0, 0).RGBA()
			if wantFormat == "png" && a != 0 {
				t.Fatal("PNG lost transparency")
			}
			if wantFormat == "jpeg" && (r < 65000 || g < 65000 || b < 65000 || a != 65535) {
				t.Fatal("JPEG did not flatten transparency onto white")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if tc.width == 0 && (len(f.widths) != 2 || f.widths[0] != 0 || f.widths[1] != 4) {
				t.Fatalf("native render width not requested: %v", f.widths)
			}
		})
	}
}

func TestWikiFileDownloadOriginalAndBinaryStdout(t *testing.T) {
	for _, format := range []string{"original", "png"} {
		t.Run(format, func(t *testing.T) {
			f := newWikiDownloadFixture(t)
			data, err := runWikiDownload(t, f, "--format", format, "--output", "-")
			if err != nil {
				t.Fatal(err)
			}
			if format == "original" {
				if string(data) != `<svg xmlns="http://www.w3.org/2000/svg" width="4" height="2"/>` {
					t.Fatalf("original bytes changed: %q", data)
				}
			} else if _, err := png.Decode(bytes.NewReader(data)); err != nil {
				t.Fatalf("binary stdout was not PNG: %v", err)
			}
			if bytes.Contains(data, []byte("Downloaded")) {
				t.Fatal("binary stdout contains progress text")
			}
		})
	}
}

func TestWikiFileDownloadFailurePreservesExistingOutput(t *testing.T) {
	for _, kind := range []string{"exists", "overwrite", "bad preview", "HTTP 503", "missing file", "missing thumbnail", "thumbnail error"} {
		t.Run(kind, func(t *testing.T) {
			f := newWikiDownloadFixture(t)
			output := filepath.Join(t.TempDir(), "existing.png")
			if err := os.WriteFile(output, []byte("keep me"), 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"--format", "png", "--width", "2", "-o", output}
			if kind != "exists" {
				args = append(args, "--overwrite")
			}
			switch kind {
			case "bad preview":
				f.preview = []byte("<html>upstream error</html>")
			case "HTTP 503":
				f.status = 503
			case "missing file":
				f.missing = true
			case "missing thumbnail":
				f.omitThumb = true
			case "thumbnail error":
				f.thumbError = "Renderer failed"
			}
			_, err := runWikiDownload(t, f, args...)
			data, readErr := os.ReadFile(output)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if kind == "overwrite" {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := png.Decode(bytes.NewReader(data)); err != nil {
					t.Fatal(err)
				}
			} else if err == nil || string(data) != "keep me" {
				t.Fatalf("failure damaged existing file: error=%v bytes=%q", err, data)
			}
			matches, _ := filepath.Glob(filepath.Join(filepath.Dir(output), ".mwcli-download-*"))
			if len(matches) > 0 {
				t.Fatalf("left temporary output files: %v", matches)
			}
		})
	}
}

func TestWikiFileDownloadInfersWikiFromDescriptionURL(t *testing.T) {
	f := newWikiDownloadFixture(t)
	cmd := NewWikiCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"file", "download", f.base + "/wiki/File:Example.svg", "--format", "png", "--width", "2", "-o", "-"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(&output); err != nil {
		t.Fatal(err)
	}
}

func TestWikiFileDownloadValidationBeforeNetwork(t *testing.T) {
	for _, flags := range [][]string{
		{"--format", "svg"},
		{"--width", "-1", "--format", "png"},
		{"--width", "16385", "--format", "png"},
		{"--width", "200"},
	} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			f := newWikiDownloadFixture(t)
			if _, err := runWikiDownload(t, f, flags...); err == nil {
				t.Fatal("invalid request accepted")
			}
			if f.queries != 0 || f.assets != 0 {
				t.Fatal("invalid request contacted wiki")
			}
		})
	}
	for _, input := range []string{"", "File:", "File:A|B.png", "ftp://example.org/wiki/File:A.png", "https://user:secret@example.org/wiki/File:A.png", "https://example.org/thumb/file.png"} {
		if _, _, err := wikiDownloadSource(input); err == nil {
			t.Fatalf("invalid source accepted: %q", input)
		}
	}
}

func TestWikiFileDownloadRejectsHugeDecodedImage(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeWikiDownload(bytes.NewReader(input.Bytes()), int(maxConvertedImagePixels)+1); err == nil {
		t.Fatal("excessive output dimensions accepted")
	}
	// Supply a valid IHDR describing an enormous input canvas. Refuse it
	// before decoding pixel data or allocating that canvas.
	data := input.Bytes()
	binary.BigEndian.PutUint32(data[16:20], 100000)
	binary.BigEndian.PutUint32(data[20:24], 100000)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width != 100000 {
		t.Fatalf("invalid huge-image fixture: %+v %v", config, err)
	}
	if _, err := decodeWikiDownload(bytes.NewReader(data), 1); err == nil {
		t.Fatal("excessive input dimensions accepted")
	}
}

func TestWikiFileDownloadDefaultFilename(t *testing.T) {
	t.Chdir(t.TempDir())
	f := newWikiDownloadFixture(t)
	if _, err := runWikiDownload(t, f, "--format", "png"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("Example.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}
