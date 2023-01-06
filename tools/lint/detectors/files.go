package detectors

import (
	"os"
	"path/filepath"
	"strings"
)

type FileIssue struct {
	File    string
	Code    string
	Text    string
	Context string
	Level   Level
}

type FileDetector struct {
	Path     string
	Function func(string) *FileIssue
}

func fileDetectorList() []FileDetector {
	return []FileDetector{
		// yml-extension: .yml extensions should be used for docker-compose files
		{
			Path: "internal/mwdd/files/embed",
			Function: func(file string) *FileIssue {
				if strings.HasSuffix(file, ".yaml") {
					return &FileIssue{
						File:    file,
						Level:   ErrorLevel,
						Code:    "yml-extension",
						Text:    "YAML files should use .yml extensions only",
						Context: file,
					}
				}
				return nil
			},
		},
	}
}

func DetectFileIssues(directory string) []FileIssue {
	issues := []FileIssue{}
	for _, detector := range fileDetectorList() {
		files := listFiles(detector.Path)
		for _, file := range files {
			issue := detector.Function(file)
			if issue != nil {
				issues = append(issues, *issue)
			}
		}
	}
	return issues
}

func listFiles(directory string) []string {
	var files []string

	// TODO recursive?
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return files
}
