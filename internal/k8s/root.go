package k8s

import (
	"os"

	"gitlab.wikimedia.org/releng/cli/internal/cli"
	"gitlab.wikimedia.org/releng/cli/internal/k8s/files"
)

type K8S string

/*DefaultForUser returns the default k8s working directory for the user.*/
func DefaultForUser() K8S {
	return K8S(k8sUserDirectory() + string(os.PathSeparator) + cli.Context())
}

func k8sUserDirectory() string {
	return cli.UserDirectoryPathForCmd("kubernetes")
}

/*Directory the directory containing the development environment.*/
func (k K8S) Directory() string {
	return string(k)
}

/*EnsureReady ...*/
func (k K8S) EnsureReady() {
	files.EnsureReady(k.Directory())
}
