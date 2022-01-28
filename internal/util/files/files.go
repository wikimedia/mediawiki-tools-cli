package files

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"strings"
)

/*AddLinesUnique adds all lines to the file if each one will be the only occourance of the string.*/
func AddLinesUnique(lines []string, filename string) {
	for _, line := range lines {
		AddLineUnique(line, filename)
	}
}

/*AddLineUnique adds the line to the file if it will be the only occourance of the string.*/
func AddLineUnique(line string, fileName string) {
	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(file)
	s := buf.String()

	if !strings.Contains(s, line) {
		if _, err := file.WriteString(line + "\n"); err != nil {
			panic(err)
		}
	}
}

func AddLine(line string, fileName string) {
	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	if _, err := file.WriteString(line + "\n"); err != nil {
		panic(err)
	}
}

/*Lines reads all lines from a file.*/
func Lines(fileName string) []string {
	_, err := os.Stat(fileName)
	if err != nil {
		return []string{}
	}
	file, err := os.Open(fileName)
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
	bytes, err := ioutil.ReadFile(fileName)
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
		os.Remove(fileName)
	}
}
