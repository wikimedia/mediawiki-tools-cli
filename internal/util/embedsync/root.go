/*Package embedsync deals with syncing go embeded files onto the systme disk

NOTE: this requires an index of the files to be part of the embed.
This can be generated in the MakeFile using a line like this...

@cd ./internal/mwdd/files/embed/ && find . -type f > files.txt

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
	Embed     embed.FS
	EmbedPath string
	DiskPath  string
}

func (e EmbeddingDiskSync) EnsureFilesOnDisk() {
	embededFiles := e.files()

	// Ensure each file is on disk and up to date
	for _, embedFileName := range embededFiles {
		strippedFileName := e.strippedFileName(embedFileName)
		fileTargetOnDisk := e.DiskPath + string(os.PathSeparator) + strippedFileName
		embededBytes := e.fileBytes(strippedFileName)

		if _, err := os.Stat(fileTargetOnDisk); os.IsNotExist(err) {
			// TODO only output the below line with verbose logging
			// fmt.Println(fileTargetOnDisk + " doesn't exist, so write it...")
			writeBytesToDisk(embededBytes, fileTargetOnDisk)
		} else {
			onDiskBytes := files.Bytes(fileTargetOnDisk)
			if !bytes.Equal(onDiskBytes, embededBytes) {
				// TODO only output the below line with verbose logging
				// fmt.Println(fileTargetOnDisk + " out of date, so writing...")
				writeBytesToDisk(embededBytes, fileTargetOnDisk)
			}
		}
	}
}

func (e EmbeddingDiskSync) EnsureNoExtraFilesOnDisk() {
	// TODO https://phabricator.wikimedia.org/T282361
	panic("not implemented")
}

func (e EmbeddingDiskSync) files() []string {
	// "./" switched for EmbedPath as from content of files.txt
	return utilstrings.ReplaceInAll(strings.Split(strings.Trim(e.indexString(), "\n"), "\n"), "./", e.EmbedPath+string(os.PathSeparator))
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

func (e EmbeddingDiskSync) fileBytes(name string) []byte {
	fileReader := e.fileReaderOrExit(name)
	bytes, _ := ioutil.ReadAll(fileReader)
	return bytes
}

func (e EmbeddingDiskSync) fileReaderOrExit(name string) fs.File {
	innerName := e.EmbedPath + string(os.PathSeparator) + name
	fileReader, err := e.Embed.Open(innerName)
	if err != nil {
		fmt.Println("Failed to open file: " + innerName)
		fmt.Println(err)
		panic(err)
	}
	return fileReader
}

func (e EmbeddingDiskSync) strippedFileName(name string) string {
	return strings.TrimPrefix(name, e.EmbedPath+string(os.PathSeparator))
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
