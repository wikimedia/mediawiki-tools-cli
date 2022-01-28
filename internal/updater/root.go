package updater

/*CanUpdate will check for updates.*/
func CanUpdate(currentVersion string, gitSummary string, verboseOutput bool) (bool, string) {
	canUpdate, release := CanUpdateFromGitlab(currentVersion, gitSummary, verboseOutput)
	if canUpdate {
		return canUpdate, release
	}

	message := "No update available"

	if verboseOutput {
		message = message + "\nCurrent version is: " + currentVersion + "\nLatest available is: " + release
	}

	// When canUpdate is false, we dont have a release to get the version string of
	return canUpdate, message
}

/*Update perform the latest update.*/
func Update(currentVersion string, gitSummary string, verboseOutput bool) (bool, string) {
	return UpdateFromGitlab(currentVersion, gitSummary, verboseOutput)
}

func CanMoveToVersion(targetVersion string) bool {
	return CanMoveToVersionFromGitlab(targetVersion)
}

func MoveToVersion(targetVersion string) (success bool, message string) {
	return MoveToVersionFromGitlab(targetVersion)
}
