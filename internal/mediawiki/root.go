/*Package mediawiki is used to interact with MediaWiki

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
package mediawiki

import (
	"bytes"
	"strings"
	"io/ioutil"
	"fmt"
	"os"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
	"time"
	"github.com/briandowns/spinner"
	"github.com/manifoldco/promptui"
	"log"
)

/*InitialSetup tba*/
func InitialSetup( options exec.HandlerOptions) {
	CheckIfInCoreDirectory()
	makeCacheDirectory()

	if composerDependenciesNeedInstallation(options) {
		promptToInstallComposerDependencies(options)
	}

	if !vectorIsPresent() {
		promptToCloneVector(options)
	}

	if !localSettingsIsPresent() {
		promptToInstallMediaWiki(options)
	}
}

/*CheckIfInCoreDirectory checks that the current working directory looks like a MediaWiki directory*/
func CheckIfInCoreDirectory() {
	b, err := ioutil.ReadFile(".gitreview")
	if err != nil || !strings.Contains(string(b), "project=mediawiki/core.git") {
		log.Fatal("❌ Please run this command within the root of the MediaWiki core repository.")
	}
}

func makeCacheDirectory() {
	err := os.MkdirAll("cache", 0700)
	if err != nil {
		log.Fatal(err)
	}
}

func composerDependenciesNeedInstallation(options exec.HandlerOptions) bool {
	// Detect if composer dependencies are not installed and prompt user to install
	err := exec.RunCommand(options,
		exec.DockerComposeCommand(
			"exec",
			"-T",
			"mediawiki",
			"php",
			"-r",
			"require_once dirname( __FILE__ ) . '/includes/PHPVersionCheck.php'; $phpVersionCheck = new PHPVersionCheck(); $phpVersionCheck->checkVendorExistence();",
		))
	return err != nil
}

func promptToInstallComposerDependencies(options exec.HandlerOptions) {
	fmt.Println("MediaWiki has some external dependencies that need to be installed")
	prompt := promptui.Prompt{
		IsConfirm: true,
		Label:     "Install dependencies now",
	}
	_, err := prompt.Run()
	if err == nil {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = "Installing Composer dependencies (this may take a few minutes) "
		s.FinalMSG = s.Prefix + "(done)\n"

		options := exec.HandlerOptions{
			Spinner: s,
			Verbosity: options.Verbosity,
		}
		exec.RunCommand(options,
			exec.DockerComposeCommand(
				"exec",
				"-T",
				"mediawiki",
				"composer",
				"update",
			))
	}
}

func vectorIsPresent() bool {
	info, err := os.Stat("skins/Vector")
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func promptToCloneVector(options exec.HandlerOptions) {
	prompt := promptui.Prompt{
		IsConfirm: true,
		Label:     "Download and use the Vector skin",
	}
	_, err := prompt.Run()
	if err == nil {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = "Downloading Vector "
		s.FinalMSG = s.Prefix + "(done)\n"

		options := exec.HandlerOptions{
			Spinner: s,
			Verbosity: options.Verbosity,
			HandleError: func(stderr bytes.Buffer, err error) {
				if err != nil {
					log.Fatal(err)
				}
			},
		}

		exec.RunCommand(options, exec.Command(
			"git",
			"clone",
			"https://gerrit.wikimedia.org/r/mediawiki/skins/Vector",
			"skins/Vector"))
	}
}

func promptToInstallMediaWiki(options exec.HandlerOptions) {
	prompt := promptui.Prompt{
		IsConfirm: true,
		Label:     "Install MediaWiki database tables and create LocalSettings.php",
	}
	_, err := prompt.Run()
	if err == nil {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = "Installing "
		s.FinalMSG = s.Prefix + "(done)\n"
		options := exec.HandlerOptions{
			Spinner: s,
			Verbosity: options.Verbosity,
		}
		exec.RunCommand(
			options,
			exec.DockerComposeCommand(
				"exec",
				"-T",
				"mediawiki",
				"/bin/bash",
				"/docker/install.sh"))
	}
}

func localSettingsIsPresent() bool {
	info, err := os.Stat("LocalSettings.php")
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

/*RenameLocalSettings ...*/
func RenameLocalSettings() {
	const layout = "2006-01-02T15:04:05-0700"

	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "Renaming LocalSettings file "
	s.FinalMSG = s.Prefix + "(done)\n"

	s.Start()
	t := time.Now()
	localSettingsName := "LocalSettings-" + t.Format(layout) + ".php"

	err := os.Rename("LocalSettings.php", localSettingsName)

	if err != nil {
		log.Fatal(err)
	}

	s.Stop()
}

/*DeleteCache ...*/
func DeleteCache() {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "Deleting cache files "
	s.FinalMSG = s.Prefix + "(done)\n"

	s.Start()

	err := os.Rename("cache/.htaccess", ".htaccess")
	if err != nil {
		log.Fatal(err)
	}

	err = os.RemoveAll("./cache/")
	if err != nil {
		log.Fatal(err)
	}

	err = os.Mkdir("cache", 0700)
	if err != nil {
		log.Fatal(err)
	}

	err = os.Rename(".htaccess", "cache/.htaccess")
	if err != nil {
		log.Fatal(err)
	}

	s.Stop()
}

/*DeleteVendor ...*/
func DeleteVendor() {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "Deleting vendor files "
	s.FinalMSG = s.Prefix + "(done)\n"

	s.Start()

	err := os.RemoveAll("./vendor")
	if err != nil {
		log.Fatal(err)
	}

	err = os.Mkdir("vendor", 0700)
	if err != nil {
		log.Fatal(err)
	}

	s.Stop()
}