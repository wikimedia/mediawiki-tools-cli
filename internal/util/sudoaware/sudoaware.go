package sudoaware

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// CurrentUser will ignore the root user if sudo is being used, getting the real current user.
func CurrentUser() (*user.User, error) {
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

func currentUserUIDGidOrFatal() (int, int) {
	user, err := CurrentUser()
	if err != nil {
		panic(err)
	}
	uid, uidErr := strconv.Atoi(user.Uid)
	if uidErr != nil {
		panic(uidErr)
	}
	gid, gidErr := strconv.Atoi(user.Gid)
	if gidErr != nil {
		panic(gidErr)
	}
	return uid, gid
}

// IsDifferentUser will return true if the current user is not the same as the user running the program, such as when sudo is used.
func IsDifferentUser() bool {
	callingUser, err1 := user.Current()
	sudoAware, err2 := CurrentUser()
	if err1 != nil || err2 != nil {
		panic("Could not compare current users")
	}
	return callingUser.Uid != sudoAware.Uid
}

// OpenFile implementation that will chown files created to the current user if sudo is used.
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	file, err := os.OpenFile(name, flag, perm)
	// sudoaware modification start
	if IsDifferentUser() {
		uid, gid := currentUserUIDGidOrFatal()
		file.Chown(uid, gid)
	}
	// sudoaware modification end
	return file, err
}

// MkdirAll implementation that will chown directories created to the current user if sudo is used.
func MkdirAll(path string, perm os.FileMode) error {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := os.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	if j > 1 {
		// Create parent.
		err = MkdirAll(fixRootDirectory(path[:j-1]), perm)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = os.Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := os.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}

	// sudoaware modification start
	if IsDifferentUser() {
		uid, gid := currentUserUIDGidOrFatal()
		os.Chown(path, uid, gid)
	}
	// sudoaware modification end

	return nil
}

func fixRootDirectory(p string) string {
	return p
}
