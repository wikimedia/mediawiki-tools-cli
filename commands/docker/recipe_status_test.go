package docker

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	composecmd "gitlab.wikimedia.org/repos/releng/cli/commands/docker/dockercompose"
)

func TestRecipeStatusHintNamesExistingCommand(t *testing.T) {
	root := &cobra.Command{Use: "mw"}
	docker := &cobra.Command{Use: "docker"}
	compose := composecmd.NewCmd()
	docker.AddCommand(compose)
	root.AddCommand(docker)
	words := strings.Fields(recipeStatusCommand)
	found, args, err := root.Find(words[1:])
	if err != nil || found != compose || len(args) != 1 || args[0] != "ps" {
		t.Fatalf("invalid recipe hint %q: command=%v args=%v err=%v", recipeStatusCommand, found, args, err)
	}
}
