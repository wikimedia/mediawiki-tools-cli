package updater

import "github.com/sirupsen/logrus"

/*CanUpdate will check for updates.*/
func CanUpdate(currentVersion string, gitSummary string) (bool, string) {
	canUpdate, release := CanUpdateFromGitlab(currentVersion, gitSummary)
	if canUpdate {
		return canUpdate, release
	}
	logrus.Debug("Current version is: " + currentVersion + "\nLatest available is: " + release)

	// When canUpdate is false, we don't have a release to get the version string of
	return canUpdate, "No update available"
}

/*Update perform the latest update.*/
func Update(currentVersion string, gitSummary string) (bool, string) {
	return UpdateFromGitlab(currentVersion, gitSummary)
}

func CanMoveToVersion(targetVersion string) bool {
	return CanMoveToVersionFromGitlab(targetVersion)
}

func MoveToVersion(targetVersion string) (success bool, message string) {
	return MoveToVersionFromGitlab(targetVersion)
}
