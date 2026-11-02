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

func (m MediaWiki) VendorDirectoryIsPresent() bool {
	vendorDir := m.Path("vendor")
	_, err := os.Stat(vendorDir)
	return err == nil
}

func errorIfDirectoryDoesNotLookLikeVector(directory string) error {
	return errorIfDirectoryMissingGitReviewForProject(directory, "mediawiki/skins/Vector")
}

/*VectorIsPresent ...*/
func (m MediaWiki) VectorIsPresent() bool {
	return errorIfDirectoryDoesNotLookLikeVector(m.Path("skins/Vector")) == nil
}

func (m MediaWiki) ComposerLocalJsonPath() string {
	return m.Path("composer.local.json")
}

func (m MediaWiki) ComposerLocalJsonExists() bool {
	_, err := os.Stat(m.ComposerLocalJsonPath())
	return !os.IsNotExist(err)
}

func (m MediaWiki) ComposerJsonPath() string {
	return m.Path("composer.json")
}

func (m MediaWiki) ComposerJsonExists() bool {
	_, err := os.Stat(m.ComposerJsonPath())
	return !os.IsNotExist(err)
}

func (m MediaWiki) LocalSettingsPath() string {
	return m.Path("LocalSettings.php")
}

func (m MediaWiki) LocalSettingsContents() (string, error) {
	bytes, err := os.ReadFile(m.LocalSettingsPath())
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (m MediaWiki) ExtensionsCheckedOut() ([]string, error) {
	return directoriesInDirectory(m.Path("extensions"))
}

func (m MediaWiki) SkinsCheckedOut() ([]string, error) {
	return directoriesInDirectory(m.Path("skins"))
}

func directoriesInDirectory(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	directories := []string{}
	for _, e := range entries {
		if e.IsDir() {
			directories = append(directories, e.Name())
		}
	}
	return directories, nil
}
