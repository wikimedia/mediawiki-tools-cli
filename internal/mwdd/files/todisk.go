package files

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
)

/*EnsureInMemoryFilesAreOnDisk makes sure that up to date copies of our in app docker-compose files are on disk
TODO this should be called only when we update the bin?
TODO This is way to complex, and should be more clever.. checking a hash or something? and be more lightweight.*/
func ensureInMemoryFilesAreOnDisk(projectDirectory string) {
	embededFiles := files()

	// Ensure each file is on disk and up to date
	for _, embedFileName := range embededFiles {
		strippedFileName := strippedFileName(embedFileName)
		fileTargetOnDisk := projectDirectory + string(os.PathSeparator) + strippedFileName
		packagedBytes := fileBytes(embedFileName)

		if _, err := os.Stat(fileTargetOnDisk); os.IsNotExist(err) {
			// TODO only output the below line with verbose logging
			// fmt.Println(fileTargetOnDisk + " doesn't exist, so write it...")
			writeBytesToDisk(packagedBytes, fileTargetOnDisk)
		} else {
			onDiskBytes := diskFileToBytes(fileTargetOnDisk)
			if !bytes.Equal(onDiskBytes, packagedBytes) {
				// TODO only output the below line with verbose logging
				// fmt.Println(fileTargetOnDisk + " out of date, so writing...")
				writeBytesToDisk(packagedBytes, fileTargetOnDisk)
			}
		}
	}
}

func diskFileToBytes(file string) []byte {
	bytes, _ := ioutil.ReadFile(file)
	// TODO check error?
	return bytes
}

func getAssumedFilePerms(filePath string) os.FileMode {
	// Set all .sh files as +x when creating them
	// All users should be able to read and execute these files so users in containers can use them
	// XXX: Currently if you change these file permissions on disk files will need to be deleted and re added..
	if filepath.Ext(filePath) == ".sh" {
		return 0o755
	}
	return 0o655
}

func writeBytesToDisk(bytes []byte, filePath string) {
	ensureDirectoryForFileOnDisk(filePath)
	ioutil.WriteFile(filePath, bytes, getAssumedFilePerms(filePath))
	// TODO check error?
}

func ensureDirectoryForFileOnDisk(file string) {
	ensureDirectoryOnDisk(filepath.Dir(file))
}

func ensureDirectoryOnDisk(dirPath string) {
	if _, err := os.Stat(dirPath); err != nil {
		os.MkdirAll(dirPath, 0o755)
	}
}
