// Package lookpath provides wrapping functionality around os.exec.LookPath
package lookpath

import (
	"fmt"
	"os/exec"
	"strings"
)

// HasExecutable returns true if the given executable is in PATH.
func HasExecutable(executable string) bool {
	_, err := exec.LookPath(executable)
	return err == nil
}

// NeedExecutables checks to see if the given executables are in PATH, erroring and returning the missing executables if not.
func NeedExecutables(executables []string) (missing []string, err error) {
	for _, executable := range executables {
		if !HasExecutable(executable) {
			missing = append(missing, executable)
		}
	}
	if len(missing) > 0 {
		return missing, fmt.Errorf("missing executables: %s", strings.Join(missing, ", "))
	}
	return missing, nil
}

// NeedCommands checks to see if the given executable, and sub command, are in PATH, erroring and returning the missing whole commands if not.
// For example, a command could be "docker compose", which should check for the "docker" executable, and then make sure "docker compose" exits with status 0.
func NeedCommands(commands []string) (missing []string, err error) {
	for _, command := range commands {
		parts := strings.Split(command, " ")
		if len(parts) == 0 {
			return missing, fmt.Errorf("command is empty")
		}
		if !HasExecutable(parts[0]) {
			missing = append(missing, parts[0])
		}
		if len(parts) > 1 {
			cmd := exec.Command(parts[0], parts[1:]...)
			if err := cmd.Run(); err != nil {
				missing = append(missing, command)
			}
		}
	}
	if len(missing) > 0 {
		return missing, fmt.Errorf("missing commands: %s", strings.Join(missing, ", "))
	}
	return missing, nil
}
