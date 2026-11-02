// Package devremote connects a local development client to an SSH-hosted environment.
package devremote

import (
	"context"
	"crypto/sha256"
	"debug/elf"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	osexec "os/exec"
	ossignal "os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/mount"
)

// Options describes one persistent remote development workspace.
type Options struct {
	SSHArgs                 []string
	Context, Recipe, Editor string
	Port                    int
	NoBrowser, InstallDeps  bool
}

//go:embed setup.sh
var setupScript string

//go:embed dependencies.sh
var dependenciesScript string

var safeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

var safeHost = regexp.MustCompile(`^[a-zA-Z0-9_@.:\[\]-]+$`)

// ValidateHost rejects SSH option injection and malformed editor authorities.
func ValidateHost(host string) error {
	if !safeHost.MatchString(host) || strings.HasPrefix(host, "-") {
		return fmt.Errorf("provide an SSH hostname or alias, with connection options in your SSH configuration")
	}
	return nil
}

func (o Options) validate() error {
	if !safeName.MatchString(o.Context) || !safeName.MatchString(o.Recipe) {
		return fmt.Errorf("context and recipe names must contain only letters, digits, underscores and hyphens (maximum 64 characters)")
	}
	if o.Port < 0 || o.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, or 0 for automatic selection")
	}
	if o.Editor != "code" && o.Editor != "none" && o.Editor != "web" {
		return fmt.Errorf("editor must be code, web or none")
	}
	if _, err := mount.DevContent.ReadFile("dev/recipes/" + o.Recipe + ".yml"); o.Recipe != "none" && err != nil {
		return fmt.Errorf("unknown built-in recipe %q", o.Recipe)
	}
	if len(o.SSHArgs) == 0 {
		return fmt.Errorf("missing SSH connection")
	}
	return ValidateHost(o.SSHArgs[len(o.SSHArgs)-1])
}

// Open prepares the remote workspace and keeps browser forwards alive until interrupted.
func Open(parent context.Context, o Options, stdout, stderr io.Writer) error {
	return runConnection(parent, Context{Options: o}, connectionHooks{}, stdout, stderr)
}

type preparedContext struct {
	workspace, metadata, source string
	port, webPort               int
}

type connectionHooks struct {
	bridgePort int
	bridgeKey  string
	sshControl string
	located    func(preparedContext) error
	prepare    func(preparedContext) error
	ready      func()
}

func runConnection(parent context.Context, c Context, hooks connectionHooks, stdout, stderr io.Writer) error {
	o := c.Options
	if err := o.validate(); err != nil {
		return err
	}
	ctx, cancel := ossignal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	t := transport{args: o.SSHArgs, stderr: stderr, control: hooks.sshControl}
	fmt.Fprintln(stdout, "Checking the remote machine...")
	probe, err := t.output(ctx, `printf 'MWREMOTE\t%s\t%s\t%s\n' "$(uname -s)" "$(uname -m)" "$HOME"`)
	if err != nil {
		return err
	}
	home, machine, err := parseProbe(probe)
	if err != nil {
		return err
	}
	binary, digest, err := remoteBinary(ctx, machine, stdout)
	if err != nil {
		return err
	}
	defer binary.Close()
	workspace := c.Workspace
	if workspace == "" {
		workspace = path.Join(home, "wm", o.Context)
	}
	metadata := c.Metadata
	if metadata == "" {
		metadata = path.Join(workspace, ".wm")
	}
	source := c.Source
	if source == "" {
		name := c.SourceName
		if name == "" {
			name = "mediawiki"
		}
		if !safeName.MatchString(name) {
			return errors.New("invalid source name")
		}
		source = path.Join(workspace, name)
	}
	if o.InstallDeps {
		fmt.Fprintln(stdout, "Installing remote dependencies using passwordless sudo...")
		if err := t.script(ctx, dependenciesScript, nil, stdout); err != nil {
			return err
		}
	}
	if _, err := t.output(ctx, `set -eu; command -v bash; command -v git; command -v flock; command -v ss; docker compose version; docker info >/dev/null`); err != nil {
		return fmt.Errorf("remote Git, Docker Compose and Docker access are required; on supported systems retry with --install-deps: %w", err)
	}
	stored, err := t.output(ctx, "if test -f "+quote(metadata+"/port")+"; then cat "+quote(metadata+"/port")+"; fi")
	if err != nil {
		return err
	}
	listening, err := t.output(ctx, "ss -H -ltn")
	if err != nil {
		return err
	}
	port, err := selectPort(strings.TrimSpace(stored), o.Port, listeningPorts(listening))
	if err != nil {
		return err
	}
	// Establish forwarding before changing the VM, so occupied local ports fail early.
	fmt.Fprintf(stdout, "Connecting local port %d to the remote environment...\n", port)
	if err := t.run(ctx, "umask 077; mkdir -p "+quote(metadata), nil, stdout); err != nil {
		return err
	}
	forward := []string{"-N", "-o", "ExitOnForwardFailure=yes", "-L", fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", port, port)}
	bridgeFile := ""
	webPort := c.WebPort
	webForwardPort, remoteEditorPort := 0, 0
	var editorListener net.Listener
	if o.Editor == "web" {
		webPort, err = selectPort("", webPort, map[int]bool{port: true})
		if err != nil {
			return err
		}
		editorListener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", webPort))
		if err != nil {
			return err
		}
		defer editorListener.Close()
		allocation, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
		webForwardPort = allocation.Addr().(*net.TCPAddr).Port
		allocation.Close()
		storedEditor, err := t.output(ctx, "if test -f "+quote(metadata+"/editor-port")+";then cat "+quote(metadata+"/editor-port")+";fi")
		if err != nil {
			return err
		}
		remoteEditorPort, _ = strconv.Atoi(strings.TrimSpace(storedEditor))
		if remoteEditorPort < 1024 || remoteEditorPort > 65535 {
			remoteEditorPort, err = freeRemotePort(listeningPorts(listening), 46300)
			if err != nil {
				return err
			}
		}
		forward = append(forward, "-L", fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", webForwardPort, remoteEditorPort))
	}
	if hooks.located != nil {
		if err := hooks.located(preparedContext{workspace, metadata, source, port, webPort}); err != nil {
			return err
		}
	}
	tunnel := t.command(ctx, forward, "")
	tunnel.Stdout = stderr
	if err := tunnel.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- tunnel.Wait() }()
	defer func() {
		cancel()
		<-done
		if bridgeFile != "" {
			cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			t.output(cleanup, "rm -f -- "+quote(bridgeFile))
		}
	}()
	if err := waitTunnel(ctx, port, done); err != nil {
		return err
	}
	remoteCache := path.Join(home, ".cache/a-wikimedia-cli/bin")
	remoteBinary := remoteCache + "/mw-" + digest
	// Publish the reference under the same lock used by cache cleanup, before testing the cache.
	reference := "set -eu; umask 077; mkdir -p " + quote(remoteCache) + "; flock " + quote(path.Join(home, ".cache/a-wikimedia-cli/remote-cache.lock")) + " bash -c " + quote("printf '%s\n' "+quote(remoteBinary)+" > "+quote(metadata+"/binary")+"; printf '%s\n' "+quote(o.Context)+" > "+quote(metadata+"/context"))
	if err := t.run(ctx, reference, nil, io.Discard); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Uploading the CLI; local profiles and credentials stay on this computer.")
	cached, err := t.output(ctx, "if test -x "+quote(remoteBinary)+"; then printf cached; fi")
	if err != nil {
		return err
	}
	if cached != "cached" {
		upload := "set -eu; umask 077; mkdir -p " + quote(remoteCache) + "; dst=" + quote(remoteBinary) + `; tmp=$(mktemp "$dst.XXXXXX"); trap 'rm -f "$tmp"' EXIT HUP INT TERM; cat > "$tmp"; chmod 700 "$tmp"; mv "$tmp" "$dst"`
		if err := t.run(ctx, upload, binary, stdout); err != nil {
			return fmt.Errorf("upload remote CLI: %w", err)
		}
	}
	if _, err := t.output(ctx, "XDG_CONFIG_HOME="+quote(metadata+"/config")+" "+quote(remoteBinary)+" --no-interaction version"); err != nil {
		return fmt.Errorf("uploaded CLI cannot run on this VM; install the matching release again: %w", err)
	}
	var relayDone chan error
	if hooks.bridgePort != 0 {
		remotePort, relay, err := startRelay(ctx, t, remoteBinary, hooks.bridgePort)
		if err != nil {
			return err
		}
		relayDone = make(chan error, 1)
		go func() { relayDone <- relay.Wait() }()
		defer func() { relay.Process.Kill(); <-relayDone }()
		bridgeFile = metadata + "/session.json"
		data, err := json.Marshal(bridgeEndpoint{Address: fmt.Sprintf("http://127.0.0.1:%d", remotePort), Key: hooks.bridgeKey})
		if err != nil {
			return err
		}
		if err := t.run(ctx, "umask 077; cat > "+quote(bridgeFile), strings.NewReader(string(data)), io.Discard); err != nil {
			return err
		}
	}
	if hooks.prepare != nil {
		if err := hooks.prepare(preparedContext{workspace, metadata, source, port, webPort}); err != nil {
			return err
		}
	}
	gitBridge := "no"
	if c.GitRemote != "" {
		gitBridge = "yes"
	}
	args := []string{metadata, remoteBinary, o.Context, o.Recipe, strconv.Itoa(port), source, bridgeFile, workspace, gitBridge, c.GitRemote}
	fmt.Fprintln(stdout, "Preparing remote source and services (the first run can take several minutes)...")
	if err := t.script(ctx, setupScript, args, stdout); err != nil {
		return fmt.Errorf("remote setup failed; rerun the command to retry: %w", err)
	}
	url := fmt.Sprintf("http://default.mediawiki.local.wmftest.net:%d/wiki/Main_Page", port)
	if o.Recipe != "none" {
		if err := waitSite(ctx, port); err != nil {
			return err
		}
	}
	if o.Editor == "web" {
		if err := t.script(ctx, webEditorScript, []string{workspace, metadata, source, "start", o.Context, strconv.Itoa(remoteEditorPort)}, stdout); err != nil {
			return err
		}
		password, err := t.output(ctx, "cat "+quote(metadata+"/editor-password"))
		if err != nil {
			return err
		}
		proxy, err := browserEditorProxy(ctx, webForwardPort, webPort, strings.TrimSpace(password))
		if err != nil {
			return err
		}
		server := &http.Server{Handler: proxy, ReadHeaderTimeout: 10 * time.Second}
		go server.Serve(editorListener)
		defer server.Close()
	}
	fmt.Fprintln(stdout, "Remote environment ready.")
	if o.Recipe != "none" {
		fmt.Fprintf(stdout, "  Site: %s\n", url)
	}
	fmt.Fprintf(stdout, "  Source: %s\n", source)
	if hooks.ready != nil {
		hooks.ready()
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-relayDone:
		relayDone <- err
		return fmt.Errorf("SSH authentication bridge disconnected: %v", err)
	case err := <-done:
		// Put the result back for the deferred reap.
		done <- err
		if err == nil {
			return errors.New("SSH forwarding disconnected; rerun the command to reconnect")
		}
		return fmt.Errorf("SSH forwarding disconnected; rerun the command to reconnect: %w", err)
	}
}

func freeRemotePort(busy map[int]bool, start int) (int, error) {
	for port := start; port < start+100; port++ {
		if !busy[port] {
			return port, nil
		}
	}
	return 0, errors.New("no free remote session ports")
}

type transport struct {
	args    []string
	stderr  io.Writer
	control string
}

func (t transport) command(ctx context.Context, extra []string, script string) *osexec.Cmd {
	args := []string{"-C", "-T", "-o", "ForwardAgent=no", "-o", "RemoteCommand=none", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=3", "-o", "ConnectTimeout=15"}
	if t.control != "" {
		args = append(args, "-o", "ControlMaster=auto", "-o", "ControlPersist=no", "-S", t.control)
	}
	args = append(args, extra...)
	args = append(args, t.args...)
	if script != "" {
		args = append(args, script)
	}
	cmd := osexec.CommandContext(ctx, "ssh", args...) // #nosec G204 -- validated host, explicit arguments, quoted remote data.
	cmd.Stderr = t.stderr
	return cmd
}

func (t transport) output(ctx context.Context, script string) (string, error) {
	data, err := t.command(ctx, nil, script).Output()
	if err != nil {
		return "", fmt.Errorf("SSH command failed: %w", err)
	}
	return string(data), nil
}

func (t transport) run(ctx context.Context, script string, stdin io.Reader, stdout io.Writer) error {
	cmd := t.command(ctx, nil, script)
	cmd.Stdin, cmd.Stdout = stdin, stdout
	return cmd.Run()
}

func (t transport) script(ctx context.Context, script string, args []string, stdout io.Writer) error {
	command := "bash -s --"
	for _, arg := range args {
		command += " " + quote(arg)
	}
	return t.run(ctx, command, strings.NewReader(script), stdout)
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func parseProbe(probe string) (home, machine string, err error) {
	for _, line := range strings.Split(probe, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 || fields[0] != "MWREMOTE" {
			continue
		}
		if fields[1] != "Linux" || !strings.HasPrefix(fields[3], "/") || strings.ContainsAny(fields[3], "\r\x00") {
			break
		}
		return fields[3], fields[2], nil
	}
	return "", "", errors.New("remote development requires a Linux VM with an absolute home directory")
}

func openBinary(filename, machine string) (*os.File, string, error) {
	f, err := elf.Open(filename)
	if err != nil {
		return nil, "", fmt.Errorf("expected a Linux CLI executable: %w", err)
	}
	want, ok := map[string]elf.Machine{"x86_64": elf.EM_X86_64, "aarch64": elf.EM_AARCH64}[machine]
	valid := ok && f.Machine == want && (f.OSABI == elf.ELFOSABI_NONE || f.OSABI == elf.ELFOSABI_LINUX)
	f.Close()
	if !valid {
		return nil, "", fmt.Errorf("CLI binary does not match remote Linux %s; use a CLI build matching the VM", machine)
	}
	binary, err := os.Open(filepath.Clean(filename))
	if err != nil {
		return nil, "", err
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, binary); err == nil {
		_, err = binary.Seek(0, io.SeekStart)
	}
	if err != nil {
		binary.Close()
		return nil, "", err
	}
	return binary, fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func selectPort(stored string, requested int, remoteBusy map[int]bool) (int, error) {
	if stored != "" {
		port, err := strconv.Atoi(stored)
		if err != nil || port < 1024 || port > 65535 {
			return 0, fmt.Errorf("invalid saved remote port %q", stored)
		}
		if requested != 0 && requested != port {
			return 0, fmt.Errorf("existing environment uses port %d; use that port or a different --context", port)
		}
		requested = port
	}
	start, end := requested, requested
	if requested == 0 {
		start, end = 6195, 6295
	}
	for port := start; port <= end; port++ {
		if port < 1024 {
			return 0, errors.New("use an unprivileged port of at least 1024")
		}
		if stored == "" && remoteBusy[port] {
			continue
		}
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("environment port unavailable locally or remotely; close the conflicting listener or use a different --context and --port")
}

func listeningPorts(output string) map[int]bool {
	ports := make(map[int]bool)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		i := strings.LastIndex(fields[3], ":")
		if i < 0 {
			continue
		}
		if port, err := strconv.Atoi(fields[3][i+1:]); err == nil {
			ports[port] = true
		}
	}
	return ports
}

func waitTunnel(ctx context.Context, port int, done chan error) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case err := <-done:
			done <- err
			return fmt.Errorf("SSH could not establish forwarding: %v", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("timed out establishing SSH forwarding")
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
			if err == nil {
				conn.Close()
				return nil
			}
		}
	}
}

func waitSite(ctx context.Context, port int) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/wiki/Main_Page", port), nil)
		if err != nil {
			return err
		}
		req.Host = fmt.Sprintf("default.mediawiki.local.wmftest.net:%d", port)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return errors.New("wiki did not become ready through the SSH tunnel; inspect the remote containers and retry")
		case <-time.After(time.Second):
		}
	}
}
