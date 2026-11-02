package hosts

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	hostutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/hosts"
)

func TestChangeResultKeepsStatusOffStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := handleChangeResult(cmd, hostutil.ChangeResult{Success: true, Altered: true, WriteFile: "/tmp/hosts"}); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Hosts file altered and updated") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestChangeResultReturnsFailureWithRecoveryInstructions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := handleChangeResult(cmd, hostutil.ChangeResult{Altered: true, WriteFile: "/tmp/prepared-hosts"}); err == nil {
		t.Fatal("expected failed write error")
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "sudo cp") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
