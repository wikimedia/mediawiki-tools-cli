package dockercompose

import (
	"os"
	"testing"
)

func TestCmdReturnsErrorWhenComposeIsUnavailable(t *testing.T) {
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })

	project := Project{Name: "test", Directory: t.TempDir()}
	if _, err := project.Cmd([]string{"ps"}); err == nil {
		t.Fatal("expected an error when Docker Compose is unavailable")
	}
	if err := project.Command([]string{"ps"}).RunAttached(); err == nil {
		t.Fatal("expected command execution to preserve command construction error")
	}
}
