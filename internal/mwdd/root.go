/*Package mwdd is used to interact a mwdd v2 setup

Copyright © 2020 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package mwdd

import (
	"os"
	"os/user"

	"gitlab.wikimedia.org/releng/cli/internal/mwdd/files"
)

/*MWDD representation of a mwdd v2 setup.*/
type MWDD string

func mwddContext() string {
	_, inGitlabCi := os.LookupEnv("GITLAB_CI")
	if !inGitlabCi && os.Getenv("MWCLI_CONTEXT_TEST") != "" {
		return "test"
	}
	return "default"
}

/*DefaultForUser returns the default mwdd working directory for the user.*/
func DefaultForUser() MWDD {
	return MWDD(mwddUserDirectory() + string(os.PathSeparator) + mwddContext())
}

func mwddUserDirectory() string {
	// user home dir can not be used in Gitlab CI, must use the project dir instead!
	// https://medium.com/@patrick.winters/mounting-volumes-in-sibling-containers-with-gitlab-ci-534e5edc4035
	// TODO maybe this should be pushed further up and the whole mwcli dir should be moved?!
	_, inGitlabCi := os.LookupEnv("GITLAB_CI")
	if inGitlabCi {
		ciDir, _ := os.LookupEnv("CI_PROJECT_DIR")
		return ciDir + ".mwcli/mwdd"
	}

	currentUser, err := user.Current()
	if err != nil {
		panic(err)
	}

	// If we are root, check to see if we can detect sudo being used
	if currentUser.Uid == "0" {
		sudoUID := os.Getenv("SUDO_UID")
		if sudoUID == "" {
			panic("detected sudo but no SUDO_UID")
		}
		currentUser, err = user.LookupId(sudoUID)
		if err != nil {
			panic(err)
		}
	}

	projectDirectory := currentUser.HomeDir + string(os.PathSeparator) + ".mwcli/mwdd"
	return projectDirectory
}

/*Directory the directory containing the development environment.*/
func (m MWDD) Directory() string {
	return string(m)
}

/*EnsureReady ...*/
func (m MWDD) EnsureReady() {
	files.EnsureReady(m.Directory())
	m.Env().EnsureExists()
}
