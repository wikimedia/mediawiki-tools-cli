/*
Package embedsync deals with syncing go embedded files onto the system disk

NOTE: this requires an index of the files to be part of the embed.
This can be generated in the MakeFile using a line like this...

@cd ./internal/mwdd/files/embed/ && find . -type f | sort > files.txt
*/
package embedsync

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/dirs"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/files"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

type EmbeddingDiskSync struct {
	Embed       embed.FS
	EmbedPath   string
	DiskPath    string
	IgnoreFiles []string
}

// DO NOT use os.PathSeparator, embeds ALWAYS uses "/".
var embedSeperator = "/"

func (e EmbeddingDiskSync) EnsureFilesOnDisk() {
	logrus.Trace("embedsync.EnsureFilesOnDisk")
	embeddedFiles := e.embeddedFiles()

	// Ensure each file is on disk and up to date
	for _, embedFile := range embeddedFiles {
		logrus.Trace("Checking " + embedFile)
		agnosticFile := e.agnosticFileFromEmbed(embedFile)
		diskFile := e.DiskPath + string(os.PathSeparator) + agnosticFile
		embedBytes := e.agnosticEmbedBytes(agnosticFile)

		if _, err := os.Stat(diskFile); os.IsNotExist(err) {
			logrus.Trace(diskFile + " doesn't exist, so write it...")
			writeBytesToDisk(embedBytes, diskFile)
		} else if !bytes.Equal(files.Bytes(diskFile), embedBytes) {
			logrus.Trace(diskFile + " has different byte count, so write it...")
			writeBytesToDisk(embedBytes, diskFile)
		} else {
			stats, _ := os.Stat(diskFile)
			if stats.Mode() != getAssumedFilePerms(diskFile) {
				logrus.Trace(diskFile + " has different permissions, so set correct permissions...")
				err := os.Chmod(diskFile, getAssumedFilePerms(diskFile))
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	}
}

func (e EmbeddingDiskSync) EnsureNoExtraFilesOnDisk() {
	logrus.Trace("embedsync.EnsureNoExtraFilesOnDisk")
	diskFiles := e.diskFiles()
	embeddedFiles := e.embeddedFiles()

	for _, diskFile := range diskFiles {
		agnosticFile := e.agnosticFileFromDisk(diskFile)
		embedFile := e.EmbedPath + embedSeperator + agnosticFile
		if !stringsutil.StringInSlice(embedFile, embeddedFiles) && !stringsutil.StringInRegexSlice(agnosticFile, e.IgnoreFiles) {
			logrus.Trace(diskFile + " no longer needed, so removing")
			err := os.Remove(diskFile)
			if err != nil {
				fmt.Println("Failed to remove file: " + diskFile)
				fmt.Println(err)
				panic(err)
			}
		}
	}
}

func (e EmbeddingDiskSync) embeddedFiles() []string {
	// "./" switched for EmbedPath as from content of files.txt
	return stringsutil.ReplaceInAll(strings.Split(strings.Trim(e.indexString(), "\n"), "\n"), "./", e.EmbedPath+embedSeperator)
}

func (e EmbeddingDiskSync) diskFiles() []string {
	return dirs.FilesIn(e.DiskPath)
}

// indexString returns the contents of the files.txt file in the embed.
func (e EmbeddingDiskSync) indexString() string {
	return e.fileString("files.txt")
}

func (e EmbeddingDiskSync) fileString(name string) string {
	fileReader := e.fileReaderOrExit(name)
	buf := bytes.NewBuffer(nil)
	_, ioErr := io.Copy(buf, fileReader)
	if ioErr != nil {
		fmt.Println(ioErr)
	}
	err := fileReader.Close()
	if err != nil {
		fmt.Println("Failed to close file: " + name)
		fmt.Println(err)
		panic(err)
	}
	return buf.String()
}

func (e EmbeddingDiskSync) agnosticEmbedBytes(name string) []byte {
	fileReader := e.fileReaderOrExit(name)
	bytes, _ := io.ReadAll(fileReader)
	return bytes
}

func (e EmbeddingDiskSync) fileReaderOrExit(name string) fs.File {
	innerName := e.EmbedPath + embedSeperator + name
	fileReader, err := e.Embed.Open(innerName)
	if err != nil {
		fmt.Println("Failed to open file: " + innerName)
		fmt.Println(err)
		panic(err)
	}
	return fileReader
}

func (e EmbeddingDiskSync) agnosticFileFromEmbed(name string) string {
	return strings.TrimPrefix(name, e.EmbedPath+embedSeperator)
}

// agnosticFileFromDisk takes an on disk path and returns the agnostic path
// Example on Linux "/home/adam/.config/mwcli/mwdd/default/shellbox-timeline.yml" => "embed/shellbox-timeline.yml".
func (e EmbeddingDiskSync) agnosticFileFromDisk(name string) string {
	path := strings.TrimPrefix(name, e.DiskPath+string(os.PathSeparator))
	// As the input is a disk path, we also need to normalize the separator to the one used by embeds
	return strings.Replace(path, string(os.PathSeparator), embedSeperator, -1)
}

func writeBytesToDisk(bytes []byte, file string) {
	dirs.EnsureExists(filepath.Dir(file))
	err := os.WriteFile(file, bytes, getAssumedFilePerms(file))
	if err != nil {
		fmt.Println("Failed to write file: " + file)
		fmt.Println(err)
		panic(err)
	}
}

func getAssumedFilePerms(filePath string) os.FileMode {
	if filepath.Ext(filePath) == ".sh" {
		// All users should be able to read and execute these files so users in containers can use them
		return 0o755
	}
	// All users should be able to read these files so users in containers can use them
	return 0o644
}
