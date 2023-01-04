package mediawiki

import (
	"os"
)

/*MediaWiki representation of a MediaWiki install directory.*/
type MediaWiki string

/*Directory the directory containing MediaWiki.*/
func (m MediaWiki) Directory() string {
	return string(m)
}

/*Path within the MediaWiki directory.*/
func (m MediaWiki) Path(subPath string) string {
	return m.Directory() + string(os.PathSeparator) + subPath
}

/*MediaWikiIsPresent ...*/
func (m MediaWiki) MediaWikiIsPresent() bool {
	return errorIfDirectoryDoesNotLookLikeCore(m.Directory()) == nil
}

func errorIfDirectoryDoesNotLookLikeVector(directory string) error {
	return errorIfDirectoryMissingGitReviewForProject(directory, "mediawiki/skins/Vector")
}

/*VectorIsPresent ...*/
func (m MediaWiki) VectorIsPresent() bool {
	return errorIfDirectoryDoesNotLookLikeVector(m.Path("skins/Vector")) == nil
}
