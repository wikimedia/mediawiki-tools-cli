package tools

import (
	"context"
	"encoding/json"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
)

func TestReplicaCredentialsScript(t *testing.T) {
	python, err := osexec.LookPath("python3")
	if err != nil {
		t.Skip("python3 required to exercise remote credential extraction")
	}
	for _, test := range []struct {
		name, file string
		invalid    bool
	}{
		{"normal", "[client]\nuser=s12345\npassword=REAL_REPLICA_SECRET\n", false},
		{"quoted", "[client]\nuser='s12345'\npassword=\"REAL_REPLICA_SECRET\"\n", false},
		{"missing", "[client]\nuser=s12345\n", true},
		{"malformed", "[client]\nREAL_REPLICA_SECRET\n", true},
		{"multiline", "[client]\nuser=s12345\npassword=REAL_REPLICA_SECRET\n continued\n", true},
		{"invalid user", "[client]\nuser=REAL_REPLICA_SECRET@host\npassword=REAL_REPLICA_SECRET\n", true},
		{"large", strings.Repeat("REAL_REPLICA_SECRET", 1024), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			folder := t.TempDir()
			if err := os.WriteFile(filepath.Join(folder, "replica.my.cnf"), []byte(test.file), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := osexec.Command(python, "-c", replicaCredentialsScript)
			cmd.Env = append(os.Environ(), "HOME="+folder, "USERPROFILE="+folder)
			output, err := cmd.Output()
			if test.invalid {
				if err == nil || len(output) != 0 || strings.Contains(err.Error(), "REAL_REPLICA_SECRET") {
					t.Fatalf("invalid file leaked output or was accepted")
				}
				if exit, ok := err.(*osexec.ExitError); ok && strings.Contains(string(exit.Stderr), "REAL_REPLICA_SECRET") {
					t.Fatal("parser error exposed file contents")
				}
				return
			}
			if err != nil {
				t.Fatal("valid credential file rejected")
			}
			var value replicaCredentials
			if err := json.Unmarshal(output, &value); err != nil || value.User != "s12345" || value.Password != "REAL_REPLICA_SECRET" {
				t.Fatal("incorrect extracted credentials")
			}
		})
	}
}

func TestReplicaCredentialsSSHOutputIsValidatedWithoutDisclosure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake SSH")
	}
	folder := t.TempDir()
	t.Setenv("PATH", folder+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, output := range []string{`{"user":"s12345","password":"REAL_REPLICA_SECRET"}`, `REAL_REPLICA_SECRET`, `{"user":"s12345","password":""}`, `{"user":"bad@user","password":"REAL_REPLICA_SECRET"}`} {
		script := "#!/bin/sh\nprintf '%s' '" + output + "'\n"
		if err := os.WriteFile(filepath.Join(folder, "ssh"), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		connection, err := toolforge.NewConnection("", false, false)
		if err != nil {
			t.Fatal(err)
		}
		value, err := readReplicaCredentials(context.Background(), connection, nil, io.Discard)
		if strings.Contains(output, `"password":"REAL_REPLICA_SECRET"`) && strings.Contains(output, `"user":"s12345"`) {
			if err != nil || value.User != "s12345" || value.Password != "REAL_REPLICA_SECRET" {
				t.Fatal("valid SSH output rejected")
			}
		} else if err == nil || strings.Contains(err.Error(), "REAL_REPLICA_SECRET") {
			t.Fatal("invalid SSH output accepted or leaked")
		}
	}
}

func TestReplicaSSHStreamUsesFixedDestinationAndClosesOnCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake SSH")
	}
	folder := t.TempDir()
	argsFile := filepath.Join(folder, "args")
	t.Setenv("MW_REPLICA_SSH_ARGS", argsFile)
	t.Setenv("PATH", folder+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$MW_REPLICA_SSH_ARGS\"\nexec cat\n"
	if err := os.WriteFile(filepath.Join(folder, "ssh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	connection, err := toolforge.NewConnection("", false, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := replicaSSHStream(ctx, connection, "enwiki.analytics.db.svc.wikimedia.cloud")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := stream.Write([]byte("packet")); err != nil {
		t.Fatal(err)
	}
	output := make([]byte, 6)
	if _, err := io.ReadFull(stream, output); err != nil || string(output) != "packet" {
		t.Fatalf("output=%q err=%v", output, err)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"-W\nenwiki.analytics.db.svc.wikimedia.cloud:3306\n", "BatchMode=yes", "ClearAllForwardings=yes", "PermitLocalCommand=no", "login.toolforge.org"} {
		if !strings.Contains(string(args), required) {
			t.Fatalf("missing SSH option %q", required)
		}
	}
	if strings.Contains(string(args), "REAL_REPLICA_SECRET") {
		t.Fatal("password passed to SSH in arguments")
	}
	cancel()
	if _, err := stream.Read(output); err == nil {
		t.Fatal("canceled SSH stream stayed open")
	}
}
