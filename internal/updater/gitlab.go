package updater

import (
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	gitlabb "gitlab.wikimedia.org/repos/releng/cli/internal/gitlab"
	"gitlab.wikimedia.org/repos/releng/cli/internal/releaseversion"
)

// TODO adopt more of the logic and code from the update command again

func RelengCliGetReleasesBetweenTags(from, to string) ([]*gitlab.Release, error) {
	return gitlabb.RelengCliGetReleasesBetweenTags(from, to)
}

/*CanUpdateFromGitlab ...*/
func CanUpdateFromGitlab(version cli.Version, gitSummary string) (bool, string) {
	targetRelease, err := gitlabb.RelengCliLatestRelease()
	if err != nil {
		return false, "Could not fetch latest release version from Gitlab"
	}

	newVersion, newErr := releaseversion.Parse(targetRelease.TagName)
	currentVersion, currentErr := releaseversion.Parse(version.String())

	if newErr != nil {
		return false, "Could not parse latest release version from Gitlab"
	}
	if currentErr != nil {
		return false, "Could not parse current version '" + version.String() + "'. Next release would be " + newVersion.String()
	}

	return currentVersion.Compare(newVersion) == -1, newVersion.String()
}
