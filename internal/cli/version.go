package cli

// Version string such as "0.10.0". Could also be "latest".
type Version string

type VersionAttributes struct {
	GitCommit  string  // holds short commit hash of source tree.
	GitBranch  string  // holds current branch name the code is built off.
	GitState   string  // shows whether there are uncommitted changes.
	GitSummary string  // holds output of git describe --tags --dirty --always.
	BuildDate  string  // holds RFC3339 formatted UTC date (build time).
	Version    Version // hold contents of ./VERSION file, if exists, or the value passed via the -version option. eg. 0.10.0
}

var VersionDetails VersionAttributes

func VersionFromUserInput(input string) Version {
	// Trim leading v
	if len(input) >= 1 && input[:1] == "v" {
		return Version(input[1:])
	}
	return Version(input)
}

func (v Version) String() string {
	return string(v)
}

func (v Version) Tag() string {
	if v == "latest" {
		return "latest"
	}
	return "v" + string(v)
}

func (v Version) ReleasePage() string {
	if v == "latest" {
		return "https://gitlab.wikimedia.org/repos/releng/cli/-/releases"
	}
	return "https://gitlab.wikimedia.org/repos/releng/cli/-/releases/" + v.Tag()
}

func (v Version) ReleaseNotesCommand() string {
	if v == "latest" {
		return "mw gitlab release view --repo repos/releng/cli"
	}
	return "mw gitlab release view --repo repos/releng/cli " + v.Tag()
}
