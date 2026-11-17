package debug

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestDebugAgentCommand(t *testing.T) {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		t.Setenv(name, "")
	}
	for _, name := range []string{"", "custom-agent"} {
		t.Run("agent="+name, func(t *testing.T) {
			t.Setenv("AI_AGENT", name)
			cmd := Cmd()
			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(out)
			cmd.SetArgs([]string{"agent"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var got struct {
				IsAgent bool    `json:"is_agent"`
				Agent   *string `json:"agent"`
			}
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.IsAgent != (name != "") || (name == "" && got.Agent != nil) || (name != "" && (got.Agent == nil || *got.Agent != name)) {
				t.Fatalf("unexpected detection JSON: %s", out)
			}
		})
	}
	cmd := DebugAgentCmd()
	if !cmd.Hidden || cmd.Annotations[cli.TelemetrySkipAnnotation] != "true" {
		t.Fatal("agent diagnostics must be hidden and skip telemetry")
	}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"unexpected"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("unexpected argument accepted")
	}
}

type failingAgentWriter struct{}

func (failingAgentWriter) Write([]byte) (int, error) {
	return 0, errors.New("fixture write failure")
}

func TestDebugAgentOutputFailure(t *testing.T) {
	t.Setenv("AI_AGENT", "custom-agent")
	cmd := DebugAgentCmd()
	cmd.SetOut(failingAgentWriter{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "fixture write failure") {
		t.Fatalf("output error = %v", err)
	}
}
