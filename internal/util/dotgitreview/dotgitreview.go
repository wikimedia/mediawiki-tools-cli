package dotgitreview

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

type GitReview struct {
	Host       string
	Port       string
	Project    string
	RawProject string
}

func ForCWD() (*GitReview, error) {
	dir, _ := os.Getwd()
	return ForDirectory(dir)
}

func ForDirectory(dir string) (*GitReview, error) {
	for {
		if _, err := os.Stat(dir + "/.gitreview"); os.IsNotExist(err) {
			dir = filepath.Dir(dir)
		} else {
			break
		}
		if dir == "/" {
			return nil, errors.New("not in a Wikimedia Gerrit repository")
		}
	}

	// Ignore error, as it only happens if the file doesnt exist, and we check that
	gitReviewFile, _ := ini.Load(dir + "/.gitreview")

	gitReview := &GitReview{
		Host:       gitReviewFile.Section("gerrit").Key("host").String(),
		Port:       gitReviewFile.Section("gerrit").Key("port").String(),
		RawProject: gitReviewFile.Section("gerrit").Key("project").String(),
		Project:    strings.TrimSuffix(gitReviewFile.Section("gerrit").Key("project").String(), ".git"),
	}

	return gitReview, nil
}
