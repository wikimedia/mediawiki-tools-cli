package wiki

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestWikiFileUpload(t *testing.T) {
	cases := []struct {
		name        string
		warning     string
		flags       []string
		wantErr     string
		wantUploads int32
	}{
		{name: "new file", wantUploads: 1},
		{name: "existing file refused", warning: `{"exists":"Demo.webm"}`, wantErr: "--replace", wantUploads: 1},
		{name: "new version", warning: `{"exists":"Demo.webm"}`, flags: []string{"--replace"}, wantUploads: 2},
		{name: "ignore warnings cannot overwrite", warning: `{"exists":"Demo.webm"}`, flags: []string{"--ignore-warnings"}, wantErr: "--replace", wantUploads: 1},
		{name: "replace does not ignore unrelated warnings", warning: `{"exists":"Demo.webm","duplicate":["Other.webm"]}`, flags: []string{"--replace"}, wantErr: "--ignore-warnings", wantUploads: 1},
		{name: "other warnings explicitly accepted", warning: `{"duplicate":["Other.webm"]}`, flags: []string{"--ignore-warnings"}, wantUploads: 2},
		{name: "both permissions explicit", warning: `{"exists":"Demo.webm","duplicate":["Other.webm"]}`, flags: []string{"--replace", "--ignore-warnings"}, wantUploads: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte("binary-video\x00\xff\r\n")
			dir := t.TempDir()
			file := filepath.Join(dir, "video.webm")
			password := filepath.Join(dir, "password")
			description := filepath.Join(dir, "description.wiki")
			for name, data := range map[string][]byte{
				file:        payload,
				password:    []byte("fixture-secret\n"),
				description: []byte("== Description ==\nDemo.\n{{CC-BY-SA-4.0}}"),
			} {
				if err := os.WriteFile(name, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var uploads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.UserAgent() == "" || !strings.Contains(r.UserAgent(), cli.HTTPUserAgent) {
					t.Error("missing CLI user agent")
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
					return
				}
				if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						t.Error(err)
						return
					}
					defer r.MultipartForm.RemoveAll()
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.Form.Get("action") {
				case "query":
					if r.Form.Get("type") == "login" {
						fmt.Fprint(w, `{"query":{"tokens":{"logintoken":"login-fixture"}}}`)
					} else {
						cookie, err := r.Cookie("wiki-session")
						if err != nil || cookie.Value != "authenticated" {
							t.Error("CSRF request lost login session")
						}
						fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"csrf-fixture"}}}`)
					}
				case "login":
					if r.Form.Get("lgname") != "User@bot" || r.Form.Get("lgpassword") != "fixture-secret" || r.Form.Get("lgtoken") != "login-fixture" {
						t.Error("incorrect login parameters")
					}
					http.SetCookie(w, &http.Cookie{Name: "wiki-session", Value: "authenticated", Path: "/"})
					fmt.Fprint(w, `{"login":{"result":"Success"}}`)
				case "upload":
					count := uploads.Add(1)
					cookie, err := r.Cookie("wiki-session")
					if err != nil || cookie.Value != "authenticated" {
						t.Error("upload lost login session")
					}
					for key, expected := range map[string]string{
						"assert": "user", "token": "csrf-fixture", "filename": "Demo.webm", "comment": "Update demo", "text": "== Description ==\nDemo.\n{{CC-BY-SA-4.0}}",
					} {
						if actual := r.Form.Get(key); actual != expected {
							t.Errorf("%s = %q, want %q", key, actual, expected)
						}
					}
					if count == 1 {
						if r.Form.Get("ignorewarnings") != "" {
							t.Error("first request must not ignore warnings")
						}
						part, _, err := r.FormFile("file")
						if err != nil {
							t.Error(err)
							return
						}
						actual, err := io.ReadAll(part)
						part.Close()
						if err != nil || !bytes.Equal(actual, payload) {
							t.Errorf("file changed in transit: %q (%v)", actual, err)
						}
						if tc.warning != "" {
							fmt.Fprintf(w, `{"upload":{"result":"Warning","filekey":"stash-fixture","warnings":%s}}`, tc.warning)
							return
						}
					} else {
						if r.Form.Get("filekey") != "stash-fixture" || r.Form.Get("ignorewarnings") != "1" {
							t.Error("retry must commit the previously stashed file")
						}
						if r.MultipartForm != nil {
							t.Error("retry resent file data")
						}
					}
					fmt.Fprint(w, `{"upload":{"result":"Success","filename":"Demo.webm","imageinfo":{"descriptionurl":"https://example.test/wiki/File:Demo.webm"}}}`)
				default:
					t.Errorf("unexpected action: %s", r.Form.Get("action"))
					http.Error(w, "unexpected action", http.StatusBadRequest)
				}
			}))
			defer server.Close()
			var output bytes.Buffer
			cmd := NewWikiCmd()
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			args := []string{"file", "upload", file, "--wiki", server.URL + "/w/api.php", "--user", "User@bot", "--password-file", password, "--filename", "Demo.webm", "--comment", "Update demo", "--text-file", description}
			cmd.SetArgs(append(args, tc.flags...))
			err := cmd.Execute()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(output.String(), "https://example.test/wiki/File:Demo.webm") {
					t.Errorf("missing success URL: %s", &output)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			if actual := uploads.Load(); actual != tc.wantUploads {
				t.Errorf("%d upload requests, want %d", actual, tc.wantUploads)
			}
			if strings.Contains(output.String(), "fixture-secret") {
				t.Error("password leaked to output")
			}
		})
	}
}

func TestWikiFileUploadDryRun(t *testing.T) {
	file := filepath.Join(t.TempDir(), "demo.webm")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	cmd := NewWikiCmd()
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"file", "upload", file, "--wiki", "https://invalid.invalid/w/api.php", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "as demo.webm") {
		t.Fatalf("wrong default filename: %s", &output)
	}
}

func TestWikiFileUploadFailures(t *testing.T) {
	cases := []struct {
		name, response, want string
		status               int
	}{
		{"API error", `{"error":{"code":"permissiondenied","info":"No upload right"}}`, "permissiondenied", 200},
		{"bad response", `not JSON`, "decode upload response", 200},
		{"HTTP error", `unavailable`, "503", 503},
		{"missing result", `{}`, "unexpected upload result", 200},
		{"missing stash key", `{"upload":{"result":"Warning","warnings":{"exists":"Demo.webm"}}}`, "filekey", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			f, err := os.CreateTemp(t.TempDir(), "video")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			f.WriteString("fixture")
			f.Seek(0, io.SeekStart)
			_, err = uploadWikiFile(t.Context(), server.Client(), server.URL, "fixture", "token", f, 7, fileUploadOptions{filename: "Demo.webm", replace: true})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestWikiFileUploadValidation(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "demo.webm")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"credentials required", nil, "require --user"},
		{"anonymous upload refused", []string{"--anon"}, "omit --anon"},
		{"namespace prefix refused", []string{"--filename", "File:Demo.webm"}, "namespace prefix"},
		{"password options exclusive", []string{"--user", "User@bot", "--password", "fixture", "--password-file", "unused"}, "none of the others can be"},
		{"description options exclusive", []string{"--text", "description", "--text-file", "unused"}, "none of the others can be"},
		{"missing description", []string{"--text-file", filepath.Join(dir, "missing")}, "read description"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewWikiCmd()
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			cmd.SetArgs(append([]string{"file", "upload", file, "--wiki", "https://invalid.invalid/w/api.php"}, tc.args...))
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
