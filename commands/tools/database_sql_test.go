package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
)

func TestDatabaseSQLCommand(t *testing.T) {
	tests := []struct {
		name, tool, host, database string
		toolsDB                    bool
		args, clientArgs           []string
	}{
		{"replica default", "", "enwiki.analytics.db.svc.wikimedia.cloud", "enwiki_p", false, []string{"enwiki_p", "--", "-e", "SELECT 1"}, []string{"-e", "SELECT 1"}},
		{"replica web", "mytool", "dewiki.web.db.svc.wikimedia.cloud", "dewiki_p", false, []string{"dewiki", "--tool", "mytool", "--service", "web"}, nil},
		{"toolsdb primary", "mytool", "tools.db.svc.wikimedia.cloud", "s123__db", true, []string{"s123__db", "--tool", "mytool"}, nil},
		{"toolsdb readonly", "", "tools-readonly.db.svc.wikimedia.cloud", "s123__db", true, []string{"s123__db", "--read-only", "--", "--batch"}, []string{"--batch"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := &cobra.Command{Use: "toolforge"}
			root.PersistentFlags().String("tool", "", "Tool")
			root.PersistentFlags().Bool("beta", false, "Beta")
			root.PersistentFlags().Bool("lima", false, "Lima")
			var stdout, stderr bytes.Buffer
			input := strings.NewReader("SELECT 1;\n")
			called := false
			cmd := newDatabaseSQLCmd(test.toolsDB, func(ssh *osexec.Cmd) error {
				called = true
				conn, err := toolforge.NewConnection(test.tool, false, false)
				if err != nil {
					t.Fatal(err)
				}
				want := append([]string{"ssh"}, databaseSQLSSHArgs(conn, replicaForward{database: test.database, host: test.host}, test.clientArgs, false)...)
				if !reflect.DeepEqual(ssh.Args, want) {
					t.Fatalf("SSH args = %q, want %q", ssh.Args, want)
				}
				if ssh.Stdin != input || ssh.Stdout != &stdout || ssh.Stderr != &stderr {
					t.Fatal("SSH did not inherit command streams")
				}
				return nil
			})
			root.AddCommand(cmd)
			root.SetIn(input)
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs(append([]string{"sql"}, test.args...))
			if err := root.Execute(); err != nil || !called || stdout.Len() != 0 {
				t.Fatalf("called=%v stdout=%q err=%v", called, stdout.String(), err)
			}
		})
	}
}

func TestDatabaseSQLRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		toolsDB bool
		args    []string
	}{
		{false, nil},
		{false, []string{"enwiki", "dewiki"}},
		{false, []string{"s1"}},
		{false, []string{"enwiki", "--service", "other"}},
		{false, []string{"enwiki", "-e", "SELECT 1"}},
		{true, []string{"invalid"}},
		{true, []string{"s1__db", "other"}},
		{true, []string{"s1__db", "--beta"}},
		{true, []string{"s1__db", "--lima"}},
	} {
		root := &cobra.Command{Use: "toolforge"}
		root.PersistentFlags().Bool("beta", false, "Beta")
		root.PersistentFlags().Bool("lima", false, "Lima")
		root.AddCommand(newDatabaseSQLCmd(test.toolsDB, func(*osexec.Cmd) error {
			t.Fatal("invalid input started SSH")
			return nil
		}))
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs(append([]string{"sql"}, test.args...))
		if err := root.Execute(); err == nil {
			t.Errorf("toolsDB=%v accepted %q", test.toolsDB, test.args)
		}
	}
}

func TestDatabaseSQLSSHMode(t *testing.T) {
	connection := toolforge.Connection{Host: "login.toolforge.org"}
	for _, terminal := range []bool{false, true} {
		args := databaseSQLSSHArgs(connection, replicaForward{database: "enwiki_p", host: "example"}, nil, terminal)
		mode := "-T"
		if terminal {
			mode = "-t"
		}
		if args[0] != mode {
			t.Fatalf("terminal=%v got %q", terminal, args)
		}
	}
	if terminalIO(strings.NewReader(""), io.Discard) {
		t.Fatal("pipes should not allocate a terminal")
	}
}

func TestDatabaseSQLRemoteArgumentsAndInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the SSH stand-ins use POSIX shell and Python")
	}
	if _, err := osexec.LookPath("python3"); err != nil {
		t.Skip("python3 is needed by the SSH stand-ins")
	}
	dir := t.TempDir()
	sudo := `#!/usr/bin/python3
import os, sys
args = sys.argv[sys.argv.index('--') + 1:]
command = ' '.join(''.join(c if c.isascii() and (c.isalnum() or c in '_-$') else '\\' + c for c in arg) for arg in args)
os.execv('/bin/sh', ['sh', '-c', command])
`
	ssh := `#!/usr/bin/python3
import os, sys
command = sys.argv[-1].replace("'/usr/bin/sudo'", "'" + os.environ['FAKE_SUDO'] + "'")
os.execv('/bin/sh', ['sh', '-c', command])
`
	mariadb := `#!/usr/bin/python3
import json, sys
print(json.dumps({"args": sys.argv[1:], "stdin": sys.stdin.read()}))
`
	for name, content := range map[string]string{"sudo": sudo, "ssh": ssh, "mariadb": mariadb} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FAKE_SUDO", filepath.Join(dir, "sudo"))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	query := "SELECT '$HOME', 'a b', \"$(printf injected)\", `literal`;\n"
	for _, tool := range []string{"", "mytool"} {
		t.Run("tool="+tool, func(t *testing.T) {
			root := &cobra.Command{Use: "toolforge"}
			root.PersistentFlags().String("tool", "", "Tool")
			root.AddCommand(newDatabaseSQLCmd(true, runDatabaseSQL))
			var output bytes.Buffer
			root.SetIn(strings.NewReader("SELECT 'from stdin';\n"))
			root.SetOut(&output)
			root.SetErr(io.Discard)
			root.SetArgs([]string{"sql", "s123__db", "--tool", tool, "--", "-e", query, "--batch"})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			var got struct {
				Args  []string `json:"args"`
				Stdin string   `json:"stdin"`
			}
			if err := json.Unmarshal(output.Bytes(), &got); err != nil {
				t.Fatalf("output %q: %v", output.String(), err)
			}
			want := []string{"--defaults-file=" + filepath.Join(os.Getenv("HOME"), "replica.my.cnf"), "--host=tools.db.svc.wikimedia.cloud", "--database=s123__db", "-e", query, "--batch"}
			if !reflect.DeepEqual(got.Args, want) || got.Stdin != "SELECT 'from stdin';\n" {
				t.Fatalf("args=%q stdin=%q", got.Args, got.Stdin)
			}
		})
	}
}

func TestDatabaseSQLExitAndCancellation(t *testing.T) {
	root := &cobra.Command{Use: "toolforge"}
	root.AddCommand(newDatabaseSQLCmd(false, func(ssh *osexec.Cmd) error {
		replicaSSHHelperCommand(ssh, "failure")
		return ssh.Run()
	}))
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"sql", "enwiki"})
	var exit *osexec.ExitError
	if err := root.Execute(); !errors.As(err, &exit) || root.Annotations["exitCode"] != "42" {
		t.Fatalf("exit error=%v annotations=%v", err, root.Annotations)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	cmd := newDatabaseSQLCmd(false, func(ssh *osexec.Cmd) error {
		replicaSSHHelperCommand(ssh, "wait")
		if err := ssh.Start(); err != nil {
			return err
		}
		close(started)
		return ssh.Wait()
	})
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"enwiki"})
	done := make(chan error, 1)
	go func() { done <- cmd.Execute() }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("SSH did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SSH did not stop after cancellation")
	}
}
