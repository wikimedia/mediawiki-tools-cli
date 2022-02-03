package k8s

import (
	"fmt"
	"os"
)

func (k K8S) ValuesFileName(name string) string {
	return k.Directory() + string(os.PathSeparator) + name + ".yml"
}

func (k K8S) ValuesFileExistsOrExit(fileName string) {
	filePath := k.ValuesFileName(fileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Println("values file " + filePath + " does not exist")
		os.Exit(1)
	}
}
