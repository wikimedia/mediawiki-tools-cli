package mwdd

import (
	"os"
	"regexp"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd/files"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

/*MWDD representation of a mwdd v2 setup.*/
type MWDD struct {
	BaseDir string
}

var Context string

/*DefaultForUser returns the default mwdd working directory for the user.*/
func DefaultForUser() MWDD {
	return MWDD{
		BaseDir: mwddUserDirectory(),
	}
}

func mwddUserDirectory() string {
	return cli.UserDirectoryPathForCmd("mwdd")
}

/*Directory the directory containing the development environment.*/
func (m MWDD) Directory() string {
	return m.BaseDir + string(os.PathSeparator) + Context
}

/*EnsureReady ...*/
func (m MWDD) EnsureReady() {
	files.EnsureReady(m.Directory())
	m.Env().EnsureExists()
}

func (m MWDD) DockerCompose() dockercompose.Project {
	return dockercompose.Project{
		Name:      "mwcli-mwdd-" + Context,
		Directory: m.Directory(),
	}
}

/*CommandAndEnvFromArgs takes arguments passed to a cobra command and extracts any prefixing env var definitions from them.*/
func CommandAndEnvFromArgs(args []string) ([]string, []string) {
	extractedArgs := []string{}
	extractedEnvs := []string{}
	regex, _ := regexp.Compile(`^\w+=\w+$`)
	for _, arg := range args {
		matched := regex.MatchString(arg)
		if matched {
			extractedEnvs = append(extractedEnvs, arg)
		} else {
			extractedArgs = append(extractedArgs, arg)
		}
	}
	return extractedArgs, extractedEnvs
}

func (m MWDD) dockerNetworkName() string {
	// Default network is always dps...
	return m.DockerCompose().NetworkName("dps")
}
