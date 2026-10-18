package devenv

import (
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotenv"
)

/*Env ...*/
func (m Environment) Env() dotenv.File {
	return dotenv.FileForDirectory(m.Directory())
}
