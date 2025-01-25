package updater

import (
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	gitlabb "gitlab.wikimedia.org/repos/releng/cli/internal/gitlab"
)

func setLogLevelForSelfUpdate() {
	if cli.Opts.Verbosity > 0 {
		selfupdate.EnableLog()
	}
}

func RelengCliGetReleasesBetweenTags(from, to string) ([]*gitlab.Release, error) {
	return gitlabb.RelengCliGetReleasesBetweenTags(from, to)
}

/*CanUpdateFromGitlab ...*/
func CanUpdateFromGitlab(version cli.Version, gitSummary string) (bool, string) {
	setLogLevelForSelfUpdate()

	latestRelease, latestErr := gitlabb.RelengCliLatestRelease()
	if latestErr != nil {
		return false, "Could not fetch latest release version from Gitlab"
	}

	newVersion, newErr := semver.Parse(strings.Trim(latestRelease.TagName, "v"))
	currentVersion, currentErr := semver.Parse(version.String())

	if newErr != nil {
		return false, "Could not parse latest release version from Gitlab"
	}
	if currentErr != nil {
		return false, "Could not parse current version '" + version.String() + "'. Next release would be " + newVersion.String()
	}

	return currentVersion.Compare(newVersion) == -1, newVersion.String()
}

// UpdateFromGitlab will update the binary to the latest version from Gitlab.
func UpdateFromGitlab(currentVersion cli.Version, gitSummary string) (success bool, message string) {
	setLogLevelForSelfUpdate()

	canUpdate, newVersionOrMessage := CanUpdateFromGitlab(currentVersion, gitSummary)
	if !canUpdate {
		return false, "No update found: " + newVersionOrMessage
	}

	// TODO refactor to avoid 2 API calls
	release, err := gitlabb.RelengCliLatestRelease()
	if err != nil {
		panic(err)
	}
	link, err := gitlabb.RelengCliLatestReleaseBinary()
	if err != nil {
		panic(err)
	}

	cmdPath, err := os.Executable()
	if err != nil {
		return false, "Failed to grab local executable location"
	}

	err = selfupdate.UpdateTo(link.DirectAssetURL, cmdPath)
	if err != nil {
		return false, "Binary update failed" + err.Error()
	}

	return true, "Successfully updated to version " + release.TagName + "\n\n" + release.Description
}

func CanMoveToVersionFromGitlab(targetVersion cli.Version) bool {
	_, err := gitlabb.RelengCliReleaseBinary(targetVersion.Tag())
	return err == nil
}

func MoveToVersionFromGitlab(targetVersion cli.Version) (success bool, message string) {
	// TODO refactor to avoid 2 API calls
	release, err := gitlabb.RelengCliRelease(targetVersion.Tag())
	if err != nil {
		panic(err)
	}
	link, err := gitlabb.RelengCliReleaseBinary(targetVersion.Tag())
	if err != nil {
		panic(err)
	}

	cmdPath, err := os.Executable()
	if err != nil {
		return false, "Failed to grab local executable location"
	}

	err = selfupdate.UpdateTo(link.DirectAssetURL, cmdPath)
	if err != nil {
		return false, "Binary update failed" + err.Error()
	}

	return true, "Successfully updated to version " + release.TagName + "\n\n" + release.Description
}
