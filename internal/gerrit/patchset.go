package gerrit

import (
	gogerrit "github.com/andygrunwald/go-gerrit"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

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

// RevisionFetch returns the anonymous fetch URL and the ref advertised by Gerrit.
func RevisionFetch(project string, revision gogerrit.RevisionInfo) (address, ref string) {
	ref = revision.Ref
	if fetch, ok := revision.Fetch["anonymous http"]; ok {
		if fetch.Ref != "" {
			ref = fetch.Ref
		}
		return fetch.URL, ref
	}
	return secrets.GerritEndpoint + "/" + project, ref
}
