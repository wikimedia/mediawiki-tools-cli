/*Package embedsync deals with syncing go embeded files onto the systme disk

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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/util/dirs"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/files"
	utilstrings "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

type EmbeddingDiskSync struct {
	Embed       embed.FS
	EmbedPath   string
	DiskPath    string
	IgnoreFiles []string
}

// DO NOT use os.PathSeparator, embeds ALWAYS uses "/"
var embedSeperator = "/"

func (e EmbeddingDiskSync) EnsureFilesOnDisk() {
	embededFiles := e.embededFiles()

	// Ensure each file is on disk and up to date
	for _, embedFile := range embededFiles {
		agnosticFile := e.agnosticFileFromEmbed(embedFile)
		diskFile := e.DiskPath + string(os.PathSeparator) + agnosticFile
		embedBytes := e.agnosticEmbedBytes(agnosticFile)

		if _, err := os.Stat(diskFile); os.IsNotExist(err) {
			// TODO only output the below line with verbose logging
			// fmt.Println(diskFile + " doesn't exist, so write it...")
			writeBytesToDisk(embedBytes, diskFile)
		} else {
			diskBytes := files.Bytes(diskFile)
			if !bytes.Equal(diskBytes, embedBytes) {
				// TODO only output the below line with verbose logging
				// fmt.Println(diskFile + " out of date, so writing...")
				writeBytesToDisk(embedBytes, diskFile)
			}
		}
	}
}

func (e EmbeddingDiskSync) EnsureNoExtraFilesOnDisk() {
	diskFiles := e.diskFiles()
	embededFiles := e.embededFiles()

	for _, diskFile := range diskFiles {
		agnosticFile := e.agnosticFileFromDisk(diskFile)
		embedFile := e.EmbedPath + embedSeperator + agnosticFile
		if !utilstrings.StringInSlice(embedFile, embededFiles) && !utilstrings.StringInSlice(agnosticFile, e.IgnoreFiles) {
			// TODO only output the below line with verbose logging
			// fmt.Println(diskFile + " no longer needed, so removing")
			err := os.Remove(diskFile)
			if err != nil {
				fmt.Println("Failed to remove file: " + diskFile)
				fmt.Println(err)
				panic(err)
			}
		}
	}
}

func (e EmbeddingDiskSync) embededFiles() []string {
	// "./" switched for EmbedPath as from content of files.txt
	return utilstrings.ReplaceInAll(strings.Split(strings.Trim(e.indexString(), "\n"), "\n"), "./", e.EmbedPath+embedSeperator)
}

func (e EmbeddingDiskSync) diskFiles() []string {
	return dirs.FilesIn(e.DiskPath)
}

// indexString returns the contents of the files.txt file in the embed
func (e EmbeddingDiskSync) indexString() string {
	return e.fileString("files.txt")
}

func (e EmbeddingDiskSync) fileString(name string) string {
	fileReader := e.fileReaderOrExit(name)
	buf := bytes.NewBuffer(nil)
	io.Copy(buf, fileReader)
	fileReader.Close()
	return buf.String()
}

func (e EmbeddingDiskSync) agnosticEmbedBytes(name string) []byte {
	fileReader := e.fileReaderOrExit(name)
	bytes, _ := ioutil.ReadAll(fileReader)
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
// On Linux "/home/adam/.mwcli/mwdd/default/shellbox-timeline.yml" => "embed/shellbox-timeline.yml"
// On Windows "C:\Users\adam\.mwcli\mwdd\default\shellbox-timeline.yml" => "embed/shellbox-timeline.yml"
func (e EmbeddingDiskSync) agnosticFileFromDisk(name string) string {
	path := strings.TrimPrefix(name, e.DiskPath+string(os.PathSeparator))
	// As the input is a disk path, we also need to normalize the seperator to the one used by embeds
	return strings.Replace(path, string(os.PathSeparator), embedSeperator, -1)
}

func writeBytesToDisk(bytes []byte, file string) {
	dirs.EnsureExists(filepath.Dir(file))
	ioutil.WriteFile(file, bytes, getAssumedFilePerms(file))
	// TODO check error?
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
