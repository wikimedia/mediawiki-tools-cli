package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"
)

var pluginName = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// runPlugin handles an installed top-level command before normal CLI startup.
func runPlugin(args []string, dir string, root func() *cobra.Command) (int, bool) {
	if len(args) == 0 || !filepath.IsAbs(dir) || !pluginName.MatchString(args[0]) {
		return 0, false
	}
	switch args[0] {
	case "help", "completion":
		return 0, false
	}

	path, err := exec.LookPath(filepath.Join(dir, "wm-"+args[0]))
	if err != nil {
		return 0, false
	}
	commands := root()
	if command, _, _ := commands.Find(args[:1]); command != nil && command != commands {
		return 0, false
	}

	plugin := exec.Command(path, args[1:]...)
	plugin.Stdin = os.Stdin
	plugin.Stdout = os.Stdout
	plugin.Stderr = os.Stderr
	if err := plugin.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() >= 0 {
			return exit.ExitCode(), true
		}
		fmt.Fprintf(os.Stderr, "%s: %v\n", args[0], err)
		return 1, true
	}
	return 0, true
}
