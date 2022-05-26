package user

import (
	"os"
	"os/user"
)

// CurrentUserIgnoringRootIfSudo will ignore the root user if sudo is being used, getting the calling user.
func CurrentUserIgnoringRootIfSudo() (*user.User, error) {
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
