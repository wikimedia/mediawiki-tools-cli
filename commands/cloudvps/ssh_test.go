package cloudvps

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	osexec "os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestInstanceHostname(t *testing.T) {
	for _, tc := range []struct {
		name, project, region, want string
	}{
		{"vm", "mwcli", "eqiad1", "vm.mwcli.eqiad1.wikimedia.cloud"},
		{"vm.other.codfw1dev.wikimedia.cloud", "", "", "vm.other.codfw1dev.wikimedia.cloud"},
		{"--proxy", "mwcli", "eqiad1", ""},
		{"vm;echo", "mwcli", "eqiad1", ""},
		{"vm", "../other", "eqiad1", ""},
		{"vm", "", "eqiad1", ""},
		{"vm.attacker.example", "mwcli", "eqiad1", ""},
		{"a..wikimedia.cloud", "", "", ""},
	} {
		t.Run(tc.name+tc.project, func(t *testing.T) {
			got, err := instanceHostname(tc.name, tc.project, tc.region)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("accepted invalid target %q", got)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("hostname = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestProjectInstances(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       []string
	}{
		{"valid", "z.mwcli.eqiad1.wikimedia.cloud\na.mwcli.eqiad1.wikimedia.cloud", 200, []string{"a.mwcli.eqiad1.wikimedia.cloud", "z.mwcli.eqiad1.wikimedia.cloud"}},
		{"filtered", "vm.other.eqiad1.wikimedia.cloud\n--bad.mwcli.eqiad1.wikimedia.cloud\nvm.mwcli.eqiad1.wikimedia.cloud", 200, []string{"vm.mwcli.eqiad1.wikimedia.cloud"}},
		{"empty", "", 200, nil},
		{"error", "unavailable", 503, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/dsh/project/mwcli" || r.Header.Get("Authorization") != "" {
					t.Errorf("unexpected discovery request: %s", r.URL)
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			got, err := projectInstances(context.Background(), server.Client(), server.URL, "mwcli", "eqiad1")
			if tc.want == nil {
				if err == nil {
					t.Fatal("expected actionable discovery error")
				}
			} else if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("instances = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestSSHRespectsNativeConfiguration(t *testing.T) {
	ssh, err := osexec.LookPath("ssh")
	if err != nil {
		t.Skip("OpenSSH not installed")
	}
	for _, proxy := range []string{"", "ProxyJump custom@jump.example", "ProxyCommand nc %h %p"} {
		t.Run(proxy, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config")
			contents := "Host *\n  User developer\n  ConnectTimeout 7\n  " + proxy + "\n"
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			out, err := osexec.Command(ssh, "-F", path, "-G", "vm.mwcli.eqiad1.wikimedia.cloud").Output()
			if err != nil {
				t.Fatal(err)
			}
			got := sshSessionArgs("vm.mwcli.eqiad1.wikimedia.cloud", nil, sshOptions{}, parseSSHConfig(string(out)))
			joined := strings.Join(got, " ")
			if proxy == "" && !strings.Contains(joined, "-J developer@bastion.wmcloud.org") {
				t.Fatalf("missing automatic bastion: %v", got)
			}
			if proxy != "" && strings.Contains(joined, "-J ") {
				t.Fatalf("overrode native proxy: %v", got)
			}
			if strings.Contains(joined, "ConnectTimeout=15") {
				t.Fatalf("overrode configured timeout: %v", got)
			}
		})
	}
}

func TestRemoteArgumentsRemainLiteral(t *testing.T) {
	if _, err := osexec.LookPath("sh"); err != nil {
		t.Skip("POSIX shell unavailable")
	}
	words := []string{"two words", "it's quoted", "", "$(exit 9)", "; exit 7", "\"hello\"", "line\nbreak"}
	// Exercise the remote shell's parsing, rather than compare quoting syntax.
	result, err := osexec.Command("sh", "-c", "printf '%s\\000' "+shellWords(words)).Output()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(string(result), "\x00"), "\x00")
	if !reflect.DeepEqual(got, words) {
		t.Fatalf("remote words = %#v; want %#v", got, words)
	}
}

func TestSSHBatchAndRemoteCommand(t *testing.T) {
	old := cli.Opts
	t.Cleanup(func() { cli.Opts = old })
	cli.Opts.NoInteraction = true
	cli.Opts.Verbosity = 5
	args := sshSessionArgs("vm.mwcli.eqiad1.wikimedia.cloud", []string{"printf", "%s", "two words"}, sshOptions{user: "developer", jumpHost: "none", noTTY: true}, map[string]string{"connecttimeout": "none"})
	joined := strings.Join(args, " ")
	for _, want := range []string{"-l developer", "-J none", "BatchMode=yes", "ConnectTimeout=15", "-vvv", "-T", "'printf' '%s' 'two words'"} {
		if !strings.Contains(joined, want) {
			t.Errorf("%v missing %q", args, want)
		}
	}
}

func TestExplicitProxyOptionDisablesAutomaticBastion(t *testing.T) {
	for _, option := range []string{"ProxyJump=none", "ProxyCommand=none", "proxyjump custom@jump.example"} {
		args := sshSessionArgs("vm.project.eqiad1.wikimedia.cloud", nil, sshOptions{options: []string{option}}, nil)
		if strings.Contains(strings.Join(args, " "), "-J ") {
			t.Fatalf("overrode explicit proxy option %q: %v", option, args)
		}
	}
}
