package dirs

import (
	"os"
	"path/filepath"

	userutil "gitlab.wikimedia.org/releng/cli/internal/util/user"
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

	currentUser, err := userutil.CurrentUserIgnoringRootIfSudo()
	if err != nil {
		panic(err)
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

/*ListRawYamlFilesInContextOfProjectDirectory ...*/
func ListRawYamlFilesInContextOfProjectDirectory(projectDirectory string) []string {
	var files []string

	for _, file := range listRawFiles(projectDirectory) {
		if filepath.Ext(file) == ".yml" {
			files = append(files, filepath.Base(file))
		}
	}

	return files
}

/*listRawFiles lists the raw docker-compose file paths that are currently on disk.*/
func listRawFiles(projectDirectory string) []string {
	var files []string

	err := filepath.Walk(projectDirectory, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return files
}
