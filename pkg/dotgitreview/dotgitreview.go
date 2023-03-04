package dotgitreview

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gopkg.in/ini.v1"
)

// GitReview represents a .gitreview file.
type GitReview struct {
	Host       string
	Port       string
	Project    string
	RawProject string
}

// ForCWD returns the GitReview file for the current working directory.
func ForCWD() (*GitReview, error) {
	dir, _ := os.Getwd()
	return ForDirectory(dir)
}

// ForDirectory returns the GitReview file for the given directory.
func ForDirectory(dir string) (*GitReview, error) {
	mwdd.DefaultForUser()
	for {
		if _, err := os.Stat(dir + "/.gitreview"); os.IsNotExist(err) {
			dir = filepath.Dir(dir)
		} else {
			break
		}
		if dir == "/" {
			return nil, errors.New("not in a Gerrit repository")
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
