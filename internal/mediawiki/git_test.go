package mediawiki

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestGitCloneUserAgent(t *testing.T) {
	previousVersion := cli.VersionDetails.Version
	t.Cleanup(func() { cli.VersionDetails.Version = previousVersion })
	for _, tt := range []struct {
		name, version, override, want string
	}{
		{"release", "1.2.3", "", "mwcli/1.2.3 (https://www.mediawiki.org/wiki/Cli)"},
		{"development", "", "", "mwcli/dev (https://www.mediawiki.org/wiki/Cli)"},
		{"user override", "1.2.3", "custom-agent/1.0", "custom-agent/1.0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cli.VersionDetails.Version = cli.Version(tt.version)
			t.Setenv("GIT_HTTP_USER_AGENT", tt.override)
			if tt.override == "" {
				if err := os.Unsetenv("GIT_HTTP_USER_AGENT"); err != nil {
					t.Fatal(err)
				}
			}
			agents := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case agents <- r.UserAgent():
				default:
				}
				w.WriteHeader(http.StatusForbidden)
			}))
			defer server.Close()
			cmd := gitCloneCommand(filepath.Join(t.TempDir(), "clone"), server.URL+"/repo", true)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("clone unexpectedly succeeded against a forbidden endpoint")
			}
			select {
			case got := <-agents:
				if got != tt.want {
					t.Errorf("User-Agent = %q, want %q", got, tt.want)
				}
			default:
				t.Fatalf("Git did not reach the HTTP endpoint: %s", output)
			}
			if got := os.Getenv("GIT_HTTP_USER_AGENT"); got != tt.override {
				t.Errorf("parent user agent changed to %q", got)
			}
		})
	}
}

func TestCloneOpts_AreThereThingsToClone(t *testing.T) {
	tests := []struct {
		name string
		cp   CloneOpts
		want bool
	}{
		{
			name: "GetMediaWiki is true",
			cp: CloneOpts{
				GetMediaWiki: true,
			},
			want: true,
		},
		{
			name: "GetVector is true",
			cp: CloneOpts{
				GetVector: true,
			},
			want: true,
		},
		{
			name: "GetGerritSkins is not empty",
			cp: CloneOpts{
				GetGerritSkins: []string{"skin1", "skin2"},
			},
			want: true,
		},
		{
			name: "GetGerritExtensions is not empty",
			cp: CloneOpts{
				GetGerritExtensions: []string{"extension1", "extension2"},
			},
			want: true,
		},
		{
			name: "All options are false or empty",
			cp:   CloneOpts{},
			want: false,
		},
		{
			name: "extensions set but empty",
			cp: CloneOpts{
				GetGerritExtensions: []string{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cp.AreThereThingsToClone()
			if got != tt.want {
				t.Errorf("CloneOpts.AreThereThingsToClone() = %v, want %v", got, tt.want)
			}
		})
	}
}
