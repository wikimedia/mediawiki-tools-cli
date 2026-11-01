package tools

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestCreateToolBrowserAndManualFallback(t *testing.T) {
	for _, test := range []struct {
		name          string
		args          []string
		noInteraction bool
		browserError  bool
		wantOpened    bool
		wantError     bool
	}{
		{name: "browser", wantOpened: true},
		{name: "manual", args: []string{"--no-browser"}},
		{name: "noninteractive", noInteraction: true},
		{name: "browser unavailable", browserError: true, wantOpened: true, wantError: true},
		{name: "extra argument", args: []string{"unexpected"}, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			previous := cli.Opts
			t.Cleanup(func() { cli.Opts = previous })
			cli.Opts.NoInteraction = test.noInteraction
			opened := false
			cmd := newCreateToolCmd(func(url string) error {
				opened = true
				if url != toolCreateURL {
					t.Fatalf("wrong creation URL: %s", url)
				}
				if test.browserError {
					return errors.New("launcher failed")
				}
				return nil
			})
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(test.args)
			err := cmd.Execute()
			if opened != test.wantOpened || (err != nil) != test.wantError {
				t.Fatalf("browser opened = %t, error = %v", opened, err)
			}
			if test.name != "extra argument" && !strings.Contains(output.String(), toolCreateURL) {
				t.Fatal("creation URL missing, including when browser launch fails")
			}
		})
	}
}

func TestToolforgeCreateNeedsNoExistingTool(t *testing.T) {
	for _, args := range [][]string{{"create", "--no-browser"}, {"create", "--no-browser", "--beta"}, {"create", "--no-browser", "--lima"}} {
		cmd := NewToolforgeCmd()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		err := cmd.Execute()
		if len(args) == 2 {
			if err != nil || !strings.Contains(output.String(), toolCreateURL) {
				t.Fatalf("creation requires existing credentials or tool: %v", err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "production Toolforge") {
			t.Fatalf("non-production creation opened the production form: %v", err)
		}
	}
}
