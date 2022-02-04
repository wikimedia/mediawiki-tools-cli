package updater

import (
	log "github.com/sirupsen/logrus"
)

/*CanUpdate will check for updates.*/
func CanUpdate(currentVersion string, gitSummary string) (bool, string) {
	canUpdate, release := CanUpdateFromGitlab(currentVersion, gitSummary)
	if canUpdate {
		return canUpdate, release
	}
	log.Info("Current version is: " + currentVersion + "\nLatest available is: " + release)

	// When canUpdate is false, we dont have a release to get the version string of
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
