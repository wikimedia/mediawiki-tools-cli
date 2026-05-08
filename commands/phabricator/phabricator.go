// Package phabricator provides the "phabricator" (alias "phab") command.
// It embeds phab.py from https://gitlab.wikimedia.org/jiji/phab and runs it
// via the system python3 interpreter, passing all arguments through unchanged.
package phabricator

import (
	_ "embed"
	"os"
	"os/exec"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
)

//go:embed phab.py
var phabScript []byte

// setExitCode stores the given integer exit code in the root command annotations
// so that cmd/root.go can call os.Exit with the correct code.
func setExitCode(cmd *cobra.Command, code int) {
	cmd.Root().Annotations = make(map[string]string)
	cmd.Root().Annotations["exitCode"] = strconv.Itoa(code)
}

// NewPhabricatorCmd returns the "phabricator" cobra command.
func NewPhabricatorCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "phabricator",
		Aliases: []string{"phab"},
		GroupID: "service",
		Short:   "Interact with Wikimedia Phabricator",
		Long: `Runs the phab CLI tool (https://gitlab.wikimedia.org/jiji/phab).

All arguments are forwarded directly to the embedded phab.py Python script.
Requires python3 to be available in PATH.`,
		// DisableFlagParsing passes every argument (including flags) straight
		// through to phab.py rather than letting cobra intercept them.
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			if !lookpath.HasExecutable("python3") {
				logrus.Error("python3 is required to use this command but was not found in PATH")
				setExitCode(cmd, 1)
				return
			}

			tmp, err := os.CreateTemp("", "phab-*.py")
			if err != nil {
				logrus.Errorf("failed to create temporary script file: %s", err)
				setExitCode(cmd, 1)
				return
			}
			defer os.Remove(tmp.Name())

			if _, err := tmp.Write(phabScript); err != nil {
				logrus.Errorf("failed to write temporary script file: %s", err)
				setExitCode(cmd, 1)
				return
			}
			if err := tmp.Close(); err != nil {
				logrus.Errorf("failed to close temporary script file: %s", err)
				setExitCode(cmd, 1)
				return
			}

			c := exec.Command("python3", append([]string{tmp.Name()}, args...)...) // #nosec G204
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			if err := c.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					setExitCode(cmd, exitErr.ExitCode())
				} else {
					logrus.Errorf("failed to run phabricator command: %s", err)
					setExitCode(cmd, 1)
				}
			}
		},
	}
}
