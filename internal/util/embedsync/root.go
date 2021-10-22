/*Package embedsync deals with syncing go embeded files onto the systme disk

NOTE: this requires an index of the files to be part of the embed.
This can be generated in the MakeFile using a line like this...

@cd ./internal/mwdd/files/embed/ && find . -type f | sort > files.txt

Copyright © 2021 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
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

	"gitlab.wikimedia.org/releng/cli/internal/util/dirs"
	"gitlab.wikimedia.org/releng/cli/internal/util/files"
	utilstrings "gitlab.wikimedia.org/releng/cli/internal/util/strings"
)

type EmbeddingDiskSync struct {
	Embed       embed.FS
	EmbedPath   string
	DiskPath    string
	IgnoreFiles []string
}

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
		// Must be os.PathSeparator for cross platform usage, as this is used to compare against actual embededFiles on disk
		embedFile := e.EmbedPath + string(os.PathSeparator) + agnosticFile
		if !utilstrings.StringInSlice(embedFile, embededFiles) && !utilstrings.StringInSlice(agnosticFile, e.IgnoreFiles) {
			// TODO only output the below line with verbose logging
			// fmt.Println(diskFile + " no logner needed, so removing")
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
	return utilstrings.ReplaceInAll(strings.Split(strings.Trim(e.indexString(), "\n"), "\n"), "./", e.EmbedPath+string(os.PathSeparator))
}

func (e EmbeddingDiskSync) diskFiles() []string {
	return getFilesInDirectory(e.DiskPath)
}

func getFilesInDirectory(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	var files []string
	for _, entry := range entries {
		fullPath := dir + string(os.PathSeparator) + entry.Name()
		if entry.IsDir() {
			files = append(files, getFilesInDirectory(fullPath)...)
			continue
		}
		files = append(files, fullPath)
	}
	return files
}

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
	// `/` is used here, as we are reading from the golang embed, which will always use `/`
	innerName := e.EmbedPath + "/" + name
	fileReader, err := e.Embed.Open(innerName)
	if err != nil {
		fmt.Println("Failed to open file: " + innerName)
		fmt.Println(err)
		panic(err)
	}
	return fileReader
}

func (e EmbeddingDiskSync) agnosticFileFromEmbed(name string) string {
	return strings.TrimPrefix(name, e.EmbedPath+string(os.PathSeparator))
}

func (e EmbeddingDiskSync) agnosticFileFromDisk(name string) string {
	return strings.TrimPrefix(name, e.DiskPath+string(os.PathSeparator))
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
