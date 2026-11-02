package devremote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

type invocation struct {
	Args  []string `json:"args"`
	Input []byte   `json:"input,omitempty"`
	CWD   string   `json:"cwd,omitempty"`
}

type bridgeEndpoint struct {
	Address string `json:"address"`
	Key     string `json:"key"`
}

type frame struct {
	Stream string `json:"stream,omitempty"`
	Data   []byte `json:"data,omitempty"`
	Exit   *int   `json:"exit,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Early handles remote transports before local configuration, Docker checks or telemetry.
func Early(args []string, stdin *os.File, stdout, stderr io.Writer) (bool, int) {
	ctx := context.Background()
	if len(args) == 1 && args[0] == "__remote-relay" {
		return true, report(serveRelay(ctx, stdin, stdout), stderr)
	}
	if len(args) == 2 && args[0] == "__remote-session" {
		return true, report(RunSession(ctx, args[1]), stderr)
	}
	if os.Getenv("MW_REMOTE_LOCAL") != "" {
		return false, 0
	}
	if len(args) > 0 && args[0] == "__remote-git" {
		return true, delegate(ctx, "/git", args[1:], stdin, stdout, stderr)
	}
	root, name, control := routeArguments(args)
	if root == "" || control {
		return false, 0
	}
	if bridge := os.Getenv("MW_REMOTE_BRIDGE"); bridge != "" && (root == "phab" || root == "phabricator") {
		return true, delegate(ctx, "/invoke", args, stdin, stdout, stderr)
	}
	if root != "dev" && root != "docker" {
		return false, 0
	}
	if !safeName.MatchString(name) {
		return false, 0
	}
	c, err := LoadContext(name)
	if os.IsNotExist(err) {
		return false, 0
	}
	if err != nil {
		return true, report(err, stderr)
	}
	if c.Directory != "" {
		if err := controlRequest(ctx, name, "/sync", nil); err != nil {
			return true, report(fmt.Errorf("synchronize the context before running the command; reopen the remote session: %w", err), stderr)
		}
	}
	if c.Workspace == "" {
		return true, report(errors.New("remote context is not ready; run dev remote open first"), stderr)
	}
	t := transport{args: c.SSHArgs, stderr: stderr}
	var extra []string
	outputFile, outputIsFile := stdout.(*os.File)
	if outputIsFile && term.IsTerminal(int(stdin.Fd())) && term.IsTerminal(int(outputFile.Fd())) {
		extra = []string{"-tt"}
	}
	command := quote(c.Metadata + "/bin/mw")
	for _, arg := range args {
		command += " " + quote(arg)
	}
	child := t.command(ctx, extra, command)
	child.Stdin, child.Stdout = stdin, stdout
	return true, report(child.Run(), stderr)
}

func routeArguments(args []string) (root, name string, control bool) {
	name = os.Getenv("CONTEXT")
	if name == "" {
		name = "default"
	}
	var words []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if a == "--help" || a == "-h" {
			control = true
		}
		if a == "--context" || a == "-c" {
			if i+1 < len(args) {
				i++
				name = args[i]
			}
			continue
		}
		if strings.HasPrefix(a, "--context=") || strings.HasPrefix(a, "-c=") {
			_, name, _ = strings.Cut(a, "=")
			continue
		}
		if strings.HasPrefix(a, "-c") && len(a) > 2 {
			name = a[2:]
			continue
		}
		if !strings.HasPrefix(a, "-") {
			words = append(words, a)
		}
	}
	if len(words) > 0 {
		root = words[0]
	}
	if len(words) > 1 && (words[1] == "remote" || root == "help" || root == "completion" || strings.HasPrefix(root, "__complete")) {
		control = true
	}
	return
}

func report(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	var exit *osexec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	fmt.Fprintln(stderr, "ERROR", err)
	return 1
}

func socketClient(socket string) *http.Client {
	return &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialControl(ctx, socket)
	}}}
}

func delegate(ctx context.Context, endpoint string, args []string, stdin *os.File, stdout, stderr io.Writer) int {
	configuration := os.Getenv("MW_REMOTE_BRIDGE")
	if configuration == "" {
		return report(errors.New("no active local authentication bridge; reopen the remote session"), stderr)
	}
	data, err := os.ReadFile(filepath.Clean(configuration))
	if err != nil {
		return report(errors.New("local session is disconnected; reopen it on your local machine"), stderr)
	}
	var endpointConfig bridgeEndpoint
	if err := json.Unmarshal(data, &endpointConfig); err != nil {
		return report(errors.New("invalid session capability file"), stderr)
	}
	args, input, err := remoteInput(args, stdin)
	if err != nil {
		return report(err, stderr)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return report(err, stderr)
	}
	return invokeBridge(ctx, endpointConfig, endpoint, invocation{Args: args, Input: input, CWD: cwd}, stdout, stderr)
}

func remoteInput(args []string, stdin *os.File) ([]string, []byte, error) {
	args = append([]string(nil), args...)
	var input []byte
	readStdin := false
	for i, arg := range args {
		key, filename, equals := strings.Cut(arg, "=")
		if key != "--file" && key != "--description-file" && key != "--input" {
			continue
		}
		index := i
		if !equals {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("%s needs a filename", key)
			}
			index, filename = i+1, args[i+1]
		}
		if filename == "-" {
			readStdin = true
			continue
		}
		if input != nil {
			return nil, nil, errors.New("delegate one input file at a time")
		}
		f, err := os.Open(filepath.Clean(filename))
		if err != nil {
			return nil, nil, err
		}
		input, err = io.ReadAll(io.LimitReader(f, 8<<20+1))
		f.Close()
		if err != nil || len(input) > 8<<20 {
			return nil, nil, errors.New("delegated input must be at most 8 MiB")
		}
		if equals {
			args[i] = key + "=-"
		} else {
			args[index] = "-"
		}
	}
	if input == nil && (readStdin || !term.IsTerminal(int(stdin.Fd()))) {
		var err error
		input, err = io.ReadAll(io.LimitReader(stdin, 8<<20+1))
		if err != nil || len(input) > 8<<20 {
			return nil, nil, errors.New("delegated stdin must be at most 8 MiB")
		}
	}
	return args, input, nil
}

func invokeBridge(ctx context.Context, connection bridgeEndpoint, endpoint string, request invocation, stdout, stderr io.Writer) int {
	data, err := json.Marshal(request)
	if err != nil {
		return report(err, stderr)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, connection.Address+endpoint, bytes.NewReader(data))
	if err != nil {
		return report(err, stderr)
	}
	req.Header.Set("Authorization", "Bearer "+connection.Key)
	resp, err := client.Do(req)
	if err != nil {
		return report(fmt.Errorf("local session bridge unavailable: %w", err), stderr)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return report(fmt.Errorf("bridge rejected operation: %s", strings.TrimSpace(string(message))), stderr)
	}
	decoder := json.NewDecoder(resp.Body)
	for {
		var f frame
		if err := decoder.Decode(&f); err != nil {
			return report(fmt.Errorf("bridge response interrupted: %w", err), stderr)
		}
		if f.Error != "" {
			fmt.Fprintln(stderr, f.Error)
		}
		if f.Exit != nil {
			return *f.Exit
		}
		if f.Stream == "stderr" {
			stderr.Write(f.Data)
		} else {
			stdout.Write(f.Data)
		}
	}
}
