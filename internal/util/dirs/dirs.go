package dirs

import (
	"os"
)

/*EnsureExists Ensures that a directory exists, if it doesn't it and all parent directories will be created.*/
func EnsureExists(dirPath string) {
	if _, err := os.Stat(dirPath); err != nil {
		mkerr := os.MkdirAll(dirPath, 0o755)
		if mkerr != nil {
			panic(mkerr)
		}
	}
}

/*FilesIn list full paths of all files in a directory (recursively).*/
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
