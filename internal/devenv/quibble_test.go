package devenv

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestQuibbleCompatibilityWrapper(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python 3 is needed to test the Quibble wrapper")
	}
	cmd := exec.Command(python, filepath.Join("..", "..", "tests", "test-quibble-wrapper.py"))
	cmd.Env = append(cmd.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Quibble wrapper tests failed: %v\n%s", err, output)
	}
}
