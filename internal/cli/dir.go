package cli

import (
	"os"

	"github.com/adrg/xdg"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/dirs"
)

// UserDirectoryPath is the application configuration directory path for the user.
func UserDirectoryPath() string {
	// user home dir can not be used in Gitlab CI, must use the project dir instead!
	// https://medium.com/@patrick.winters/mounting-volumes-in-sibling-containers-with-gitlab-ci-534e5edc4035
	_, inGitlabCi := os.LookupEnv("GITLAB_CI")
	if inGitlabCi {
		return CIUserDirectoryPath()
	}
	return XDGUserDirectoryPath()
}

// UserDirectoryPathForCmd is the application configuration directory path for the user for a specific command.
func UserDirectoryPathForCmd(cmdName string) string {
	return UserDirectoryPath() + string(os.PathSeparator) + cmdName
}

// XDGUserDirectoryPath is the application configuration directory path for the user determined by XDG.
func XDGUserDirectoryPath() string {
	return xdg.ConfigHome + string(os.PathSeparator) + APPNAME
}

func CIUserDirectoryPath() string {
	ciDir, found := os.LookupEnv("CI_PROJECT_DIR")
	if !found {
		panic("No CI_PROJECT_DIR found in environment")
	}
	return ciDir
}

// LegacyUserDirectoryPath is the application configuration directory path for the user determined by legacy method and should be considered deprecated.
func LegacyUserDirectoryPath() string {
	return dirs.LegacyUserDirectoryPath(".mwcli")
}
