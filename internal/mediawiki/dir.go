package mediawiki

import (
	"os"
	"path/filepath"
	"strings"
)

/*NotMediaWikiDirectory error when a directory appears to not contain MediaWiki code.*/
type NotMediaWikiDirectory struct {
	directory string
}

func (e *NotMediaWikiDirectory) Error() string {
	return e.directory + " doesn't look like a MediaWiki directory"
}

/*GuessMediaWikiDirectoryBasedOnContext ...*/
func GuessMediaWikiDirectoryBasedOnContext() string {
	suggestedMwDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	for {
		if _, err := ForDirectory(suggestedMwDir); err == nil {
			return suggestedMwDir
		}
		parent := filepath.Dir(suggestedMwDir)
		if parent == suggestedMwDir {
			return "~/git/gerrit/mediawiki/core"
		}
		suggestedMwDir = parent
	}
}

/*ForDirectory returns a MediaWiki for the current working directory.*/
func ForDirectory(directory string) (MediaWiki, error) {
	return MediaWiki(directory), errorIfDirectoryDoesNotLookLikeCore(directory)
}

/*ForCurrentWorkingDirectory returns a MediaWiki for the current working directory.*/
func ForCurrentWorkingDirectory() (MediaWiki, error) {
	currentWorkingDirectory, _ := os.Getwd()
	return ForDirectory(currentWorkingDirectory)
}

func errorIfDirectoryMissingGitReviewForProject(directory string, expectedProject string) error {
	b, err := os.ReadFile(filepath.Clean(directory) + string(os.PathSeparator) + ".gitreview")
	if err != nil || !strings.Contains(string(b), "project="+expectedProject) {
		return &NotMediaWikiDirectory{directory}
	}
	return nil
}

func errorIfDirectoryDoesNotLookLikeCore(directory string) error {
	return errorIfDirectoryMissingGitReviewForProject(directory, "mediawiki/core")
}
