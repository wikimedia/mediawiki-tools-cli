// Package lookpath provides wrapping functionality around os.exec.LookPath
package lookpath

import (
	"fmt"
	"os/exec"
	"strings"
)

// HasExecutable returns true if the given executable is in PATH
func HasExecutable(executable string) bool {
	_, err := exec.LookPath(executable)
	return err == nil
}

// NeedExecutables checks to see if the given executables are in PATH, erroring and returning the missing executables if not
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
