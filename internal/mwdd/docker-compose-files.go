package mwdd

import (
	"fmt"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
	"io/ioutil"
	"os"

	"gitlab.wikimedia.org/releng/cli/internal/util/dotenv"
	"gopkg.in/yaml.v3"
)

/*DockerComposeProjectName the name of the docker-compose project.*/
func (m MWDD) DockerComposeProjectName() string {
	return "mwcli-mwdd-" + cli.Context()
}

/*Env ...*/
func (m MWDD) Env() dotenv.File {
	return dotenv.FileForDirectory(m.Directory())
}

func (m MWDD) DockerComposeFileName(name string) string {
	return m.Directory() + string(os.PathSeparator) + name + ".yml"
}

type DockerComposeFile struct {
	Version  string                 `yaml:"version"`
	Services map[string]Service     `json:"services"`
	Volumes  map[string]interface{} `json:"volumes"`
}

func (dcf DockerComposeFile) ServiceNames() []string {
	var serviceNames []string
	for serviceName := range dcf.Services {
		serviceNames = append(serviceNames, serviceName)
	}
	return serviceNames
}

func (dcf DockerComposeFile) VolumeNames() []string {
	var volumeNames []string
	for volumeName := range dcf.Volumes {
		volumeNames = append(volumeNames, volumeName)
	}
	return volumeNames
}

type Service struct {
	Image       string   `yaml:"image"`
	Entrypoint  string   `yaml:"entrypoint"`
	Volumes     []string `yaml:"volumes"`
	Environment []string `yaml:"environment"`
	DependsOn   []string `yaml:"depends_on"`
	DNS         []string `yaml:"dns"`
	Networks    []string `yaml:"networks"`
}

func (m MWDD) dockerComposeFile(fileName string) DockerComposeFile {
	yamlFile, err := ioutil.ReadFile(m.DockerComposeFileName(fileName))
	if err != nil {
		panic(err)
	}
	var dcFile DockerComposeFile

	err = yaml.Unmarshal(yamlFile, &dcFile)
	if err != nil {
		panic(err)
	}

	return dcFile
}

func (m MWDD) DockerComposeFileServices(fileName string) []string {
	return m.dockerComposeFile(fileName).ServiceNames()
}

func (m MWDD) DockerComposeFileVolumes(fileName string) []string {
	return m.dockerComposeFile(fileName).VolumeNames()
}

func (m MWDD) DockerComposeFileExistsOrExit(fileName string) {
	filePath := m.DockerComposeFileName(fileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Println("docker-compose file " + filePath + " does not exist")
		os.Exit(1)
	}
}
