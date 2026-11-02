package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf"
	koanfjson "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gopkg.in/yaml.v2"
)

/*Path path of the config file.*/
func Path() string {
	return cli.UserDirectoryPath() + string(os.PathSeparator) + "config.json"
}

func ensureExists() error {
	if err := secureConfigPath(); err != nil {
		return err
	}
	if _, err := os.Lstat(Path()); os.IsNotExist(err) {
		return writePrivateConfig([]byte("{}"))
	} else {
		return err
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
	kLoaded             = false
	initializationError error
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
		initializationError = load()
	}
	return &ConfigState{
		Effective:      &c,
		OnDisk:         &cOnDisk,
		EffectiveKoanf: k,
		OnDiskKoanf:    kOnDisk,
	}
}

// Can contain various config migrations and other things
func preload() error {
	oldPath := filepath.Join(cli.UserDirectoryPath(), "gerrit.yaml")
	data, err := os.ReadFile(oldPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy Gerrit configuration: %w", err)
	}
	var old struct{ Username, Password string }
	if yaml.Unmarshal(data, &old) != nil {
		return fmt.Errorf("parse legacy Gerrit configuration")
	}
	for key, value := range map[string]string{"gerrit.username": old.Username, "gerrit.password": old.Password} {
		if value != "" {
			if err := soloPutKeyValueOnDisk(key, value); err != nil {
				return err
			}
		}
	}
	return os.Remove(oldPath)
}

// Initialize establishes the CLI's configuration error boundary before command
// construction. State retains lazy access for packages/tests using this facade.
func Initialize() error {
	if !kLoaded {
		initializationError = load()
	}
	return initializationError
}

func load() error {
	if kLoaded {
		return nil
	}
	if !isConfigCompletion() {
		unlock, err := lockConfigFile(context.Background())
		if err != nil {
			return err
		}
		defer unlock()
	}
	if err := ensureExists(); err != nil {
		return err
	}
	if err := preload(); err != nil {
		return err
	}

	slog.Debug("loading config")
	slog.Debug("loading defaults")
	if err := loadDefaults(); err != nil {
		return err
	}
	slog.Debug("loading json")
	f := file.Provider(Path())
	if err := loadJson(k, f); err != nil {
		return err
	}
	if err := loadJson(kOnDisk, f); err != nil {
		return err
	}
	migrated, changed, err := migratePhabricatorProfiles(kOnDisk)
	if err != nil {
		return err
	}
	gerritMigrated, gerritChanged, err := migrateGerritProfiles(migrated)
	if err != nil {
		return err
	}
	migrated = gerritMigrated
	changed = changed || gerritChanged
	wikiMigrated, wikiChanged, err := migrateWikiProfiles(migrated)
	if err != nil {
		return err
	}
	migrated = wikiMigrated
	changed = changed || wikiChanged
	cloudMigrated, cloudChanged, err := migrateCloudVPSProfiles(migrated)
	if err != nil {
		return err
	}
	migrated = cloudMigrated
	changed = changed || cloudChanged
	if changed {
		// Completion may translate metadata in memory, but must not write config.
		if !isConfigCompletion() {
			if err := putDiskConfig(migrated); err != nil {
				return fmt.Errorf("migrate account profiles: %w", err)
			}
		}
		kOnDisk = migrated
		k = koanf.New(".")
		if err := loadDefaults(); err != nil {
			return err
		}
		_ = k.Merge(kOnDisk)
	}
	slog.Debug("loading env")
	if err := loadEnv(); err != nil {
		return err
	}
	c, err = koanfToConfig(k)
	if err != nil {
		return err
	}
	cOnDisk, err = koanfToConfig(kOnDisk)
	if err != nil {
		return err
	}
	slog.Debug("config loaded")

	kLoaded = true
	return nil
}

// Take the koanf instance and convert it to a Config struct
func koanfToConfig(k *koanf.Koanf) (Config, error) {
	// Convert to json, and then marshal it back to a struct
	b, err := k.Marshal(koanfjson.Parser())
	if err != nil {
		return Config{}, fmt.Errorf("marshal configuration: %w", err)
	}
	c := Config{}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return Config{}, fmt.Errorf("configuration has invalid setting types")
	}
	// Other commands use the default Gerrit identity for cloning/SSH. Project
	// that public username into the legacy struct without recreating disk keys.
	if c.Gerrit.Username == "" {
		name := k.String("gerrit.default_profile")
		if name != "" {
			c.Gerrit.Username = k.String("gerrit.profiles." + name + ".username")
		}
	}
	return c, nil
}

func Marshal(k *koanf.Koanf) ([]byte, error) {
	return k.Marshal(koanfjson.Parser())
}

func PrettyPrint(k *koanf.Koanf) (string, error) {
	m, err := Marshal(k)
	if err != nil {
		return "", err
	}
	var indented bytes.Buffer
	err = json.Indent(&indented, m, "", "  ")
	if err != nil {
		return "", err
	}
	return indented.String(), nil
}

// PutDiskConfig explicitly replaces configuration. Ordinary mutations must use
// UpdateDiskConfig so they apply to the latest file rather than a cached copy.
func PutDiskConfig(kToPut *koanf.Koanf) error {
	unlock, err := lockConfigFile(context.Background())
	if err != nil {
		return err
	}
	defer unlock()
	return commitDiskConfig(kToPut)
}

// UpdateDiskConfig serializes a read-modify-write with credential operations.
// The callback receives current on-disk data and must not start another mutation.
func UpdateDiskConfig(update func(*koanf.Koanf) error) error {
	unlock, err := lockCredentials(context.Background())
	if err != nil {
		return err
	}
	defer unlock()
	disk := State().OnDiskKoanf.Copy()
	if err := update(disk); err != nil {
		return err
	}
	return commitDiskConfig(disk)
}

// commitDiskConfig requires the configuration mutation lock to be held.
func commitDiskConfig(kToPut *koanf.Koanf) error {
	err := putDiskConfig(kToPut)
	if err != nil {
		return err
	}
	return ReApplyKoanfConf(kToPut)
}

func putDiskConfig(kToPut *koanf.Koanf) error {
	slog.Debug("persisting configuration")
	data, err := Marshal(kToPut)
	if err != nil {
		return err
	}
	return writePrivateConfig(data)
}

func PutKeyValueOnDisk(key string, value string) error {
	slog.Debug("updating a configuration setting")
	return UpdateDiskConfig(func(disk *koanf.Koanf) error { return disk.Set(key, value) })
}

func DeleteKeyValueFromDisk(key string) error {
	return UpdateDiskConfig(func(disk *koanf.Koanf) error { disk.Delete(key); return nil })
}

// Can be called before whole config is actually loaded..
func soloPutKeyValueOnDisk(key string, value string) error {
	// Load the config from json in here
	kTemp := koanf.New(".")
	f := file.Provider(Path())
	if err := loadJson(kTemp, f); err != nil {
		return err
	}
	kTemp.Set(key, value)
	return putDiskConfig(kTemp) // without reapply
}

func ReApplyKoanfConf(override *koanf.Koanf) error {
	diskConfig, err := koanfToConfig(override)
	if err != nil {
		return err
	}
	kOnDisk = override.Copy()
	cOnDisk = diskConfig
	k = koanf.New(".")
	if err = loadDefaults(); err != nil {
		return err
	}
	if err = k.Merge(kOnDisk); err != nil {
		return err
	}
	if err = loadEnv(); err != nil {
		return err
	}
	c, err = koanfToConfig(k)
	return err
}

func loadDefaults() error {
	return k.Load(structs.Provider(defaultConfig(), "koanf"), nil)
}

func loadJson(i *koanf.Koanf, f *file.File) error {
	if err := i.Load(f, koanfjson.Parser()); err != nil {
		return fmt.Errorf("load configuration %s: %w", Path(), err)
	}
	return nil
}

func loadEnv() error {
	return k.Load(env.Provider("MWCLI_", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(s, "MWCLI_")), "_", ".")
	}), nil)
}
