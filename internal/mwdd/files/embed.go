package files

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"strings"
)

//go:embed embed
var content embed.FS

func strippedFileName(name string) string {
	return strings.TrimPrefix(name, "embed/")
}

func files() []string {
	return replaceInAllStrings(strings.Split(strings.Trim(indexString(), "\n"), "\n"), "./", "embed/")
}

func fileBytes(name string) []byte {
	fileReader := fileReaderOrExit(name)
	bytes, _ := ioutil.ReadAll(fileReader)
	return bytes
}

func fileString(name string) string {
	fileReader := fileReaderOrExit(name)
	buf := bytes.NewBuffer(nil)
	io.Copy(buf, fileReader)
	fileReader.Close()
	return buf.String()
}

func fileReaderOrExit(name string) fs.File {
	fileReader, err := content.Open(name)
	if err != nil {
		fmt.Println("Failed to open file: " + name)
		fmt.Println(err)
		panic(err)
	}
	return fileReader
}

func replaceInAllStrings(list []string, find string, replace string) []string {
	for i, s := range list {
		list[i] = strings.Replace(s, find, replace, -1)
	}
	return list
}

func indexString() string {
	return fileString("embed/files.txt")
}
