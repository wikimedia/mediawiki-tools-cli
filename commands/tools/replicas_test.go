package tools

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	osexec "os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestReplicaForwards(t *testing.T) {
	got, err := replicaForwards([]string{"enwiki_p", "commonswiki", "be_x_oldwiki"}, "analytics", 4711)
	if err != nil {
		t.Fatal(err)
	}
	want := []replicaForward{
		{database: "enwiki_p", host: "enwiki.analytics.db.svc.wikimedia.cloud", port: 4711},
		{database: "commonswiki_p", host: "commonswiki.analytics.db.svc.wikimedia.cloud", port: 4712},
		{database: "be_x_oldwiki_p", host: "be_x_oldwiki.analytics.db.svc.wikimedia.cloud", port: 4713},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("forwards = %#v, want %#v", got, want)
	}
	web, err := replicaForwards([]string{"wikidatawiki"}, "web", 65535)
	if err != nil || len(web) != 1 || web[0].host != "wikidatawiki.web.db.svc.wikimedia.cloud" || web[0].port != 65535 {
		t.Fatalf("web endpoint = %#v, error = %v", web, err)
	}
}

func TestReplicaTunnelRejectsInvalidInputBeforeSSH(t *testing.T) {
	for _, args := range [][]string{
		{},
		{""},
		{"_p"},
		{"enwiki", "enwiki_p"},
		{"ENWIKI"},
		{"enwiki;id"},
		{"enwiki\n"},
		{"enwiki.analytics.db.svc.wikimedia.cloud"},
		{"localhost:3306"},
		{"127.0.0.1"},
		{"[::1]"},
		{"links.commonswiki"},
		{"s1"},
		{"https://example.org"},
		{"../enwiki"},
		{strings.Repeat("a", 64)},
		{"enwiki", "--service", "unknown"},
		{"enwiki", "--service", "analytics.evil"},
		{"enwiki", "--port", "0"},
		{"enwiki", "--port", "-1"},
		{"enwiki", "--port", "65536"},
		{"enwiki", "dewiki", "--port", "65535"},
		{"enwiki", "--host", "example.org"},
		{"enwiki", "--", "-D", "1080"},
		{"enwiki", "--", "-R", "3306"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := newReplicaTunnelCmd(func(*osexec.Cmd) error { t.Fatal("invalid input started SSH"); return nil })
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("accepted %q", args)
			}
		})
	}
}

func TestReplicaTunnelSSHArgumentsAndIO(t *testing.T) {
	var output, stderr bytes.Buffer
	input := strings.NewReader("")
	called := false
	cmd := newReplicaTunnelCmd(func(ssh *osexec.Cmd) error {
		called = true
		want := []string{
			"ssh", "-N", "-T", "-o", "ExitOnForwardFailure=yes", "-o", "ControlMaster=no",
			"-o", "ControlPath=none", "-o", "IgnoreUnknown=ForkAfterAuthentication", "-o", "ForkAfterAuthentication=no",
			"-L", "127.0.0.1:13306:enwiki.web.db.svc.wikimedia.cloud:3306",
			"-L", "127.0.0.1:13307:dewiki.web.db.svc.wikimedia.cloud:3306", "login.toolforge.org",
		}
		if !reflect.DeepEqual(ssh.Args, want) {
			t.Fatalf("SSH arguments = %q, want %q", ssh.Args, want)
		}
		if ssh.Stdin != input || ssh.Stdout != &output || ssh.Stderr != &stderr {
			t.Fatal("SSH does not inherit command I/O")
		}
		return nil
	})
	cmd.SetIn(input)
	cmd.SetOut(&output)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"enwiki", "dewiki_p", "--service", "web", "--port", "13306"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called || !strings.Contains(output.String(), "enwiki_p  127.0.0.1:13306") || !strings.Contains(output.String(), "dewiki_p  127.0.0.1:13307") {
		t.Fatalf("missing SSH call or mappings: %s", output.String())
	}
}

func TestReplicaTunnelDryRunAndInheritedFlags(t *testing.T) {
	for _, test := range []struct {
		args      []string
		wantError bool
	}{
		{args: []string{"replicas", "tunnel", "enwiki", "dewiki", "--dry-run"}},
		{args: []string{"replicas", "tunnel", "enwiki", "--dry-run", "--beta"}, wantError: true},
		{args: []string{"replicas", "tunnel", "enwiki", "--dry-run", "--lima"}, wantError: true},
		{args: []string{"replicas", "tunnel", "enwiki", "--dry-run", "--tool", "mytool"}, wantError: true},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			cmd := NewToolforgeCmd()
			// The upstream module shares the tool flag storage between roots.
			for _, name := range []string{"tool", "beta", "lima"} {
				value := "false"
				if name == "tool" {
					value = ""
				}
				if err := cmd.PersistentFlags().Set(name, value); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = cmd.PersistentFlags().Set(name, value) })
			}
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(test.args)
			err := cmd.Execute()
			if !test.wantError {
				if err != nil || !strings.Contains(output.String(), "no connection opened") || !strings.Contains(output.String(), "'127.0.0.1:4712:dewiki.analytics.db.svc.wikimedia.cloud:3306'") {
					t.Fatalf("dry run = %s, error = %v", output.String(), err)
				}
			} else if err == nil {
				t.Fatalf("accepted unsupported Toolforge flags: %v", test.args)
			}
		})
	}
}

func replicaSSHHelperCommand(ssh *osexec.Cmd, mode string) {
	ssh.Path = os.Args[0]
	ssh.Args = []string{os.Args[0], "-test.run=^TestReplicaSSHHelper$"}
	ssh.Env = append(os.Environ(), "MWCLI_REPLICA_SSH_HELPER="+mode)
}

func TestReplicaSSHHelper(t *testing.T) {
	switch os.Getenv("MWCLI_REPLICA_SSH_HELPER") {
	case "failure":
		os.Exit(42)
	case "wait":
		time.Sleep(time.Hour)
	}
}

func TestReplicaTunnelFailurePreservesExitCode(t *testing.T) {
	root := &cobra.Command{Use: "mw"}
	cmd := newReplicaTunnelCmd(func(ssh *osexec.Cmd) error {
		replicaSSHHelperCommand(ssh, "failure")
		return ssh.Run()
	})
	root.AddCommand(cmd)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"tunnel", "enwiki"})
	var exit *osexec.ExitError
	if err := root.Execute(); !errors.As(err, &exit) || root.Annotations["exitCode"] != "42" || !strings.Contains(err.Error(), "SSH access and local port availability") {
		t.Fatalf("lost SSH failure: error = %v, annotations = %v", err, root.Annotations)
	}
}

func TestReplicaTunnelCancellationStopsSSH(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan *osexec.Cmd, 1)
	cmd := newReplicaTunnelCmd(func(ssh *osexec.Cmd) error {
		replicaSSHHelperCommand(ssh, "wait")
		if err := ssh.Start(); err != nil {
			return err
		}
		started <- ssh
		return ssh.Wait()
	})
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"enwiki"})
	done := make(chan error, 1)
	go func() { done <- cmd.Execute() }()
	var ssh *osexec.Cmd
	select {
	case ssh = <-started:
	case err := <-done:
		t.Fatalf("SSH never started: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out starting SSH")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil || ssh.ProcessState == nil {
			t.Fatalf("cancellation did not cleanly reap SSH: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("SSH was not stopped on cancellation")
	}
}
