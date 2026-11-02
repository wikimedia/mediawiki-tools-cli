package devremote

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRemoteInputValidation(t *testing.T) {
	for _, host := range []string{"", "-oProxyCommand=bad", "vm\nHost *", "vm;touch /tmp/no", "vm name"} {
		if err := ValidateHost(host); err == nil {
			t.Errorf("accepted unsafe host %q", host)
		}
	}
	for _, host := range []string{"awm-1.boxd", "developer@vm.example", "127.0.0.1", "::1"} {
		if err := ValidateHost(host); err != nil {
			t.Errorf("rejected host %q: %v", host, err)
		}
	}
	valid := Options{SSHArgs: []string{"vm"}, Context: "remote-test", Recipe: "mediawiki", Editor: "none"}
	if err := valid.validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Options){
		func(o *Options) { o.Context = "../other" },
		func(o *Options) { o.Recipe = "missing" },
		func(o *Options) { o.Editor = "sh" },
		func(o *Options) { o.Port = 65536 },
	} {
		opts := valid
		change(&opts)
		if opts.validate() == nil {
			t.Fatalf("accepted invalid options: %+v", opts)
		}
	}
}

func TestProbeParsing(t *testing.T) {
	home, machine, err := parseProbe("login banner\nMWREMOTE\tLinux\tx86_64\t/home/a user's\n")
	if err != nil || home != "/home/a user's" || machine != "x86_64" {
		t.Fatalf("parse = %q, %q, %v", home, machine, err)
	}
	for _, probe := range []string{"Linux", "MWREMOTE\tDarwin\tarm64\t/home/x", "MWREMOTE\tLinux\tx86_64\t~"} {
		if _, _, err := parseProbe(probe); err == nil {
			t.Fatalf("accepted invalid probe %q", probe)
		}
	}
}

func TestSavedPortIsPreservedAndConflictsFail(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	stored := strconv.Itoa(port)
	if _, err := selectPort(stored, 0, nil); err == nil {
		t.Fatal("silently changed an occupied saved port")
	}
	listener.Close()
	got, err := selectPort(stored, 0, nil)
	if err != nil || got != port {
		t.Fatalf("saved port = %d, %v; want %d", got, err, port)
	}
	if _, err := selectPort(stored, port-1, nil); err == nil {
		t.Fatal("silently changed an existing environment's advertised port")
	}
	if _, err := selectPort("garbage", 0, nil); err == nil {
		t.Fatal("accepted corrupt state")
	}
	if _, err := selectPort("", port, map[int]bool{port: true}); err == nil {
		t.Fatal("selected a port already occupied on the VM")
	}
	if got, err := selectPort(stored, 0, map[int]bool{port: true}); err != nil || got != port {
		t.Fatalf("rejected existing environment's own listener: %d, %v", got, err)
	}
}

func TestRemoteListeningPorts(t *testing.T) {
	ports := listeningPorts("LISTEN 0 4096 127.0.0.1:6195 0.0.0.0:*\nLISTEN 0 4096 [::]:8080 [::]:*\nLISTEN 0 128 *:22 *:*\n")
	if len(ports) != 3 || !ports[6195] || !ports[8080] || !ports[22] {
		t.Fatalf("unexpected remote listeners: %v", ports)
	}
}

func TestTunnelFailureIsReportedAndRetainedForCleanup(t *testing.T) {
	done := make(chan error, 1)
	done <- errors.New("bind: address already in use")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := waitTunnel(ctx, 1, done); err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("unexpected tunnel error: %v", err)
	}
	select {
	case <-done:
	default:
		t.Fatal("cleanup would block after a tunnel failure")
	}
}

func TestReadinessPreservesVirtualHostAndDoesNotFollowRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Host, "default.mediawiki.local.wmftest.net:") || r.URL.Path != "/wiki/Main_Page" {
			t.Errorf("unexpected forwarded request: %s %s", r.Host, r.URL)
		}
		w.Header().Set("Location", "http://unreachable.invalid/wiki/Main_Page")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	if err := waitSite(context.Background(), server.Listener.Addr().(*net.TCPAddr).Port); err != nil {
		t.Fatal(err)
	}
}

func TestSetupCanRetryThenResumeWithoutReinstalling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("remote setup executes on Linux")
	}
	bash, err := osexec.LookPath("bash")
	if err != nil {
		t.Skip("Bash unavailable")
	}
	if _, err := osexec.LookPath("flock"); err != nil {
		t.Skip("flock unavailable")
	}
	directory := t.TempDir()
	workspace := filepath.Join(directory, "user's workspace")
	binary := filepath.Join(directory, "fake-mw")
	log := filepath.Join(directory, "calls")
	// A real shell executes the setup script against a fake CLI. Docker and
	// application setup never run, and argument quoting is exercised end to end.
	stub := `#!/bin/bash
set -eu
printf '%s|%s|%s\n' "$XDG_CONFIG_HOME" "$CONTEXT" "$*" >> "$REMOTE_TEST_LOG"
mkdir -p "$XDG_CONFIG_HOME/container-bind"
printf 'asset' > "$XDG_CONFIG_HOME/container-bind/asset"
if [[ "$*" == *'dev recipe '* ]]; then
    if [[ ! -f "$REMOTE_TEST_LOG.failed" ]]; then
        touch "$REMOTE_TEST_LOG.failed"
        exit 42
    fi
fi
`
	if err := os.WriteFile(binary, []byte(stub), 0o700); err != nil {
		t.Fatal(err)
	}
	run := func(recipe, port string) error {
		cmd := osexec.Command(bash, "-s", "--", workspace, binary, "test-context", recipe, port)
		cmd.Stdin = strings.NewReader(setupScript)
		cmd.Env = append(os.Environ(), "REMOTE_TEST_LOG="+log)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, out)
		}
		return nil
	}
	if err := run("mediawiki", "6195"); err == nil {
		t.Fatal("failed setup was marked successful")
	}
	if _, err := os.Stat(filepath.Join(workspace, "ready")); !os.IsNotExist(err) {
		t.Fatal("failure created a ready marker")
	}
	oldBinary := filepath.Join(workspace, "bin", "mw-"+strings.Repeat("a", 64))
	otherFile := filepath.Join(workspace, "bin", "mw-user-file")
	for _, filename := range []string{oldBinary, otherFile} {
		if err := os.WriteFile(filename, []byte("old content"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := run("mediawiki", "6195"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldBinary); !os.IsNotExist(err) {
		t.Fatal("successful setup retained an obsolete binary")
	}
	if _, err := os.Stat(otherFile); err != nil {
		t.Fatalf("cleanup removed an unrelated file: %v", err)
	}
	if err := run("mediawiki", "6195"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "dev recipe mediawiki") != 2 || strings.Count(string(data), "dev start --remember-running") != 2 {
		t.Fatalf("unexpected retry/resume behavior:\n%s", data)
	}
	if !strings.Contains(string(data), workspace+"/config|test-context|") {
		t.Fatalf("remote CLI did not use isolated configuration:\n%s", data)
	}
	for filename, mode := range map[string]os.FileMode{
		filepath.Join(workspace, "config"):                            0o700,
		filepath.Join(workspace, "config", "container-bind"):          0o755,
		filepath.Join(workspace, "config", "container-bind", "asset"): 0o644,
	} {
		info, err := os.Stat(filename)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("unexpected permissions for %s: %v, %v; want %o", filename, info, err, mode)
		}
	}
	if err := run("wikibase-repoclient", "6195"); err == nil {
		t.Fatal("silently switched an existing environment's recipe")
	}
	if err := run("mediawiki", "6196"); err == nil {
		t.Fatal("silently switched an existing environment's port")
	}
}

func TestEditorSSHConfigKeepsBastionAndKeysWithoutSessionSideEffects(t *testing.T) {
	ssh, err := osexec.LookPath("ssh")
	if err != nil {
		t.Skip("OpenSSH unavailable")
	}
	directory := t.TempDir()
	original := filepath.Join(directory, "original")
	if err := os.WriteFile(original, []byte("Host jump\n HostName bastion.example\n User jumper\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved := "host original\nhostname vm.example\nuser developer\nidentityfile ~/.ssh/id_ed25519\nproxyjump jump\nrequesttty force\nremotecommand unwanted\nlocalforward 8080 localhost:80\nforwardagent yes\n"
	config := filepath.Join(directory, "generated")
	if err := os.WriteFile(config, []byte(editorSSHConfig("managed", resolved, original)), 0o600); err != nil {
		t.Fatal(err)
	}
	for host, expected := range map[string]string{"managed": "hostname vm.example", "jump": "hostname bastion.example"} {
		out, err := osexec.Command(ssh, "-G", "-F", config, host).CombinedOutput()
		if err != nil || !strings.Contains(string(out), expected) {
			t.Fatalf("generated config for %s: %v\n%s", host, err, out)
		}
		if host == "managed" && (!strings.Contains(string(out), "proxyjump jump") || strings.Contains(string(out), "forwardagent yes") || strings.Contains(string(out), "unwanted")) {
			t.Fatalf("lost bastion or retained session side effects:\n%s", out)
		}
	}
}

func TestWindowsEditorArgumentQuoting(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{""}, `""`},
		{[]string{"--file-uri", "vscode-remote://ssh-remote+host/a%20b"}, `"--file-uri" "vscode-remote://ssh-remote+host/a%20b"`},
		{[]string{`C:\some path\`, `a"b`}, `"C:\some path\\" "a\"b"`},
	} {
		if got := windowsCommandLine(tc.args); got != tc.want {
			t.Fatalf("quoted %q as %q; want %q", tc.args, got, tc.want)
		}
	}
}

func TestWSLSSHWaitsForWindowsForwardingAndPreservesExitStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("WSL adapter executes on Linux")
	}
	bash, err := osexec.LookPath("bash")
	if err != nil {
		t.Skip("Bash unavailable")
	}
	directory := t.TempDir()
	for name, script := range map[string]string{
		"ssh":        "#!/bin/sh\nprintf 'server ready\\n'\nexit 7\n",
		"wslpath":    "#!/bin/sh\nprintf '%s\\n' \"$2\"\n",
		"powershell": "#!/bin/sh\nif test \"${FAIL_PROBE:-}\" = yes; then exit 1; fi\n: > \"$PROBE_READY\"\n",
		"cat":        "#!/bin/sh\ntest -f \"$PROBE_READY\" || exit 88\nexec /bin/cat\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PROBE_READY", filepath.Join(directory, "ready"))
	run := func() (string, error) {
		cmd := osexec.Command(bash, "-s", "--", "-D", "12345", "-F", "/tmp/config", "remote", "sh")
		cmd.Stdin = strings.NewReader(wslSSHScript("", filepath.Join(directory, "powershell")))
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	out, err := run()
	var exit *osexec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 || out != "server ready\n" {
		t.Fatalf("SSH status or output lost: %q, %v", out, err)
	}
	t.Setenv("FAIL_PROBE", "yes")
	if out, err := run(); err == nil || !strings.Contains(out, "Windows could not reach") {
		t.Fatalf("missing Windows forwarding failure: %q, %v", out, err)
	}
}
