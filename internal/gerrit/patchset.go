package gerrit

import gogerrit "github.com/andygrunwald/go-gerrit"

// Revision finds the requested patchset, or the current revision when patchset is zero.
func Revision(change *gogerrit.ChangeInfo, patchset int) (string, gogerrit.RevisionInfo, bool) {
	if patchset == 0 {
		if revision, ok := change.Revisions[change.CurrentRevision]; ok {
			return change.CurrentRevision, revision, true
		}
		patchset = change.CurrentRevisionNumber
	}
	for commit, revision := range change.Revisions {
		if patchset > 0 && revision.Number == patchset {
			return commit, revision, true
		}
	}
	return "", gogerrit.RevisionInfo{}, false
}
