// Package jsonfile reads and writes JSON objects on disk.
package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type JSONFile struct {
	FilePath string
	Contents map[string]interface{}
}

func (j JSONFile) FileName() string {
	return filepath.Base(j.FilePath)
}

func (j JSONFile) EnsureExists() error {
	if _, err := os.Stat(j.FilePath); !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(j.FilePath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(j.FilePath, []byte("{}"), 0o600)
}

func LoadFromDisk(path string) (JSONFile, error) {
	file := JSONFile{FilePath: path}
	if err := file.EnsureExists(); err != nil {
		return file, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return file, err
	}
	err = json.Unmarshal(data, &file.Contents)
	if err == nil && file.Contents == nil {
		err = fmt.Errorf("JSON file must contain an object")
	}
	return file, err
}

func (j JSONFile) WriteToDisk() error {
	data, err := json.Marshal(j.Contents)
	if err != nil {
		return err
	}
	return os.WriteFile(j.FilePath, append(data, '\n'), 0o600)
}

func (j JSONFile) String() (string, error) {
	data, err := json.MarshalIndent(j.Contents, "", "  ")
	return string(data), err
}

func (j *JSONFile) Clear() { j.Contents = map[string]interface{}{} }
