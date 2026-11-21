package gerrit

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestGerritDemoInfersPatchsetAndComparesReadyWikis(t *testing.T) {
	setupAuthTest(t)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	directory := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := gitOutput(t.Context(), directory, args...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return out
	}
	git("init", "--quiet")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "--allow-empty", "-m", "fixture")
	head := git("rev-parse", "HEAD")
	t.Chdir(directory)
	http.DefaultTransport = transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.Header.Get("Authorization") != "" || req.UserAgent() != cli.HTTPUserAgent() {
			t.Errorf("unexpected method or headers on %s", req.URL)
		}
		switch req.URL.Host + strings.TrimSuffix(req.URL.Path, "/") {
		case "gerrit.wikimedia.org/r/changes":
			if req.URL.Query().Get("q") != "commit:"+head {
				t.Errorf("unexpected change search: %s", req.URL)
			}
			return reviewResponse(req, `[{"_number":1370181}]`)
		case "gerrit.wikimedia.org/r/changes/1370181/detail":
			return reviewResponse(req, fmt.Sprintf(`{"project":"mediawiki/extensions/WSOAuth","current_revision_number":5,"revisions":{%q:{"_number":4}}}`, head))
		case "patchdemo.wmcloud.org/api.php":
			if req.URL.Query().Get("action") != "findwikis" || req.URL.Query().Get("change") != "1370181" {
				t.Errorf("unexpected Patch Demo lookup: %s", req.URL)
			}
			return demoResponse(req, `[
				{"url":"https://older.catalyst.wmcloud.org/wiki/Main_Page","patches":["1370181,3"]},
				{"url":"https://patchdemo.wmcloud.org/wikis/fixture/w/","patches":["1370181,4","123,1"]},
				{"url":"https://newer.catalyst.wmcloud.org/w/","patches":["1370181,5"]},
				{"url":"https://unrelated.catalyst.wmcloud.org/w/","patches":["123,4"]},
				{"url":"https://external.example/w/","patches":["1370181,4"]}
			]`)
		default:
			t.Fatalf("unexpected request %s", req.URL)
			return nil, nil
		}
	})
	out, err := runGerritReadTest(t, "demo", "--public-only")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "Change 1370181, patchset 4") {
		t.Fatalf("did not infer the checked-out patchset:\n%s", out)
	}
	for _, want := range []string{
		"3 OLDER https://older.catalyst.wmcloud.org/wiki/Main_Page",
		"4 MATCH https://patchdemo.wmcloud.org/wikis/fixture/w/",
		"5 NEWER https://newer.catalyst.wmcloud.org/w/",
	} {
		if !strings.Contains(strings.Join(strings.Fields(out), " "), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "unrelated") || strings.Contains(out, "external.example") {
		t.Fatalf("listed a wiki unrelated to the selected Gerrit change or Patch Demo:\n%s", out)
	}
}

func TestGerritDemoNewPrintsPinnedPatchsetAndHonorsBrowserSettings(t *testing.T) {
	setupAuthTest(t)
	http.DefaultTransport = transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "gerrit.wikimedia.org" || req.URL.Path != "/r/changes/1370181/detail" {
			t.Fatalf("creation handoff made unexpected request %s", req.URL)
		}
		return reviewResponse(req, `{"project":"mediawiki/extensions/WSOAuth","current_revision_number":5,"revisions":{"older":{"_number":4}}}`)
	})
	for _, tt := range []struct {
		name                      string
		noBrowser, nonInteractive bool
		openErr                   error
		wantOpen                  bool
	}{
		{name: "browser", wantOpen: true},
		{name: "no browser", noBrowser: true},
		{name: "noninteractive", nonInteractive: true},
		{name: "browser failure", openErr: errors.New("browser unavailable"), wantOpen: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			initial := cli.Opts.NoInteraction
			cli.Opts.NoInteraction = tt.nonInteractive
			t.Cleanup(func() { cli.Opts.NoInteraction = initial })
			opened := false
			cmd := newDemoCmd(func(address string) error {
				opened = true
				if address != patchDemoURL {
					t.Errorf("opened unexpected URL %s", address)
				}
				return tt.openErr
			})
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			args := []string{"1370181", "--patchset", "4", "--new"}
			if tt.noBrowser {
				args = append(args, "--no-browser")
			}
			cmd.SetArgs(args)
			err := cmd.ExecuteContext(t.Context())
			if (err != nil) != (tt.openErr != nil) || opened != tt.wantOpen {
				t.Fatalf("opened=%t, err=%v", opened, err)
			}
			for _, link := range []string{patchDemoURL, "https://gerrit.wikimedia.org/r/c/mediawiki/extensions/WSOAuth/+/1370181/4"} {
				if !strings.Contains(stdout.String(), link) {
					t.Errorf("missing %s in %s", link, stdout.String())
				}
			}
		})
	}
}

func demoResponse(request *http.Request, body string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
}
