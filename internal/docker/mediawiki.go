/*Package docker is used to interact with docker development environment services

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
package docker

import (
	"path/filepath"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
	"os"
	osexec "os/exec"
)

/*ComposeCommand ...*/
func ComposeCommand (command string, arg ...string) *osexec.Cmd {
	projectDir, _ := os.Getwd()
	context := exec.ComposeCommandContext{
		ProjectDirectory:     projectDir,
		ProjectName:   "mw-" + filepath.Base(projectDir),
	}
	return exec.ComposeCommand(
		context,
		"exec",
		"-T",
		"mediawiki",
		"/bin/bash",
		"/docker/install.sh",
		)
}


/*MediaWikiInstall ...*/
func MediaWikiInstall( options exec.HandlerOptions ) {
	exec.RunCommand(
		options,
		ComposeCommand(
			"exec",
			"-T",
			"mediawiki",
			"/bin/bash",
			"/docker/install.sh",
			))
}

/*MediaWikiComposerUpdate ...*/
func MediaWikiComposerUpdate( options exec.HandlerOptions ) {
	exec.RunCommand(
		options,
		ComposeCommand(
			"exec",
			"-T",
			"mediawiki",
			"composer",
			"update",
		))
}

func mediaWikiPHPVersionCheck( options exec.HandlerOptions ) error {
	return exec.RunCommand(options,
		ComposeCommand(
			"exec",
			"-T",
			"mediawiki",
			"php",
			"-r",
			"require_once dirname( __FILE__ ) . '/includes/PHPVersionCheck.php'; $phpVersionCheck = new PHPVersionCheck(); $phpVersionCheck->checkVendorExistence();",
		))
}

/*MediaWikiComposerDependenciesNeedInstallation ...*/
func MediaWikiComposerDependenciesNeedInstallation(options exec.HandlerOptions) bool {
	err := mediaWikiPHPVersionCheck(options)
	return err != nil
}

/*EnsureDockerComposeUserOverrideExists Ensures that a docker-compose.override files exists with a mediawiki user and gid override*/
func EnsureDockerComposeUserOverrideExists() (bool, error){
	// TODO: We should also check the contents for correctness, maybe
	// using docker-compose config and asserting that UID/GID mapping is present
	// and with correct values.
	_, err := os.Stat("docker-compose.override.yml")
	if err != nil {
		var data = `
version: '3.7'
services:
  mediawiki:
    user: "${MW_DOCKER_UID}:${MW_DOCKER_GID}"
`
		file, err := os.Create("docker-compose.override.yml")
		if err != nil {
			return false, err
		}
		defer file.Close()
		_, err = file.WriteString(data)
		if err != nil {
			return false, err
		}
		file.Sync()
		return true, nil;
	}
	return false, nil;
}