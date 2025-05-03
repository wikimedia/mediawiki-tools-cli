package mediawiki

import (
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotenv"
)

/*Env ...*/
func (m MediaWiki) Env() dotenv.File {
	return dotenv.FileForDirectory(m.Directory())
}
