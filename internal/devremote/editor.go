package devremote

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/browser"
)

func checkEditor(ctx context.Context, stderr io.Writer) error {
	if secrets.IsWSL() || runtime.GOOS == "windows" {
		_, err := windowsCode(ctx, nil, true, nil, stderr)
		return err
	}
	_, err := osexec.LookPath("code")
	return err
}

func openEditor(ctx context.Context, t transport, c Context, stdout io.Writer) error {
	contextName := c.Context
	hash := sha256.Sum256([]byte(strings.Join(t.args, "\x00") + "\x00" + contextName))
	alias := fmt.Sprintf("mw-remote-%x", hash[:8])
	directory := filepath.Join(cli.UserDirectoryPathForCmd("remote"), alias)
	wsl := secrets.IsWSL()
	if wsl {
		// Keep Electron's user data on the Windows filesystem. Its SSH process
		// delegates to this WSL distribution, retaining Linux keys and agents.
		data, err := windowsCode(ctx, nil, true, nil, t.stderr)
		if err != nil {
			return err
		}
		converted, err := osexec.CommandContext(ctx, "wslpath", "-u", strings.TrimSpace(data)).Output()
		if err != nil {
			return fmt.Errorf("locate Windows editor data directory: %w", err)
		}
		directory = filepath.Join(strings.TrimSpace(string(converted)), "a-wikimedia-cli", "remote", alias)
	}
	if err := os.MkdirAll(filepath.Join(directory, "vscode", "User"), 0o700); err != nil {
		return err
	}
	probe := append([]string{"-T", "-G"}, t.args...)
	resolved := osexec.CommandContext(ctx, "ssh", probe...) // #nosec G204 -- SSH options are constructed by the CLI.
	resolved.Stderr = t.stderr
	data, err := resolved.Output()
	if err != nil {
		return fmt.Errorf("resolve editor SSH connection: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	config := editorSSHConfig(alias, string(data), filepath.ToSlash(filepath.Join(home, ".ssh", "config")))
	configFile := filepath.Join(directory, "ssh-config")
	if err := os.WriteFile(configFile, []byte(config), 0o600); err != nil {
		return err
	}
	settings := map[string]any{
		"remote.SSH.configFile":     configFile,
		"remote.SSH.remotePlatform": map[string]string{alias: "linux"},
		"terminal.integrated.env.linux": map[string]string{
			"PATH": c.Metadata + "/bin:${env:PATH}", "CONTEXT": contextName, "XDG_CONFIG_HOME": c.Metadata + "/config",
			"MW_REMOTE_BRIDGE": c.Metadata + "/session.json", "MW_REMOTE_WORKSPACE": c.Workspace, "MW_REMOTE_METADATA": c.Metadata, "MW_REMOTE_SOURCE": c.Source,
		},
	}
	if wsl {
		shim, err := writeWSLSSHShim(ctx, directory)
		if err != nil {
			return err
		}
		windowsConfig, err := windowsPath(ctx, configFile)
		if err != nil {
			return err
		}
		settings["remote.SSH.path"] = shim
		settings["remote.SSH.configFile"] = windowsConfig
	}
	if err := writeJSON(filepath.Join(directory, "vscode", "User", "settings.json"), settings); err != nil {
		return err
	}
	userData := filepath.Join(directory, "vscode")
	if wsl {
		userData, err = windowsPath(ctx, userData)
		if err != nil {
			return err
		}
	}
	run := func(args ...string) error {
		args = append([]string{"--user-data-dir", userData}, args...)
		if wsl || runtime.GOOS == "windows" {
			_, err := windowsCode(ctx, args, false, stdout, t.stderr)
			return err
		}
		cmd := osexec.CommandContext(ctx, "code", args...)
		cmd.Stdout, cmd.Stderr = stdout, t.stderr
		return cmd.Run()
	}
	if err := run("--install-extension", "ms-vscode-remote.remote-ssh"); err != nil {
		return err
	}
	uri := (&url.URL{Scheme: "vscode-remote", Host: "ssh-remote+" + alias, Path: c.Source}).String()
	return run("--new-window", "--folder-uri", uri)
}

func openSessionUI(ctx context.Context, c Context, stdout, stderr io.Writer) error {
	if c.Editor == "code" {
		if c.Directory != "" {
			command := osexec.CommandContext(ctx, "code", "--new-window", c.Directory)
			command.Stdout, command.Stderr = stdout, stderr
			if err := command.Run(); err != nil {
				return err
			}
		} else {
			if err := checkEditor(ctx, stderr); err != nil {
				return err
			}
			if err := openEditor(ctx, transport{args: c.SSHArgs, stderr: stderr}, c, stdout); err != nil {
				return err
			}
		}
	}
	var urls []string
	if c.Recipe != "none" {
		urls = append(urls, fmt.Sprintf("http://default.mediawiki.local.wmftest.net:%d/wiki/Main_Page", c.Port))
	}
	if c.Editor == "web" {
		urls = append(urls, fmt.Sprintf("http://127.0.0.1:%d/", c.WebPort))
	}
	for _, target := range urls {
		fmt.Fprintln(stdout, target)
		if !c.NoBrowser {
			if err := browser.OpenURL(target); err != nil {
				fmt.Fprintln(stderr, "Open the displayed URL manually:", err)
			}
		}
	}
	fmt.Fprintf(stdout, "Session is running in the background. Disconnect with: mw dev remote disconnect --context %s\n", c.Context)
	return nil
}

func editorSSHConfig(alias, resolved, originalConfig string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Host %s\n  ForwardAgent no\n", alias)
	// Copy resolved connection/authentication options, but let Remote SSH own
	// sessions and forwarding. Include the user's config for jump-host aliases.
	for _, line := range strings.Split(resolved, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "host", "forwardagent", "localforward", "remoteforward", "dynamicforward", "controlpath", "controlmaster", "controlpersist", "remotecommand", "requesttty", "sessiontype", "clearallforwardings":
			continue
		}
		fmt.Fprintf(&b, "  %s\n", line)
	}
	fmt.Fprintf(&b, "Host *\n  Include \"%s\"\n", strings.ReplaceAll(originalConfig, "\"", "\\\""))
	return b.String()
}

func writeJSON(filename string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, append(data, '\n'), 0o600)
}

func windowsPath(ctx context.Context, filename string) (string, error) {
	data, err := osexec.CommandContext(ctx, "wslpath", "-w", filename).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

var safeDistro = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func writeWSLSSHShim(ctx context.Context, directory string) (string, error) {
	distro := os.Getenv("WSL_DISTRO_NAME")
	if !safeDistro.MatchString(distro) {
		return "", fmt.Errorf("WSL_DISTRO_NAME must identify this distribution to connect Windows VS Code")
	}
	wrapper := filepath.Join(directory, "wsl-ssh.sh")
	powershell, err := secrets.WindowsPowerShellPath()
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(wrapper, []byte(wslSSHScript(os.Getenv("SSH_AUTH_SOCK"), powershell)), 0o700); err != nil {
		return "", err
	}
	shim := filepath.Join(directory, "wsl-ssh.cmd")
	// The wrapper lives on the mounted Windows drive, so its WSL path is
	// directly usable by wsl.exe without an extra quoting/interpreting shell.
	if strings.ContainsAny(wrapper, "\"%\r\n") {
		return "", fmt.Errorf("unsupported characters in editor bridge path")
	}
	// WSL's --distribution parser can retain literal quotes from a .cmd file.
	// Only use validated distribution names that need no shell quoting.
	data := fmt.Sprintf("@echo off\r\nwsl.exe --distribution %s --exec /bin/bash \"%s\" %%*\r\n", distro, wrapper)
	if err := os.WriteFile(shim, []byte(data), 0o600); err != nil {
		return "", err
	}
	return windowsPath(ctx, shim)
}

func wslSSHScript(agent, powershell string) string {
	script := "#!/bin/bash\nset -euo pipefail\n"
	if agent != "" {
		script += "export SSH_AUTH_SOCK=" + quote(agent) + "\n"
	}
	script += "powershell=" + quote(powershell) + "\n"
	script += `args=()
socks_port=
while (($#)); do
    if [[ $1 == -F && $# -ge 2 ]]; then
        args+=(-F "$(wslpath -u "$2")")
        shift 2
    elif [[ $1 == -D && $# -ge 2 ]]; then
        socks_port=${2##*:}
        args+=("$1" "$2")
        shift 2
    else
        args+=("$1")
        shift
    fi
done
if [[ $socks_port =~ ^[0-9]+$ ]]; then
    # Windows can see SSH output before WSL's localhost relay has registered
    # the new SOCKS listener. Hold stdout until the Windows side can connect.
    ssh "${args[@]}" | {
        "$powershell" -NoProfile -NonInteractive -Command "\$p=$socks_port; for(\$i=0; \$i -lt 100; \$i++) { \$c=New-Object Net.Sockets.TcpClient; try { if(\$c.ConnectAsync('127.0.0.1',\$p).Wait(200)) { \$c.Close(); exit 0 } } catch {} finally { \$c.Close() }; Start-Sleep -Milliseconds 100 }; exit 1" </dev/null || {
            echo 'Windows could not reach the WSL SSH forwarding port. Check WSL localhost forwarding.' >&2
            exit 1
        }
        cat
    }
    exit $?
fi
exec ssh "${args[@]}"
`
	return script
}

//go:embed editor.ps1
var windowsCodeScript string

func windowsCode(ctx context.Context, args []string, locate bool, stdout, stderr io.Writer) (string, error) {
	powershell, err := secrets.WindowsPowerShellPath()
	if err != nil {
		return "", err
	}
	// Pass all arguments as JSON data over stdin, never as PowerShell source.
	request, err := json.Marshal(map[string]any{"arguments": args, "locate": locate, "launch": slices.Contains(args, "--folder-uri"), "commandLine": windowsCommandLine(args)})
	if err != nil {
		return "", err
	}
	cmd := osexec.CommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-Command", windowsCodeScript)
	// Avoid cmd.exe's UNC-current-directory warning when invoked from WSL.
	cmd.Dir = filepath.Dir(powershell)
	cmd.Stdin, cmd.Stderr = strings.NewReader(string(request)), stderr
	if stdout != nil {
		cmd.Stdout = stdout
		return "", cmd.Run()
	}
	data, err := cmd.Output()
	return string(data), err
}

// windowsCommandLine quotes argv for CreateProcess, without invoking cmd.exe.
func windowsCommandLine(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		var b strings.Builder
		b.WriteByte('"')
		slashes := 0
		for _, r := range arg {
			if r == '\\' {
				slashes++
				continue
			}
			if r == '"' {
				b.WriteString(strings.Repeat("\\", slashes*2+1))
			} else {
				b.WriteString(strings.Repeat("\\", slashes))
			}
			b.WriteRune(r)
			slashes = 0
		}
		b.WriteString(strings.Repeat("\\", slashes*2))
		b.WriteByte('"')
		quoted = append(quoted, b.String())
	}
	return strings.Join(quoted, " ")
}
