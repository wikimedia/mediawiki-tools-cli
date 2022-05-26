package mwdd

import (
	"os"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd/files"
)

/*MWDD representation of a mwdd v2 setup.*/
type MWDD struct {
	BaseDir string
}

var Context string

/*DefaultForUser returns the default mwdd working directory for the user.*/
func DefaultForUser() MWDD {
	return MWDD{
		BaseDir: mwddUserDirectory(),
	}
}

func mwddUserDirectory() string {
	return cli.UserDirectoryPathForCmd("mwdd")
}

/*Directory the directory containing the development environment.*/
func (m MWDD) Directory() string {
	return m.BaseDir + string(os.PathSeparator) + Context
}

/*EnsureReady ...*/
func (m MWDD) EnsureReady() {
	files.EnsureReady(m.Directory())
	m.Env().EnsureExists()
}
