// Package jsonfile is for interacting with a JSON file on disk.
package jsonfile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
)

/*JSONFile representation of a json file.*/
type JSONFile struct {
	FilePath string
	Contents map[string]interface{}
}

/*FileName the file name extracted from the full path.*/
func (j JSONFile) FileName() string {
	_, file := path.Split(j.FilePath)
	return file
}

/*EnsureExists makes sure the file exists on disk, will be empty json if created.*/
func (j JSONFile) EnsureExists() {
	if _, err := os.Stat(j.FilePath); err != nil {
		err := os.MkdirAll(strings.Replace(j.FilePath, j.FileName(), "", -1), 0o700)
		if err != nil {
			logrus.Fatal(err)
		}
		file, err := os.OpenFile(j.FilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			logrus.Fatal(err)
		}
		defer file.Close()
		w := bufio.NewWriter(file)
		_, err = w.WriteString("{}")
		if err != nil {
			logrus.Fatal(err)
		}
		err = w.Flush()
		if err != nil {
			logrus.Fatal(err)
		}
	}
}

/*LoadFromDisk loads the config.json from disk.*/
func LoadFromDisk(filePath string) JSONFile {
	file := JSONFile{
		FilePath: filePath,
	}
	file.EnsureExists()

	openedFile, err := os.Open(file.FilePath)
	if err != nil {
		fmt.Println(err.Error())
	}
	defer openedFile.Close()
	jsonParser := json.NewDecoder(openedFile)
	jsonErr := jsonParser.Decode(&file.Contents)
	if jsonErr != nil {
		logrus.Fatal(jsonErr)
	}
	return file
}

/*WriteToDisk writers the config to disk.*/
func (j JSONFile) WriteToDisk() {
	file, err := os.OpenFile(j.FilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		logrus.Fatal(err)
	}
	defer file.Close()
	w := bufio.NewWriter(file)
	jsonEncoder := json.NewEncoder(w)
	jsonErr := jsonEncoder.Encode(j.Contents)
	if jsonErr != nil {
		logrus.Fatal(jsonErr)
	}
	flushErr := w.Flush()
	if flushErr != nil {
		logrus.Fatal(flushErr)
	}
}

/*PrettyPrint outputs the current config as a pretty string.*/
func (j JSONFile) PrettyPrint() {
	fmt.Printf("%s\n", j.String())
}

/*String current content as a string.*/
func (j JSONFile) String() string {
	empJSON, err := json.MarshalIndent(j.Contents, "", "  ")
	if err != nil {
		logrus.Fatalf(err.Error())
	}
	return string(empJSON)
}

/*Clear the contents of the file, setting it to an empty json object.*/
func (j *JSONFile) Clear() {
	j.Contents = map[string]interface{}{}
}
