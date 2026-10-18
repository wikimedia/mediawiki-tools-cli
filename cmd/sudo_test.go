package cmd

import (
	"io"
	"os"
	"testing"

	"github.com/adrg/xdg"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestIsSudoInvocation(t *testing.T) {
	for _, tc := range []struct {
		uid  int
		sudo string
		want bool
	}{
		{0, "1000", true},
		{0, "", false},
		{0, "0", false},
		{1000, "1000", false},
		{-1, "1000", false},
	} {
		if got := isSudoInvocation(tc.uid, tc.sudo); got != tc.want {
			t.Fatalf("%+v: %v", tc, got)
		}
	}
}

func TestSudoUpdateLeavesConfigUntouched(t *testing.T) {
	originalHome, originalOpts := xdg.ConfigHome, cli.Opts
	t.Cleanup(func() { xdg.ConfigHome = originalHome; cli.Opts = originalOpts })
	xdg.ConfigHome = t.TempDir()
	for _, args := range [][]string{
		{"update", "--help"},
		{"--no-interaction", "update", "--dry-run", "--version=/tmp/mwcli-placeholder"},
		{"-vv", "update", "--dry-run", "--no-interaction", "--version=/tmp/mwcli-placeholder"},
	} {
		t.Run(args[0], func(t *testing.T) {
			root := sudoUpdateCommand(args)
			if root == nil {
				t.Fatal("sudo update was not selected")
			}
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(xdg.ConfigHome)
			if err != nil || len(entries) != 0 {
				t.Fatalf("sudo update changed user config: %v %v", entries, err)
			}
		})
	}
}

func TestOtherSudoCommandsKeepNormalStartup(t *testing.T) {
	originalOpts := cli.Opts
	t.Cleanup(func() { cli.Opts = originalOpts })
	for _, args := range [][]string{
		{"docker", "hosts", "add"},
		{"docker", "--context", "work", "hosts", "remove"},
		{"config", "show"},
		{"--help"},
		{},
	} {
		if root := sudoUpdateCommand(args); root != nil {
			t.Fatalf("unexpectedly replaced startup for %q", args)
		}
	}
}
