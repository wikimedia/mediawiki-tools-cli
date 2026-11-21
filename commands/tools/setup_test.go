package tools

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
)

func TestToolforgeSetupChecksLoginBeforeToolAccess(t *testing.T) {
	for _, tt := range []struct {
		name      string
		tool      string
		loginErr  error
		toolErr   error
		wantCalls int
		want      []string
	}{
		{
			name: "login only", wantCalls: 1,
			want: []string{"SSH (login.toolforge.org): authenticated as developer", "mw toolforge create"},
		},
		{
			name: "SSH rejected", tool: "mytool", loginErr: errors.New("Permission denied (publickey)"), wantCalls: 1,
			want: []string{"Permission denied", "membership or pending request", toolsAdminURL},
		},
		{
			name: "tool access rejected", tool: "mytool", toolErr: errors.New("sudo: not allowed to run command"), wantCalls: 2,
			want: []string{"authenticated as developer", "Tool account tools.mytool: sudo:", "check your maintainer access"},
		},
		{
			name: "tool access works", tool: "mytool", wantCalls: 2,
			want: []string{"authenticated as developer", "Tool account tools.mytool: accessible", toolforgeQuickstart + "#Build_and_deploy_your_first_tool"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			probe := func(_ context.Context, connection toolforge.Connection, username string) (string, error) {
				calls++
				if connection.Host != "login.toolforge.org" || username != "developer" {
					t.Fatalf("wrong SSH destination: %s@%s", username, connection.Host)
				}
				if calls == 1 {
					if connection.ToolUser != "" {
						t.Fatalf("login used tool account %s", connection.ToolUser)
					}
					return "developer", tt.loginErr
				}
				if connection.ToolUser != "tools."+tt.tool {
					t.Fatalf("wrong tool account: %s", connection.ToolUser)
				}
				return connection.ToolUser, tt.toolErr
			}
			cmd := newToolforgeSetupCmd(probe)
			cmd.Flags().String("tool", "", "Tool account to check")
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"--check", "--username", "developer", "--tool", tt.tool})
			if err := cmd.ExecuteContext(t.Context()); err != nil {
				t.Fatal(err)
			}
			if calls != tt.wantCalls {
				t.Fatalf("made %d SSH probes, want %d", calls, tt.wantCalls)
			}
			for _, want := range tt.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in %q", want, out.String())
				}
			}
			if strings.Contains(out.String(), "\x1b") || strings.ContainsAny(out.String(), "✅⚠") {
				t.Errorf("redirected output contains terminal styling: %q", out.String())
			}
		})
	}
}
