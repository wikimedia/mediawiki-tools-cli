package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Data struct {
	Images []struct {
		Image        string `yaml:"image"`
		RequireRegex string `yaml:"requireRegex,omitempty"`
		NoCheck      bool   `yaml:"noCheck,omitempty"`
	} `yaml:"images"`
	Directories []string `yaml:"directories"`
	Files       []string `yaml:"files"`
}

func main() {
	while := true
	for i := 1; while; i += 2 {
		if len(os.Args) < i+2 {
			while = false
			continue
		}
		find := os.Args[i]
		replace := os.Args[i+1]
		data := getData()

		fmt.Println("Replacing: " + find)
		fmt.Println("With: " + replace)
		fmt.Println("In files and directories defined in data.yml")

		for _, v := range data.Directories {
			replaceInDirectory(v, find, replace)
		}
		for _, v := range data.Files {
			replaceInFile(v, find, replace)
		}
		replaceInFile("tools/image-update/data.yml", find, replace)
	}
}

func replaceInDirectory(dirPath string, find string, replace string) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if file.IsDir() {
			replaceInDirectory(dirPath+"/"+file.Name(), find, replace)
			continue
		}
		replaceInFile(dirPath+"/"+file.Name(), find, replace)
	}
}

func replaceInFile(filePath string, find string, replace string) {
	fileContent, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		logrus.Printf("os.ReadFile err   #%v ", err)
	}

	text := string(fileContent)
	newText := strings.ReplaceAll(text, find, replace)

	if text != newText {
		err := os.WriteFile(filePath, []byte(newText), 0o755) // #nosec G306
		if err != nil {
			logrus.Printf("ioutil.WriteFile err   #%v ", err)
		}
		fmt.Println("Updated " + filePath)
	}
}

func getData() Data {
	var data Data
	content, err := os.ReadFile("tools/image-update/data.yml")
	if err != nil {
		panic(err.Error())
	}
	err = yaml.Unmarshal(content, &data)
	if err != nil {
		logrus.Fatal("Failed to parse data file ", err)
	}
	return data
}
