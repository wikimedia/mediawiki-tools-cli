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
	syncer := embedsync.Syncer(m.Directory(), []string{
		// Used by docker-compose to store current environment variables in
		".env",
		// Used by the dev environment to store hosts that need adding to the hosts file
		"record-hosts",
		// Used by folks that want to define a custom set of docker-compose services
		"custom.yml",
	})

	syncer.EnsureFilesOnDisk()
	syncer.EnsureNoExtraFilesOnDisk()
	m.Env().EnsureExists()
}
