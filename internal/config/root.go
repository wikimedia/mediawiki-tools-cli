package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

/*Path path of the config file.*/
func Path() string {
	return cli.UserDirectoryPath() + string(os.PathSeparator) + "config.json"
}

func ensureExists() {
	if _, err := os.Stat(Path()); err != nil {
		err := os.MkdirAll(strings.Replace(Path(), "config.json", "", -1), 0o700)
		if err != nil {
			logrus.Fatal(err)
		}
		file, err := os.OpenFile(Path(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			logrus.Fatal(err)
		}
		defer file.Close()
		w := bufio.NewWriter(file)
		_, err = w.WriteString("{}")
		if err != nil {
			panic(err)
		}
		flushErr := w.Flush()
		if flushErr != nil {
			logrus.Error(flushErr)
		}
	}
}

/*LoadFromDisk loads the config.json from disk.*/
func LoadFromDisk() Config {
	ensureExists()
	var config Config
	configFile, err := os.Open(Path())
	if err != nil {
		fmt.Println(err.Error())
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		panic(err)
	}
	return config
}

/*WriteToDisk writers the config to disk.*/
func (c Config) WriteToDisk() {
	file, err := os.OpenFile(Path(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		logrus.Fatal(err)
	}
	defer file.Close()
	w := bufio.NewWriter(file)
	jsonEncoder := json.NewEncoder(w)
	jsonErr := jsonEncoder.Encode(c)
	if jsonErr != nil {
		logrus.Error(jsonErr)
	}
	flushErr := w.Flush()
	if flushErr != nil {
		logrus.Error(flushErr)
	}
}

/*PrettyPrint writes the config to disk.*/
func (c Config) PrettyPrint() {
	empJSON, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		logrus.Fatalf(err.Error())
	}
	fmt.Printf("%s\n", string(empJSON))
}
