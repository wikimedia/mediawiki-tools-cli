package paths

import (
	"os"
	"strings"
)

/*ResolveMountForCwd ...*/
func ResolveMountForCwd(mountFrom string, mountTo string) *string {
	cwd, _ := os.Getwd()
	return resolveMountForDirectory(mountFrom, mountTo, cwd)
}

func resolveMountForDirectory(mountFrom string, mountTo string, directory string) *string {
	// If the directory that we are in is part of the mount point
	if strings.HasPrefix(directory, mountFrom) {
		// We can use that mount point with any path suffix (other directories) appended
		modified := strings.Replace(directory, mountFrom, mountTo, 1)
		return &modified
	}

	// Otherwise we don't know where we are and can't help
	return nil
}
