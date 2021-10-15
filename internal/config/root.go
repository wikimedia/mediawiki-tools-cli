/*Package config for interacting with the cli config

Copyright © 2020 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"strings"
)

func configPath() string {
	return mwcliDirectory() + string(os.PathSeparator) + "config.json"
}

func mwcliDirectory() string {
	currentUser, _ := user.Current()
	projectDirectory := currentUser.HomeDir + string(os.PathSeparator) + ".mwcli"
	return projectDirectory
}

func ensureExists() {
	if _, err := os.Stat(configPath()); err != nil {
		err := os.MkdirAll(strings.Replace(configPath(), "config.json", "", -1), 0o700)
		if err != nil {
			log.Fatal(err)
		}
		file, err := os.OpenFile(configPath(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		w := bufio.NewWriter(file)
		_, err = w.WriteString("{}")
		if err != nil {
			log.Fatal(err)
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
		log.Fatal(err)
	}
	defer file.Close()
	w := bufio.NewWriter(file)
	jsonEncoder := json.NewEncoder(w)
	jsonEncoder.Encode(c)
	w.Flush()
}

/*PrettyPrint writers the config to disk.*/
func (c Config) PrettyPrint() {
	empJSON, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Fatalf(err.Error())
	}
	fmt.Printf("%s\n", string(empJSON))
}
