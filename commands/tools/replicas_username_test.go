package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
)

func TestReplicaUsernameCommandPrintsOnlyUser(t *testing.T) {
	for _, source := range []string{"", "mytool"} {
		root := &cobra.Command{Use: "toolforge"}
		root.PersistentFlags().String("tool", "", "Tool name")
		var stdout, stderr bytes.Buffer
		cmd := newReplicaUsernameCmd(func(_ context.Context, conn toolforge.Connection, _ io.Reader, errOut io.Writer) (string, error) {
			wantTool := ""
			if source != "" {
				wantTool = "tools." + source
			}
			if conn.Host != "login.toolforge.org" || conn.ToolUser != wantTool {
				t.Fatalf("connection = %#v", conn)
			}
			fmt.Fprintln(errOut, "SSH diagnostic")
			return "s12345", nil
		})
		root.AddCommand(cmd)
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"username", "--tool", source})
		if err := root.Execute(); err != nil || stdout.String() != "s12345\n" || stderr.String() != "SSH diagnostic\n" {
			t.Fatalf("stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
		}
	}
}

func TestReplicaUsernameDefaultsToStdout(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	previous := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = previous }()
	cmd := newReplicaUsernameCmd(func(context.Context, toolforge.Connection, io.Reader, io.Writer) (string, error) {
		return "s12345", nil
	})
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil || string(output) != "s12345\n" {
		t.Fatalf("stdout=%q err=%v", output, err)
	}
}

func TestReplicaUsernameFailureDoesNotExposeFileContents(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		cmd := newReplicaUsernameCmd(func(context.Context, toolforge.Connection, io.Reader, io.Writer) (string, error) {
			if invalid {
				return "user:SECRET_SENTINEL", nil
			}
			return "", errors.New("SSH lookup failed")
		})
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(nil)
		err := cmd.Execute()
		if err == nil || stdout.Len() != 0 || strings.Contains(err.Error()+stderr.String(), "SECRET_SENTINEL") {
			t.Fatalf("stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
		}
	}
}

func TestReplicaUsernameRejectsOtherEnvironmentsAndArguments(t *testing.T) {
	for _, args := range [][]string{{"username", "--beta"}, {"username", "--lima"}, {"username", "unexpected"}} {
		root := &cobra.Command{Use: "toolforge"}
		root.PersistentFlags().Bool("beta", false, "Beta")
		root.PersistentFlags().Bool("lima", false, "Lima")
		root.AddCommand(newReplicaUsernameCmd(func(context.Context, toolforge.Connection, io.Reader, io.Writer) (string, error) {
			t.Fatal("invalid input performed SSH lookup")
			return "", nil
		}))
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs(args)
		if err := root.Execute(); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
}

func TestReplicaUsernameScriptReturnsOnlyUsername(t *testing.T) {
	python, err := osexec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is needed to exercise the script used on Toolforge")
	}
	for _, test := range []struct {
		name, content string
		wantError     bool
	}{
		{name: "normal", content: "[client]\nuser=s12345\npassword=SECRET_SENTINEL\n"},
		{name: "quoted", content: "[client]\nuser='s12345'\npassword=SECRET_SENTINEL\n"},
		{name: "missing user", content: "[client]\npassword=SECRET_SENTINEL\n", wantError: true},
		{name: "malformed", content: "[client]\nuser=s12345\npassword SECRET_SENTINEL\n", wantError: true},
		{name: "multiline user", content: "[client]\nuser=s12345\n  password=SECRET_SENTINEL\n", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			if err := os.WriteFile(filepath.Join(home, "replica.my.cnf"), []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := osexec.CommandContext(t.Context(), python, "-c", replicaUsernameScript)
			cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home)
			value, err := cmd.CombinedOutput()
			if (err != nil) != test.wantError || bytes.Contains(value, []byte("SECRET_SENTINEL")) {
				t.Fatalf("credential filtering failed: %q, %v", value, err)
			}
			if !test.wantError && string(value) != "s12345\n" && string(value) != "s12345\r\n" {
				t.Fatalf("unexpected username: %q", value)
			}
		})
	}
}

func TestReplicaUsernameLookupIsBoundedAndDoesNotEchoInvalidOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the test SSH stand-in is a POSIX shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte("#!/bin/sh\nprintf '%s\\n' \"$MWCLI_REPLICA_TEST_USER\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	for _, user := range []string{"s12345", "user:SECRET_SENTINEL", "s12345\npassword=SECRET_SENTINEL"} {
		t.Setenv("MWCLI_REPLICA_TEST_USER", user)
		got, err := readReplicaUsername(t.Context(), toolforge.Connection{Host: "login.toolforge.org"}, nil, io.Discard)
		if user == "s12345" {
			if err != nil || got != user {
				t.Fatalf("lookup = %q, %v", got, err)
			}
		} else if err == nil || got != "" || strings.Contains(err.Error(), "SECRET_SENTINEL") {
			t.Fatalf("lookup exposed invalid output: %q, %v", got, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := readReplicaUsername(ctx, toolforge.Connection{Host: "login.toolforge.org"}, nil, io.Discard); err == nil {
		t.Fatal("canceled username lookup succeeded")
	}
}
