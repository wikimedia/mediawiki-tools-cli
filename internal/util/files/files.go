package files

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

/*AddLinesUnique adds all lines to the file if each one will be the only occurrence of the string.*/
func AddLinesUnique(lines []string, filename string) {
	for _, line := range lines {
		AddLineUnique(line, filename)
	}
}

/*AddLineUnique adds the line to the file if it will be the only occurrence of the string.*/
func AddLineUnique(line string, fileName string) {
	file, err := os.OpenFile(filepath.Clean(fileName), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	buf := new(bytes.Buffer)
	_, bufErr := buf.ReadFrom(file)
	if bufErr != nil {
		panic(bufErr)
	}
	s := buf.String()

	if !strings.Contains(s, line) {
		if _, err := file.WriteString(line + "\n"); err != nil {
			panic(err)
		}
	}
}

func AddLine(line string, fileName string) {
	file, err := os.OpenFile(filepath.Clean(fileName), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	if _, err := file.WriteString(line + "\n"); err != nil {
		panic(err)
	}
}

func RemoveAllLinesMatching(line string, fileName string) {
	file, err := os.OpenFile(filepath.Clean(fileName), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	buf := new(bytes.Buffer)
	_, bufErr := buf.ReadFrom(file)
	if bufErr != nil {
		panic(bufErr)
	}
	s := buf.String()
	s = strings.ReplaceAll(s, line+"\n", "")
	truncErr := file.Truncate(0)
	if truncErr != nil {
		panic(truncErr)
	}
	_, seekErr := file.Seek(0, 0)
	if seekErr != nil {
		panic(seekErr)
	}
	_, writeErr := file.WriteString(s)
	if writeErr != nil {
		panic(writeErr)
	}
}

/*Lines reads all lines from a file.*/
func Lines(fileName string) []string {
	_, err := os.Stat(fileName)
	if err != nil {
		return []string{}
	}
	file, err := os.Open(filepath.Clean(fileName))
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

/*Bytes gets bytes of a file or panics.*/
func Bytes(fileName string) []byte {
	bytes, err := os.ReadFile(filepath.Clean(fileName))
	if err != nil {
		panic(err)
	}
	return bytes
}

func Exists(fileName string) bool {
	_, err := os.Stat(fileName)
	return !os.IsNotExist(err)
}

func RemoveIfExists(fileName string) {
	if Exists(fileName) {
		err := os.Remove(fileName)
		if err != nil {
			panic(err)
		}
	}
}
