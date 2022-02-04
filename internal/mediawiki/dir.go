package mediawiki

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
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
		// Check if the current directory looks like core
		_, checkError := ForDirectory(suggestedMwDir)
		if checkError == nil {
			// If it does look like core, break out of the loop
			break
		}
		// Otherwise, get the parent directory to try with
		suggestedMwDir = filepath.Dir(suggestedMwDir)
		if suggestedMwDir == "/" {
			// But if we reach the root level, then provide some sensible default
			suggestedMwDir = "~/git/gerrit/mediawiki/core"
			break
		}
	}

	return suggestedMwDir
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

/*CheckIfInCoreDirectory checks that the current working directory looks like a MediaWiki directory.*/
func CheckIfInCoreDirectory() {
	_, err := ForCurrentWorkingDirectory()
	if err != nil {
		logrus.Fatal("❌ Please run this command within the root of the MediaWiki core repository.")
	}
}

func errorIfDirectoryMissingGitReviewForProject(directory string, expectedProject string) error {
	b, err := ioutil.ReadFile(directory + string(os.PathSeparator) + ".gitreview")
	if err != nil || !strings.Contains(string(b), "project="+expectedProject) {
		return &NotMediaWikiDirectory{directory}
	}
	return nil
}

func errorIfDirectoryDoesNotLookLikeCore(directory string) error {
	return errorIfDirectoryMissingGitReviewForProject(directory, "mediawiki/core")
}
