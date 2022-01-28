package dirs

import (
	"os"
	"os/user"
)

/*UserDirectoryPath returns a path to a directory in the user directory.*/
func UserDirectoryPath(subPath string) string {
	// user home dir can not be used in Gitlab CI, must use the project dir instead!
	// https://medium.com/@patrick.winters/mounting-volumes-in-sibling-containers-with-gitlab-ci-534e5edc4035
	// TODO maybe this should be pushed further up and the whole mwcli dir should be moved?!
	_, inGitlabCi := os.LookupEnv("GITLAB_CI")
	if inGitlabCi {
		ciDir, _ := os.LookupEnv("CI_PROJECT_DIR")
		return ciDir + string(os.PathSeparator) + subPath
	}

	currentUser, err := user.Current()
	if err != nil {
		panic(err)
	}

	// If we are root, check to see if we can detect sudo being used
	if currentUser.Uid == "0" {
		sudoUID := os.Getenv("SUDO_UID")
		if sudoUID == "" {
			panic("detected sudo but no SUDO_UID")
		}
		currentUser, err = user.LookupId(sudoUID)
		if err != nil {
			panic(err)
		}
	}

	return currentUser.HomeDir + string(os.PathSeparator) + subPath
}

/*EnsureExists Ensures that a directory exists, if it doesn't it and all parent directories will be created.*/
func EnsureExists(dirPath string) {
	if _, err := os.Stat(dirPath); err != nil {
		mkerr := os.MkdirAll(dirPath, 0o755)
		if mkerr != nil {
			panic(mkerr)
		}
	}
}

/*FilesIn list full paths of all files in a directory (recursively)*/
func FilesIn(dirPath string) []string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		panic(err)
	}
	var files []string
	for _, entry := range entries {
		fullPath := dirPath + string(os.PathSeparator) + entry.Name()
		if entry.IsDir() {
			files = append(files, FilesIn(fullPath)...)
			continue
		}
		files = append(files, fullPath)
	}
	return files
}
