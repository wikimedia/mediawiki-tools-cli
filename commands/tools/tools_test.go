package tools

import "testing"

func TestNewToolsCmdHasOneEnvvarQuotaCommand(t *testing.T) {
	command := NewToolsCmd()
	envvar, _, err := command.Find([]string{"envvar"})
	if err != nil {
		t.Fatalf("find envvar command: %v", err)
	}

	quotaCount := 0
	for _, child := range envvar.Commands() {
		if child.Name() == "quota" {
			quotaCount++
		}
	}
	if quotaCount != 1 {
		t.Fatalf("expected one envvar quota command, got %d", quotaCount)
	}
}
