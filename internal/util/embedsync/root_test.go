/*Package embedsync deals with syncing go embedded files onto the system disk

NOTE: this requires an index of the files to be part of the embed.
This can be generated in the MakeFile using a line like this...

@cd ./internal/mwdd/files/embed/ && find . -type f | sort > files.txt
*/
package embedsync

import (
	"embed"
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
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
		logrus.Fatal(err)
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
		logrus.Fatal(err)
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
