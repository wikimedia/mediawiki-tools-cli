/*
Package embedsync deals with syncing go embedded files onto the system disk

NOTE: this requires an index of the files to be part of the embed.
This can be generated in the MakeFile using a line like this...

@cd ./mount/dev/ && find . -type f -not -name '*_test.go' | sort > files.txt
*/
package embedsync

import (
	"bytes"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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

func (e EmbeddingDiskSync) EnsureFilesOnDisk() error {
	embeddedFiles, err := e.embeddedFiles()
	if err != nil {
		return err
	}
	for _, embedFile := range embeddedFiles {
		name := e.agnosticFileFromEmbed(embedFile)
		data, err := e.Embed.ReadFile(embedFile)
		if err != nil {
			return err
		}
		diskFile := filepath.Join(e.DiskPath, name)
		info, err := os.Stat(diskFile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.IsDir() {
			if err = os.RemoveAll(diskFile); err != nil {
				return err
			}
		}
		previous, readErr := os.ReadFile(diskFile)
		if readErr != nil && !os.IsNotExist(readErr) {
			return readErr
		}
		if !bytes.Equal(previous, data) || readErr != nil {
			if err = os.MkdirAll(filepath.Dir(diskFile), 0o755); err != nil {
				return err
			}
			if err = os.WriteFile(diskFile, data, getAssumedFilePerms(diskFile)); err != nil {
				return err
			}
		}
		if err = os.Chmod(diskFile, getAssumedFilePerms(diskFile)); err != nil {
			return err
		}
	}
	return nil
}

func (e EmbeddingDiskSync) EnsureNoExtraFilesOnDisk() error {
	embedded, err := e.embeddedFiles()
	if err != nil {
		return err
	}
	return filepath.WalkDir(e.DiskPath, func(diskFile string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		name := e.agnosticFileFromDisk(diskFile)
		if !stringsutil.StringInSlice(e.EmbedPath+embedSeperator+name, embedded) && !stringsutil.StringInRegexSlice(name, e.IgnoreFiles) {
			return os.Remove(diskFile)
		}
		return nil
	})
}

func (e EmbeddingDiskSync) embeddedFiles() ([]string, error) {
	index, err := e.Embed.ReadFile(e.EmbedPath + embedSeperator + "files.txt")
	if err != nil {
		return nil, err
	}
	return stringsutil.ReplaceInAll(strings.Split(strings.Trim(string(index), "\n"), "\n"), "./", e.EmbedPath+embedSeperator), nil
}

func (e EmbeddingDiskSync) agnosticFileFromEmbed(name string) string {
	return strings.TrimPrefix(name, e.EmbedPath+embedSeperator)
}

// agnosticFileFromDisk takes an on disk path and returns the agnostic path
// Example on Linux "/home/adam/.config/mwcli/mwdd/default/compose/shellbox-timeline/compose.yml" => "dev/compose/shellbox-timeline/compose.yml".
func (e EmbeddingDiskSync) agnosticFileFromDisk(name string) string {
	path := strings.TrimPrefix(name, e.DiskPath+string(os.PathSeparator))
	// As the input is a disk path, we also need to normalize the separator to the one used by embeds
	return strings.Replace(path, string(os.PathSeparator), embedSeperator, -1)
}

func getAssumedFilePerms(filePath string) os.FileMode {
	if filepath.Ext(filePath) == ".sh" {
		// All users should be able to read and execute these files so users in containers can use them
		return 0o755
	}
	// All users should be able to read these files so users in containers can use them
	return 0o644
}
