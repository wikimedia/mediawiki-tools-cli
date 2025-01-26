package config

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf"
	koanfjson "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
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

var (
	k       = koanf.New(".")
	c       = Config{}
	kLoaded = false
)

func Instance() (*Config, *koanf.Koanf) {
	if !kLoaded {
		Load()
	}
	return &c, k
}

func Load() *koanf.Koanf {
	if kLoaded {
		return k
	}
	ensureExists()

	loadDefaults()
	f := file.Provider(Path())
	loadJson(k, f)
	loadEnv()
	c = koanfToConfig(k)

	f.Watch(func(event interface{}, err error) {
		if err != nil {
			logrus.Errorf("watch error: %v", err)
			return
		}

		logrus.Trace("config file changed. Reloading...")
		loadDefaults()
		loadJson(k, f)
		loadEnv()
		c = koanfToConfig(k)
	})

	kLoaded = true
	return k
}

// Take the koanf instance and convert it to a Config struct
func koanfToConfig(k *koanf.Koanf) Config {
	// Convert to json, and then marshal it back to a struct
	b, err := k.Marshal(koanfjson.Parser())
	if err != nil {
		logrus.Fatalf("error marshalling config: %v", err)
	}
	c := Config{}
	err = json.Unmarshal(b, &c)
	if err != nil {
		logrus.Fatalf("error unmarshalling config: %v", err)
	}
	return c
}

func GetDiskConfig() *koanf.Koanf {
	n := koanf.New(".") // Create a new koanf instance.
	f := file.Provider(Path())
	loadJson(n, f)
	return n
}

func Marshal(k *koanf.Koanf) ([]byte, error) {
	return k.Marshal(koanfjson.Parser())
}

func PrettyPrint(k *koanf.Koanf) {
	m, err := Marshal(k)
	if err != nil {
		logrus.Fatalf("%v", err)
	}
	var indented bytes.Buffer
	err = json.Indent(&indented, m, "", "  ")
	if err != nil {
		logrus.Fatalf("%v", err)
	}
	fmt.Printf("%s\n", indented.String())
}

func PutDiskConfig(k *koanf.Koanf) error {
	bytes, err := Marshal(k)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(Path(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	_, err = w.WriteString(string(bytes))
	if err != nil {
		return err
	}
	flushErr := w.Flush()
	if flushErr != nil {
		return flushErr
	}
	return nil
}

func PutKeyValueOnDisk(key string, value string) error {
	k := GetDiskConfig()
	k.Set(key, value)
	return PutDiskConfig(k)
}

func loadDefaults() {
	defaultConf := defaultConfig()
	// Load default config.
	k.Unmarshal(".", defaultConf)
}

func loadJson(i *koanf.Koanf, f *file.File) {
	err := i.Load(f, koanfjson.Parser())
	if err != nil {
		// TODO if an issue persists with config getting message up, we might want to make this better
		// By moving the old file to a backup and creating a new one?
		// https://phabricator.wikimedia.org/T294195
		logrus.Errorf("error loading config: %v", err)
	}
}

func loadEnv() {
	k.Load(env.Provider("MWCLI_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "MWCLI_")), "_", ".", -1)
	}), nil)
}
