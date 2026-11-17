package agent

import (
	"os"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		t.Setenv(name, "")
	}
	for _, test := range []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "no agent"},
		{name: "custom takes priority", env: map[string]string{"AI_AGENT": " custom-agent ", "CLAUDECODE": "1"}, want: "custom-agent"},
		{name: "cursor", env: map[string]string{"CURSOR_AGENT": "1"}, want: "cursor-cli"},
		{name: "claude", env: map[string]string{"CLAUDECODE": "1"}, want: "claude_code"},
		{name: "codex", env: map[string]string{"CODEX_THREAD_ID": "fixture"}, want: "codex_cli"},
		{name: "gemini", env: map[string]string{"GEMINI_CLI": "true"}, want: "gemini_cli"},
		{name: "copilot", env: map[string]string{"COPILOT_MODEL": "fixture"}, want: "github-copilot"},
		{name: "copilot SDK fallback", env: map[string]string{"COPILOT_AGENT_SESSION_ID": "fixture"}, want: "github-copilot"},
		{name: "SDK fallback preserves upstream priority", env: map[string]string{"COPILOT_AGENT_SESSION_ID": "fixture", "AI_AGENT": "custom"}, want: "custom"},
		{name: "false upstream indicator", env: map[string]string{"CLAUDECODE": "false"}, want: "claude_code"},
		{name: "zero upstream indicator", env: map[string]string{"GEMINI_CLI": "0"}, want: "gemini_cli"},
		{name: "false SDK indicator", env: map[string]string{"COPILOT_AGENT_SESSION_ID": "false"}},
		{name: "zero SDK indicator", env: map[string]string{"COPILOT_AGENT_SESSION_ID": "0"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for name, value := range test.env {
				t.Setenv(name, value)
			}
			detected, err := Detect()
			if err != nil {
				t.Fatal(err)
			}
			if test.want == "" {
				if detected != nil {
					t.Fatalf("unexpected agent: %+v", detected)
				}
			} else if detected == nil || detected.Name != test.want {
				t.Fatalf("agent = %+v, want %q", detected, test.want)
			}
		})
	}
}
