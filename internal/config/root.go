package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
)

func configPath() string {
	return cli.UserDirectoryPath() + string(os.PathSeparator) + "config.json"
}

func ensureExists() {
	if _, err := os.Stat(configPath()); err != nil {
		err := os.MkdirAll(strings.Replace(configPath(), "config.json", "", -1), 0o700)
		if err != nil {
			logrus.Fatal(err)
		}
		file, err := os.OpenFile(configPath(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			logrus.Fatal(err)
		}
		defer file.Close()
		w := bufio.NewWriter(file)
		_, err = w.WriteString("{}")
		if err != nil {
			logrus.Fatal(err)
		}
		w.Flush()
	}
}

/*LoadFromDisk loads the config.json from disk.*/
func LoadFromDisk() Config {
	ensureExists()
	var config Config
	configFile, err := os.Open(configPath())
	if err != nil {
		fmt.Println(err.Error())
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)
	return config
}

/*WriteToDisk writers the config to disk.*/
func (c Config) WriteToDisk() {
	file, err := os.OpenFile(configPath(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		logrus.Fatal(err)
	}
	defer file.Close()
	w := bufio.NewWriter(file)
	jsonEncoder := json.NewEncoder(w)
	jsonEncoder.Encode(c)
	w.Flush()
}

/*PrettyPrint writes the config to disk.*/
func (c Config) PrettyPrint() {
	empJSON, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		logrus.Fatalf(err.Error())
	}
	fmt.Printf("%s\n", string(empJSON))
}
