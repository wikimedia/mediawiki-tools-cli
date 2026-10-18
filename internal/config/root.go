package config

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf"
	koanfjson "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gopkg.in/yaml.v2"
)

/*Path path of the config file.*/
func Path() string {
	return cli.UserDirectoryPath() + string(os.PathSeparator) + "config.json"
}

func ensureExists() {
	logrus.Trace("ensuring config exists")
	if _, err := os.Stat(Path()); err != nil {
		logrus.Trace("config does not exist. Creating...")
		err := os.MkdirAll(strings.Replace(Path(), "config.json", "", -1), 0o700)
		if err != nil {
			logrus.Fatal(err)
		}
		file, err := os.OpenFile(Path(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)
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
	// Effective config
	k = koanf.New(".")
	// Effective config as a struct
	c = Config{}
	// Config that is actually on disk
	kOnDisk = koanf.New(".")
	// Config that is actually on disk as a struct
	cOnDisk = Config{}
	// Has the config been loaded?
	kLoaded = false
)

// TODO just use the state instead of a bunch of vars

type ConfigState struct {
	// The current state of the config
	Effective *Config
	// The current state of the config on disk
	OnDisk *Config
	// The koanf instance of the config
	EffectiveKoanf *koanf.Koanf
	// The koanf instance of the config on disk
	OnDiskKoanf *koanf.Koanf
}

func State() *ConfigState {
	if !kLoaded {
		load()
	}
	return &ConfigState{
		Effective:      &c,
		OnDisk:         &cOnDisk,
		EffectiveKoanf: k,
		OnDiskKoanf:    kOnDisk,
	}
}

// Can contain various config migrations and other things
func preload() {
	// move the legacy Gerrit config, into the main config file :)
	{
		// Load the old config
		gerritYaml := filepath.Join(cli.UserDirectoryPath(), "gerrit.yaml")
		type gerritConfig struct {
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		}
		gc := gerritConfig{}
		if _, err := os.Stat(gerritYaml); os.IsNotExist(err) {
			// do nothing, as we have an empty one already..
		} else {
			fileContents, err := os.ReadFile(filepath.Clean(gerritYaml))
			if err != nil {
				logrus.Errorf("Error while reading file. %v", err)
				panic(err)
			}
			err = yaml.Unmarshal(fileContents, &gc)
			if err != nil {
				logrus.Error(err)
				panic(err)
			}
			gerritMoveWorked := true
			if gc.Username != "" {
				logrus.Tracef("moving gerrit.username to config.json: %s", gc.Username)
				err := soloPutKeyValueOnDisk("gerrit.username", gc.Username)
				if err != nil {
					gerritMoveWorked = false
				}
			}
			if gc.Password != "" {
				logrus.Tracef("moving gerrit.password to config.json")
				err := soloPutKeyValueOnDisk("gerrit.password", gc.Password)
				if err != nil {
					gerritMoveWorked = false
				}
			}
			if gerritMoveWorked {
				// Delete the old config file
				err := os.Remove(gerritYaml)
				if err != nil {
					logrus.Errorf("error removing old gerrit config: %v", err)
				}
			}
		}
	}
}

func load() *koanf.Koanf {
	if kLoaded {
		return k
	}
	ensureExists()

	preload()

	logrus.Trace("loading config")
	logrus.Trace(PrettyPrint(k))
	logrus.Trace("loading defaults")
	loadDefaults()
	logrus.Trace(PrettyPrint(k))
	logrus.Trace("loading json")
	f := file.Provider(Path())
	loadJson(k, f)
	loadJson(kOnDisk, f)
	logrus.Trace(PrettyPrint(k))
	logrus.Trace("loading env")
	PrettyPrint(k)
	loadEnv()
	c = koanfToConfig(k)
	cOnDisk = koanfToConfig(kOnDisk)
	logrus.Trace("config loaded")

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

func Marshal(k *koanf.Koanf) ([]byte, error) {
	return k.Marshal(koanfjson.Parser())
}

func PrettyPrint(k *koanf.Koanf) string {
	m, err := Marshal(k)
	if err != nil {
		logrus.Fatalf("%v", err)
	}
	var indented bytes.Buffer
	err = json.Indent(&indented, m, "", "  ")
	if err != nil {
		logrus.Fatalf("%v", err)
	}
	return indented.String()
}

func PutDiskConfig(kToPut *koanf.Koanf) error {
	err := putDiskConfig(kToPut)
	if err != nil {
		return err
	}
	// XXX: TODO validate, and check, this probably doesn't apply deletes correctly
	ReApplyKoanfConf(kToPut)
	return nil
}

func putDiskConfig(kToPut *koanf.Koanf) error {
	logrus.Tracef("putting config on disk: %s", PrettyPrint(kToPut))
	bytes, err := Marshal(kToPut)
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
	logrus.Tracef("setting %s to %s", key, value)
	odk := State().OnDiskKoanf
	odk.Set(key, value)
	return PutDiskConfig(odk)
}

func DeleteKeyValueFromDisk(key string) error {
	odk := State().OnDiskKoanf
	odk.Delete(key)
	return PutDiskConfig(odk)
}

// Can be called before whole config is actually loaded..
func soloPutKeyValueOnDisk(key string, value string) error {
	// Load the config from json in here
	kTemp := koanf.New(".")
	f := file.Provider(Path())
	loadJson(kTemp, f)
	kTemp.Set(key, value)
	return putDiskConfig(kTemp) // without reapply
}

func ReApplyKoanfConf(override *koanf.Koanf) {
	k.Merge(override)
	kOnDisk.Merge(override)
	c = koanfToConfig(k)
	cOnDisk = koanfToConfig(kOnDisk)
}

func loadDefaults() {
	defaultConf := defaultConfig()
	err := k.Load(structs.Provider(defaultConf, "koanf"), nil)
	if err != nil {
		logrus.Errorf("error loading defaults: %v", err)
	}
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
