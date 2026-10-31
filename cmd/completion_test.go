package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/knadh/koanf"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestCompletionStartupDoesNotWriteTimersOrPrompt(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	if err := config.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	previousArgs, previousOutput, previousOptions, previousDetails := os.Args, os.Stdout, cli.Opts, cli.VersionDetails
	t.Cleanup(func() {
		os.Args, os.Stdout, cli.Opts, cli.VersionDetails = previousArgs, previousOutput, previousOptions, previousDetails
	})
	output, err := os.CreateTemp(t.TempDir(), "completion")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	os.Stdout = output
	os.Args = []string{"mw", "__completeNoDesc", "wiki", "page", "list", "--wiki", "test.wikipedia."}
	Execute("", "", "", "", "", "latest")
	after, err := os.ReadFile(config.Path())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("completion startup modified configuration")
	}
	data, err := os.ReadFile(output.Name())
	if err != nil || !bytes.Contains(data, []byte("test.wikipedia.org/w/api.php")) {
		t.Fatalf("completion did not return the site: %q, %v", data, err)
	}
}
