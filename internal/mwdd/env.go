package mwdd

import (
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/dotenv"
)

/*Env ...*/
func (m MWDD) Env() dotenv.File {
	return dotenv.FileForDirectory(m.Directory())
}
