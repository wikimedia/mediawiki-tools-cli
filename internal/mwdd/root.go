package mwdd

import (
	"os"

	"gitlab.wikimedia.org/releng/cli/internal/cli"
	"gitlab.wikimedia.org/releng/cli/internal/util/embedsync"
)

/*MWDD representation of a mwdd v2 setup.*/
type MWDD string

/*DefaultForUser returns the default mwdd working directory for the user.*/
func DefaultForUser() MWDD {
	return MWDD(mwddUserDirectory() + string(os.PathSeparator) + cli.Context())
}

func mwddUserDirectory() string {
	return cli.UserDirectoryPathForCmd("mwdd")
}

/*Directory the directory containing the development environment.*/
func (m MWDD) Directory() string {
	return string(m)
}

/*EnsureReady ...*/
func (m MWDD) EnsureReady() {
	embedsync.EnsureReady(m.Directory())
	m.Env().EnsureExists()
}
