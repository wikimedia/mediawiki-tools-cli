package main

import (
	"gitlab.wikimedia.org/repos/releng/cli/cmd"
)

// Following variables will be statically linked at the time of compiling

/*GitCommit holds short commit hash of source tree.*/
var GitCommit string

/*GitBranch holds current branch name the code is built off.*/
var GitBranch string

/*GitState shows whether there are uncommitted changes.*/
var GitState string

/*GitSummary holds output of git describe --tags --dirty --always.*/
var GitSummary string

/*BuildDate holds RFC3339 formatted UTC date (build time).*/
var BuildDate string

/*Version holds contents of ./VERSION file, if exists, or the value passed via the -version option.*/
var Version string

func main() {
	// Alternatively, execute the command
	cmd.Execute(GitCommit, GitBranch, GitState, GitSummary, BuildDate, Version)
}
