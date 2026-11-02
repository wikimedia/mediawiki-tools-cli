package cmd

import "testing"

func TestDelegationUsesRealCommandDefinitions(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	for _, args := range [][]string{
		{"phab", "task", "list", "--project", "a_wikimedia_cli", "--output", "json"},
		{"phab", "view", "T123"},
		{"phab", "paste", "send", "--title", "example", "--file", "-", "--yes"},
		{"phab", "task", "create", "--input", "-", "--yes"},
	} {
		if err := ValidateRemoteInvocation(args); err != nil {
			t.Fatalf("rejected %v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"config", "secrets", "list"},
		{"phab", "shell"},
		{"phab", "auth", "login"},
		{"phab", "auth", "remove", "addshore"},
		{"phab", "paste", "create", "--file", "/etc/passwd", "--yes"},
		{"phab", "task", "create", "--input", "/etc/passwd", "--yes"},
		{"phab", "view", "T123", "--output", "web"},
	} {
		if err := ValidateRemoteInvocation(args); err == nil {
			t.Fatalf("accepted unsafe delegation %v", args)
		}
	}
}
