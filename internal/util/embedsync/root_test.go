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
	"embed"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

//go:embed testembed
var testContent embed.FS

func checkFileContent(t *testing.T, file string, expected string) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != expected {
		t.Errorf("Expected %s, got %s", expected, string(content))
	}
}

func writeFileContent(t *testing.T, file string, content string) {
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.WriteString(content)
}

func TestEmbeddingDiskSync_EnsureFilesOnDisk(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestEmbeddingDiskSync_EnsureFilesOnDisk")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	e := EmbeddingDiskSync{
		Embed:     testContent,
		EmbedPath: "testembed",
		DiskPath:  dir,
	}

	t.Run("Ensure files are actually on disk and look correct", func(t *testing.T) {
		e.EnsureFilesOnDisk()

		files, err := ioutil.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}

		// Check count of files
		if len(files) != 3 {
			t.Errorf("Expected 3 files, got %d", len(files))
		}

		// Check file contents
		checkFileContent(t, dir+"/testfile1", "foo")
		checkFileContent(t, dir+"/testfile2.txt", "bar")
		checkFileContent(t, dir+"/adir/test3", "hi")
	})

	t.Run("Ensure files overwritten if changed", func(t *testing.T) {
		e.EnsureFilesOnDisk()

		writeFileContent(t, dir+"/testfile1", "changed")
		checkFileContent(t, dir+"/testfile1", "changed")

		e.EnsureFilesOnDisk()

		checkFileContent(t, dir+"/testfile1", "foo")
	})
}

func TestEmbeddingDiskSync_EnsureNoExtraFilesOnDisk(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestEmbeddingDiskSync_EnsureNoExtraFilesOnDisk")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	e := EmbeddingDiskSync{
		Embed:     testContent,
		EmbedPath: "testembed",
		DiskPath:  dir,
		IgnoreFiles: []string{
			"ignoreme",
		},
	}

	e.EnsureFilesOnDisk()

	writeFileContent(t, dir+"/ignoreme", "ignored")
	writeFileContent(t, dir+"/deleteme", "gone")

	e.EnsureNoExtraFilesOnDisk()

	checkFileContent(t, dir+"/ignoreme", "ignored")

	_, err = os.Stat(dir + "/deleteme")
	if !errors.Is(err, os.ErrNotExist) {
		t.Error("Expected file to be deleted", err)
	}
}
