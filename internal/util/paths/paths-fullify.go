package paths

import (
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

/*FullifyUserProvidedPath fullify people entering ~/ or ./ paths and them not being handeled anywhere.*/
func FullifyUserProvidedPath(userProvidedPath string) string {
	if strings.HasPrefix(userProvidedPath, "/") || pathIsWindowsAbs(userProvidedPath) {
		return userProvidedPath
	}

	currentWorkingDirectory, _ := os.Getwd()

	if userProvidedPath == "." {
		return currentWorkingDirectory
	}
	if strings.HasPrefix(userProvidedPath, "./") || strings.HasPrefix(userProvidedPath, `.\`) {
		return filepath.Join(currentWorkingDirectory, userProvidedPath[2:])
	}

	usr, _ := user.Current()
	usrDir := usr.HomeDir

	if userProvidedPath == "~" {
		return usrDir
	}
	if strings.HasPrefix(userProvidedPath, "~/") || strings.HasPrefix(userProvidedPath, `~\`) {
		return filepath.Join(usrDir, userProvidedPath[2:])
	}

	// Fallback to what we were provided
	return filepath.Join(currentWorkingDirectory, userProvidedPath)
}

func pathIsWindowsAbs(path string) bool {
	if len(path) < 3 {
		return false
	}
	firstThree := path[:3]
	regularExpression := regexp.MustCompile(`\w\:\\`)
	return regularExpression.MatchString(firstThree)
}
