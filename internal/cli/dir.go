package cli

import (
	"os"
	"os/user"

	"github.com/adrg/xdg"
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
	subPath := ".mwcli"

	// user home dir can not be used in Gitlab CI, must use the project dir instead!
	// https://medium.com/@patrick.winters/mounting-volumes-in-sibling-containers-with-gitlab-ci-534e5edc4035
	// TODO maybe this should be pushed further up and the whole mwcli dir should be moved?!
	_, inGitlabCi := os.LookupEnv("GITLAB_CI")
	if inGitlabCi {
		ciDir, _ := os.LookupEnv("CI_PROJECT_DIR")
		return ciDir + string(os.PathSeparator) + subPath
	}

	currentUserIgnoringRootIfSudo := func() (*user.User, error) {
		currentUser, err := user.Current()
		// Bail early if we can't even get the current user.
		if err != nil {
			return currentUser, err
		}

		// If we are root, check to see if we can detect sudo being used
		if currentUser.Uid == "0" {
			sudoUID := os.Getenv("SUDO_UID")
			if sudoUID == "" {
				// User is probably actually running as root?
				return currentUser, nil
			}
			return user.LookupId(sudoUID)
		}

		// Otherwise we are just the current user
		return currentUser, err
	}

	currentUser, err := currentUserIgnoringRootIfSudo()
	if err != nil {
		panic(err)
	}

	return currentUser.HomeDir + string(os.PathSeparator) + subPath
}
