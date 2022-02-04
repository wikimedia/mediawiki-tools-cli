package dotgitreview

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
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

	gitReviewFile, err := ini.Load(dir + "/.gitreview")
	if err != nil {
		logrus.Fatal(err)
	}

	gitReview := &GitReview{
		Host:       gitReviewFile.Section("gerrit").Key("host").String(),
		Port:       gitReviewFile.Section("gerrit").Key("port").String(),
		RawProject: gitReviewFile.Section("gerrit").Key("project").String(),
		Project:    strings.TrimSuffix(gitReviewFile.Section("gerrit").Key("project").String(), ".git"),
	}

	return gitReview, nil
}
