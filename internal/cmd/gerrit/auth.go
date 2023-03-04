package gerrit

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gopkg.in/yaml.v2"
)

func NewGerritAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate mw with Wikimedia Gerrit",
	}
	cmd.AddCommand(NewGerritAuthLoginCmd())
	cmd.AddCommand(NewGerritAuthLogoutCmd())
	cmd.AddCommand(NewGerritAuthStatusCmd())
	return cmd
}

type Config struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func ConfigFileLocation() string {
	return filepath.Join(cli.UserDirectoryPath(), "gerrit.yaml")
}

/*LoadFromDisk loads the config.json from disk.*/
func LoadConfig() Config {
	fileName := ConfigFileLocation()
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return Config{}
	}
	var config Config
	fileContents, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Printf("Error while reading file. %v", err)
		panic(err)
	}
	yaml.Unmarshal(fileContents, &config)
	return config
}

// Store config.
func (c *Config) Write() {
	fileName := ConfigFileLocation()
	yamlData, err := yaml.Marshal(&c)
	if err != nil {
		fmt.Printf("Error while Marshaling. %v", err)
		panic(err)
	}
	err = ioutil.WriteFile(fileName, yamlData, 0o600)
	if err != nil {
		panic("Unable to write data into the file")
	}
}
