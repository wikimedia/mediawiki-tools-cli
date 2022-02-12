package cli

type VersionAttributes struct {
	GitCommit  string // holds short commit hash of source tree.
	GitBranch  string // holds current branch name the code is built off.
	GitState   string // shows whether there are uncommitted changes.
	GitSummary string // holds output of git describe --tags --dirty --always.
	BuildDate  string // holds RFC3339 formatted UTC date (build time).
	Version    string // hold contents of ./VERSION file, if exists, or the value passed via the -version option.
}

var VersionDetails VersionAttributes
